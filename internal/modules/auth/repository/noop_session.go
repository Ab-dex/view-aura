package repository

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/auth/domain"
	userdomain "github.com/Ab-dex/view-aura/internal/modules/user/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

// NoopSessionRepository is a safe fallback when Redis is unavailable.
// It implements SessionRepository but performs no side effects.
type NoopSessionRepository struct{}

// NewNoopSessionRepository returns a safe in-memory no-op implementation.
func NewNoopSessionRepository() SessionRepository {
	return &NoopSessionRepository{}
}

func (n *NoopSessionRepository) Create(ctx context.Context, s *domain.UserSession) (*domain.UserSession, error) {
	// simulate success without persistence
	return s, nil
}

func (n *NoopSessionRepository) GetByID(ctx context.Context, id string) (*domain.UserSession, error) {
	return nil, apierror.ErrSessionNotFound
}

func (n *NoopSessionRepository) RevokeByID(ctx context.Context, id string) error {
	// nothing to revoke, but treat as success (idempotent behavior)
	return nil
}

func (n *NoopSessionRepository) RevokeAllForUser(ctx context.Context, userID userdomain.UserID) error {
	// no sessions exist in noop mode
	return nil
}

func (n *NoopSessionRepository) ListByUser(ctx context.Context, userID userdomain.UserID) ([]*domain.UserSession, error) {
	return []*domain.UserSession{}, nil
}

func (n *NoopSessionRepository) AddToBlocklist(ctx context.Context, jti string, ttlSeconds int64) error {
	return nil
}

func (n *NoopSessionRepository) IsBlocklisted(ctx context.Context, jti string) (bool, error) {
	return false, nil
}
