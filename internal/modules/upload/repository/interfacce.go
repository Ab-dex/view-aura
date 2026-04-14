package repository

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/upload/domain"
)

// UploadSessionRepository manages ephemeral upload session state in Redis.
// Sessions are keyed by upload ID and expire automatically via TTL.
type UploadSessionRepository interface {
	// Create stores a new session. Fails if the upload ID already exists.
	Create(ctx context.Context, s *domain.UploadSession) (*domain.UploadSession, error)

	// GetByID retrieves a session. Returns apierror.ErrUploadNotFound when the
	// key is missing (session expired or never created).
	GetByID(ctx context.Context, id string) (*domain.UploadSession, error)

	// UpdateStatus atomically reads the session, changes its status field, and
	// writes it back within the remaining TTL.
	UpdateStatus(ctx context.Context, id string, status domain.UploadStatus) error

	// Delete removes the session key immediately (used on abort/complete).
	Delete(ctx context.Context, id string) error
}

// MediaAssetRepository manages persistent asset records in Postgres.
// A MediaAsset row is created when the client calls /complete.
type MediaAssetRepository interface {
	// Create inserts a new asset row.
	Create(ctx context.Context, a *domain.MediaAsset) (*domain.MediaAsset, error)

	// GetByID fetches an asset by its UUID.
	GetByID(ctx context.Context, id string) (*domain.MediaAsset, error)

	// GetByUploadID fetches the asset linked to a given upload session.
	// Returns nil, nil when no asset has been created for the session yet.
	GetByUploadID(ctx context.Context, uploadID string) (*domain.MediaAsset, error)

	// UpdateStatus sets the asset pipeline status.
	// Called by the Temporal workflow activities as they progress.
	UpdateStatus(ctx context.Context, id, status string) error

	// LinkMovie associates a completed asset with a movie record.
	// Called by editorial / admin after the pipeline completes.
	LinkMovie(ctx context.Context, assetID, movieID string) error

	// ListByUser returns all assets belonging to a user, newest first.
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.MediaAsset, int, error)
}
