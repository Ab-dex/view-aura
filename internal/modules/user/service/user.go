package service

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/user/domain"
	"github.com/Ab-dex/view-aura/internal/modules/user/repository"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

// UserService owns only user identity and preference operations.
// Auth operations (Register, Login, RefreshTokens, Logout, ChangePassword)
// have been moved to auth/service.AuthService where they belong.
type UserService interface {
	GetByID(ctx context.Context, id domain.UserID) (*domain.User, error)
	GetAccountDetails(ctx context.Context, userID domain.UserID) (*domain.UserProfile, error)
	UpdateAccountDetails(ctx context.Context, cmd domain.UpdateProfileCmd) (*domain.UserProfile, error)
	GetPreferences(ctx context.Context, userID domain.UserID) (*domain.UserPreferences, error)
	UpdatePreferences(ctx context.Context, cmd domain.UpdatePreferencesCmd) (*domain.UserPreferences, error)
	SoftDelete(ctx context.Context, userID domain.UserID) error
}

type userService struct {
	users    repository.UserRepository
	profiles repository.ProfileRepository
	prefs    repository.PreferencesRepository
}

func NewUserService(
	users repository.UserRepository,
	profiles repository.ProfileRepository,
	prefs repository.PreferencesRepository,
) UserService {
	return &userService{users: users, profiles: profiles, prefs: prefs}
}

func (s *userService) GetByID(ctx context.Context, id domain.UserID) (*domain.User, error) {
	return s.users.GetByID(ctx, id)
}

func (s *userService) GetAccountDetails(ctx context.Context, userID domain.UserID) (*domain.UserProfile, error) {
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return nil, err
	}
	return s.profiles.GetByUserID(ctx, userID)
}

func (s *userService) UpdateAccountDetails(ctx context.Context, cmd domain.UpdateProfileCmd) (*domain.UserProfile, error) {
	existing, err := s.profiles.GetByUserID(ctx, cmd.UserID)
	if err != nil {
		return nil, err
	}
	applyIfSet(cmd.AvatarURL, &existing.AvatarURL)
	applyIfSet(cmd.BannerURL, &existing.BannerURL)
	applyIfSet(cmd.Bio, &existing.Bio)
	applyIfSet(cmd.Website, &existing.Website)
	if cmd.Birthdate != nil {
		existing.Birthdate = cmd.Birthdate
	}
	applyIfSet(cmd.Gender, &existing.Gender)
	if cmd.Visibility != nil {
		existing.Visibility = domain.Visibility(*cmd.Visibility)
	}
	return s.profiles.Upsert(ctx, existing)
}

func (s *userService) GetPreferences(ctx context.Context, userID domain.UserID) (*domain.UserPreferences, error) {
	return s.prefs.GetByUserID(ctx, userID)
}

func (s *userService) UpdatePreferences(ctx context.Context, cmd domain.UpdatePreferencesCmd) (*domain.UserPreferences, error) {
	existing, err := s.prefs.GetByUserID(ctx, cmd.UserID)
	if err != nil {
		return nil, err
	}
	if cmd.PreferredGenres != nil {
		existing.PreferredGenres = *cmd.PreferredGenres
	}
	if cmd.DislikedGenres != nil {
		existing.DislikedGenres = *cmd.DislikedGenres
	}
	if cmd.PreferredLanguages != nil {
		existing.PreferredLanguages = *cmd.PreferredLanguages
	}
	if cmd.AdultContent != nil {
		existing.AdultContent = *cmd.AdultContent
	}
	if cmd.DarkMode != nil {
		existing.DarkMode = *cmd.DarkMode
	}
	if cmd.AutoplayTrailers != nil {
		existing.AutoplayTrailers = *cmd.AutoplayTrailers
	}
	return s.prefs.Upsert(ctx, existing)
}

// SoftDelete removes the user's identity record.
// Session revocation is handled by auth.AuthService.LogoutAll — the caller
// (handler) must invoke both when the user deletes their account.
func (s *userService) SoftDelete(ctx context.Context, userID domain.UserID) error {
	if err := s.users.SoftDelete(ctx, userID); err != nil {
		return apierror.Internal("soft delete user", err)
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func applyIfSet(src *string, dst *string) {
	if src != nil {
		*dst = *src
	}
}
