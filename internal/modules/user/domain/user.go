package domain

import "time"

// UserID is a typed string wrapper to prevent ID confusion across services.
type UserID string

func (id UserID) String() string { return string(id) }

// Role enumerates every role a user account can hold.
type Role string

const (
	RoleUser     Role = "user"
	RoleProducer Role = "producer"
	RoleCritic   Role = "critic"
	RoleAdmin    Role = "admin"
)

// Status enumerates account lifecycle states.
type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
	StatusDeleted   Status = "deleted"
	StatusPending   Status = "pending_verification"
)

// AuthProvider identifies the identity provider used to create the account.
type AuthProvider string

const (
	ProviderEmail  AuthProvider = "email"
	ProviderGoogle AuthProvider = "google"
	ProviderApple  AuthProvider = "apple"
	ProviderGitHub AuthProvider = "github"
)

// Visibility controls who can see a user profile.
type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
	VisibilityFriends Visibility = "friends"
)

// ─── Core entities ────────────────────────────────────────────────────────────

// User is the canonical identity record.
type User struct {
	ID            UserID
	Email         string
	EmailVerified bool
	Username      string
	DisplayName   string
	Role          Role
	Status        Status
	AuthProvider  AuthProvider
	ProviderID    *string
	Locale        string
	Timezone      string
	Country       string
	IsDeleted     bool
	DeletedAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (u *User) IsActive() bool    { return u.Status == StatusActive && !u.IsDeleted }
func (u *User) IsSuspended() bool { return u.Status == StatusSuspended }
func (u *User) IsAdmin() bool     { return u.Role == RoleAdmin }

// LinkedAuthProvider stores an external OAuth provider credential linked to a
// user account (supports multiple providers per account).
type LinkedAuthProvider struct {
	ID           string
	UserID       UserID
	Provider     AuthProvider
	ProviderID   string
	AccessToken  *string
	RefreshToken *string
	LinkedAt     time.Time
	LastUsedAt   *time.Time
}

// UserProfile holds mutable presentation data separated from the hot
// identity table to reduce write amplification on the users shard.
type UserProfile struct {
	UserID     UserID
	AvatarURL  string
	BannerURL  string
	Bio        string
	Website    string
	Birthdate  *time.Time
	Gender     string
	Visibility Visibility
	UpdatedAt  time.Time
}

// UserPreferences holds per-user content and UI settings.
// Using concrete fields (not JSONB) makes the preference store queryable.
type UserPreferences struct {
	UserID             UserID
	PreferredGenres    []string
	DislikedGenres     []string
	PreferredLanguages []string
	AdultContent       bool
	DarkMode           bool
	AutoplayTrailers   bool
	UpdatedAt          time.Time
}

// UserSecurity stores authentication state checked on every login.
type UserSecurity struct {
	UserID           UserID
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

// UserSession represents a single authenticated session persisted for audit.
type UserSession struct {
	ID               string
	UserID           UserID
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

// ─── Command / value objects ──────────────────────────────────────────────────

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

// UpdateProfileCmd carries mutable profile fields. Pointer fields mean
// "present in this update request".
type UpdateProfileCmd struct {
	UserID     UserID
	AvatarURL  *string
	BannerURL  *string
	Bio        *string
	Website    *string
	Birthdate  *time.Time
	Gender     *string
	Visibility *string
}

// UpdatePreferencesCmd carries mutable preference fields.
type UpdatePreferencesCmd struct {
	UserID             UserID
	PreferredGenres    *[]string
	DislikedGenres     *[]string
	PreferredLanguages *[]string
	AdultContent       *bool
	DarkMode           *bool
	AutoplayTrailers   *bool
}

// TokenPair bundles access and refresh tokens returned after successful auth.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // seconds until access token expiry
}
