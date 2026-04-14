package domain

import "time"

// ProfileID is a typed string to prevent confusion with UserID.
type ProfileID string

type UserID string

func (id ProfileID) String() string { return string(id) }

// ProfileType distinguishes household profiles.
type ProfileType string

const (
	ProfileTypeStandard ProfileType = "standard"
	ProfileTypeKids     ProfileType = "kids" // MPAA G/PG filter enforced
)

// MPAARating enumerates content ratings used by the kids-mode filter.
type MPAARating string

const (
	MPAARatingG    MPAARating = "G"
	MPAARatingPG   MPAARating = "PG"
	MPAARatingPG13 MPAARating = "PG-13"
	MPAARatingR    MPAARating = "R"
	MPAARatingNC17 MPAARating = "NC-17"
)

// MaxProfilesPerUser is the household limit (mirrors Netflix's 5-profile cap).
const MaxProfilesPerUser = 5

// ─── Profile entity ───────────────────────────────────────────────────────────

// Profile is a viewing identity within a user account.
// All recommendation, watchlist, viewing history, and rating data is scoped to
// a ProfileID, not a UserID. This isolates tastes between household members.
type Profile struct {
	ID        ProfileID
	UserID    UserID
	Name      string
	AvatarURL string
	Type      ProfileType

	// PINHash is a bcrypt hash of the profile's 4-digit PIN.
	// Empty means no PIN protection. Kids profiles should always have a PIN
	// set by a parent to prevent easy switching.
	PINHash string

	// MaxRating is the maximum MPAA rating visible in this profile.
	// Enforced for ProfileTypeKids (ceiling = PG).
	// For standard profiles nil means "no restriction".
	MaxRating *MPAARating

	// PreferredLanguages overrides user-level language preferences for this profile.
	PreferredLanguages []string

	// IsDefault is true for the primary profile that is selected on login.
	IsDefault bool

	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsKids returns true when this profile is in kids mode.
func (p *Profile) IsKids() bool { return p.Type == ProfileTypeKids }

// AllowsRating reports whether content with the given MPAA rating is visible.
func (p *Profile) AllowsRating(rating MPAARating) bool {
	if p.MaxRating == nil {
		return true
	}
	order := map[MPAARating]int{
		MPAARatingG: 0, MPAARatingPG: 1, MPAARatingPG13: 2,
		MPAARatingR: 3, MPAARatingNC17: 4,
	}
	return order[rating] <= order[*p.MaxRating]
}

// ─── Commands ─────────────────────────────────────────────────────────────────

// CreateProfileCmd carries validated input for creating a new profile.
type CreateProfileCmd struct {
	UserID    UserID
	Name      string
	AvatarURL string
	Type      ProfileType
	PIN       string      // plaintext; hashed in service layer
	MaxRating *MPAARating // nil = no restriction; required when Type = kids
}

// UpdateProfileCmd carries mutable profile fields.
type UpdateProfileCmd struct {
	UserID    UserID
	ProfileID ProfileID
	Name      *string
	AvatarURL *string
	PIN       *string // plaintext; service will re-hash
	MaxRating *MPAARating
	IsDefault *bool
	SortOrder *int
}

// SwitchProfileCmd is used when a user switches the active profile.
type SwitchProfileCmd struct {
	UserID    UserID
	ProfileID ProfileID
	PIN       string // required when the profile has a PIN
}

// DeleteProfileCmd removes a profile.  Cannot delete the last remaining profile
// or a default profile without first designating a new default.
type DeleteProfileCmd struct {
	UserID    UserID
	ProfileID ProfileID
}
