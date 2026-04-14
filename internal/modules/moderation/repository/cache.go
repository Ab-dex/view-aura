package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
)

const lockTTL = 5 * time.Minute

type redisLockRepository struct{ client *sharedcache.Client }

func NewLockRepository(client *sharedcache.Client) LockRepository {
	return &redisLockRepository{client: client}
}

func lockKey(caseID string) string { return "moderation:lock:" + caseID }

// Acquire uses SET NX (set if not exists) as an atomic lock acquisition.
// Returns true if the lock was acquired, false if already held.
func (r *redisLockRepository) Acquire(ctx context.Context, caseID, moderatorID string) (bool, error) {
	key := lockKey(caseID)
	set, err := r.client.SetNX(ctx, key, moderatorID, lockTTL).Result()
	if err != nil {
		return false, fmt.Errorf("moderation: acquire lock: %w", err)
	}
	return set, nil
}

// Release deletes the lock key.
func (r *redisLockRepository) Release(ctx context.Context, caseID string) error {
	return r.client.Del(ctx, lockKey(caseID)).Err()
}

// IsLocked returns the moderatorID holding the lock, or "" if unlocked.
func (r *redisLockRepository) IsLocked(ctx context.Context, caseID string) (string, error) {
	val, err := r.client.Get(ctx, lockKey(caseID)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return "", nil
		}
		return "", fmt.Errorf("moderation: check lock: %w", err)
	}
	return val, nil
}
