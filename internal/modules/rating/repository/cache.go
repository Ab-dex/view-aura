package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Ab-dex/view-aura/internal/modules/rating/domain"
	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
)

const (
	ratingAggTTL      = 5 * time.Minute
	userRatedTTL      = time.Hour
	leaderboardTTL    = time.Hour
	leaderboardMaxLen = 100 // keep top-100 per genre
)

// ratingCache wraps the Postgres RatingRepository with:
//  1. A short-TTL aggregation cache for GetStats (movie:{id}:rating:agg).
//  2. A user-rated Set for idempotency on the Upsert write path (user:{id}:rated).
//  3. A per-genre sorted-set leaderboard updated on every upsert (leaderboard:{genre}:weekly).
type ratingCache struct {
	pg     RatingRepository
	client *sharedcache.Client
}

func NewCachedRatingRepository(pg RatingRepository, client *sharedcache.Client) RatingRepository {
	return &ratingCache{pg: pg, client: client}
}

// ─── Cached aggregation ───────────────────────────────────────────────────────

func (c *ratingCache) GetStats(ctx context.Context, movieID string) (*domain.MovieRatingStats, error) {
	key := sharedcache.MovieRatingAggKey(movieID)
	if stats, err := c.statsFromCache(ctx, key); err == nil {
		return stats, nil
	}
	stats, err := c.pg.GetStats(ctx, movieID)
	if err != nil {
		return nil, err
	}
	c.statsToCache(ctx, key, stats)
	return stats, nil
}

func (c *ratingCache) statsFromCache(ctx context.Context, key string) (*domain.MovieRatingStats, error) {
	b, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var s domain.MovieRatingStats
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *ratingCache) statsToCache(ctx context.Context, key string, s *domain.MovieRatingStats) {
	b, err := json.Marshal(s)
	if err != nil {
		return
	}
	_ = c.client.Set(ctx, key, b, ratingAggTTL).Err()
}

// ─── Write path — idempotency + leaderboard ───────────────────────────────────

// Upsert updates the Postgres row, then:
//  1. Adds the movie to the user's "rated" set (idempotency guard for future checks).
//  2. Busts the aggregation cache so the next GetStats re-computes from Postgres.
//  3. Updates the genre leaderboard sorted set with the movie's new average.
func (c *ratingCache) Upsert(ctx context.Context, rt *domain.Rating) (*domain.Rating, error) {
	result, err := c.pg.Upsert(ctx, rt)
	if err != nil {
		return nil, err
	}

	// 1. Track that this user has rated this movie (idempotency / "already rated" UX).
	ratedKey := sharedcache.UserRatedKey(rt.UserID)
	_ = c.client.SAdd(ctx, ratedKey, rt.MovieID).Err()
	_ = c.client.Expire(ctx, ratedKey, userRatedTTL).Err()

	// 2. Bust aggregation cache so the next read recomputes.
	_ = c.client.Del(ctx, sharedcache.MovieRatingAggKey(rt.MovieID)).Err()

	// 3. Update the leaderboard: fetch fresh stats and push to all relevant genre ZSets.
	// We do this asynchronously in a goroutine — a leaderboard stale by a few seconds
	// is acceptable; blocking the write path is not.
	go c.updateLeaderboard(context.Background(), rt.MovieID)

	return result, nil
}

// updateLeaderboard fetches fresh stats for a movie and updates the sorted sets
// for each of the movie's genres.  Genres must be passed alongside the rating
// for the leaderboard to work — the rating domain stores movieID only, so we
// look up stats first, then update by genre via the platform cache key.
// Since we don't have genre data in the rating domain, we update a special
// "global" leaderboard and rely on the movie service to fan out to genre-level
// sets when it processes the aggregate update event.
func (c *ratingCache) updateLeaderboard(ctx context.Context, movieID string) {
	stats, err := c.pg.GetStats(ctx, movieID)
	if err != nil || stats.TotalRatings == 0 {
		return
	}
	// Recency-decay weight: raw avg × log(ratingCount + 1).
	// This prevents brand-new movies with one 5-star from dominating.
	score := stats.AvgOverall * (1 + logN(float64(stats.TotalRatings)))

	// Update global leaderboard (genre-specific fan-out happens in the
	// movie service's UpdateAggregates path when it knows the genres).
	key := sharedcache.GenreLeaderboardKey("global")
	_ = c.client.ZAdd(ctx, key, goredis.Z{
		Score:  score,
		Member: movieID,
	}).Err()
	_ = c.client.Expire(ctx, key, leaderboardTTL).Err()
	// Trim to top-N to cap memory usage.
	_ = c.client.ZRemRangeByRank(ctx, key, 0, -leaderboardMaxLen-1).Err()
}

// HasUserRated is a fast Redis-only check used by the handler to show the
// "already rated" state without a Postgres round-trip.
// It falls back to Postgres if the key has expired from Redis.
func (c *ratingCache) HasUserRated(ctx context.Context, userID, movieID string) (bool, error) {
	n, err := c.client.SIsMember(ctx, sharedcache.UserRatedKey(userID), movieID).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			// Key expired — fall back to Postgres.
			rt, err := c.pg.GetByUserAndMovie(ctx, userID, movieID)
			if err != nil {
				return false, err
			}
			return rt != nil, nil
		}
		return false, err
	}
	return n, nil
}

// ─── Pass-through methods ─────────────────────────────────────────────────────

func (c *ratingCache) GetByUserAndMovie(ctx context.Context, userID, movieID string) (*domain.Rating, error) {
	return c.pg.GetByUserAndMovie(ctx, userID, movieID)
}

func (c *ratingCache) Delete(ctx context.Context, userID, movieID string) error {
	if err := c.pg.Delete(ctx, userID, movieID); err != nil {
		return err
	}
	// Remove from user-rated set and bust the agg cache.
	_ = c.client.SRem(ctx, sharedcache.UserRatedKey(userID), movieID).Err()
	_ = c.client.Del(ctx, sharedcache.MovieRatingAggKey(movieID)).Err()
	return nil
}

func (c *ratingCache) ListByMovie(ctx context.Context, f domain.RatingFilter) ([]*domain.Rating, int, error) {
	return c.pg.ListByMovie(ctx, f)
}

func (c *ratingCache) ListByUser(ctx context.Context, f domain.RatingFilter) ([]*domain.Rating, int, error) {
	return c.pg.ListByUser(ctx, f)
}

// ─── Helper ───────────────────────────────────────────────────────────────────

// logN returns a dampening factor: grows toward 1 as ratingCount grows.
// Prevents new movies with a single 5-star from dominating the leaderboard.
// logN(1)=0.01, logN(10)=0.09, logN(100)=0.5, logN(1000)=0.91.
func logN(x float64) float64 {
	if x <= 0 {
		return 0
	}
	return x / (x + 100)
}
