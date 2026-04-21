package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Ab-dex/view-aura/internal/modules/auth/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

// redisClient is the narrow interface used by this package.
// Satisfied by *redis.Client and *redis.ClusterClient from go-redis/v9,
// as well as the platform/cache.Client wrapper (which embeds *redis.Client).
type redisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *goredis.StatusCmd
	Get(ctx context.Context, key string) *goredis.StringCmd
	Del(ctx context.Context, keys ...string) *goredis.IntCmd
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *goredis.Cmd
}

// ─── Key helpers ─────────────────────────────────────────────────────────────

func challengeKey(id string) string     { return "auth:mfa_challenge:" + id }
func oauthStateKey(state string) string { return "auth:oauth_state:" + state }

// ─── redisMFARepo — MFA challenge store ──────────────────────────────────────

type redisMFARepo struct{ rdb redisClient }

func (r *redisMFARepo) CreateChallenge(ctx context.Context, ch *domain.MFAChallenge) error {
	data, err := json.Marshal(ch)
	if err != nil {
		return fmt.Errorf("auth: marshal mfa challenge: %w", err)
	}
	ttl := time.Until(ch.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("auth: mfa challenge already expired at creation")
	}
	return r.rdb.Set(ctx, challengeKey(ch.ID), data, ttl).Err()
}

func (r *redisMFARepo) GetChallenge(ctx context.Context, id string) (*domain.MFAChallenge, error) {
	raw, err := r.rdb.Get(ctx, challengeKey(id)).Bytes()
	if err == goredis.Nil {
		return nil, apierror.ErrTokenInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("auth: get mfa challenge: %w", err)
	}
	var ch domain.MFAChallenge
	if err := json.Unmarshal(raw, &ch); err != nil {
		return nil, fmt.Errorf("auth: unmarshal mfa challenge: %w", err)
	}
	if ch.IsExpired() {
		// TTL hasn't fired yet but logically expired — treat as not found.
		return nil, apierror.ErrTokenInvalid
	}
	return &ch, nil
}

func (r *redisMFARepo) DeleteChallenge(ctx context.Context, id string) error {
	return r.rdb.Del(ctx, challengeKey(id)).Err()
}

// ─── OAuthStateRepository ────────────────────────────────────────────────────

type redisOAuthStateRepo struct{ rdb redisClient }

func NewOAuthStateRepository(rdb redisClient) OAuthStateRepository {
	return &redisOAuthStateRepo{rdb: rdb}
}

func (r *redisOAuthStateRepo) Save(ctx context.Context, state *domain.OAuthState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("auth: marshal oauth state: %w", err)
	}
	ttl := time.Until(state.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("auth: oauth state already expired")
	}
	return r.rdb.Set(ctx, oauthStateKey(state.State), data, ttl).Err()
}

// GetAndDelete atomically retrieves and removes the state record using a Lua
// script.  This prevents CSRF replay — any state nonce can only be consumed
// once even under concurrent callback requests.
//
// Returns nil, nil (not an error) when the state is absent or expired so the
// caller can return a user-facing message without leaking internal details.
func (r *redisOAuthStateRepo) GetAndDelete(ctx context.Context, state string) (*domain.OAuthState, error) {
	const lua = `
		local v = redis.call('GET', KEYS[1])
		if v then
			redis.call('DEL', KEYS[1])
		end
		return v`

	val, err := r.rdb.Eval(ctx, lua, []string{oauthStateKey(state)}).Text()
	if err == goredis.Nil {
		return nil, nil // state not found or already consumed
	}
	if err != nil {
		return nil, fmt.Errorf("auth: get-and-delete oauth state: %w", err)
	}

	var s domain.OAuthState
	if err := json.Unmarshal([]byte(val), &s); err != nil {
		return nil, fmt.Errorf("auth: unmarshal oauth state: %w", err)
	}
	if s.IsExpired() {
		return nil, nil // logically expired — treat as not found
	}
	return &s, nil
}
