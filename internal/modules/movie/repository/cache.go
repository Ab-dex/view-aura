package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Ab-dex/view-aura/internal/modules/movie/domain"
	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
)

const movieMetaTTL = time.Hour

// movieCache wraps the Postgres MovieRepository with a Redis read-through cache.
// Only GetByID and GetBySlug are cached — they are the two hottest paths
// (detail page load). List queries are not cached because filter diversity
// makes cache keys impractical and query time is dominated by indexes.
type movieCache struct {
	pg     MovieRepository // the real Postgres implementation
	client *sharedcache.Client
}

// NewCachedMovieRepository wraps a MovieRepository with a Redis read-through layer.
func NewCachedMovieRepository(pg MovieRepository, client *sharedcache.Client) MovieRepository {
	return &movieCache{pg: pg, client: client}
}

// ─── Cached reads ─────────────────────────────────────────────────────────────

func (c *movieCache) GetByID(ctx context.Context, id domain.MovieID) (*domain.Movie, error) {
	key := sharedcache.MovieMetaKey(id.String())
	if m, err := c.fromCache(ctx, key); err == nil {
		return m, nil
	}
	// Cache miss — fetch from Postgres and populate.
	m, err := c.pg.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	c.toCache(ctx, key, m)
	return m, nil
}

func (c *movieCache) GetBySlug(ctx context.Context, slug string) (*domain.Movie, error) {
	// Slugs are stable — cache under the same key pattern using the slug itself.
	key := sharedcache.MovieMetaKey("slug:" + slug)
	if m, err := c.fromCache(ctx, key); err == nil {
		return m, nil
	}
	m, err := c.pg.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	c.toCache(ctx, key, m)
	return m, nil
}

// ─── Cache-invalidating writes ────────────────────────────────────────────────

func (c *movieCache) Create(ctx context.Context, movie *domain.Movie) (*domain.Movie, error) {
	return c.pg.Create(ctx, movie)
}

func (c *movieCache) Update(ctx context.Context, movie *domain.Movie) (*domain.Movie, error) {
	result, err := c.pg.Update(ctx, movie)
	if err != nil {
		return nil, err
	}
	// Invalidate both the ID and slug cache entries so stale data is never served.
	c.invalidate(ctx, result.ID.String(), "slug:"+result.Slug)
	return result, nil
}

func (c *movieCache) Delete(ctx context.Context, id domain.MovieID) error {
	// Fetch the slug before deleting so we can invalidate its cache key too.
	if m, err := c.pg.GetByID(ctx, id); err == nil {
		c.invalidate(ctx, id.String(), "slug:"+m.Slug)
	}
	return c.pg.Delete(ctx, id)
}

func (c *movieCache) UpdateAggregates(ctx context.Context, id domain.MovieID, avgRating float64, ratingCount int, popularity float64) error {
	if err := c.pg.UpdateAggregates(ctx, id, avgRating, ratingCount, popularity); err != nil {
		return err
	}
	// Aggregates changed — bust the cached movie so avg_rating is fresh.
	c.invalidate(ctx, id.String())
	return nil
}

// ─── Pass-through (no caching benefit) ───────────────────────────────────────

func (c *movieCache) List(ctx context.Context, filter domain.MovieFilter) ([]*domain.Movie, int, error) {
	return c.pg.List(ctx, filter)
}

func (c *movieCache) ExistsByIMDbID(ctx context.Context, imdbID string) (bool, error) {
	return c.pg.ExistsByIMDbID(ctx, imdbID)
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (c *movieCache) fromCache(ctx context.Context, key string) (*domain.Movie, error) {
	if c.client == nil {
		return nil, goredis.Nil
	}

	b, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, goredis.Nil // cache miss
		}
		return nil, err // Redis error
	}

	var m domain.Movie
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}

	return &m, nil
}

func (c *movieCache) toCache(ctx context.Context, key string, m *domain.Movie) {
	if c.client == nil {
		return // Redis not available → silently skip caching
	}

	b, err := json.Marshal(m)
	if err != nil {
		return // serialization failure is non-fatal
	}

	_ = c.client.Set(ctx, key, b, movieMetaTTL).Err()
}

func (c *movieCache) invalidate(ctx context.Context, keySuffixes ...string) {
	if c.client == nil {
		return
	}

	for _, s := range keySuffixes {
		_ = c.client.Del(ctx, sharedcache.MovieMetaKey(s)).Err()
	}

	keys, err := c.client.Keys(ctx, "http:GET:*movies*").Result()
	if err == nil && len(keys) > 0 {
		_ = c.client.Del(ctx, keys...).Err()
	}
}
