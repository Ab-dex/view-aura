package repository

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/user/domain"
)

// UserRepository defines all persistence operations for the User aggregate.
type UserRepository interface {
	// Create inserts a new user row and returns the persisted entity.
	Create(ctx context.Context, user *domain.User) (*domain.User, error)

	// GetByID fetches a user by primary key.
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

// ProfileRepository manages user_profiles rows (bio, avatar, website).
// Not to be confused with the household profile (internal/modules/profile).
type ProfileRepository interface {
	Upsert(ctx context.Context, profile *domain.UserProfile) (*domain.UserProfile, error)
	GetByUserID(ctx context.Context, userID domain.UserID) (*domain.UserProfile, error)
}

// PreferencesRepository manages user_preferences rows.
type PreferencesRepository interface {
	Upsert(ctx context.Context, prefs *domain.UserPreferences) (*domain.UserPreferences, error)
	GetByUserID(ctx context.Context, userID domain.UserID) (*domain.UserPreferences, error)
}
