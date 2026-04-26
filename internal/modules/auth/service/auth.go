package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"github.com/Ab-dex/view-aura/internal/events"
	"github.com/Ab-dex/view-aura/internal/modules/auth/domain"
	authrepo "github.com/Ab-dex/view-aura/internal/modules/auth/repository"
	notifservice "github.com/Ab-dex/view-aura/internal/modules/notification/service"
	userdomain "github.com/Ab-dex/view-aura/internal/modules/user/domain"
	userrepo "github.com/Ab-dex/view-aura/internal/modules/user/repository"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

const numRecoveryCodes = 8

// AuthService owns every operation that touches credentials, tokens, or sessions.
//
//go:generate mockgen -source=auth.go -destination=../mocks/auth_mock.go
type AuthService interface {
	// ── Credential-based auth ────────────────────────────────────────────────
	Register(ctx context.Context, cmd domain.RegisterCmd) (*userdomain.User, *domain.TokenPair, error)
	Login(ctx context.Context, cmd domain.LoginCmd) (*userdomain.User, *domain.TokenPair, error)
	RefreshTokens(ctx context.Context, refreshToken, deviceID, ip, ua string) (*domain.TokenPair, error)
	Logout(ctx context.Context, accessJTI, sessionID string, remainingTTL time.Duration) error
	LogoutAll(ctx context.Context, userID userdomain.UserID) error
	ChangePassword(ctx context.Context, cmd domain.ChangePasswordCmd) error

	// ── Email verification ───────────────────────────────────────────────────
	SendVerificationEmail(ctx context.Context, cmd domain.SendVerificationEmailCmd) error
	VerifyEmail(ctx context.Context, cmd domain.VerifyEmailCmd) error

	// ── Password reset ───────────────────────────────────────────────────────
	SendPasswordReset(ctx context.Context, cmd domain.SendPasswordResetCmd) error
	ResetPassword(ctx context.Context, cmd domain.ResetPasswordCmd) (*userdomain.User, *domain.TokenPair, error)

	// ── OAuth2 social login ──────────────────────────────────────────────────
	OAuthBegin(ctx context.Context, cmd domain.OAuthBeginCmd) (*domain.OAuthBeginResult, error)
	OAuthCallback(ctx context.Context, cmd domain.OAuthCallbackCmd) (*userdomain.User, *domain.TokenPair, error)

	// ── MFA ──────────────────────────────────────────────────────────────────
	EnrollTOTP(ctx context.Context, cmd domain.EnrollTOTPCmd) (*domain.TOTPEnrollmentResult, error)
	ConfirmTOTP(ctx context.Context, cmd domain.VerifyTOTPEnrollmentCmd) (recoveryCodes []string, err error)
	DisableMFA(ctx context.Context, cmd domain.DisableMFACmd) error
	VerifyMFA(ctx context.Context, cmd domain.VerifyMFACmd) (*userdomain.User, *domain.TokenPair, error)
	SendMFAEmailOTP(ctx context.Context, userID userdomain.UserID) error

	// ── Session management (exposed to user handler via thin delegation) ──────
	ListSessions(ctx context.Context, userID userdomain.UserID) ([]*domain.UserSession, error)
	RevokeSession(ctx context.Context, userID userdomain.UserID, sessionID string) error
}

// OAuthProvider is the interface each social provider adapter must satisfy.
type OAuthProvider interface {
	BuildAuthURL(state, pkceChallenge, redirectURI string) string
	ExchangeCode(ctx context.Context, code, pkceVerifier, redirectURI string) (*domain.OAuthUserInfo, error)
}

type authService struct {
	users          userrepo.UserRepository
	profiles       userrepo.ProfileRepository
	prefs          userrepo.PreferencesRepository
	authProviders  authrepo.AuthProviderRepository
	sessions       authrepo.SessionRepository
	security       authrepo.SecurityRepository
	verifyTokens   authrepo.VerificationTokenRepository
	oauthStates    authrepo.OAuthStateRepository
	mfa            authrepo.MFARepository
	oauthProviders map[domain.AuthProvider]OAuthProvider
	tokens         TokenService
	notifier       notifservice.NotificationSender
	pub            events.Producer
	appBaseURL     string
	jwt            config.JWTConfig
	tx             shareddb.TxManager
}

func NewAuthService(
	users userrepo.UserRepository,
	profiles userrepo.ProfileRepository,
	prefs userrepo.PreferencesRepository,
	authProviders authrepo.AuthProviderRepository,
	sessions authrepo.SessionRepository,
	security authrepo.SecurityRepository,
	verifyTokens authrepo.VerificationTokenRepository,
	oauthStates authrepo.OAuthStateRepository,
	mfa authrepo.MFARepository,
	oauthProviders map[domain.AuthProvider]OAuthProvider,
	tokens TokenService,
	notifier notifservice.NotificationSender,
	pub events.Producer,
	tx shareddb.TxManager,
	cfg *config.Config,
) AuthService {
	return &authService{
		users: users, profiles: profiles, prefs: prefs,
		authProviders: authProviders, sessions: sessions, security: security,
		verifyTokens: verifyTokens, oauthStates: oauthStates, mfa: mfa,
		oauthProviders: oauthProviders,
		tokens:         tokens, notifier: notifier, pub: pub,
		appBaseURL: cfg.Auth.BaseURL, jwt: cfg.JWT,
		tx: tx,
	}
}

// ─── registrationPublisher ────────────────────────────────────────────────────

// registrationPublisher tries Kafka first, then falls back to sending the
// verification email directly via the notifier when Kafka is unavailable.
//
// This mirrors the FallbackPublisher pattern: the user must always receive
// their verification email regardless of whether the event bus is reachable.
// When Kafka is healthy the notification-service consumer handles the email;
// when it is down we send it inline so the user is never left waiting.

type registrationPublisher struct {
	kafka      events.Producer
	notifier   notifservice.NotificationSender
	appBaseURL string
	verifyRepo authrepo.VerificationTokenRepository
}

func (p *registrationPublisher) publish(
	ctx context.Context,
	created *userdomain.User,
	rawToken string,
) {
	if created == nil {
		return // never panic in production flow
	}

	if p.notifier == nil {
		return // fail silently or log depending on your policy
	}

	log := logger.FromContext(ctx)

	verifyURL := fmt.Sprintf("%s/auth/verify-email?token=%s", p.appBaseURL, rawToken)

	// ── Attempt event publish (best effort) ─────────────────────
	err := p.kafka.Publish(ctx, events.TopicUserEvents, "user.registered", events.UserRegistered{
		UserID:       created.ID.String(),
		Email:        created.Email,
		DisplayName:  created.DisplayName,
		Provider:     "email",
		Locale:       created.Locale,
		Country:      created.Country,
		RegisteredAt: time.Now(),
	})

	if err == nil {
		// Kafka accepted event (best effort success)
		return
	}

	log.Warn().
		Err(err).
		Msg("kafka publish failed — sending direct email fallback")

	// ── Guaranteed email fallback ───────────────────────────────
	_ = p.notifier.SendEmail(
		ctx,
		created.Email,
		"Verify your ViewAura email",
		"email_verification",
		map[string]any{
			"verify_url":   verifyURL,
			"display_name": created.DisplayName,
		},
	)
}

// ─── Register ────────────────────────────────────────────────────────────────

func (s *authService) Register(ctx context.Context, cmd domain.RegisterCmd) (*userdomain.User, *domain.TokenPair, error) {
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

	hash, err := bcrypt.GenerateFromPassword([]byte(cmd.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, apierror.Internal("hash password", err)
	}

	user := &userdomain.User{
		ID:          userdomain.UserID(uuid.New().String()),
		Email:       strings.ToLower(strings.TrimSpace(cmd.Email)),
		DisplayName: cmd.DisplayName,
		Role:        userdomain.RoleUser,
		Status:      userdomain.StatusPending,
		Locale:      orDefault(cmd.Locale, "en"),
		Timezone:    "UTC",
		Country:     cmd.Country,
	}

	var created *userdomain.User

	err = s.tx.WithTx(ctx, func(ctx context.Context) error {

		var err error

		// 1. create user
		created, err = s.users.Create(ctx, user)
		if err != nil {
			return err
		}

		// 2. auth provider (password hash lives here)
		if err := s.authProviders.Link(ctx, &domain.LinkedAuthProvider{
			ID:          uuid.New().String(),
			UserID:      created.ID,
			Provider:    domain.ProviderEmail,
			ProviderID:  created.Email,
			AccessToken: strPtr(string(hash)),
		}); err != nil {
			return err
		}

		// 3. profile seed
		if _, err := s.profiles.Upsert(ctx, &userdomain.UserProfile{
			UserID:     created.ID,
			Visibility: userdomain.VisibilityPublic,
		}); err != nil {
			return err
		}

		// 4. preferences seed
		if _, err := s.prefs.Upsert(ctx, &userdomain.UserPreferences{
			UserID:             created.ID,
			DarkMode:           false,
			PreferredGenres:    []string{},
			DislikedGenres:     []string{},
			PreferredLanguages: []string{"eng"},
		}); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	// ── After commit — safe side effects ──────────────────────────────────────

	pair, err := s.tokens.Issue(ctx, created)
	if err != nil {
		return nil, nil, err
	}

	// Generate the verification token now so the fallback publisher can embed
	// it in the email when Kafka is unavailable.  The token is also stored in
	// the DB so VerifyEmail works regardless of which delivery path was used.
	rawToken := generateSecureToken(32)
	_ = s.verifyTokens.DeleteAllForUser(ctx, created.ID.String(), domain.PurposeEmailVerify)
	_ = s.verifyTokens.Create(ctx, &domain.VerificationToken{
		ID:        uuid.New().String(),
		UserID:    created.ID.String(),
		Purpose:   domain.PurposeEmailVerify,
		TokenHash: hashToken(rawToken),
		ExpiresAt: time.Now().Add(domain.TTL(domain.PurposeEmailVerify)),
		CreatedAt: time.Now(),
	})

	// Publish via the fallback publisher: Kafka → direct email (in that order).
	rp := &registrationPublisher{
		kafka:      s.pub,
		notifier:   s.notifier,
		appBaseURL: s.appBaseURL,
		verifyRepo: s.verifyTokens,
	}
	rp.publish(ctx, created, rawToken)

	log.Info().Str("user_id", created.ID.String()).Msg("auth: user registered")

	return created, pair, nil
}

// ─── Login ────────────────────────────────────────────────────────────────────

func (s *authService) Login(ctx context.Context, cmd domain.LoginCmd) (*userdomain.User, *domain.TokenPair, error) {
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

	ap, err := s.authProviders.GetByProvider(ctx, domain.ProviderEmail, user.Email)
	if err != nil || ap.AccessToken == nil {
		return nil, nil, apierror.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*ap.AccessToken), []byte(cmd.Password)); err != nil {
		if _, incErr := s.security.IncrementFailedLogins(ctx, user.ID); incErr != nil {
			log.Error().Err(incErr).Msg("auth: increment failed logins")
		}
		return nil, nil, apierror.ErrInvalidCredentials
	}

	_ = s.security.ResetFailedLogins(ctx, user.ID)
	_ = s.security.RecordLogin(ctx, user.ID, cmd.IPAddress)

	return s.issueSessionTokens(ctx, user, cmd.DeviceID, cmd.IPAddress, cmd.UserAgent)
}

// ─── Refresh ──────────────────────────────────────────────────────────────────

func (s *authService) RefreshTokens(ctx context.Context, refreshToken, deviceID, ip, ua string) (*domain.TokenPair, error) {
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
		// Token reuse — revoke all sessions (refresh token rotation attack).
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
	if _, err := s.sessions.Create(ctx, &domain.UserSession{
		ID:               newClaims.ID,
		UserID:           user.ID,
		DeviceID:         deviceID,
		IPAddress:        ip,
		UserAgent:        ua,
		RefreshTokenHash: HashToken(pair.RefreshToken),
		CreatedAt:        time.Now(),
		ExpiresAt:        time.Now().Add(s.jwt.RefreshTokenTTL),
	}); err != nil {
		return nil, err
	}
	return pair, nil
}

// ─── Logout ───────────────────────────────────────────────────────────────────

func (s *authService) Logout(ctx context.Context, accessJTI, sessionID string, remainingTTL time.Duration) error {
	if err := s.tokens.Invalidate(ctx, accessJTI, remainingTTL); err != nil {
		logger.FromContext(ctx).Error().Err(err).Msg("auth: blocklist JTI on logout")
	}
	return s.sessions.RevokeByID(ctx, sessionID)
}

func (s *authService) LogoutAll(ctx context.Context, userID userdomain.UserID) error {
	return s.sessions.RevokeAllForUser(ctx, userID)
}

// ─── Change password ──────────────────────────────────────────────────────────

func (s *authService) ChangePassword(ctx context.Context, cmd domain.ChangePasswordCmd) error {
	if err := validatePassword(cmd.NewPassword); err != nil {
		return err
	}
	user, err := s.users.GetByID(ctx, userdomain.UserID(cmd.UserID))
	if err != nil {
		return err
	}
	ap, err := s.authProviders.GetByProvider(ctx, domain.ProviderEmail, user.Email)
	if err != nil || ap.AccessToken == nil {
		return apierror.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*ap.AccessToken), []byte(cmd.OldPassword)); err != nil {
		return apierror.ErrInvalidCredentials
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(cmd.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return apierror.Internal("hash new password", err)
	}
	hashStr := string(newHash)
	ap.AccessToken = &hashStr
	if err := s.authProviders.Link(ctx, ap); err != nil {
		return apierror.Internal("update password hash", err)
	}
	return s.sessions.RevokeAllForUser(ctx, userdomain.UserID(cmd.UserID))
}

// ─── Email verification ───────────────────────────────────────────────────────

func (s *authService) SendVerificationEmail(ctx context.Context, cmd domain.SendVerificationEmailCmd) error {
	_ = s.verifyTokens.DeleteAllForUser(ctx, cmd.UserID, domain.PurposeEmailVerify)

	rawToken := generateSecureToken(32)
	if err := s.verifyTokens.Create(ctx, &domain.VerificationToken{
		ID:        uuid.New().String(),
		UserID:    cmd.UserID,
		Purpose:   domain.PurposeEmailVerify,
		TokenHash: hashToken(rawToken),
		ExpiresAt: time.Now().Add(domain.TTL(domain.PurposeEmailVerify)),
		CreatedAt: time.Now(),
		IPAddress: cmd.IPAddress,
		UserAgent: cmd.UserAgent,
	}); err != nil {
		return fmt.Errorf("auth: store verification token: %w", err)
	}

	verifyURL := fmt.Sprintf("%s/auth/verify-email?token=%s", s.appBaseURL, rawToken)
	_ = s.notifier.SendEmail(ctx, cmd.Email,
		"Verify your ViewAura email", "email_verification",
		map[string]any{"verify_url": verifyURL},
	)
	return nil
}

func (s *authService) VerifyEmail(ctx context.Context, cmd domain.VerifyEmailCmd) error {
	record, err := s.verifyTokens.GetByHash(ctx, hashToken(cmd.Token))
	if err != nil {
		return err
	}
	if !record.IsUsable() {
		return apierror.ErrTokenInvalid
	}
	if err := s.verifyTokens.MarkRedeemed(ctx, record.ID); err != nil {
		return err
	}

	user, err := s.users.GetByID(ctx, userdomain.UserID(record.UserID))
	if err != nil {
		return err
	}
	user.EmailVerified = true
	if user.Status == userdomain.StatusPending {
		user.Status = userdomain.StatusActive
	}
	if _, err := s.users.Update(ctx, user); err != nil {
		return fmt.Errorf("auth: activate user: %w", err)
	}
	_ = s.pub.Publish(ctx, events.TopicUserEvents, "user.email_verified", events.UserEmailVerified{
		UserID: user.ID.String(), Email: user.Email, VerifiedAt: time.Now(),
	})
	logger.FromContext(ctx).Info().Str("user_id", user.ID.String()).Msg("auth: email verified")
	return nil
}

// ─── Password reset ───────────────────────────────────────────────────────────

func (s *authService) SendPasswordReset(ctx context.Context, cmd domain.SendPasswordResetCmd) error {
	user, err := s.users.GetByEmail(ctx, cmd.Email)
	if err != nil {
		return nil // swallow — do not leak whether the email exists
	}
	_ = s.verifyTokens.DeleteAllForUser(ctx, user.ID.String(), domain.PurposePasswordReset)
	rawToken := generateSecureToken(32)
	if err := s.verifyTokens.Create(ctx, &domain.VerificationToken{
		ID:        uuid.New().String(),
		UserID:    user.ID.String(),
		Purpose:   domain.PurposePasswordReset,
		TokenHash: hashToken(rawToken),
		ExpiresAt: time.Now().Add(domain.TTL(domain.PurposePasswordReset)),
		CreatedAt: time.Now(),
		IPAddress: cmd.IPAddress,
		UserAgent: cmd.UserAgent,
	}); err != nil {
		return fmt.Errorf("auth: store reset token: %w", err)
	}
	resetURL := fmt.Sprintf("%s/auth/reset-password?token=%s", s.appBaseURL, rawToken)
	_ = s.notifier.SendEmail(ctx, cmd.Email,
		"Reset your ViewAura password", "password_reset",
		map[string]any{"reset_url": resetURL},
	)
	return nil
}

func (s *authService) ResetPassword(ctx context.Context, cmd domain.ResetPasswordCmd) (*userdomain.User, *domain.TokenPair, error) {
	record, err := s.verifyTokens.GetByHash(ctx, hashToken(cmd.Token))
	if err != nil {
		return nil, nil, err
	}
	if !record.IsUsable() {
		return nil, nil, apierror.ErrTokenInvalid
	}
	if err := s.verifyTokens.MarkRedeemed(ctx, record.ID); err != nil {
		return nil, nil, err
	}
	if err := validatePassword(cmd.NewPassword); err != nil {
		return nil, nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(cmd.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, apierror.Internal("hash password", err)
	}
	user, err := s.users.GetByID(ctx, userdomain.UserID(record.UserID))
	if err != nil {
		return nil, nil, err
	}
	ap, err := s.authProviders.GetByProvider(ctx, domain.ProviderEmail, user.Email)
	if err != nil {
		return nil, nil, err
	}
	hashStr := string(hash)
	ap.AccessToken = &hashStr
	if err := s.authProviders.Link(ctx, ap); err != nil {
		return nil, nil, fmt.Errorf("auth: update password hash: %w", err)
	}
	_ = s.sessions.RevokeAllForUser(ctx, user.ID)
	logger.FromContext(ctx).Info().Str("user_id", user.ID.String()).Msg("auth: password reset")
	return s.issueSessionTokens(ctx, user, "", cmd.IPAddress, "")
}

// ─── OAuth2 ───────────────────────────────────────────────────────────────────

func (s *authService) OAuthBegin(ctx context.Context, cmd domain.OAuthBeginCmd) (*domain.OAuthBeginResult, error) {
	prov, ok := s.oauthProviders[cmd.Provider]
	if !ok {
		return nil, apierror.New(400, "UNSUPPORTED_PROVIDER", fmt.Sprintf("%q is not configured", cmd.Provider))
	}
	verifier := generateSecureToken(32)
	state := generateSecureToken(16)
	if err := s.oauthStates.Save(ctx, &domain.OAuthState{
		State:        state,
		Provider:     cmd.Provider,
		RedirectURI:  cmd.RedirectURI,
		PKCEVerifier: verifier,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}); err != nil {
		return nil, fmt.Errorf("auth: store oauth state: %w", err)
	}
	return &domain.OAuthBeginResult{
		AuthorizationURL: prov.BuildAuthURL(state, pkceS256Challenge(verifier), cmd.RedirectURI),
		State:            state,
	}, nil
}

func (s *authService) OAuthCallback(ctx context.Context, cmd domain.OAuthCallbackCmd) (*userdomain.User, *domain.TokenPair, error) {
	oauthState, err := s.oauthStates.GetAndDelete(ctx, cmd.State)
	if err != nil {
		return nil, nil, fmt.Errorf("auth: retrieve oauth state: %w", err)
	}
	if oauthState == nil || oauthState.IsExpired() {
		return nil, nil, apierror.New(400, "OAUTH_STATE_INVALID", "OAuth flow expired — please try again")
	}
	if oauthState.Provider != cmd.Provider {
		return nil, nil, apierror.New(400, "OAUTH_PROVIDER_MISMATCH", "provider mismatch")
	}
	prov, ok := s.oauthProviders[cmd.Provider]
	if !ok {
		return nil, nil, apierror.New(400, "UNSUPPORTED_PROVIDER", fmt.Sprintf("%q is not configured", cmd.Provider))
	}
	userInfo, err := prov.ExchangeCode(ctx, cmd.Code, oauthState.PKCEVerifier, oauthState.RedirectURI)
	if err != nil {
		return nil, nil, fmt.Errorf("auth: oauth code exchange: %w", err)
	}
	user, err := s.upsertOAuthUser(ctx, userInfo)
	if err != nil {
		return nil, nil, err
	}
	return s.issueSessionTokens(ctx, user, cmd.DeviceID, cmd.IPAddress, cmd.UserAgent)
}

func (s *authService) upsertOAuthUser(ctx context.Context, info *domain.OAuthUserInfo) (*userdomain.User, error) {
	// Check if this provider+ID is already linked to an account.
	existing, err := s.authProviders.GetByProviderID(ctx, info.Provider, info.ProviderID)
	if err == nil && existing != nil {
		return s.users.GetByID(ctx, existing.UserID)
	}
	// Fall back to email match — link the new provider to an existing account.
	user, err := s.users.GetByEmail(ctx, info.Email)
	if err != nil {
		// No existing user — create a new one.
		user = &userdomain.User{
			ID:            userdomain.UserID(uuid.New().String()),
			Email:         info.Email,
			DisplayName:   info.DisplayName,
			EmailVerified: info.Verified,
			Status:        userdomain.StatusActive,
			Role:          userdomain.RoleUser,
			CreatedAt:     time.Now(),
		}
		if _, err := s.users.Create(ctx, user); err != nil {
			return nil, fmt.Errorf("auth: create oauth user: %w", err)
		}
		_, _ = s.profiles.Upsert(ctx, &userdomain.UserProfile{UserID: user.ID, Visibility: userdomain.VisibilityPublic})
		_, _ = s.prefs.Upsert(ctx, &userdomain.UserPreferences{UserID: user.ID, DarkMode: true})
	}
	if err := s.authProviders.Link(ctx, &domain.LinkedAuthProvider{
		ID:         uuid.New().String(),
		UserID:     user.ID,
		Provider:   info.Provider,
		ProviderID: info.ProviderID,
		AvatarURL:  info.AvatarURL,
	}); err != nil {
		return nil, fmt.Errorf("auth: link oauth provider: %w", err)
	}
	return user, nil
}

// ─── TOTP MFA ─────────────────────────────────────────────────────────────────

func (s *authService) EnrollTOTP(ctx context.Context, cmd domain.EnrollTOTPCmd) (*domain.TOTPEnrollmentResult, error) {
	secretBytes := make([]byte, 20)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("auth: generate totp secret: %w", err)
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secretBytes)

	user, err := s.users.GetByID(ctx, userdomain.UserID(cmd.UserID))
	if err != nil {
		return nil, err
	}
	enrollment := &domain.MFAEnrollment{
		ID:              uuid.New().String(),
		UserID:          cmd.UserID,
		Method:          domain.MFAMethodTOTP,
		EncryptedSecret: []byte(secret), // repository layer AES-encrypts before storing
	}
	if err := s.mfa.CreateEnrollment(ctx, enrollment); err != nil {
		return nil, fmt.Errorf("auth: store totp enrollment: %w", err)
	}
	qrURI := fmt.Sprintf(
		"otpauth://totp/ViewAura:%s?secret=%s&issuer=ViewAura&algorithm=SHA1&digits=6&period=30",
		user.Email, secret,
	)
	return &domain.TOTPEnrollmentResult{
		EnrollmentID:    enrollment.ID,
		Secret:          secret,
		ProvisioningURI: qrURI,
	}, nil
}

func (s *authService) ConfirmTOTP(ctx context.Context, cmd domain.VerifyTOTPEnrollmentCmd) ([]string, error) {
	enrollment, err := s.mfa.GetEnrollment(ctx, cmd.UserID)
	if err != nil {
		return nil, err
	}
	if enrollment == nil {
		return nil, apierror.New(400, "MFA_NOT_ENROLLED", "TOTP enrollment not started")
	}
	if enrollment.Verified {
		return nil, apierror.New(400, "MFA_ALREADY_ENABLED", "TOTP is already enabled")
	}
	if !totp.Validate(cmd.Code, string(enrollment.EncryptedSecret)) {
		return nil, apierror.New(401, "MFA_CODE_INVALID", "TOTP code is invalid or expired")
	}
	rawCodes := make([]string, numRecoveryCodes)
	hashedCodes := make([]string, numRecoveryCodes)
	for i := range rawCodes {
		rawCodes[i] = generateRecoveryCode()
		h, err := bcrypt.GenerateFromPassword([]byte(rawCodes[i]), bcrypt.MinCost)
		if err != nil {
			return nil, fmt.Errorf("auth: hash recovery code: %w", err)
		}
		hashedCodes[i] = string(h)
	}
	if err := s.mfa.MarkEnrollmentVerified(ctx, enrollment.ID); err != nil {
		return nil, fmt.Errorf("auth: verify totp enrollment: %w", err)
	}
	if err := s.mfa.SaveBackupCodes(ctx, enrollment.ID, hashedCodes); err != nil {
		return nil, fmt.Errorf("auth: save backup codes: %w", err)
	}
	logger.FromContext(ctx).Info().Str("user_id", cmd.UserID).Msg("auth: TOTP MFA enabled")
	return rawCodes, nil
}

func (s *authService) DisableMFA(ctx context.Context, cmd domain.DisableMFACmd) error {
	user, err := s.users.GetByID(ctx, userdomain.UserID(cmd.UserID))
	if err != nil {
		return err
	}
	ap, err := s.authProviders.GetByProvider(ctx, domain.ProviderEmail, user.Email)
	if err != nil || ap.AccessToken == nil {
		return apierror.ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*ap.AccessToken), []byte(cmd.Password)); err != nil {
		return apierror.ErrInvalidCredentials
	}
	if err := s.mfa.DeleteEnrollment(ctx, cmd.UserID); err != nil {
		return fmt.Errorf("auth: delete mfa enrollment: %w", err)
	}
	logger.FromContext(ctx).Info().Str("user_id", cmd.UserID).Msg("auth: MFA disabled")
	return nil
}

func (s *authService) VerifyMFA(ctx context.Context, cmd domain.VerifyMFACmd) (*userdomain.User, *domain.TokenPair, error) {
	enrollment, err := s.mfa.GetEnrollment(ctx, cmd.UserID)
	if err != nil {
		return nil, nil, err
	}
	if enrollment == nil || !enrollment.Verified {
		return nil, nil, apierror.ErrTwoFactorRequired
	}
	var valid bool
	switch cmd.Method {
	case domain.MFAMethodTOTP:
		if totp.Validate(cmd.Code, string(enrollment.EncryptedSecret)) {
			valid = true
		} else {
			// Fall back to backup code (hashed).
			valid, err = s.mfa.ConsumeBackupCode(ctx, enrollment.ID, hashToken(cmd.Code))
			if err != nil {
				return nil, nil, err
			}
		}
	case domain.MFAMethodEmail:
		record, err := s.verifyTokens.GetByHash(ctx, hashToken(cmd.Code))
		if err != nil || !record.IsUsable() || record.UserID != cmd.UserID {
			return nil, nil, apierror.New(401, "MFA_CODE_INVALID", "OTP is invalid or expired")
		}
		_ = s.verifyTokens.MarkRedeemed(ctx, record.ID)
		valid = true
	}
	if !valid {
		return nil, nil, apierror.New(401, "MFA_CODE_INVALID", "MFA code is invalid or expired")
	}
	go func() { _ = s.mfa.UpdateLastUsed(context.Background(), enrollment.ID) }()

	user, err := s.users.GetByID(ctx, userdomain.UserID(cmd.UserID))
	if err != nil {
		return nil, nil, err
	}
	return s.issueSessionTokens(ctx, user, cmd.DeviceID, cmd.IPAddress, cmd.UserAgent)
}

func (s *authService) SendMFAEmailOTP(ctx context.Context, userID userdomain.UserID) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	otp := generateNumericOTP(6)
	if err := s.verifyTokens.Create(ctx, &domain.VerificationToken{
		ID:        uuid.New().String(),
		UserID:    userID.String(),
		Purpose:   domain.PurposeMFAEmailOTP,
		TokenHash: hashToken(otp),
		ExpiresAt: time.Now().Add(domain.TTL(domain.PurposeMFAEmailOTP)),
		CreatedAt: time.Now(),
	}); err != nil {
		return fmt.Errorf("auth: store mfa otp: %w", err)
	}
	_ = s.notifier.SendEmail(ctx, user.Email, "Your ViewAura login code", "mfa_otp",
		map[string]any{"otp": otp})
	return nil
}

// ─── Session management ───────────────────────────────────────────────────────

func (s *authService) ListSessions(ctx context.Context, userID userdomain.UserID) ([]*domain.UserSession, error) {
	return s.sessions.ListByUser(ctx, userID)
}

func (s *authService) RevokeSession(ctx context.Context, userID userdomain.UserID, sessionID string) error {
	session, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session.UserID != userID {
		return apierror.Forbidden("cannot revoke another user's session")
	}
	return s.sessions.RevokeByID(ctx, sessionID)
}

// ─── Shared: issue tokens + create session ────────────────────────────────────

func (s *authService) issueSessionTokens(ctx context.Context, user *userdomain.User, deviceID, ip, ua string) (*userdomain.User, *domain.TokenPair, error) {
	pair, err := s.tokens.Issue(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	claims, err := s.tokens.ValidateRefresh(ctx, pair.RefreshToken)
	if err != nil {
		return nil, nil, err
	}
	if _, err := s.sessions.Create(ctx, &domain.UserSession{
		ID:               claims.ID,
		UserID:           user.ID,
		DeviceID:         deviceID,
		IPAddress:        ip,
		UserAgent:        ua,
		RefreshTokenHash: HashToken(pair.RefreshToken),
		CreatedAt:        time.Now(),
		ExpiresAt:        time.Now().Add(s.jwt.RefreshTokenTTL),
	}); err != nil {
		return nil, nil, err
	}
	_ = s.pub.Publish(ctx, events.TopicUserEvents, "user.logged_in", events.UserLoggedIn{
		UserID: user.ID.String(), DeviceID: deviceID, IPAddress: ip,
		UserAgent: ua, LoggedInAt: time.Now(),
	})
	return user, pair, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum)
}

// HashToken is exported so the middleware can hash refresh tokens for session lookup.
func HashToken(token string) string { return hashToken(token) }

func generateSecureToken(bytesLen int) string {
	b := make([]byte, bytesLen)
	_, _ = rand.Read(b)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}

func generateRecoveryCode() string {
	b := make([]byte, 9)
	_, _ = rand.Read(b)
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)[:12]
	return fmt.Sprintf("%s-%s-%s", enc[:4], enc[4:8], enc[8:])
}

func generateNumericOTP(digits int) string {
	const charset = "0123456789"
	buf := make([]byte, digits)
	rb := make([]byte, digits)
	_, _ = rand.Read(rb)
	for i, v := range rb {
		buf[i] = charset[int(v)%len(charset)]
	}
	return string(buf)
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
		return apierror.Validation("password must contain uppercase, lowercase, and a digit", nil)
	}
	return nil
}

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

	if len(strings.TrimSpace(cmd.FirstName)) < 2 {
		errs = append(errs, fieldErr{"first_name", "must be at least 2 characters"})
	}
	if len(strings.TrimSpace(cmd.LastName)) < 2 {
		errs = append(errs, fieldErr{"last_name", "must be at least 2 characters"})
	}
	if len(errs) > 0 {
		return apierror.Validation("registration input is invalid", errs)
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

func pkceS256Challenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func strPtr(s string) *string { return &s }
