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
	Initiate(ctx context.Context, cmd domain.InitiateUploadCmd) (*domain.UploadSession, error)
	GetProgress(ctx context.Context, uploadID, userID string) (*domain.UploadSession, error)
	Complete(ctx context.Context, cmd domain.CompleteUploadCmd) (*domain.MediaAsset, error)
	Abort(ctx context.Context, cmd domain.AbortUploadCmd) error
	GetAsset(ctx context.Context, assetID, userID string) (*domain.MediaAsset, error)
	ListMyAssets(ctx context.Context, userID string, limit, offset int) ([]*domain.MediaAsset, int, error)
}

type uploadService struct {
	sessions repository.UploadSessionRepository
	assets   repository.MediaAssetRepository
	quota    quotasvc.QuotaService
	// r2 accepts r2.ObjectStore so either *r2.Client (real) or
	// *ResilientR2Client (three-tier fallback) can be injected.
	r2       r2pkg.ObjectStore
	producer events.Producer
}

// NewUploadService constructs the service.
// r2 is typed as r2pkg.ObjectStore — inject *ResilientR2Client via Wire for
// automatic three-tier R2 fallback, or *r2pkg.Client directly in tests.
func NewUploadService(
	sessions repository.UploadSessionRepository,
	assets repository.MediaAssetRepository,
	quota quotasvc.QuotaService,
	r2 r2pkg.ObjectStore,
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
	result, err := s.quota.CheckUploadAllowed(ctx, cmd.UserID, cmd.SizeBytes)
	if err != nil {
		return nil, err
	}
	if !result.Allowed {
		return nil, apierror.New(403, apierror.Code(result.DenialCode), result.DenialMsg)
	}

	uploadID := uuid.New().String()
	assetKey := fmt.Sprintf("uploads/%s/%s/%s", cmd.UserID, uploadID, cmd.FileName)

	// PresignPutObject degrades automatically via ResilientR2Client:
	//   Tier 1: R2 presigned URL
	//   Tier 2: local temp path URL
	//   Tier 3: hard 503 (no silent data loss)
	presignedURL, err := s.r2.PresignPutObject(ctx, assetKey, cmd.ContentType)
	if err != nil {
		return nil, err // ResilientR2Client already mapped this to apierror
	}

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
		Int64("size_bytes", cmd.SizeBytes).
		Msg("upload: session initiated")

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

	if existing, err := s.assets.GetByUploadID(ctx, cmd.UploadID); err == nil && existing != nil {
		return existing, nil
	}

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

	_ = s.sessions.UpdateStatus(ctx, cmd.UploadID, domain.StatusCompleted)
	_ = s.quota.RecordUpload(ctx, cmd.UserID, session.SizeBytes)

	// Publish UploadCompleted — ResilientProducer handles Kafka fallback.
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
		Msg("upload: completed")

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

	// Best-effort deletion — ResilientR2Client logs failures rather than panicking.
	if deleteErr := s.r2.DeleteObject(ctx, session.AssetKey); deleteErr != nil {
		logger.FromContext(ctx).Warn().
			Err(deleteErr).
			Str("asset_key", session.AssetKey).
			Msg("upload: R2 delete failed on abort — orphaned object may remain")
	}

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
