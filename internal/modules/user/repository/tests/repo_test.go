package tests

import (
	"context"
	"sync"
	"time"

	"github.com/Ab-dex/view-aura/internal/modules/user/domain"
	"github.com/Ab-dex/view-aura/internal/modules/user/repository"
	"github.com/Ab-dex/view-aura/internal/modules/user/service"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

// ─── MockUserRepository ───────────────────────────────────────────────────────

type MockUserRepository struct {
	mu      sync.RWMutex
	byID    map[domain.UserID]*domain.User
	byEmail map[string]*domain.User

	CreateErr     error
	GetByIDErr    error
	GetByEmailErr error
	UpdateErr     error
}

var _ repository.UserRepository = (*MockUserRepository)(nil)

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		byID:    make(map[domain.UserID]*domain.User),
		byEmail: make(map[string]*domain.User),
	}
}

func (m *MockUserRepository) Create(_ context.Context, u *domain.User) (*domain.User, error) {
	if m.CreateErr != nil {
		return nil, m.CreateErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.byEmail[u.Email]; exists {
		return nil, apierror.ErrEmailAlreadyExists
	}
	clone := *u
	clone.CreatedAt = time.Now()
	clone.UpdatedAt = time.Now()
	m.byID[clone.ID] = &clone
	m.byEmail[clone.Email] = &clone
	return &clone, nil
}

func (m *MockUserRepository) GetByID(_ context.Context, id domain.UserID) (*domain.User, error) {
	if m.GetByIDErr != nil {
		return nil, m.GetByIDErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.byID[id]
	if !ok || u.IsDeleted {
		return nil, apierror.ErrUserNotFound
	}
	clone := *u
	return &clone, nil
}

func (m *MockUserRepository) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	if m.GetByEmailErr != nil {
		return nil, m.GetByEmailErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.byEmail[email]
	if !ok || u.IsDeleted {
		return nil, apierror.ErrUserNotFound
	}
	clone := *u
	return &clone, nil
}

func (m *MockUserRepository) GetByUsername(_ context.Context, username string) (*domain.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, u := range m.byID {
		if u.Username == username && !u.IsDeleted {
			clone := *u
			return &clone, nil
		}
	}
	return nil, apierror.ErrUserNotFound
}

func (m *MockUserRepository) ExistsByEmail(_ context.Context, email string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.byEmail[email]
	return exists, nil
}

func (m *MockUserRepository) ExistsByUsername(_ context.Context, username string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, u := range m.byID {
		if u.Username == username {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockUserRepository) Update(_ context.Context, u *domain.User) (*domain.User, error) {
	if m.UpdateErr != nil {
		return nil, m.UpdateErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.byID[u.ID]; !exists {
		return nil, apierror.ErrUserNotFound
	}
	clone := *u
	clone.UpdatedAt = time.Now()
	m.byID[clone.ID] = &clone
	m.byEmail[clone.Email] = &clone
	return &clone, nil
}

func (m *MockUserRepository) SoftDelete(_ context.Context, id domain.UserID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	u, exists := m.byID[id]
	if !exists {
		return apierror.ErrUserNotFound
	}
	u.IsDeleted = true
	now := time.Now()
	u.DeletedAt = &now
	u.Status = domain.StatusDeleted
	return nil
}

// ─── MockProfileRepository ───────────────────────────────────────────────────

type MockProfileRepository struct {
	mu        sync.RWMutex
	profiles  map[domain.UserID]*domain.UserProfile
	UpsertErr error
}

var _ repository.ProfileRepository = (*MockProfileRepository)(nil)

func NewMockProfileRepository() *MockProfileRepository {
	return &MockProfileRepository{profiles: make(map[domain.UserID]*domain.UserProfile)}
}

func (m *MockProfileRepository) Upsert(_ context.Context, p *domain.UserProfile) (*domain.UserProfile, error) {
	if m.UpsertErr != nil {
		return nil, m.UpsertErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := *p
	clone.UpdatedAt = time.Now()
	m.profiles[p.UserID] = &clone
	return &clone, nil
}

func (m *MockProfileRepository) GetByUserID(_ context.Context, userID domain.UserID) (*domain.UserProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p, ok := m.profiles[userID]; ok {
		clone := *p
		return &clone, nil
	}
	return &domain.UserProfile{UserID: userID, Visibility: domain.VisibilityPublic}, nil
}

// ─── MockPreferencesRepository ───────────────────────────────────────────────

type MockPreferencesRepository struct {
	mu    sync.RWMutex
	prefs map[domain.UserID]*domain.UserPreferences
}

var _ repository.PreferencesRepository = (*MockPreferencesRepository)(nil)

func NewMockPreferencesRepository() *MockPreferencesRepository {
	return &MockPreferencesRepository{prefs: make(map[domain.UserID]*domain.UserPreferences)}
}

func (m *MockPreferencesRepository) Upsert(_ context.Context, p *domain.UserPreferences) (*domain.UserPreferences, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := *p
	m.prefs[p.UserID] = &clone
	return &clone, nil
}

func (m *MockPreferencesRepository) GetByUserID(_ context.Context, userID domain.UserID) (*domain.UserPreferences, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p, ok := m.prefs[userID]; ok {
		clone := *p
		return &clone, nil
	}
	return &domain.UserPreferences{UserID: userID, DarkMode: true}, nil
}

// ─── MockSecurityRepository ───────────────────────────────────────────────────

type MockSecurityRepository struct {
	mu      sync.RWMutex
	records map[domain.UserID]*domain.UserSecurity

	IncrementErr error
	ResetErr     error
}

var _ repository.SecurityRepository = (*MockSecurityRepository)(nil)

func NewMockSecurityRepository() *MockSecurityRepository {
	return &MockSecurityRepository{records: make(map[domain.UserID]*domain.UserSecurity)}
}

func (m *MockSecurityRepository) Upsert(_ context.Context, s *domain.UserSecurity) (*domain.UserSecurity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := *s
	m.records[s.UserID] = &clone
	return &clone, nil
}

func (m *MockSecurityRepository) GetByUserID(_ context.Context, userID domain.UserID) (*domain.UserSecurity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.records[userID]; ok {
		clone := *s
		return &clone, nil
	}
	return &domain.UserSecurity{UserID: userID}, nil
}

func (m *MockSecurityRepository) IncrementFailedLogins(_ context.Context, userID domain.UserID) (*domain.UserSecurity, error) {
	if m.IncrementErr != nil {
		return nil, m.IncrementErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	s := m.getOrCreate(userID)
	s.FailedLoginCount++
	now := time.Now()
	s.LastFailedLogin = &now

	if s.FailedLoginCount >= 5 {
		lockUntil := now.Add(15 * time.Minute)
		s.LockedUntil = &lockUntil
	}
	clone := *s
	return &clone, nil
}

func (m *MockSecurityRepository) ResetFailedLogins(_ context.Context, userID domain.UserID) error {
	if m.ResetErr != nil {
		return m.ResetErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.getOrCreate(userID)
	s.FailedLoginCount = 0
	s.LockedUntil = nil
	return nil
}

func (m *MockSecurityRepository) LockUntil(_ context.Context, userID domain.UserID, until interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.getOrCreate(userID)
	if t, ok := until.(time.Time); ok {
		s.LockedUntil = &t
	}
	return nil
}

func (m *MockSecurityRepository) RecordLogin(_ context.Context, userID domain.UserID, ip string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.getOrCreate(userID)
	now := time.Now()
	s.LastLoginAt = &now
	s.LastLoginIP = ip
	return nil
}

func (m *MockSecurityRepository) getOrCreate(userID domain.UserID) *domain.UserSecurity {
	if s, ok := m.records[userID]; ok {
		return s
	}
	s := &domain.UserSecurity{UserID: userID}
	m.records[userID] = s
	return s
}

// ─── MockSessionRepository ───────────────────────────────────────────────────

type MockSessionRepository struct {
	mu        sync.RWMutex
	sessions  map[string]*domain.UserSession
	blocklist map[string]struct{}

	CreateErr error
	GetErr    error
}

var _ repository.SessionRepository = (*MockSessionRepository)(nil)

func NewMockSessionRepository() *MockSessionRepository {
	return &MockSessionRepository{
		sessions:  make(map[string]*domain.UserSession),
		blocklist: make(map[string]struct{}),
	}
}

func (m *MockSessionRepository) Create(_ context.Context, s *domain.UserSession) (*domain.UserSession, error) {
	if m.CreateErr != nil {
		return nil, m.CreateErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := *s
	m.sessions[s.ID] = &clone
	return &clone, nil
}

func (m *MockSessionRepository) GetByID(_ context.Context, id string) (*domain.UserSession, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, apierror.ErrSessionNotFound
	}
	clone := *s
	return &clone, nil
}

func (m *MockSessionRepository) RevokeByID(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return apierror.ErrSessionNotFound
	}
	now := time.Now()
	s.RevokedAt = &now
	return nil
}

func (m *MockSessionRepository) RevokeAllForUser(_ context.Context, userID domain.UserID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for _, s := range m.sessions {
		if s.UserID == userID {
			t := now
			s.RevokedAt = &t
		}
	}
	return nil
}

func (m *MockSessionRepository) ListByUser(_ context.Context, userID domain.UserID) ([]*domain.UserSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*domain.UserSession
	for _, s := range m.sessions {
		if s.UserID == userID && !s.IsRevoked() && !s.IsExpired() {
			clone := *s
			out = append(out, &clone)
		}
	}
	return out, nil
}

func (m *MockSessionRepository) AddToBlocklist(_ context.Context, jti string, _ int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blocklist[jti] = struct{}{}
	return nil
}

func (m *MockSessionRepository) IsBlocklisted(_ context.Context, jti string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.blocklist[jti]
	return ok, nil
}

// ─── MockAuthProviderRepository ──────────────────────────────────────────────

// MockAuthProviderRepository is an in-memory implementation of AuthProviderRepository.
type MockAuthProviderRepository struct {
	mu        sync.RWMutex
	providers map[string]*domain.LinkedAuthProvider // key: provider:providerID
	byUser    map[domain.UserID][]*domain.LinkedAuthProvider

	LinkErr error
}

var _ repository.AuthProviderRepository = (*MockAuthProviderRepository)(nil)

func NewMockAuthProviderRepository() *MockAuthProviderRepository {
	return &MockAuthProviderRepository{
		providers: make(map[string]*domain.LinkedAuthProvider),
		byUser:    make(map[domain.UserID][]*domain.LinkedAuthProvider),
	}
}

func providerKey(provider domain.AuthProvider, providerID string) string {
	return string(provider) + ":" + providerID
}

func (m *MockAuthProviderRepository) Link(_ context.Context, p *domain.LinkedAuthProvider) error {
	if m.LinkErr != nil {
		return m.LinkErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := *p
	key := providerKey(p.Provider, p.ProviderID)
	m.providers[key] = &clone
	m.byUser[p.UserID] = append(m.byUser[p.UserID], &clone)
	return nil
}

func (m *MockAuthProviderRepository) Unlink(_ context.Context, userID domain.UserID, provider domain.AuthProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.byUser[userID]
	filtered := list[:0]
	for _, p := range list {
		if p.Provider != provider {
			filtered = append(filtered, p)
		} else {
			delete(m.providers, providerKey(p.Provider, p.ProviderID))
		}
	}
	m.byUser[userID] = filtered
	return nil
}

func (m *MockAuthProviderRepository) GetByProvider(_ context.Context, provider domain.AuthProvider, providerID string) (*domain.LinkedAuthProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.providers[providerKey(provider, providerID)]
	if !ok {
		return nil, apierror.ErrUserNotFound
	}
	clone := *p
	return &clone, nil
}

func (m *MockAuthProviderRepository) ListByUser(_ context.Context, userID domain.UserID) ([]*domain.LinkedAuthProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := m.byUser[userID]
	out := make([]*domain.LinkedAuthProvider, len(list))
	for i, p := range list {
		clone := *p
		out[i] = &clone
	}
	return out, nil
}

// ─── MockTokenService ────────────────────────────────────────────────────────

// MockTokenService satisfies service.TokenService with correct return types.
// FIX #3: ValidateAccess and ValidateRefresh previously returned (interface{}, error)
// which does not satisfy the TokenService interface — they must return (*service.Claims, error).
type MockTokenService struct {
	IssuedPair    *domain.TokenPair
	IssueErr      error
	ValidateErr   error
	InvalidateErr error

	mu          sync.RWMutex
	invalidated []string
}

var _ service.TokenService = (*MockTokenService)(nil)

func NewMockTokenService() *MockTokenService {
	return &MockTokenService{
		IssuedPair: &domain.TokenPair{
			AccessToken:  "mock.access.token",
			RefreshToken: "mock.refresh.token",
			ExpiresIn:    3600,
		},
	}
}

func (m *MockTokenService) Issue(_ context.Context, _ *domain.User) (*domain.TokenPair, error) {
	if m.IssueErr != nil {
		return nil, m.IssueErr
	}
	return m.IssuedPair, nil
}

// FIX #3: return (*service.Claims, error) — not (interface{}, error).
func (m *MockTokenService) ValidateAccess(_ context.Context, _ string) (*service.Claims, error) {
	if m.ValidateErr != nil {
		return nil, m.ValidateErr
	}
	return &service.Claims{Role: "user", TokenType: "access"}, nil
}

// FIX #3: return (*service.Claims, error) — not (interface{}, error).
func (m *MockTokenService) ValidateRefresh(_ context.Context, _ string) (*service.Claims, error) {
	if m.ValidateErr != nil {
		return nil, m.ValidateErr
	}
	return &service.Claims{TokenType: "refresh"}, nil
}

func (m *MockTokenService) Invalidate(_ context.Context, jti string, _ time.Duration) error {
	if m.InvalidateErr != nil {
		return m.InvalidateErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invalidated = append(m.invalidated, jti)
	return nil
}

func (m *MockTokenService) WasInvalidated(jti string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, id := range m.invalidated {
		if id == jti {
			return true
		}
	}
	return false
}
