package events

import "time"

// UserRegistered is published when a new account is created.
// Consumed by: notification-service (send welcome email), analytics (Flink).
type UserRegistered struct {
	UserID       string    `json:"user_id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"display_name"`
	Provider     string    `json:"provider"` // "email" | "google" | "apple" | "github"
	Locale       string    `json:"locale"`
	Country      string    `json:"country"`
	RegisteredAt time.Time `json:"registered_at"`
}

// UserEmailVerified is published when a user confirms their email address.
// Consumed by: notification-service (send confirmation), analytics.
type UserEmailVerified struct {
	UserID     string    `json:"user_id"`
	Email      string    `json:"email"`
	VerifiedAt time.Time `json:"verified_at"`
}

// UserLoggedIn is published on every successful login.
// Consumed by: analytics (session tracking, geo stats), security (anomaly detection).
type UserLoggedIn struct {
	UserID     string    `json:"user_id"`
	DeviceID   string    `json:"device_id"`
	IPAddress  string    `json:"ip_address"`
	UserAgent  string    `json:"user_agent"`
	LoggedInAt time.Time `json:"logged_in_at"`
}

// UserLoggedOut is published when a session is explicitly revoked.
// Consumed by: analytics.
type UserLoggedOut struct {
	UserID      string    `json:"user_id"`
	SessionID   string    `json:"session_id"`
	LoggedOutAt time.Time `json:"logged_out_at"`
}

// UserProfileUpdated is published when any profile field changes.
// Consumed by: search-service (update author name in review/thread indexes),
// social-service (update denormalized display name in feed entries).
type UserProfileUpdated struct {
	UserID string `json:"user_id"`
	// ChangedFields lists which top-level profile fields were modified.
	// Consumers use this to decide whether they care about the change.
	ChangedFields []string  `json:"changed_fields"`
	DisplayName   string    `json:"display_name"` // always included for denorm updates
	AvatarURL     string    `json:"avatar_url,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// UserRoleChanged is published when an admin upgrades a user to producer/critic/admin.
// Consumed by: notification-service (inform the user), analytics, audit log.
type UserRoleChanged struct {
	UserID    string    `json:"user_id"`
	OldRole   string    `json:"old_role"`
	NewRole   string    `json:"new_role"`
	ChangedBy string    `json:"changed_by"` // admin user_id
	ChangedAt time.Time `json:"changed_at"`
}

// UserSuspended is published when an account is suspended by a moderator or admin.
// Consumed by: notification-service (inform the user), social-service (remove from feeds),
// session-service (revoke all sessions).
type UserSuspended struct {
	UserID      string     `json:"user_id"`
	Reason      string     `json:"reason"`
	SuspendedBy string     `json:"suspended_by"` // admin user_id
	SuspendedAt time.Time  `json:"suspended_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"` // nil = permanent
}

// UserDeleted is published when an account is soft-deleted.
// Consumed by: all services (GDPR cascade — anonymise or delete user data),
// social-service (remove from follow graph), notification-service (cancel push tokens).
type UserDeleted struct {
	UserID    string    `json:"user_id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// UserFollowed is published when a follow relationship is created.
// Consumed by: notification-service (notify the followee),
// social-service (update activity feed).
type UserFollowed struct {
	FollowerID string    `json:"follower_id"`
	FolloweeID string    `json:"followee_id"`
	FollowedAt time.Time `json:"followed_at"`
}

// UserUnfollowed is published when a follow relationship is removed.
// Consumed by: social-service (remove from feed fan-out list).
type UserUnfollowed struct {
	FollowerID   string    `json:"follower_id"`
	FolloweeID   string    `json:"followee_id"`
	UnfollowedAt time.Time `json:"unfollowed_at"`
}
