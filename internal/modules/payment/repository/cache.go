package repository

import (
	"context"
	"fmt"
	"time"

	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
)

const webhookIdempotencyTTL = 24 * time.Hour

type redisIdempotencyRepository struct{ client *sharedcache.Client }

func NewIdempotencyRepository(client *sharedcache.Client) IdempotencyRepository {
	return &redisIdempotencyRepository{client: client}
}

// MarkSeen uses SET NX (set if not exists) as an atomic idempotency check.
// The key format is "payment:webhook:seen:{stripeEventID}".
// Returns true on first call (new event — should process).
// Returns false on subsequent calls (duplicate — skip).
func (r *redisIdempotencyRepository) MarkSeen(ctx context.Context, stripeEventID string) (bool, error) {
	key := fmt.Sprintf("payment:webhook:seen:%s", stripeEventID)
	set, err := r.client.SetNX(ctx, key, "1", webhookIdempotencyTTL).Result()
	if err != nil {
		// Redis unavailable — fail open so we don't silently drop webhook events.
		return true, nil
	}
	return set, nil
}
