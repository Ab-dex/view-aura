package repository

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/user/domain"
)

// UserRepository defines all persistence operations for the User aggregate.
// The interface lives in the domain layer; concrete implementations live in
// the infrastructure layer (postgres/, redis/).
type UserRepository interface {
	// Create inserts a new user row and returns the persisted entity.
	Create(ctx context.Context, user *domain.User) (*domain.User, error)

	// GetByID fetches a user by primary key. Returns ErrUserNotFound if absent.
	GetByID(ctx context.Context, id domain.UserID) (*domain.User, error)

	// GetByEmail fetches a user by email address (case-insensitive).
	GetByEmail(ctx context.Context, email string) (*domain.User, error)

	// GetByUsername fetches a user by username.
	GetByUsername(ctx context.Context, username string) (*domain.User, error)

	// ExistsByEmail returns true if the email is already registered.
	ExistsByEmail(ctx context.Context, email string) (bool, error)

	// ExistsByUsername returns true if the username is taken.
	ExistsByUsername(ctx context.Context, username string) (bool, error)

	// Update persists changes to the mutable fields of an existing user.
	Update(ctx context.Context, user *domain.User) (*domain.User, error)

	// SoftDelete marks a user as deleted without removing the row.
	SoftDelete(ctx context.Context, id domain.UserID) error
}

// ProfileRepository manages user_profiles rows.
type ProfileRepository interface {
	// Upsert creates or replaces a profile record.
	Upsert(ctx context.Context, profile *domain.UserProfile) (*domain.UserProfile, error)

	// GetByUserID fetches the profile for a given user.
	GetByUserID(ctx context.Context, userID domain.UserID) (*domain.UserProfile, error)
}

// PreferencesRepository manages user_preferences rows.
type PreferencesRepository interface {
	// Upsert creates or replaces a preferences record.
	Upsert(ctx context.Context, prefs *domain.UserPreferences) (*domain.UserPreferences, error)

	// GetByUserID fetches preferences for a given user.
	GetByUserID(ctx context.Context, userID domain.UserID) (*domain.UserPreferences, error)
}

// SecurityRepository manages user_security rows.
type SecurityRepository interface {
	// Upsert creates or updates the security record for a user.
	Upsert(ctx context.Context, sec *domain.UserSecurity) (*domain.UserSecurity, error)

	// GetByUserID fetches the security state for a given user.
	GetByUserID(ctx context.Context, userID domain.UserID) (*domain.UserSecurity, error)

	// IncrementFailedLogins atomically increments the failed login counter
	// and sets last_failed_login = NOW(). Returns the updated record.
	IncrementFailedLogins(ctx context.Context, userID domain.UserID) (*domain.UserSecurity, error)

	// ResetFailedLogins zeros the failed login counter and clears the lock.
	ResetFailedLogins(ctx context.Context, userID domain.UserID) error

	// LockUntil sets locked_until to the given time.
	LockUntil(ctx context.Context, userID domain.UserID, until interface{}) error

	// RecordLogin updates last_login_at and last_login_ip.
	RecordLogin(ctx context.Context, userID domain.UserID, ip string) error
}

// SessionRepository manages user_sessions rows and the Redis session cache.
type SessionRepository interface {
	// Create persists a new session both in Postgres (audit) and Redis (hot path).
	Create(ctx context.Context, session *domain.UserSession) (*domain.UserSession, error)

	// GetByID fetches a session from Redis, falling back to Postgres.
	GetByID(ctx context.Context, id string) (*domain.UserSession, error)

	// RevokeByID marks a session as revoked in both stores.
	RevokeByID(ctx context.Context, id string) error

	// RevokeAllForUser revokes every active session for a user (logout everywhere).
	RevokeAllForUser(ctx context.Context, userID domain.UserID) error

	// ListByUser returns all non-expired, non-revoked sessions for a user.
	ListByUser(ctx context.Context, userID domain.UserID) ([]*domain.UserSession, error)

	// AddToBlocklist adds a JTI to the Redis blocklist with an expiry matching
	// the remaining token lifetime.
	AddToBlocklist(ctx context.Context, jti string, ttlSeconds int64) error

	// IsBlocklisted returns true if the JTI is in the Redis blocklist.
	IsBlocklisted(ctx context.Context, jti string) (bool, error)
}

// AuthProviderRepository manages user_auth_providers rows.
type AuthProviderRepository interface {
	// Link attaches a new OAuth provider to an existing user.
	Link(ctx context.Context, provider *domain.LinkedAuthProvider) error

	// Unlink detaches an OAuth provider from a user.
	Unlink(ctx context.Context, userID domain.UserID, provider domain.AuthProvider) error

	// GetByProvider fetches the link record for a given provider + providerID.
	GetByProvider(ctx context.Context, provider domain.AuthProvider, providerID string) (*domain.LinkedAuthProvider, error)

	// ListByUser returns all providers linked to a user.
	ListByUser(ctx context.Context, userID domain.UserID) ([]*domain.LinkedAuthProvider, error)
}
