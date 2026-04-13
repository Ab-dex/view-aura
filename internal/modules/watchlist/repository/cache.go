package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Ab-dex/view-aura/internal/modules/watchlist/domain"
	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
)

const watchlistCacheTTL = 30 * time.Minute

// watchlistCache wraps the Postgres WatchlistRepository with a Redis Hash cache.
//
// Key: user:{id}:watchlist  (Hash)
// Field: movie_id
// Value: JSON-encoded WatchlistEntry
//
// This is the highest-traffic read pattern for the watchlist — the movie detail
// page shows "Add to Watchlist / Watching / Watched" based on the user's current
// status. Hitting Postgres for that on every detail page load at 25K QPS would
// be expensive.
type watchlistCache struct {
	pg     WatchlistRepository
	client *sharedcache.Client
}

func NewCachedWatchlistRepository(pg WatchlistRepository, client *sharedcache.Client) WatchlistRepository {
	return &watchlistCache{pg: pg, client: client}
}

// GetByUserAndMovie checks Redis first, falls back to Postgres on miss.
func (c *watchlistCache) GetByUserAndMovie(ctx context.Context, userID, movieID string) (*domain.WatchlistEntry, error) {
	key := sharedcache.UserWatchlistKey(userID)
	b, err := c.client.HGet(ctx, key, movieID).Bytes()
	if err == nil {
		var entry domain.WatchlistEntry
		if jsonErr := json.Unmarshal(b, &entry); jsonErr == nil {
			return &entry, nil
		}
	}
	// Cache miss — fetch from Postgres.
	entry, err := c.pg.GetByUserAndMovie(ctx, userID, movieID)
	if err != nil {
		return nil, err
	}
	if entry != nil {
		c.setField(ctx, userID, movieID, entry)
	}
	return entry, nil
}

// Upsert writes to Postgres then updates the Redis hash field.
func (c *watchlistCache) Upsert(ctx context.Context, e *domain.WatchlistEntry) (*domain.WatchlistEntry, error) {
	result, err := c.pg.Upsert(ctx, e)
	if err != nil {
		return nil, err
	}
	c.setField(ctx, e.UserID, e.MovieID, result)
	return result, nil
}

// Delete removes from Postgres and deletes the hash field.
func (c *watchlistCache) Delete(ctx context.Context, userID, movieID string) error {
	if err := c.pg.Delete(ctx, userID, movieID); err != nil {
		return err
	}
	key := sharedcache.UserWatchlistKey(userID)
	_ = c.client.HDel(ctx, key, movieID).Err()
	return nil
}

// List is not cached — full paginated lists are low-frequency and vary by
// status filter, making a Hash-based cache impractical without full invalidation.
func (c *watchlistCache) List(ctx context.Context, filter domain.EntryFilter) ([]*domain.WatchlistEntry, int, error) {
	return c.pg.List(ctx, filter)
}

func (c *watchlistCache) CountByStatus(ctx context.Context, userID string) (map[domain.WatchStatus]int, error) {
	return c.pg.CountByStatus(ctx, userID)
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (c *watchlistCache) setField(ctx context.Context, userID, movieID string, entry *domain.WatchlistEntry) {
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	key := sharedcache.UserWatchlistKey(userID)
	pipe := c.client.Pipeline()
	pipe.HSet(ctx, key, movieID, b)
	pipe.Expire(ctx, key, watchlistCacheTTL)
	_, _ = pipe.Exec(ctx)
}
