package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ab-dex/view-aura/internal/modules/user/domain"
	"github.com/Ab-dex/view-aura/internal/modules/user/repository"
	"github.com/Ab-dex/view-aura/internal/modules/user/service"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Stub token service ───────────────────────────────────────────────────────

type stubTokenService struct {
	invalidated []string
	failIssue   bool
}

func (s *stubTokenService) Issue(_ context.Context, u *domain.User) (*domain.TokenPair, error) {
	if s.failIssue {
		return nil, errors.New("token service unavailable")
	}
	return &domain.TokenPair{
		AccessToken:  "access." + u.ID.String(),
		RefreshToken: "refresh." + uuid.New().String(),
		ExpiresIn:    3600,
	}, nil
}

func (s *stubTokenService) ValidateAccess(_ context.Context, _ string) (*service.Claims, error) {
	return &service.Claims{Role: "user", TokenType: "access"}, nil
}

// FIX: ValidateRefresh must return a Claims with a non-empty ID (the JTI).
// Login and RefreshTokens both use refreshClaims.ID as the session ID; an empty
// string causes sessions.Create to store an un-lookupable "" key.
func (s *stubTokenService) ValidateRefresh(_ context.Context, _ string) (*service.Claims, error) {
	c := &service.Claims{TokenType: "refresh"}
	c.ID = uuid.New().String() // jwt.RegisteredClaims.ID — used as session ID
	return c, nil
}

func (s *stubTokenService) Invalidate(_ context.Context, jti string, _ time.Duration) error {
	s.invalidated = append(s.invalidated, jti)
	return nil
}

// ─── Test harness ─────────────────────────────────────────────────────────────

type harness struct {
	users         *inMemUserRepo
	profiles      *inMemProfileRepo
	prefs         *inMemPrefsRepo
	security      *inMemSecurityRepo
	sessions      *inMemSessionRepo
	authProviders *inMemAuthProviderRepo // FIX: was missing — needed by Register/Login/ChangePassword
	tokens        *stubTokenService
}

func newHarness() *harness {
	return &harness{
		users:         newInMemUserRepo(),
		profiles:      newInMemProfileRepo(),
		prefs:         newInMemPrefsRepo(),
		security:      newInMemSecurityRepo(),
		sessions:      newInMemSessionRepo(),
		authProviders: newInMemAuthProviderRepo(),
		tokens:        &stubTokenService{},
	}
}

// testConfig returns a minimal config sufficient for the service layer in tests.
func testConfig() *config.Config {
	return &config.Config{
		JWT: config.JWTConfig{
			RefreshTokenTTL: 30 * 24 * time.Hour,
			AccessTokenTTL:  15 * time.Minute,
			Issuer:          "test",
		},
	}
}

func (h *harness) svc() service.UserService {
	// FIX: pass authProviders and cfg — NewUserService now requires both.
	return service.NewUserService(
		h.users, h.profiles, h.prefs, h.security, h.sessions,
		h.authProviders,
		h.tokens,
		testConfig(),
	)
}

func validRegister() domain.RegisterCmd {
	return domain.RegisterCmd{
		Email:       "alice@example.com",
		Password:    "SecurePass1",
		DisplayName: "Alice Test",
		Locale:      "en",
		Country:     "NG",
	}
}

// ─── Register ─────────────────────────────────────────────────────────────────

func TestRegister_Success(t *testing.T) {
	h := newHarness()
	user, pair, err := h.svc().Register(context.Background(), validRegister())

	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)
	assert.Equal(t, "alice@example.com", user.Email)
	assert.Equal(t, domain.RoleUser, user.Role)
	assert.Equal(t, domain.StatusPending, user.Status)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.Equal(t, int64(3600), pair.ExpiresIn)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	h := newHarness()
	svc := h.svc()

	_, _, err := svc.Register(context.Background(), validRegister())
	require.NoError(t, err)

	_, _, err = svc.Register(context.Background(), validRegister())
	require.Error(t, err)
	assert.True(t, apierror.IsCode(err, apierror.CodeEmailAlreadyExists), "got %v", err)
}

func TestRegister_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		cmd  domain.RegisterCmd
	}{
		{"bad_email", domain.RegisterCmd{Email: "not-valid", Password: "SecurePass1", DisplayName: "Al"}},
		{"short_password", domain.RegisterCmd{Email: "a@b.com", Password: "short", DisplayName: "Al"}},
		{"no_uppercase", domain.RegisterCmd{Email: "a@b.com", Password: "password1", DisplayName: "Al"}},
		{"no_digit", domain.RegisterCmd{Email: "a@b.com", Password: "Password", DisplayName: "Al"}},
		{"short_display_name", domain.RegisterCmd{Email: "a@b.com", Password: "SecurePass1", DisplayName: "A"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness()
			_, _, err := h.svc().Register(context.Background(), tc.cmd)
			require.Error(t, err)
			assert.True(t, apierror.IsCode(err, apierror.CodeValidation), "got %v", err)
		})
	}
}

func TestRegister_BootstrapsProfileAndPrefs(t *testing.T) {
	h := newHarness()
	svc := h.svc()

	user, _, err := svc.Register(context.Background(), validRegister())
	require.NoError(t, err)

	profile, err := svc.GetProfile(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.VisibilityPublic, profile.Visibility)

	prefs, err := svc.GetPreferences(context.Background(), user.ID)
	require.NoError(t, err)
	assert.True(t, prefs.DarkMode)
}

// ─── Login ────────────────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	h := newHarness()
	svc := h.svc()

	reg, _, err := svc.Register(context.Background(), validRegister())
	require.NoError(t, err)

	// Activate the account so login is permitted.
	reg.Status = domain.StatusActive
	_, _ = h.users.Update(context.Background(), reg)

	user, pair, err := svc.Login(context.Background(), domain.LoginCmd{
		Email:     "alice@example.com",
		Password:  "SecurePass1",
		DeviceID:  "dev-001",
		IPAddress: "10.0.0.1",
		UserAgent: "TestAgent/1.0",
	})

	require.NoError(t, err)
	assert.Equal(t, reg.ID, user.ID)
	assert.NotEmpty(t, pair.AccessToken)
}

func TestLogin_UnknownEmail_ReturnsInvalidCredentials(t *testing.T) {
	h := newHarness()
	_, _, err := h.svc().Login(context.Background(), domain.LoginCmd{
		Email:    "ghost@example.com",
		Password: "Whatever1",
	})
	require.Error(t, err)
	// Must never return CodeUserNotFound — that would leak user existence.
	assert.True(t, apierror.IsCode(err, apierror.CodeInvalidCredentials), "got %v", err)
}

func TestLogin_WrongPassword_ReturnsInvalidCredentials(t *testing.T) {
	h := newHarness()
	svc := h.svc()

	reg, _, err := svc.Register(context.Background(), validRegister())
	require.NoError(t, err)
	reg.Status = domain.StatusActive
	_, _ = h.users.Update(context.Background(), reg)

	_, _, err = svc.Login(context.Background(), domain.LoginCmd{
		Email:    "alice@example.com",
		Password: "WrongPass1",
	})
	require.Error(t, err)
	assert.True(t, apierror.IsCode(err, apierror.CodeInvalidCredentials), "got %v", err)
}

func TestLogin_SuspendedAccount(t *testing.T) {
	h := newHarness()
	svc := h.svc()

	reg, _, _ := svc.Register(context.Background(), validRegister())
	reg.Status = domain.StatusSuspended
	_, _ = h.users.Update(context.Background(), reg)

	_, _, err := svc.Login(context.Background(), domain.LoginCmd{
		Email:    "alice@example.com",
		Password: "SecurePass1",
	})
	require.Error(t, err)
	assert.True(t, apierror.IsCode(err, apierror.CodeAccountSuspended))
}

func TestLogin_LockedAccount(t *testing.T) {
	h := newHarness()
	svc := h.svc()

	reg, _, _ := svc.Register(context.Background(), validRegister())
	reg.Status = domain.StatusActive
	_, _ = h.users.Update(context.Background(), reg)

	lockUntil := time.Now().Add(15 * time.Minute)
	_ = h.security.LockUntil(context.Background(), reg.ID, lockUntil)

	_, _, err := svc.Login(context.Background(), domain.LoginCmd{
		Email:    "alice@example.com",
		Password: "SecurePass1",
	})
	require.Error(t, err)
	assert.True(t, apierror.IsCode(err, apierror.CodeAccountLocked))
}

func TestLogin_BadPassword_IncrementsCounter(t *testing.T) {
	h := newHarness()
	svc := h.svc()

	reg, _, _ := svc.Register(context.Background(), validRegister())
	reg.Status = domain.StatusActive
	_, _ = h.users.Update(context.Background(), reg)

	for i := 0; i < 3; i++ {
		_, _, _ = svc.Login(context.Background(), domain.LoginCmd{
			Email:    "alice@example.com",
			Password: "WrongPass1", // wrong but valid-format password
		})
	}

	sec, _ := h.security.GetByUserID(context.Background(), reg.ID)
	assert.Equal(t, 3, sec.FailedLoginCount)
}

// ─── Logout ───────────────────────────────────────────────────────────────────

func TestLogout_RevokesSession(t *testing.T) {
	h := newHarness()
	svc := h.svc()

	sid := uuid.New().String()
	uid := domain.UserID(uuid.New().String())
	_, _ = h.sessions.Create(context.Background(), &domain.UserSession{
		ID:        sid,
		UserID:    uid,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	})

	err := svc.Logout(context.Background(), "jti-xyz", sid, 30*time.Minute)
	require.NoError(t, err)

	sess, _ := h.sessions.GetByID(context.Background(), sid)
	assert.True(t, sess.IsRevoked())
}

func TestLogoutAll_ClearsAllSessions(t *testing.T) {
	h := newHarness()
	svc := h.svc()
	uid := domain.UserID(uuid.New().String())

	for i := 0; i < 4; i++ {
		_, _ = h.sessions.Create(context.Background(), &domain.UserSession{
			ID:        uuid.New().String(),
			UserID:    uid,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(time.Hour),
		})
	}

	err := svc.LogoutAll(context.Background(), uid)
	require.NoError(t, err)

	active, _ := svc.ListSessions(context.Background(), uid)
	assert.Empty(t, active)
}

// ─── Profile ──────────────────────────────────────────────────────────────────

func TestUpdateProfile_PartialUpdate(t *testing.T) {
	h := newHarness()
	svc := h.svc()

	user, _, _ := svc.Register(context.Background(), validRegister())

	bio := "Film critic based in Lagos"
	profile, err := svc.UpdateProfile(context.Background(), domain.UpdateProfileCmd{
		UserID: user.ID,
		Bio:    &bio,
	})
	require.NoError(t, err)
	assert.Equal(t, bio, profile.Bio)
	assert.Equal(t, domain.VisibilityPublic, profile.Visibility) // unchanged
}

// ─── Preferences ──────────────────────────────────────────────────────────────

func TestUpdatePreferences_PartialUpdate(t *testing.T) {
	h := newHarness()
	svc := h.svc()

	user, _, _ := svc.Register(context.Background(), validRegister())

	genres := []string{"sci-fi", "noir"}
	dark := false
	prefs, err := svc.UpdatePreferences(context.Background(), domain.UpdatePreferencesCmd{
		UserID:          user.ID,
		PreferredGenres: &genres,
		DarkMode:        &dark,
	})
	require.NoError(t, err)
	assert.Equal(t, genres, prefs.PreferredGenres)
	assert.False(t, prefs.DarkMode)
	assert.True(t, prefs.AutoplayTrailers) // untouched
}

// ─── SoftDelete ───────────────────────────────────────────────────────────────

func TestSoftDelete_HidesUser(t *testing.T) {
	h := newHarness()
	svc := h.svc()

	user, _, _ := svc.Register(context.Background(), validRegister())

	err := svc.SoftDelete(context.Background(), user.ID)
	require.NoError(t, err)

	_, err = svc.GetByID(context.Background(), user.ID)
	require.Error(t, err)
	assert.True(t, apierror.IsCode(err, apierror.CodeUserNotFound))
}

func TestSoftDelete_MissingUser(t *testing.T) {
	h := newHarness()
	err := h.svc().SoftDelete(context.Background(), domain.UserID("missing"))
	require.Error(t, err)
	assert.True(t, apierror.IsCode(err, apierror.CodeUserNotFound))
}

// ─── Sessions ────────────────────────────────────────────────────────────────

func TestRevokeSession_OwnerOnly(t *testing.T) {
	h := newHarness()
	svc := h.svc()

	owner := domain.UserID(uuid.New().String())
	other := domain.UserID(uuid.New().String())
	sid := uuid.New().String()

	_, _ = h.sessions.Create(context.Background(), &domain.UserSession{
		ID:        sid,
		UserID:    owner,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	})

	// Stranger should be forbidden.
	err := svc.RevokeSession(context.Background(), other, sid)
	require.Error(t, err)
	assert.True(t, apierror.IsCode(err, apierror.CodeForbidden))

	// Owner should succeed.
	err = svc.RevokeSession(context.Background(), owner, sid)
	require.NoError(t, err)
}

// ─── apierror unit tests ──────────────────────────────────────────────────────

func TestAPIError_Sentinels(t *testing.T) {
	assert.True(t, errors.Is(apierror.ErrUserNotFound, apierror.ErrUserNotFound))
	assert.False(t, errors.Is(apierror.ErrUserNotFound, apierror.ErrEmailAlreadyExists))
}

func TestAPIError_WithCause_ChainPreserved(t *testing.T) {
	root := errors.New("db timeout")
	wrapped := apierror.ErrUserNotFound.WithCause(root)

	assert.True(t, apierror.IsCode(wrapped, apierror.CodeUserNotFound))
	assert.True(t, errors.Is(wrapped, root))
}

func TestAPIError_WithDetails(t *testing.T) {
	type fe struct {
		Field   string
		Message string
	}
	d := fe{"email", "invalid"}
	err := apierror.Validation("bad input", d)
	ae, ok := apierror.As(err)
	require.True(t, ok)
	assert.Equal(t, apierror.CodeValidation, ae.Code)
	assert.Equal(t, d, ae.Details)
}

// ─── In-memory repository implementations ────────────────────────────────────

// inMemUserRepo ───────────────────────────────────────────────────────────────

type inMemUserRepo struct {
	byID    map[domain.UserID]*domain.User
	byEmail map[string]*domain.User
}

func newInMemUserRepo() *inMemUserRepo {
	return &inMemUserRepo{byID: make(map[domain.UserID]*domain.User), byEmail: make(map[string]*domain.User)}
}

var _ repository.UserRepository = (*inMemUserRepo)(nil)

func (r *inMemUserRepo) Create(_ context.Context, u *domain.User) (*domain.User, error) {
	if _, e := r.byEmail[u.Email]; e {
		return nil, apierror.ErrEmailAlreadyExists
	}
	c := *u
	c.CreatedAt, c.UpdatedAt = time.Now(), time.Now()
	r.byID[c.ID], r.byEmail[c.Email] = &c, &c
	return &c, nil
}
func (r *inMemUserRepo) GetByID(_ context.Context, id domain.UserID) (*domain.User, error) {
	u, ok := r.byID[id]
	if !ok || u.IsDeleted {
		return nil, apierror.ErrUserNotFound
	}
	c := *u
	return &c, nil
}
func (r *inMemUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	u, ok := r.byEmail[email]
	if !ok || u.IsDeleted {
		return nil, apierror.ErrUserNotFound
	}
	c := *u
	return &c, nil
}
func (r *inMemUserRepo) GetByUsername(_ context.Context, username string) (*domain.User, error) {
	for _, u := range r.byID {
		if u.Username == username && !u.IsDeleted {
			c := *u
			return &c, nil
		}
	}
	return nil, apierror.ErrUserNotFound
}
func (r *inMemUserRepo) ExistsByEmail(_ context.Context, email string) (bool, error) {
	_, ok := r.byEmail[email]
	return ok, nil
}
func (r *inMemUserRepo) ExistsByUsername(_ context.Context, username string) (bool, error) {
	for _, u := range r.byID {
		if u.Username == username {
			return true, nil
		}
	}
	return false, nil
}
func (r *inMemUserRepo) Update(_ context.Context, u *domain.User) (*domain.User, error) {
	if _, ok := r.byID[u.ID]; !ok {
		return nil, apierror.ErrUserNotFound
	}
	c := *u
	c.UpdatedAt = time.Now()
	r.byID[c.ID], r.byEmail[c.Email] = &c, &c
	return &c, nil
}
func (r *inMemUserRepo) SoftDelete(_ context.Context, id domain.UserID) error {
	u, ok := r.byID[id]
	if !ok {
		return apierror.ErrUserNotFound
	}
	u.IsDeleted = true
	now := time.Now()
	u.DeletedAt = &now
	u.Status = domain.StatusDeleted
	return nil
}

// inMemProfileRepo ────────────────────────────────────────────────────────────

type inMemProfileRepo struct {
	data map[domain.UserID]*domain.UserProfile
}

func newInMemProfileRepo() *inMemProfileRepo {
	return &inMemProfileRepo{data: make(map[domain.UserID]*domain.UserProfile)}
}

var _ repository.ProfileRepository = (*inMemProfileRepo)(nil)

func (r *inMemProfileRepo) Upsert(_ context.Context, p *domain.UserProfile) (*domain.UserProfile, error) {
	c := *p
	c.UpdatedAt = time.Now()
	r.data[p.UserID] = &c
	return &c, nil
}
func (r *inMemProfileRepo) GetByUserID(_ context.Context, id domain.UserID) (*domain.UserProfile, error) {
	if p, ok := r.data[id]; ok {
		c := *p
		return &c, nil
	}
	return &domain.UserProfile{UserID: id, Visibility: domain.VisibilityPublic}, nil
}

// inMemPrefsRepo ──────────────────────────────────────────────────────────────

type inMemPrefsRepo struct {
	data map[domain.UserID]*domain.UserPreferences
}

func newInMemPrefsRepo() *inMemPrefsRepo {
	return &inMemPrefsRepo{data: make(map[domain.UserID]*domain.UserPreferences)}
}

var _ repository.PreferencesRepository = (*inMemPrefsRepo)(nil)

func (r *inMemPrefsRepo) Upsert(_ context.Context, p *domain.UserPreferences) (*domain.UserPreferences, error) {
	c := *p
	r.data[p.UserID] = &c
	return &c, nil
}
func (r *inMemPrefsRepo) GetByUserID(_ context.Context, id domain.UserID) (*domain.UserPreferences, error) {
	if p, ok := r.data[id]; ok {
		c := *p
		return &c, nil
	}
	return &domain.UserPreferences{UserID: id, DarkMode: true, AutoplayTrailers: true}, nil
}

// inMemSecurityRepo ───────────────────────────────────────────────────────────

type inMemSecurityRepo struct {
	data map[domain.UserID]*domain.UserSecurity
}

func newInMemSecurityRepo() *inMemSecurityRepo {
	return &inMemSecurityRepo{data: make(map[domain.UserID]*domain.UserSecurity)}
}

var _ repository.SecurityRepository = (*inMemSecurityRepo)(nil)

func (r *inMemSecurityRepo) get(id domain.UserID) *domain.UserSecurity {
	if s, ok := r.data[id]; ok {
		return s
	}
	s := &domain.UserSecurity{UserID: id}
	r.data[id] = s
	return s
}
func (r *inMemSecurityRepo) Upsert(_ context.Context, s *domain.UserSecurity) (*domain.UserSecurity, error) {
	c := *s
	r.data[s.UserID] = &c
	return &c, nil
}
func (r *inMemSecurityRepo) GetByUserID(_ context.Context, id domain.UserID) (*domain.UserSecurity, error) {
	c := *r.get(id)
	return &c, nil
}
func (r *inMemSecurityRepo) IncrementFailedLogins(_ context.Context, id domain.UserID) (*domain.UserSecurity, error) {
	s := r.get(id)
	s.FailedLoginCount++
	n := time.Now()
	s.LastFailedLogin = &n
	if s.FailedLoginCount >= 5 {
		l := n.Add(15 * time.Minute)
		s.LockedUntil = &l
	}
	c := *s
	return &c, nil
}
func (r *inMemSecurityRepo) ResetFailedLogins(_ context.Context, id domain.UserID) error {
	s := r.get(id)
	s.FailedLoginCount = 0
	s.LockedUntil = nil
	return nil
}
func (r *inMemSecurityRepo) LockUntil(_ context.Context, id domain.UserID, until interface{}) error {
	s := r.get(id)
	if t, ok := until.(time.Time); ok {
		s.LockedUntil = &t
	}
	return nil
}
func (r *inMemSecurityRepo) RecordLogin(_ context.Context, id domain.UserID, ip string) error {
	s := r.get(id)
	n := time.Now()
	s.LastLoginAt = &n
	s.LastLoginIP = ip
	return nil
}

// inMemSessionRepo ────────────────────────────────────────────────────────────

type inMemSessionRepo struct {
	data      map[string]*domain.UserSession
	blocklist map[string]struct{}
}

func newInMemSessionRepo() *inMemSessionRepo {
	return &inMemSessionRepo{
		data:      make(map[string]*domain.UserSession),
		blocklist: make(map[string]struct{}),
	}
}

var _ repository.SessionRepository = (*inMemSessionRepo)(nil)

func (r *inMemSessionRepo) Create(_ context.Context, s *domain.UserSession) (*domain.UserSession, error) {
	c := *s
	r.data[s.ID] = &c
	return &c, nil
}
func (r *inMemSessionRepo) GetByID(_ context.Context, id string) (*domain.UserSession, error) {
	s, ok := r.data[id]
	if !ok {
		return nil, apierror.ErrSessionNotFound
	}
	c := *s
	return &c, nil
}
func (r *inMemSessionRepo) RevokeByID(_ context.Context, id string) error {
	s, ok := r.data[id]
	if !ok {
		return apierror.ErrSessionNotFound
	}
	n := time.Now()
	s.RevokedAt = &n
	return nil
}
func (r *inMemSessionRepo) RevokeAllForUser(_ context.Context, uid domain.UserID) error {
	n := time.Now()
	for _, s := range r.data {
		if s.UserID == uid {
			t := n
			s.RevokedAt = &t
		}
	}
	return nil
}
func (r *inMemSessionRepo) ListByUser(_ context.Context, uid domain.UserID) ([]*domain.UserSession, error) {
	var out []*domain.UserSession
	for _, s := range r.data {
		if s.UserID == uid && !s.IsRevoked() && !s.IsExpired() {
			c := *s
			out = append(out, &c)
		}
	}
	return out, nil
}
func (r *inMemSessionRepo) AddToBlocklist(_ context.Context, jti string, _ int64) error {
	r.blocklist[jti] = struct{}{}
	return nil
}
func (r *inMemSessionRepo) IsBlocklisted(_ context.Context, jti string) (bool, error) {
	_, ok := r.blocklist[jti]
	return ok, nil
}

// inMemAuthProviderRepo ───────────────────────────────────────────────────────
// FIX: This was entirely absent. Register stores the bcrypt hash here via Link,
// and Login retrieves it via GetByProvider — without this the service cannot
// verify passwords in tests.

type inMemAuthProviderRepo struct {
	// key: "provider:providerID"
	byKey  map[string]*domain.LinkedAuthProvider
	byUser map[domain.UserID][]*domain.LinkedAuthProvider
}

func newInMemAuthProviderRepo() *inMemAuthProviderRepo {
	return &inMemAuthProviderRepo{
		byKey:  make(map[string]*domain.LinkedAuthProvider),
		byUser: make(map[domain.UserID][]*domain.LinkedAuthProvider),
	}
}

var _ repository.AuthProviderRepository = (*inMemAuthProviderRepo)(nil)

func authKey(provider domain.AuthProvider, providerID string) string {
	return string(provider) + ":" + providerID
}

func (r *inMemAuthProviderRepo) Link(_ context.Context, p *domain.LinkedAuthProvider) error {
	c := *p
	key := authKey(p.Provider, p.ProviderID)

	// Upsert: replace existing entry for this user+provider if it exists.
	r.byKey[key] = &c
	list := r.byUser[p.UserID]
	for i, existing := range list {
		if existing.Provider == p.Provider {
			list[i] = &c
			r.byUser[p.UserID] = list
			return nil
		}
	}
	r.byUser[p.UserID] = append(list, &c)
	return nil
}

func (r *inMemAuthProviderRepo) Unlink(_ context.Context, userID domain.UserID, provider domain.AuthProvider) error {
	list := r.byUser[userID]
	filtered := list[:0]
	for _, p := range list {
		if p.Provider != provider {
			filtered = append(filtered, p)
		} else {
			delete(r.byKey, authKey(p.Provider, p.ProviderID))
		}
	}
	r.byUser[userID] = filtered
	return nil
}

func (r *inMemAuthProviderRepo) GetByProvider(_ context.Context, provider domain.AuthProvider, providerID string) (*domain.LinkedAuthProvider, error) {
	p, ok := r.byKey[authKey(provider, providerID)]
	if !ok {
		return nil, apierror.ErrUserNotFound
	}
	c := *p
	return &c, nil
}

func (r *inMemAuthProviderRepo) ListByUser(_ context.Context, userID domain.UserID) ([]*domain.LinkedAuthProvider, error) {
	list := r.byUser[userID]
	out := make([]*domain.LinkedAuthProvider, len(list))
	for i, p := range list {
		c := *p
		out[i] = &c
	}
	return out, nil
}
