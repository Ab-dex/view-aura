package repository

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/moderation/domain"
)

// CaseRepository persists moderation cases to Postgres.
type CaseRepository interface {
	Create(ctx context.Context, c *domain.ModerationCase) (*domain.ModerationCase, error)
	GetByID(ctx context.Context, id string) (*domain.ModerationCase, error)
	List(ctx context.Context, filter domain.CaseFilter) ([]*domain.ModerationCase, int, error)
	Update(ctx context.Context, c *domain.ModerationCase) (*domain.ModerationCase, error)
	// GetByContentID returns the active (non-decided) case for a content item,
	// or nil if no case exists. Used to prevent duplicate case creation.
	GetByContentID(ctx context.Context, contentType domain.ContentType, contentID string) (*domain.ModerationCase, error)
}

// AppealRepository persists appeals to Postgres.
type AppealRepository interface {
	Create(ctx context.Context, a *domain.Appeal) (*domain.Appeal, error)
	GetByID(ctx context.Context, id string) (*domain.Appeal, error)
	GetByCaseID(ctx context.Context, caseID string) (*domain.Appeal, error)
	Update(ctx context.Context, a *domain.Appeal) (*domain.Appeal, error)
}

// LockRepository manages distributed locks in Redis to prevent two moderators
// from simultaneously acting on the same case.
type LockRepository interface {
	// Acquire attempts to acquire a lock on the case for moderatorID.
	// Returns true if acquired, false if already locked by another moderator.
	// The lock expires after ttl seconds if not explicitly released.
	Acquire(ctx context.Context, caseID, moderatorID string) (bool, error)
	// Release removes the lock.
	Release(ctx context.Context, caseID string) error
	// IsLocked returns the moderatorID holding the lock, or "" if unlocked.
	IsLocked(ctx context.Context, caseID string) (string, error)
}
