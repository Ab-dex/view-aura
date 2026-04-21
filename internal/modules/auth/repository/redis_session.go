package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Ab-dex/view-aura/internal/modules/auth/domain"
	userdomain "github.com/Ab-dex/view-aura/internal/modules/user/domain"
	sharedredis "github.com/Ab-dex/view-aura/internal/platform/cache"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

// sessionCache is a Redis-backed session store. Postgres is written for audit;
// Redis is the source of truth for active session validation.
type sessionCache struct {
	client *sharedredis.Client
}

// NewSessionRepository constructs the Redis-backed SessionRepository.
func NewSessionRepository(client *sharedredis.Client) SessionRepository {
	return &sessionCache{client: client}
}

// sessionData is the JSON-serialisable form of a UserSession stored in Redis.
type sessionData struct {
	ID               string `json:"id"`
	UserID           string `json:"user_id"`
	DeviceID         string `json:"device_id"`
	IPAddress        string `json:"ip_address"`
	UserAgent        string `json:"user_agent"`
	RefreshTokenHash string `json:"refresh_token_hash"`
	CreatedAt        int64  `json:"created_at"`
	ExpiresAt        int64  `json:"expires_at"`
	RevokedAt        *int64 `json:"revoked_at,omitempty"`
}

func (r *sessionCache) Create(ctx context.Context, s *domain.UserSession) (*domain.UserSession, error) {
	data := sessionData{
		ID:               s.ID,
		UserID:           s.UserID.String(),
		DeviceID:         s.DeviceID,
		IPAddress:        s.IPAddress,
		UserAgent:        s.UserAgent,
		RefreshTokenHash: s.RefreshTokenHash,
		CreatedAt:        s.CreatedAt.Unix(),
		ExpiresAt:        s.ExpiresAt.Unix(),
	}

	b, err := json.Marshal(data)
	if err != nil {
		return nil, apierror.Internal("marshal session", err)
	}

	ttl := time.Until(s.ExpiresAt)
	if ttl <= 0 {
		return nil, apierror.ErrSessionExpired
	}

	pipe := r.client.Pipeline()
	sessionKey := sharedredis.SessionKey(s.ID)
	pipe.Set(ctx, sessionKey, b, ttl)
	pipe.SAdd(ctx, sharedredis.UserSessionsKey(s.UserID.String()), s.ID)
	pipe.Expire(ctx, sharedredis.UserSessionsKey(s.UserID.String()), ttl)

	if _, err := pipe.Exec(ctx); err != nil {
		return nil, apierror.New(500, apierror.CodeCache, "failed to store session").WithCause(err)
	}
	return s, nil
}

func (r *sessionCache) GetByID(ctx context.Context, id string) (*domain.UserSession, error) {
	b, err := r.client.Get(ctx, sharedredis.SessionKey(id)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, apierror.ErrSessionNotFound
		}
		return nil, apierror.New(500, apierror.CodeCache, "failed to get session").WithCause(err)
	}

	var data sessionData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, apierror.Internal("unmarshal session", err)
	}

	return fromSessionData(data), nil
}

func (r *sessionCache) RevokeByID(ctx context.Context, id string) error {
	s, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if s.IsRevoked() {
		return nil // idempotent
	}

	now := time.Now()
	s.RevokedAt = &now

	b, err := json.Marshal(toSessionData(s))
	if err != nil {
		return apierror.Internal("marshal revoked session", err)
	}

	ttl := time.Until(s.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Second
	}

	if err := r.client.Set(ctx, sharedredis.SessionKey(id), b, ttl).Err(); err != nil {
		return apierror.New(500, apierror.CodeCache, "failed to revoke session").WithCause(err)
	}
	return nil
}

func (r *sessionCache) RevokeAllForUser(ctx context.Context, userID userdomain.UserID) error {
	ids, err := r.client.SMembers(ctx, sharedredis.UserSessionsKey(userID.String())).Result()
	if err != nil {
		return apierror.New(500, apierror.CodeCache, "failed to list sessions for user").WithCause(err)
	}
	for _, id := range ids {
		if rErr := r.RevokeByID(ctx, id); rErr != nil {
			_ = rErr // best-effort; caller's logger should capture this upstream
		}
	}
	return nil
}

func (r *sessionCache) ListByUser(ctx context.Context, userID userdomain.UserID) ([]*domain.UserSession, error) {
	ids, err := r.client.SMembers(ctx, sharedredis.UserSessionsKey(userID.String())).Result()
	if err != nil {
		return nil, apierror.New(500, apierror.CodeCache, "failed to list session ids").WithCause(err)
	}

	sessions := make([]*domain.UserSession, 0, len(ids))
	for _, id := range ids {
		s, err := r.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, apierror.ErrSessionNotFound) {
				continue // expired from Redis
			}
			return nil, err
		}
		if !s.IsExpired() && !s.IsRevoked() {
			sessions = append(sessions, s)
		}
	}
	return sessions, nil
}

func (r *sessionCache) AddToBlocklist(ctx context.Context, jti string, ttlSeconds int64) error {
	ttl := time.Duration(ttlSeconds) * time.Second
	key := sharedredis.BlocklistJTIKey(jti)
	if err := r.client.Set(ctx, key, "1", ttl).Err(); err != nil {
		return apierror.New(500, apierror.CodeCache, "failed to add JTI to blocklist").WithCause(err)
	}
	return nil
}

func (r *sessionCache) IsBlocklisted(ctx context.Context, jti string) (bool, error) {
	n, err := r.client.Exists(ctx, sharedredis.BlocklistJTIKey(jti)).Result()
	if err != nil {
		return false, apierror.New(500, apierror.CodeCache, "failed to check blocklist").WithCause(err)
	}
	return n > 0, nil
}

// ─── Conversion helpers ───────────────────────────────────────────────────────

func toSessionData(s *domain.UserSession) sessionData {
	d := sessionData{
		ID:               s.ID,
		UserID:           s.UserID.String(),
		DeviceID:         s.DeviceID,
		IPAddress:        s.IPAddress,
		UserAgent:        s.UserAgent,
		RefreshTokenHash: s.RefreshTokenHash,
		CreatedAt:        s.CreatedAt.Unix(),
		ExpiresAt:        s.ExpiresAt.Unix(),
	}
	if s.RevokedAt != nil {
		ts := s.RevokedAt.Unix()
		d.RevokedAt = &ts
	}
	return d
}

func fromSessionData(d sessionData) *domain.UserSession {
	s := &domain.UserSession{
		ID:               d.ID,
		UserID:           userdomain.UserID(d.UserID),
		DeviceID:         d.DeviceID,
		IPAddress:        d.IPAddress,
		UserAgent:        d.UserAgent,
		RefreshTokenHash: d.RefreshTokenHash,
		CreatedAt:        time.Unix(d.CreatedAt, 0),
		ExpiresAt:        time.Unix(d.ExpiresAt, 0),
	}
	if d.RevokedAt != nil {
		t := time.Unix(*d.RevokedAt, 0)
		s.RevokedAt = &t
	}
	return s
}

// Compile-time assertion: sessionCache satisfies SessionRepository.
var _ SessionRepository = (*sessionCache)(nil)
