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

// Visibility controls who can see a user profile.
type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
	VisibilityFriends Visibility = "friends"
)

// ─── Core identity entity ─────────────────────────────────────────────────────

// User is the canonical identity record.
// Auth-specific fields (AuthProvider, ProviderID) have been removed — those
// live on LinkedAuthProvider in the auth module.
type User struct {
	ID            UserID
	Email         string
	EmailVerified bool
	Username      string
	DisplayName   string
	Role          Role
	Status        Status
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

// ─── Profile and preferences ──────────────────────────────────────────────────

// UserProfile holds mutable presentation data separated from the hot identity
// table to reduce write amplification on the users shard.
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

// ─── Commands ─────────────────────────────────────────────────────────────────

// UpdateProfileCmd carries mutable profile fields.
// Pointer fields mean "present in this update request".
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
