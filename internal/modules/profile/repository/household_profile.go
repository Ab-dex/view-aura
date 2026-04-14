package repository

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/profile/domain"
)

// HouseholdProfileRepository manages the profiles that belong to a user account.
// The repository is deliberately separate from the existing ProfileRepository
// (which manages UserProfile — bio, avatar etc.) to avoid naming confusion.
//
// Naming note: the existing ProfileRepository is for user _profile_ data
// (bio, avatar, website). This is HouseholdProfileRepository for the
// multi-account household profile (name, PIN, kids mode).
type HouseholdProfileRepository interface {
	// Create inserts a new profile. Returns ErrTooManyProfiles when the user
	// already has MaxProfilesPerUser profiles.
	Create(ctx context.Context, p *domain.Profile) (*domain.Profile, error)

	// GetByID fetches a profile by its ID.  Returns ErrNotFound when absent.
	GetByID(ctx context.Context, id domain.ProfileID) (*domain.Profile, error)

	// ListByUser returns all profiles for a user ordered by sort_order ASC.
	ListByUser(ctx context.Context, userID domain.UserID) ([]*domain.Profile, error)

	// Update applies mutable fields. Returns ErrNotFound when absent.
	Update(ctx context.Context, p *domain.Profile) (*domain.Profile, error)

	// SetDefault marks the given profile as default and clears the flag on all
	// other profiles for the same user in a single transaction.
	SetDefault(ctx context.Context, userID domain.UserID, profileID domain.ProfileID) error

	// Delete removes a profile.
	Delete(ctx context.Context, userID domain.UserID, profileID domain.ProfileID) error

	// CountByUser returns the current number of profiles for a user.
	CountByUser(ctx context.Context, userID domain.UserID) (int, error)
}
