package domain

import (
	"time"

	userdomain "github.com/Ab-dex/view-aura/internal/modules/user/domain"
)

// UserID aliases user/domain.UserID so the profile domain uses the canonical
// type without re-declaring it locally.  Type aliases preserve identity —
// profile.domain.UserID IS user/domain.UserID; no casts needed at boundaries.
type UserID = userdomain.UserID

// ProfileID is a typed string to prevent confusion with UserID.
type ProfileID string

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
// a ProfileID, not a UserID.  This isolates tastes between household members.
type Profile struct {
	ID        ProfileID
	UserID    UserID // = user/domain.UserID (type alias — no cast required)
	Name      string
	AvatarURL string
	Type      ProfileType

	// PINHash is a bcrypt hash of the 4-digit PIN.
	// Empty means no PIN protection.
	PINHash string

	// MaxRating is the maximum MPAA rating visible in this profile.
	// For standard profiles nil means no restriction.
	// Kids profiles are always capped at PG.
	MaxRating *MPAARating

	PreferredLanguages []string
	IsDefault          bool
	SortOrder          int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

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

type CreateProfileCmd struct {
	UserID    UserID
	Name      string
	AvatarURL string
	Type      ProfileType
	PIN       string
	MaxRating *MPAARating
}

type UpdateProfileCmd struct {
	UserID    UserID
	ProfileID ProfileID
	Name      *string
	AvatarURL *string
	PIN       *string
	MaxRating *MPAARating
	IsDefault *bool
	SortOrder *int
}

type SwitchProfileCmd struct {
	UserID    UserID
	ProfileID ProfileID
	PIN       string
}

type DeleteProfileCmd struct {
	UserID    UserID
	ProfileID ProfileID
}
