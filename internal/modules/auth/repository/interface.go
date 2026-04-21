package repository

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/auth/domain"
	userdomain "github.com/Ab-dex/view-aura/internal/modules/user/domain"
)

// ─── AuthProviderRepository ───────────────────────────────────────────────────

// AuthProviderRepository manages user_auth_providers rows.
// This was previously in user/repository — it belongs here because linking,
// verifying, and rotating provider credentials are auth-layer concerns.
type AuthProviderRepository interface {
	// Link attaches a new provider or updates tokens for an existing link.
	// (user_id, provider) is the unique key — upsert semantics.
	Link(ctx context.Context, p *domain.LinkedAuthProvider) error

	// Unlink removes a provider link.
	Unlink(ctx context.Context, userID userdomain.UserID, provider domain.AuthProvider) error

	// GetByProvider looks up a link by (provider, provider_id).
	// Used during OAuth callback and email/password login.
	GetByProvider(ctx context.Context, provider domain.AuthProvider, providerID string) (*domain.LinkedAuthProvider, error)

	// GetByProviderID looks up a link by (provider, provider_id) by the stable
	// external ID — used to detect existing OAuth accounts on callback.
	GetByProviderID(ctx context.Context, provider domain.AuthProvider, providerID string) (*domain.LinkedAuthProvider, error)

	// ListByUser returns all providers linked to a user.
	ListByUser(ctx context.Context, userID userdomain.UserID) ([]*domain.LinkedAuthProvider, error)
}

// ─── SessionRepository ────────────────────────────────────────────────────────

// SessionRepository manages user sessions in Redis (hot path) and Postgres (audit).
// Moved here from user/repository — sessions are a credential concern.
type SessionRepository interface {
	// Create persists a new session to both Redis and Postgres.
	Create(ctx context.Context, session *domain.UserSession) (*domain.UserSession, error)

	// GetByID fetches a session from Redis, falling back to Postgres.
	GetByID(ctx context.Context, id string) (*domain.UserSession, error)

	// RevokeByID marks a session as revoked in both stores.
	RevokeByID(ctx context.Context, id string) error

	// RevokeAllForUser revokes every active session for a user (logout everywhere).
	RevokeAllForUser(ctx context.Context, userID userdomain.UserID) error

	// ListByUser returns all non-expired, non-revoked sessions for a user.
	ListByUser(ctx context.Context, userID userdomain.UserID) ([]*domain.UserSession, error)

	// AddToBlocklist adds a JTI to the Redis blocklist with the remaining token TTL.
	AddToBlocklist(ctx context.Context, jti string, ttlSeconds int64) error

	// IsBlocklisted returns true if the JTI has been revoked.
	IsBlocklisted(ctx context.Context, jti string) (bool, error)
}

// ─── SecurityRepository ───────────────────────────────────────────────────────

// SecurityRepository manages user_security rows.
// Moved here from user/repository — security state (lockouts, failed logins)
// is a credential/auth concern, not a user profile concern.
type SecurityRepository interface {
	// Upsert creates or updates the security record for a user.
	Upsert(ctx context.Context, sec *domain.UserSecurity) (*domain.UserSecurity, error)

	// GetByUserID fetches the security state.
	GetByUserID(ctx context.Context, userID userdomain.UserID) (*domain.UserSecurity, error)

	// IncrementFailedLogins atomically bumps the counter and sets last_failed_login.
	IncrementFailedLogins(ctx context.Context, userID userdomain.UserID) (*domain.UserSecurity, error)

	// ResetFailedLogins zeros the counter and clears any lockout.
	ResetFailedLogins(ctx context.Context, userID userdomain.UserID) error

	// LockUntil sets locked_until to the given time.
	LockUntil(ctx context.Context, userID userdomain.UserID, until interface{}) error

	// RecordLogin updates last_login_at and last_login_ip.
	RecordLogin(ctx context.Context, userID userdomain.UserID, ip string) error
}

// ─── VerificationTokenRepository ─────────────────────────────────────────────

// VerificationTokenRepository persists one-time email tokens in Postgres.
//
// Only the SHA-256 hash is stored — the plaintext is delivered by email.
// Lifecycle: Create → GetByHash (service calls IsUsable()) → MarkRedeemed.
// Cleanup: DeleteExpired runs nightly; DeleteAllForUser on re-issue.
type VerificationTokenRepository interface {
	// Create inserts a new token row. Callers must call DeleteAllForUser first.
	Create(ctx context.Context, token *domain.VerificationToken) error

	// GetByHash retrieves the token whose token_hash matches SHA-256(plaintext).
	// Returns apierror.ErrTokenInvalid when no matching row exists.
	GetByHash(ctx context.Context, hash string) (*domain.VerificationToken, error)

	// MarkRedeemed sets redeemed_at = NOW() atomically.
	MarkRedeemed(ctx context.Context, id string) error

	// DeleteExpired removes expired tokens for cleanup jobs.
	DeleteExpired(ctx context.Context, purpose domain.VerificationPurpose) (int64, error)

	// DeleteAllForUser removes all tokens for a user+purpose before re-issuing.
	DeleteAllForUser(ctx context.Context, userID string, purpose domain.VerificationPurpose) error
}

// ─── MFARepository ───────────────────────────────────────────────────────────

// MFARepository manages MFA enrollments (Postgres) and challenges (Redis).
type MFARepository interface {
	// ── Postgres: enrollment ─────────────────────────────────────────────────

	CreateEnrollment(ctx context.Context, e *domain.MFAEnrollment) error

	// GetEnrollment returns nil, nil when no enrollment exists.
	GetEnrollment(ctx context.Context, userID string) (*domain.MFAEnrollment, error)

	// MarkEnrollmentVerified flips verified = true after the user proves TOTP works.
	MarkEnrollmentVerified(ctx context.Context, enrollmentID string) error

	// UpdateLastUsed sets last_used_at = NOW(). Non-fatal — called async.
	UpdateLastUsed(ctx context.Context, enrollmentID string) error

	DeleteEnrollment(ctx context.Context, userID string) error

	// SaveBackupCodes replaces the entire backup code hash set for an enrollment.
	SaveBackupCodes(ctx context.Context, enrollmentID string, hashes []string) error

	// ConsumeBackupCode atomically finds and removes a matching hash.
	ConsumeBackupCode(ctx context.Context, enrollmentID string, hash string) (bool, error)

	// ── Redis: challenges ────────────────────────────────────────────────────

	// CreateChallenge stores a pending MFA challenge (TTL 5 min).
	CreateChallenge(ctx context.Context, ch *domain.MFAChallenge) error

	// GetChallenge returns apierror.ErrTokenInvalid when absent or expired.
	GetChallenge(ctx context.Context, challengeID string) (*domain.MFAChallenge, error)

	// DeleteChallenge removes a challenge after it is consumed.
	DeleteChallenge(ctx context.Context, challengeID string) error
}

// ─── OAuthStateRepository ────────────────────────────────────────────────────

// OAuthStateRepository stores CSRF nonces in Redis for the OAuth redirect flow.
// Key: auth:oauth_state:{nonce}.  TTL: 15 minutes.
type OAuthStateRepository interface {
	// Save persists the OAuthState with TTL derived from state.ExpiresAt.
	Save(ctx context.Context, state *domain.OAuthState) error

	// GetAndDelete atomically retrieves and removes the nonce (CSRF replay protection).
	// Returns nil, nil when absent or expired.
	GetAndDelete(ctx context.Context, state string) (*domain.OAuthState, error)
}
