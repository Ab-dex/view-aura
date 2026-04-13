package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Ab-dex/view-aura/internal/modules/user/domain"
	"github.com/Ab-dex/view-aura/internal/modules/user/repository"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// UserService defines all business operations exposed by the User module.
type UserService interface {
	Register(ctx context.Context, cmd domain.RegisterCmd) (*domain.User, *domain.TokenPair, error)
	Login(ctx context.Context, cmd domain.LoginCmd) (*domain.User, *domain.TokenPair, error)
	RefreshTokens(ctx context.Context, refreshToken, deviceID, ip, ua string) (*domain.TokenPair, error)
	Logout(ctx context.Context, accessJTI string, sessionID string, remainingTTL time.Duration) error
	LogoutAll(ctx context.Context, userID domain.UserID) error
	GetByID(ctx context.Context, id domain.UserID) (*domain.User, error)
	GetProfile(ctx context.Context, userID domain.UserID) (*domain.UserProfile, error)
	UpdateProfile(ctx context.Context, cmd domain.UpdateProfileCmd) (*domain.UserProfile, error)
	GetPreferences(ctx context.Context, userID domain.UserID) (*domain.UserPreferences, error)
	UpdatePreferences(ctx context.Context, cmd domain.UpdatePreferencesCmd) (*domain.UserPreferences, error)
	ChangePassword(ctx context.Context, userID domain.UserID, oldPassword, newPassword string) error
	SoftDelete(ctx context.Context, userID domain.UserID) error
	ListSessions(ctx context.Context, userID domain.UserID) ([]*domain.UserSession, error)
	RevokeSession(ctx context.Context, userID domain.UserID, sessionID string) error
}

// userService is the concrete implementation.
type userService struct {
	users         repository.UserRepository
	profiles      repository.ProfileRepository
	prefs         repository.PreferencesRepository
	security      repository.SecurityRepository
	sessions      repository.SessionRepository
	authProviders repository.AuthProviderRepository
	tokens        TokenService
	cfg           config.JWTConfig
}

// NewUserService constructs a fully wired UserService. Wire calls this.
func NewUserService(
	users repository.UserRepository,
	profiles repository.ProfileRepository,
	prefs repository.PreferencesRepository,
	security repository.SecurityRepository,
	sessions repository.SessionRepository,
	authProviders repository.AuthProviderRepository,
	tokens TokenService,
	cfg *config.Config,
) UserService {
	return &userService{
		users:         users,
		profiles:      profiles,
		prefs:         prefs,
		security:      security,
		sessions:      sessions,
		authProviders: authProviders,
		tokens:        tokens,
		cfg:           cfg.JWT,
	}
}

// ─── Register ────────────────────────────────────────────────────────────────

func (s *userService) Register(ctx context.Context, cmd domain.RegisterCmd) (*domain.User, *domain.TokenPair, error) {
	log := logger.FromContext(ctx)

	if err := validateRegister(cmd); err != nil {
		return nil, nil, err
	}

	exists, err := s.users.ExistsByEmail(ctx, cmd.Email)
	if err != nil {
		return nil, nil, err
	}
	if exists {
		return nil, nil, apierror.ErrEmailAlreadyExists
	}

	// FIX #4: hash is now stored in the auth provider record instead of being silently discarded.
	hash, err := bcrypt.GenerateFromPassword([]byte(cmd.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, apierror.Internal("hash password", err)
	}

	user := &domain.User{
		ID:           domain.UserID(uuid.New().String()),
		Email:        strings.ToLower(strings.TrimSpace(cmd.Email)),
		DisplayName:  cmd.DisplayName,
		Role:         domain.RoleUser,
		Status:       domain.StatusPending,
		AuthProvider: domain.ProviderEmail,
		Locale:       orDefault(cmd.Locale, "en"),
		Timezone:     "UTC",
		Country:      cmd.Country,
	}

	created, err := s.users.Create(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	// Store the password hash in user_auth_providers (the correct location — not on the user row).
	authProvider := &domain.LinkedAuthProvider{
		ID:          uuid.New().String(),
		UserID:      created.ID,
		Provider:    domain.ProviderEmail,
		ProviderID:  created.Email,
		AccessToken: strPtr(string(hash)),
	}
	if err := s.authProviders.Link(ctx, authProvider); err != nil {
		// Non-fatal for the scaffold but should be treated as fatal in production.
		log.Error().Err(err).Msg("failed to store auth provider record")
	}

	_, _ = s.profiles.Upsert(ctx, &domain.UserProfile{
		UserID:     created.ID,
		Visibility: domain.VisibilityPublic,
	})
	_, _ = s.prefs.Upsert(ctx, &domain.UserPreferences{
		UserID:   created.ID,
		DarkMode: true,
	})

	pair, err := s.tokens.Issue(ctx, created)
	if err != nil {
		return nil, nil, err
	}

	log.Info().Str("user_id", created.ID.String()).Msg("user registered")
	return created, pair, nil
}

// ─── Login ────────────────────────────────────────────────────────────────────

func (s *userService) Login(ctx context.Context, cmd domain.LoginCmd) (*domain.User, *domain.TokenPair, error) {
	log := logger.FromContext(ctx)

	user, err := s.users.GetByEmail(ctx, cmd.Email)
	if err != nil {
		if apierror.ErrUserNotFound.Is(err) {
			return nil, nil, apierror.ErrInvalidCredentials
		}
		return nil, nil, err
	}

	if user.IsSuspended() {
		return nil, nil, apierror.ErrAccountSuspended
	}
	if user.IsDeleted {
		return nil, nil, apierror.ErrUserNotFound
	}

	sec, err := s.security.GetByUserID(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	if sec.IsLocked() {
		return nil, nil, apierror.ErrAccountLocked.WithCause(
			fmt.Errorf("locked until %s", sec.LockedUntil),
		)
	}

	// FIX #4: fetch the stored hash from user_auth_providers and verify with bcrypt.
	authProvider, err := s.authProviders.GetByProvider(ctx, domain.ProviderEmail, user.Email)
	if err != nil {
		return nil, nil, apierror.ErrInvalidCredentials
	}
	if authProvider.AccessToken == nil {
		return nil, nil, apierror.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*authProvider.AccessToken), []byte(cmd.Password)); err != nil {
		if _, incErr := s.security.IncrementFailedLogins(ctx, user.ID); incErr != nil {
			log.Error().Err(incErr).Msg("failed to increment failed logins")
		}
		return nil, nil, apierror.ErrInvalidCredentials
	}

	if err := s.security.ResetFailedLogins(ctx, user.ID); err != nil {
		log.Error().Err(err).Msg("failed to reset failed logins")
	}
	if err := s.security.RecordLogin(ctx, user.ID, cmd.IPAddress); err != nil {
		log.Error().Err(err).Msg("failed to record login")
	}

	pair, err := s.tokens.Issue(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	refreshClaims, err := s.tokens.ValidateRefresh(ctx, pair.RefreshToken)
	if err != nil {
		return nil, nil, err
	}

	session := &domain.UserSession{
		ID:               refreshClaims.ID,
		UserID:           user.ID,
		DeviceID:         cmd.DeviceID,
		IPAddress:        cmd.IPAddress,
		UserAgent:        cmd.UserAgent,
		RefreshTokenHash: HashToken(pair.RefreshToken),
		CreatedAt:        time.Now(),
		ExpiresAt:        time.Now().Add(s.tokenRefreshTTL()),
	}
	if _, err := s.sessions.Create(ctx, session); err != nil {
		return nil, nil, err
	}

	log.Info().Str("user_id", user.ID.String()).Str("device_id", cmd.DeviceID).Msg("user logged in")
	return user, pair, nil
}

// ─── Refresh tokens ───────────────────────────────────────────────────────────

func (s *userService) RefreshTokens(ctx context.Context, refreshToken, deviceID, ip, ua string) (*domain.TokenPair, error) {
	session, err := s.sessions.GetByID(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	if session.IsRevoked() {
		return nil, apierror.ErrSessionRevoked
	}
	if session.IsExpired() {
		return nil, apierror.ErrSessionExpired
	}

	if session.RefreshTokenHash != HashToken(refreshToken) {
		_ = s.sessions.RevokeAllForUser(ctx, session.UserID)
		return nil, apierror.ErrTokenInvalid
	}

	user, err := s.users.GetByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	if err := s.sessions.RevokeByID(ctx, session.ID); err != nil {
		return nil, err
	}

	pair, err := s.tokens.Issue(ctx, user)
	if err != nil {
		return nil, err
	}

	newClaims, err := s.tokens.ValidateRefresh(ctx, pair.RefreshToken)
	if err != nil {
		return nil, err
	}

	newSession := &domain.UserSession{
		ID:               newClaims.ID,
		UserID:           user.ID,
		DeviceID:         deviceID,
		IPAddress:        ip,
		UserAgent:        ua,
		RefreshTokenHash: HashToken(pair.RefreshToken),
		CreatedAt:        time.Now(),
		ExpiresAt:        time.Now().Add(s.tokenRefreshTTL()),
	}
	if _, err := s.sessions.Create(ctx, newSession); err != nil {
		return nil, err
	}

	return pair, nil
}

// ─── Logout ───────────────────────────────────────────────────────────────────

func (s *userService) Logout(ctx context.Context, accessJTI string, sessionID string, remainingTTL time.Duration) error {
	if err := s.tokens.Invalidate(ctx, accessJTI, remainingTTL); err != nil {
		logger.FromContext(ctx).Error().Err(err).Msg("failed to blocklist JTI on logout")
	}
	return s.sessions.RevokeByID(ctx, sessionID)
}

func (s *userService) LogoutAll(ctx context.Context, userID domain.UserID) error {
	return s.sessions.RevokeAllForUser(ctx, userID)
}

// ─── Profile / Preferences ────────────────────────────────────────────────────

func (s *userService) GetByID(ctx context.Context, id domain.UserID) (*domain.User, error) {
	return s.users.GetByID(ctx, id)
}

func (s *userService) GetProfile(ctx context.Context, userID domain.UserID) (*domain.UserProfile, error) {
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return nil, err
	}
	return s.profiles.GetByUserID(ctx, userID)
}

func (s *userService) UpdateProfile(ctx context.Context, cmd domain.UpdateProfileCmd) (*domain.UserProfile, error) {
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

// ─── Password / deletion ──────────────────────────────────────────────────────

func (s *userService) ChangePassword(ctx context.Context, userID domain.UserID, oldPassword, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}

	// FIX #4: fetch current hash, verify old password, store new hash.
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	authProvider, err := s.authProviders.GetByProvider(ctx, domain.ProviderEmail, user.Email)
	if err != nil {
		return apierror.ErrInvalidCredentials
	}
	if authProvider.AccessToken == nil {
		return apierror.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*authProvider.AccessToken), []byte(oldPassword)); err != nil {
		return apierror.ErrInvalidCredentials
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return apierror.Internal("hash new password", err)
	}
	hashStr := string(newHash)
	authProvider.AccessToken = &hashStr

	if err := s.authProviders.Link(ctx, authProvider); err != nil {
		return apierror.Internal("update password hash", err)
	}

	return s.sessions.RevokeAllForUser(ctx, userID)
}

func (s *userService) SoftDelete(ctx context.Context, userID domain.UserID) error {
	if err := s.sessions.RevokeAllForUser(ctx, userID); err != nil {
		logger.FromContext(ctx).Error().Err(err).Msg("revoke sessions on delete")
	}
	return s.users.SoftDelete(ctx, userID)
}

func (s *userService) ListSessions(ctx context.Context, userID domain.UserID) ([]*domain.UserSession, error) {
	return s.sessions.ListByUser(ctx, userID)
}

func (s *userService) RevokeSession(ctx context.Context, userID domain.UserID, sessionID string) error {
	session, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session.UserID != userID {
		return apierror.Forbidden("cannot revoke another user's session")
	}
	return s.sessions.RevokeByID(ctx, sessionID)
}

// ─── Validation helpers ───────────────────────────────────────────────────────

func validateRegister(cmd domain.RegisterCmd) error {
	type fieldErr struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}
	var errs []fieldErr

	if !isValidEmail(cmd.Email) {
		errs = append(errs, fieldErr{"email", "must be a valid email address"})
	}
	if err := validatePassword(cmd.Password); err != nil {
		if ae, ok := apierror.As(err); ok {
			errs = append(errs, fieldErr{"password", ae.Message})
		}
	}
	if len(strings.TrimSpace(cmd.DisplayName)) < 2 {
		errs = append(errs, fieldErr{"display_name", "must be at least 2 characters"})
	}

	if len(errs) > 0 {
		return apierror.Validation("registration input is invalid", errs)
	}
	return nil
}

func validatePassword(p string) error {
	if len(p) < 8 {
		return apierror.Validation("password must be at least 8 characters", nil)
	}
	var hasUpper, hasLower, hasDigit bool
	for _, r := range p {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return apierror.Validation("password must contain upper, lower case and a digit", nil)
	}
	return nil
}

func isValidEmail(e string) bool {
	at := strings.LastIndex(e, "@")
	if at < 1 || at >= len(e)-1 {
		return false
	}
	dot := strings.LastIndex(e[at:], ".")
	return dot > 1 && at+dot < len(e)-1
}

func applyIfSet(src *string, dst *string) {
	if src != nil {
		*dst = *src
	}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func strPtr(s string) *string { return &s }

// FIX #5: read TTL from injected config instead of a hardcoded constant.
func (s *userService) tokenRefreshTTL() time.Duration {
	return s.cfg.RefreshTokenTTL
}
