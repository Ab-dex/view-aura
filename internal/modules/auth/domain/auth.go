package domain

import (
	"time"

	userdomain "github.com/Ab-dex/view-aura/internal/modules/user/domain"
)

// ─── Auth provider ────────────────────────────────────────────────────────────

// AuthProvider identifies the identity provider used to authenticate.
type AuthProvider string

const (
	ProviderEmail  AuthProvider = "email"
	ProviderGoogle AuthProvider = "google"
	ProviderApple  AuthProvider = "apple"
	ProviderGitHub AuthProvider = "github"
)

// LinkedAuthProvider stores an external OAuth provider credential linked to a
// user account.  Supports multiple providers per account.
type LinkedAuthProvider struct {
	ID           string
	UserID       userdomain.UserID
	Provider     AuthProvider
	ProviderID   string  // the stable ID on the provider's side (or email for ProviderEmail)
	AccessToken  *string // password bcrypt hash when Provider == email
	RefreshToken *string
	AvatarURL    string
	LinkedAt     time.Time
	LastUsedAt   *time.Time
}

// ─── Session ──────────────────────────────────────────────────────────────────

// UserSession represents a single authenticated session.
// Written to Redis (hot path) and Postgres (audit log).
type UserSession struct {
	ID               string
	UserID           userdomain.UserID
	DeviceID         string
	IPAddress        string
	UserAgent        string
	RefreshTokenHash string
	CreatedAt        time.Time
	ExpiresAt        time.Time
	RevokedAt        *time.Time
}

func (s *UserSession) IsExpired() bool { return time.Now().After(s.ExpiresAt) }
func (s *UserSession) IsRevoked() bool { return s.RevokedAt != nil }

// ─── Security ─────────────────────────────────────────────────────────────────

// UserSecurity stores authentication state checked on every login.
type UserSecurity struct {
	UserID           userdomain.UserID
	FailedLoginCount int
	LastFailedLogin  *time.Time
	LockedUntil      *time.Time
	RiskScore        float64
	TwoFactorEnabled bool
	LastLoginAt      *time.Time
	LastLoginIP      string
	UpdatedAt        time.Time
}

func (s *UserSecurity) IsLocked() bool {
	return s.LockedUntil != nil && s.LockedUntil.After(time.Now())
}

// ─── Tokens ───────────────────────────────────────────────────────────────────

// TokenPair bundles access and refresh tokens returned after successful auth.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // seconds until access token expiry
}

// ─── Verification tokens ──────────────────────────────────────────────────────

// VerificationPurpose identifies what a one-time token is for.
type VerificationPurpose string

const (
	PurposeEmailVerify   VerificationPurpose = "email_verify"
	PurposePasswordReset VerificationPurpose = "password_reset"
	PurposeMFAEmailOTP   VerificationPurpose = "mfa_email_otp"
	PurposeMagicLink     VerificationPurpose = "magic_link"
)

// TTL returns the canonical expiry duration for each purpose.
// Single source of truth — used by the service when creating tokens.
func TTL(p VerificationPurpose) time.Duration {
	switch p {
	case PurposeEmailVerify:
		return 24 * time.Hour
	case PurposePasswordReset:
		return time.Hour
	case PurposeMagicLink:
		return 15 * time.Minute
	case PurposeMFAEmailOTP:
		return 10 * time.Minute
	default:
		return time.Hour
	}
}

// VerificationToken is a short-lived, single-use credential delivered by email.
//
// Only the SHA-256 hash of the plaintext is stored (token_hash column).
// The plaintext is sent to the user — never persisted.
type VerificationToken struct {
	ID         string
	UserID     string
	Purpose    VerificationPurpose
	TokenHash  string // SHA-256(plaintext_token)
	ExpiresAt  time.Time
	RedeemedAt *time.Time
	CreatedAt  time.Time
	IPAddress  string
	UserAgent  string
}

func (t *VerificationToken) IsExpired() bool  { return time.Now().After(t.ExpiresAt) }
func (t *VerificationToken) IsRedeemed() bool { return t.RedeemedAt != nil }

// IsUsable returns true when the token can still be consumed.
func (t *VerificationToken) IsUsable() bool { return !t.IsExpired() && !t.IsRedeemed() }

// ─── MFA ─────────────────────────────────────────────────────────────────────

// MFAMethod enumerates supported second factors.
type MFAMethod string

const (
	MFAMethodTOTP   MFAMethod = "totp"
	MFAMethodEmail  MFAMethod = "email"
	MFAMethodBackup MFAMethod = "backup"
)

// MFAEnrollment stores a user's enrolled second factor in Postgres.
// The TOTP secret is stored AES-256-GCM encrypted (key from Vault).
type MFAEnrollment struct {
	ID               string
	UserID           string
	Method           MFAMethod
	EncryptedSecret  []byte   // AES-GCM(base32_totp_secret); nil for email MFA
	BackupCodeHashes []string // bcrypt($backup_code); populated after ConfirmTOTP
	Verified         bool     // false until user proves first TOTP code works
	EnrolledAt       time.Time
	LastUsedAt       *time.Time
}

// MFAChallenge is a short-lived Redis record created after first-factor auth
// when the account has MFA enabled.
//
// Redis key:  auth:mfa_challenge:{id}
// TTL:        5 minutes
type MFAChallenge struct {
	ID        string
	UserID    string
	Method    MFAMethod
	CreatedAt time.Time
	ExpiresAt time.Time
}

func (c *MFAChallenge) IsExpired() bool { return time.Now().After(c.ExpiresAt) }

// ─── OAuth ────────────────────────────────────────────────────────────────────

// OAuthState is a CSRF nonce stored in Redis during the OAuth redirect flow.
// Key: auth:oauth_state:{nonce}.  TTL: 15 minutes.
type OAuthState struct {
	State        string
	Provider     AuthProvider
	RedirectURI  string
	PKCEVerifier string // RFC 7636 code_verifier
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

func (s *OAuthState) IsExpired() bool { return time.Now().After(s.ExpiresAt) }

// OAuthUserInfo is the normalised user payload returned by any provider after
// a successful token exchange.
type OAuthUserInfo struct {
	Provider    AuthProvider
	ProviderID  string
	Email       string
	DisplayName string
	AvatarURL   string
	Verified    bool
}

// ─── Commands ────────────────────────────────────────────────────────────────

// RegisterCmd carries validated input for creating a new email/password account.
type RegisterCmd struct {
	Email       string
	Password    string
	DisplayName string
	Locale      string
	Country     string
}

// LoginCmd carries credentials for a local (email/password) login attempt.
type LoginCmd struct {
	Email     string
	Password  string
	DeviceID  string
	IPAddress string
	UserAgent string
}

type SendVerificationEmailCmd struct {
	UserID    string
	Email     string
	IPAddress string
	UserAgent string
}

type VerifyEmailCmd struct {
	Token     string
	IPAddress string
}

type SendPasswordResetCmd struct {
	Email     string
	IPAddress string
	UserAgent string
}

type ResetPasswordCmd struct {
	Token       string
	NewPassword string
	IPAddress   string
}

type ChangePasswordCmd struct {
	UserID      string
	OldPassword string
	NewPassword string
}

type EnrollTOTPCmd struct {
	UserID string
}

// VerifyTOTPEnrollmentCmd confirms the authenticator app is correctly configured.
type VerifyTOTPEnrollmentCmd struct {
	UserID string
	Code   string
}

// VerifyMFACmd completes the second-factor step after first-factor auth.
type VerifyMFACmd struct {
	UserID    string
	Code      string
	Method    MFAMethod
	DeviceID  string
	IPAddress string
	UserAgent string
}

type DisableMFACmd struct {
	UserID   string
	Password string
}

type OAuthBeginCmd struct {
	Provider    AuthProvider
	RedirectURI string
}

type OAuthCallbackCmd struct {
	Provider  AuthProvider
	Code      string
	State     string
	DeviceID  string
	IPAddress string
	UserAgent string
}

// ─── Results ─────────────────────────────────────────────────────────────────

// TOTPEnrollmentResult is returned once during TOTP setup.
type TOTPEnrollmentResult struct {
	EnrollmentID    string
	Secret          string // base32 TOTP secret for manual entry
	ProvisioningURI string // otpauth:// URI — encode as QR on the client
}

// MFAChallengeResult is returned when MFA is required after first-factor auth.
type MFAChallengeResult struct {
	ChallengeID       string
	Method            MFAMethod
	MaskedDestination string
}

// OAuthBeginResult carries the provider redirect URL and CSRF state nonce.
type OAuthBeginResult struct {
	AuthorizationURL string
	State            string
}
