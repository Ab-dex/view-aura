package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Ab-dex/view-aura/internal/events"
	quotasvc "github.com/Ab-dex/view-aura/internal/modules/quota/service"
	"github.com/Ab-dex/view-aura/internal/modules/upload/domain"
	"github.com/Ab-dex/view-aura/internal/modules/upload/repository"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
	r2pkg "github.com/Ab-dex/view-aura/internal/platform/r2"
)

// UploadService is the interface for all upload business operations.
type UploadService interface {
	// Initiate checks quota, generates a presigned R2 URL, creates a session in
	// Redis, and publishes an UploadInitiated event for quota reservation.
	// Returns the session including the presigned URL — the client uploads directly
	// to R2 using this URL without going through the API server again.
	Initiate(ctx context.Context, cmd domain.InitiateUploadCmd) (*domain.UploadSession, error)

	// GetProgress returns the current session state for the caller.
	// Used by the client to poll upload status and show a progress indicator.
	GetProgress(ctx context.Context, uploadID, userID string) (*domain.UploadSession, error)

	// Complete is called by the client after the file has fully landed in R2.
	// It creates the persistent MediaAsset row and publishes UploadCompleted,
	// which triggers the Temporal upload_pipeline workflow.
	Complete(ctx context.Context, cmd domain.CompleteUploadCmd) (*domain.MediaAsset, error)

	// Abort cancels an in-flight upload, deletes the partial R2 object, and
	// publishes UploadFailed so the quota service can release reserved storage.
	Abort(ctx context.Context, cmd domain.AbortUploadCmd) error

	// GetAsset returns a completed media asset by ID.
	GetAsset(ctx context.Context, assetID, userID string) (*domain.MediaAsset, error)

	// ListMyAssets returns all media assets belonging to the caller.
	ListMyAssets(ctx context.Context, userID string, limit, offset int) ([]*domain.MediaAsset, int, error)
}

// ─── Implementation ───────────────────────────────────────────────────────────

type uploadService struct {
	sessions repository.UploadSessionRepository
	assets   repository.MediaAssetRepository
	quota    quotasvc.QuotaService
	r2       *r2pkg.Client
	producer events.Producer
}

func NewUploadService(
	sessions repository.UploadSessionRepository,
	assets repository.MediaAssetRepository,
	quota quotasvc.QuotaService,
	r2 *r2pkg.Client,
	producer events.Producer,
) UploadService {
	return &uploadService{
		sessions: sessions,
		assets:   assets,
		quota:    quota,
		r2:       r2,
		producer: producer,
	}
}

// ─── Initiate ─────────────────────────────────────────────────────────────────

func (s *uploadService) Initiate(ctx context.Context, cmd domain.InitiateUploadCmd) (*domain.UploadSession, error) {
	// 1. Quota check — reject before generating any URL.
	result, err := s.quota.CheckUploadAllowed(ctx, cmd.UserID, cmd.SizeBytes)
	if err != nil {
		return nil, err
	}
	if !result.Allowed {
		return nil, apierror.New(403, apierror.Code(result.DenialCode), result.DenialMsg)
	}

	// 2. Generate R2 object key and presigned PUT URL.
	uploadID := uuid.New().String()
	assetKey := fmt.Sprintf("uploads/%s/%s/%s", cmd.UserID, uploadID, cmd.FileName)

	presignedURL, err := s.r2.PresignPutObject(ctx, assetKey, cmd.ContentType)
	if err != nil {
		return nil, apierror.UpstreamError("r2", err)
	}

	// 3. Create session in Redis.
	now := time.Now()
	session := &domain.UploadSession{
		ID:           uploadID,
		UserID:       cmd.UserID,
		FileName:     cmd.FileName,
		ContentType:  cmd.ContentType,
		SizeBytes:    cmd.SizeBytes,
		AssetKey:     assetKey,
		PresignedURL: presignedURL,
		Status:       domain.StatusInitiated,
		ExpiresAt:    now.Add(2 * time.Hour),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	stored, err := s.sessions.Create(ctx, session)
	if err != nil {
		return nil, err
	}

	// 4. Publish UploadInitiated so quota service reserves storage.
	_ = s.producer.Publish(ctx, events.TopicUploadInitiated, "upload.initiated", events.UploadInitiated{
		UploadID:    uploadID,
		UserID:      cmd.UserID,
		FileName:    cmd.FileName,
		ContentType: cmd.ContentType,
		SizeBytes:   cmd.SizeBytes,
		InitiatedAt: now,
	})

	logger.FromContext(ctx).Info().
		Str("upload_id", uploadID).
		Str("user_id", cmd.UserID).
		Str("file", cmd.FileName).
		Int64("size_bytes", cmd.SizeBytes).
		Msg("upload: session initiated")

	// Return the session with the presigned URL so the client can begin uploading.
	stored.PresignedURL = presignedURL
	return stored, nil
}

// ─── GetProgress ──────────────────────────────────────────────────────────────

func (s *uploadService) GetProgress(ctx context.Context, uploadID, userID string) (*domain.UploadSession, error) {
	session, err := s.sessions.GetByID(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, apierror.Forbidden("upload session belongs to another user")
	}
	return session, nil
}

// ─── Complete ─────────────────────────────────────────────────────────────────

func (s *uploadService) Complete(ctx context.Context, cmd domain.CompleteUploadCmd) (*domain.MediaAsset, error) {
	session, err := s.sessions.GetByID(ctx, cmd.UploadID)
	if err != nil {
		return nil, err
	}
	if session.UserID != cmd.UserID {
		return nil, apierror.Forbidden("upload session belongs to another user")
	}

	// Idempotent — if the asset was already created return it.
	if existing, err := s.assets.GetByUploadID(ctx, cmd.UploadID); err == nil && existing != nil {
		return existing, nil
	}

	// Create the durable asset row.
	asset := &domain.MediaAsset{
		ID:          uuid.New().String(),
		UploadID:    cmd.UploadID,
		UserID:      cmd.UserID,
		StorageKey:  session.AssetKey,
		ContentType: session.ContentType,
		SizeBytes:   session.SizeBytes,
		Status:      "uploading",
	}
	created, err := s.assets.Create(ctx, asset)
	if err != nil {
		return nil, err
	}

	// Update session status.
	_ = s.sessions.UpdateStatus(ctx, cmd.UploadID, domain.StatusCompleted)

	// Record quota usage now that the bytes have landed.
	_ = s.quota.RecordUpload(ctx, cmd.UserID, session.SizeBytes)

	// Publish UploadCompleted → triggers upload_pipeline Temporal workflow.
	_ = s.producer.Publish(ctx, events.TopicUploadCompleted, "upload.completed", events.UploadCompleted{
		UploadID:    cmd.UploadID,
		AssetID:     created.ID,
		UserID:      cmd.UserID,
		AssetKey:    session.AssetKey,
		ContentType: session.ContentType,
		SizeBytes:   session.SizeBytes,
		CompletedAt: time.Now(),
	})

	logger.FromContext(ctx).Info().
		Str("upload_id", cmd.UploadID).
		Str("asset_id", created.ID).
		Str("user_id", cmd.UserID).
		Msg("upload: completed — pipeline triggered")

	return created, nil
}

// ─── Abort ────────────────────────────────────────────────────────────────────

func (s *uploadService) Abort(ctx context.Context, cmd domain.AbortUploadCmd) error {
	session, err := s.sessions.GetByID(ctx, cmd.UploadID)
	if err != nil {
		return err
	}
	if session.UserID != cmd.UserID {
		return apierror.Forbidden("upload session belongs to another user")
	}

	// Best-effort R2 deletion — partial uploads must be cleaned up.
	if deleteErr := s.r2.DeleteObject(ctx, session.AssetKey); deleteErr != nil {
		logger.FromContext(ctx).Warn().
			Err(deleteErr).
			Str("asset_key", session.AssetKey).
			Msg("upload: R2 delete failed on abort — orphaned object may remain")
	}

	// Publish UploadFailed so quota service can release reserved storage.
	_ = s.producer.Publish(ctx, events.TopicUploadFailed, "upload.failed", events.UploadFailed{
		UploadID:    cmd.UploadID,
		UserID:      cmd.UserID,
		Reason:      cmd.Reason,
		FailureCode: cmd.Reason,
		FailedAt:    time.Now(),
	})

	return s.sessions.Delete(ctx, cmd.UploadID)
}

// ─── Asset reads ──────────────────────────────────────────────────────────────

func (s *uploadService) GetAsset(ctx context.Context, assetID, userID string) (*domain.MediaAsset, error) {
	asset, err := s.assets.GetByID(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if asset.UserID != userID {
		return nil, apierror.Forbidden("asset belongs to another user")
	}
	return asset, nil
}

func (s *uploadService) ListMyAssets(ctx context.Context, userID string, limit, offset int) ([]*domain.MediaAsset, int, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.assets.ListByUser(ctx, userID, limit, offset)
}
