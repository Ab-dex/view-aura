package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Ab-dex/view-aura/internal/modules/profile/domain"
	"github.com/Ab-dex/view-aura/internal/modules/profile/repository"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// ProfileService manages viewing profiles within a user account.
type ProfileService interface {
	// Create adds a new profile (max MaxProfiles per account).
	Create(ctx context.Context, cmd domain.CreateProfileCmd) (*domain.Profile, error)

	// GetByID fetches a profile, verifying it belongs to the given user.
	GetByID(ctx context.Context, userID domain.UserID, profileID domain.ProfileID) (*domain.Profile, error)

	// List returns all profiles for an account ordered by sort_order ASC.
	List(ctx context.Context, userID domain.UserID) ([]*domain.Profile, error)

	// Update applies partial changes to a profile.
	Update(ctx context.Context, cmd domain.UpdateProfileCmd) (*domain.Profile, error)

	// Delete removes a profile. Errors if it is the last profile or the default.
	Delete(ctx context.Context, cmd domain.DeleteProfileCmd) error

	// Switch verifies the PIN (when set) and returns the profile so the caller
	// can embed profile_id in the session or JWT claims.
	Switch(ctx context.Context, cmd domain.SwitchProfileCmd) (*domain.Profile, error)

	// SetDefault marks a profile as the account default.
	SetDefault(ctx context.Context, userID domain.UserID, profileID domain.ProfileID) error
}

type profileService struct {
	repo repository.HouseholdProfileRepository
}

// NewProfileService constructs the service. Wire calls this.
func NewProfileService(repo repository.HouseholdProfileRepository) ProfileService {
	return &profileService{repo: repo}
}

// ─── Create ───────────────────────────────────────────────────────────────────

func (s *profileService) Create(
	ctx context.Context, cmd domain.CreateProfileCmd,
) (*domain.Profile, error) {
	log := logger.FromContext(ctx)

	if err := validateCreateProfile(cmd); err != nil {
		return nil, err
	}

	pinHash := ""
	if cmd.PIN != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(cmd.PIN), bcrypt.DefaultCost)
		if err != nil {
			return nil, apierror.Internal("hash profile pin", err)
		}
		pinHash = string(h)
	}

	// Kids profiles are always capped at PG regardless of what was submitted.
	maxRating := cmd.MaxRating
	if cmd.Type == domain.ProfileTypeKids {
		pg := domain.MPAARatingPG
		maxRating = &pg
	}

	// The first profile on an account auto-becomes the default.
	count, err := s.repo.CountByUser(ctx, cmd.UserID)
	if err != nil {
		return nil, err
	}

	p := &domain.Profile{
		ID:                 domain.ProfileID(uuid.New().String()),
		UserID:             cmd.UserID,
		Name:               cmd.Name,
		AvatarURL:          cmd.AvatarURL,
		Type:               cmd.Type,
		PINHash:            pinHash,
		MaxRating:          maxRating,
		PreferredLanguages: []string{},
		IsDefault:          count == 0,
		SortOrder:          count, // append to end
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	created, err := s.repo.Create(ctx, p)
	if err != nil {
		return nil, err
	}

	log.Info().
		Str("profile_id", created.ID.String()).
		Str("user_id", string(cmd.UserID)). // Cast to string
		Str("type", string(cmd.Type)).
		Msg("household_profile: created")

	return created, nil
}

// ─── GetByID ──────────────────────────────────────────────────────────────────

func (s *profileService) GetByID(
	ctx context.Context, userID domain.UserID, profileID domain.ProfileID,
) (*domain.Profile, error) {
	p, err := s.repo.GetByID(ctx, profileID)
	if err != nil {
		return nil, err
	}
	if p.UserID != userID {
		return nil, apierror.ErrNotAllowed
	}
	return p, nil
}

// ─── List ─────────────────────────────────────────────────────────────────────

func (s *profileService) List(
	ctx context.Context, userID domain.UserID,
) ([]*domain.Profile, error) {
	return s.repo.ListByUser(ctx, userID)
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (s *profileService) Update(
	ctx context.Context, cmd domain.UpdateProfileCmd,
) (*domain.Profile, error) {
	p, err := s.repo.GetByID(ctx, cmd.ProfileID)
	if err != nil {
		return nil, err
	}
	if p.UserID != cmd.UserID {
		return nil, apierror.ErrNotAllowed
	}

	if cmd.Name != nil {
		p.Name = *cmd.Name
	}
	if cmd.AvatarURL != nil {
		p.AvatarURL = *cmd.AvatarURL
	}
	if cmd.PIN != nil {
		if *cmd.PIN == "" {
			p.PINHash = "" // caller explicitly removes the PIN
		} else {
			h, err := bcrypt.GenerateFromPassword([]byte(*cmd.PIN), bcrypt.DefaultCost)
			if err != nil {
				return nil, apierror.Internal("hash profile pin", err)
			}
			p.PINHash = string(h)
		}
	}
	if cmd.MaxRating != nil {
		if p.IsKids() {
			// Kids profiles can never exceed PG — compare by ordinal.
			pg := domain.MPAARatingPG
			if !p.AllowsRating(*cmd.MaxRating) || *cmd.MaxRating != pg {
				return nil, apierror.Validation("kids profiles cannot exceed PG rating", nil)
			}
		}
		p.MaxRating = cmd.MaxRating
	}
	if cmd.SortOrder != nil {
		p.SortOrder = *cmd.SortOrder
	}
	p.UpdatedAt = time.Now()

	updated, err := s.repo.Update(ctx, p)
	if err != nil {
		return nil, err
	}

	// SetDefault is handled separately via repo to keep the transaction atomic.
	if cmd.IsDefault != nil && *cmd.IsDefault && !updated.IsDefault {
		if err := s.repo.SetDefault(ctx, cmd.UserID, cmd.ProfileID); err != nil {
			return nil, err
		}
		updated.IsDefault = true
	}

	return updated, nil
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func (s *profileService) Delete(
	ctx context.Context, cmd domain.DeleteProfileCmd,
) error {
	profiles, err := s.repo.ListByUser(ctx, cmd.UserID)
	if err != nil {
		return err
	}
	if len(profiles) <= 1 {
		return apierror.New(409, "LAST_PROFILE",
			"cannot delete the last profile on an account")
	}

	var target *domain.Profile
	for _, p := range profiles {
		if p.ID == cmd.ProfileID {
			target = p
			break
		}
	}
	if target == nil {
		return apierror.ErrProfileNotFound
	}
	if target.IsDefault {
		return apierror.New(409, "DEFAULT_PROFILE",
			"set another profile as default before deleting this one")
	}

	return s.repo.Delete(ctx, cmd.UserID, cmd.ProfileID)
}

// ─── Switch ───────────────────────────────────────────────────────────────────

func (s *profileService) Switch(
	ctx context.Context, cmd domain.SwitchProfileCmd,
) (*domain.Profile, error) {
	p, err := s.repo.GetByID(ctx, cmd.ProfileID)
	if err != nil {
		return nil, err
	}
	if p.UserID != cmd.UserID {
		return nil, apierror.ErrNotAllowed
	}
	if p.PINHash != "" {
		if cmd.PIN == "" {
			return nil, apierror.New(403, "PIN_REQUIRED", "this profile requires a PIN")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(p.PINHash), []byte(cmd.PIN)); err != nil {
			return nil, apierror.New(403, "PIN_INVALID", "incorrect profile PIN")
		}
	}
	return p, nil
}

// ─── SetDefault ───────────────────────────────────────────────────────────────

func (s *profileService) SetDefault(
	ctx context.Context, userID domain.UserID, profileID domain.ProfileID,
) error {
	p, err := s.repo.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p.UserID != userID {
		return apierror.ErrNotAllowed
	}
	return s.repo.SetDefault(ctx, userID, profileID)
}

// ─── Validation ───────────────────────────────────────────────────────────────

func validateCreateProfile(cmd domain.CreateProfileCmd) error {
	if cmd.Name == "" {
		return apierror.Validation("profile name is required", nil)
	}
	if len(cmd.Name) > 50 {
		return apierror.Validation("profile name must be 50 characters or fewer", nil)
	}
	if cmd.Type != domain.ProfileTypeStandard && cmd.Type != domain.ProfileTypeKids {
		return apierror.Validation("type must be 'standard' or 'kids'", nil)
	}
	if cmd.PIN != "" && len(cmd.PIN) != 4 {
		return apierror.Validation("PIN must be exactly 4 digits", nil)
	}
	return nil
}
