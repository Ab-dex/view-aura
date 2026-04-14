package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Ab-dex/view-aura/internal/modules/quota/domain"
	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
)

type redisQuotaRepository struct{ client *sharedcache.Client }

func NewQuotaRepository(client *sharedcache.Client) QuotaRepository {
	return &redisQuotaRepository{client: client}
}

// ─── Key helpers ──────────────────────────────────────────────────────────────

func tierKey(userID string) string    { return "quota:tier:" + userID }
func storageKey(userID string) string { return "quota:storage:" + userID }
func dailyKey(userID string) string {
	day := time.Now().UTC().Format("2006-01-02")
	return fmt.Sprintf("quota:daily:%s:%s", userID, day)
}

// ─── Tier ────────────────────────────────────────────────────────────────────

func (r *redisQuotaRepository) GetTier(ctx context.Context, userID string) (domain.Tier, error) {
	val, err := r.client.Get(ctx, tierKey(userID)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return domain.TierFree, nil // default
		}
		return domain.TierFree, fmt.Errorf("quota: get tier: %w", err)
	}
	return domain.Tier(val), nil
}

func (r *redisQuotaRepository) SetTier(ctx context.Context, userID string, tier domain.Tier) error {
	// No TTL — tier persists until explicitly changed by a payment event.
	return r.client.Set(ctx, tierKey(userID), string(tier), 0).Err()
}

// ─── Storage ──────────────────────────────────────────────────────────────────

func (r *redisQuotaRepository) GetStorageUsed(ctx context.Context, userID string) (int64, error) {
	val, err := r.client.Get(ctx, storageKey(userID)).Int64()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return 0, nil
		}
		return 0, fmt.Errorf("quota: get storage: %w", err)
	}
	return val, nil
}

func (r *redisQuotaRepository) IncrementStorage(ctx context.Context, userID string, deltaBytes int64) error {
	key := storageKey(userID)
	result, err := r.client.IncrBy(ctx, key, deltaBytes).Result()
	if err != nil {
		return fmt.Errorf("quota: increment storage: %w", err)
	}
	// Guard against going negative (e.g. repeated delete events).
	if result < 0 {
		r.client.Set(ctx, key, 0, 0)
	}
	return nil
}

// ─── Daily uploads ────────────────────────────────────────────────────────────

func (r *redisQuotaRepository) GetDailyUploads(ctx context.Context, userID string) (int, error) {
	val, err := r.client.Get(ctx, dailyKey(userID)).Int()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return 0, nil
		}
		return 0, fmt.Errorf("quota: get daily uploads: %w", err)
	}
	return val, nil
}

func (r *redisQuotaRepository) IncrementDailyUploads(ctx context.Context, userID string) error {
	key := dailyKey(userID)
	// INCR returns the new value; use it to set TTL only on first increment.
	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("quota: increment daily uploads: %w", err)
	}
	if count == 1 {
		// First upload today — set the key to expire at midnight UTC.
		now := time.Now().UTC()
		midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
		r.client.ExpireAt(ctx, key, midnight)
	}
	return nil
}
