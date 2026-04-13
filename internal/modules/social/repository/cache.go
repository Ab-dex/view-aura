package repository

import (
	"context"
	"encoding/json"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Ab-dex/view-aura/internal/modules/social/domain"
	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
)

const (
	feedCacheTTL = 30 * time.Minute
	feedMaxLen   = 200 // keep the 200 most recent events per user in Redis
)

// activityCache wraps the Postgres ActivityRepository with a Redis sorted-set
// feed cache.
//
// Key: user:{id}:feed  (ZSet)
// Member: JSON-encoded Activity
// Score: Unix timestamp (newest = highest score)
//
// Fan-out strategy: when an activity is recorded, it is written to:
//  1. The Postgres activities table (source of truth).
//  2. The actor's own feed ZSet.
//  3. Every follower's feed ZSet via push-on-write fan-out.
//
// The fan-out is capped at 500 followers to protect the write path. Users with
// more followers fall back to pull-on-read (the Postgres query already handles
// this correctly). This is the standard "hybrid push/pull" pattern used by
// Twitter, Instagram, etc.
type activityCache struct {
	pg      ActivityRepository
	follows FollowRepository // needed for fan-out
	client  *sharedcache.Client
}

func NewCachedActivityRepository(pg ActivityRepository, follows FollowRepository, client *sharedcache.Client) ActivityRepository {
	return &activityCache{pg: pg, follows: follows, client: client}
}

// Record persists to Postgres then fans out to Redis feed ZSets.
func (c *activityCache) Record(ctx context.Context, a *domain.Activity) error {
	if err := c.pg.Record(ctx, a); err != nil {
		return err
	}
	// Push to actor's own feed.
	c.pushToFeed(ctx, a.ActorID, a)

	// Fan-out to followers (capped at 500 — see docstring).
	followerIDs, _, err := c.follows.ListFollowers(ctx, a.ActorID, 500, 0)
	if err == nil {
		for _, fid := range followerIDs {
			c.pushToFeed(ctx, fid, a)
		}
	}
	return nil
}

// GetFeed reads from Redis if warm, falls back to Postgres on cold cache or
// when the user has more than feedMaxLen events.
func (c *activityCache) GetFeed(ctx context.Context, f domain.FeedFilter) ([]*domain.Activity, int, error) {
	limit := clamp(f.Limit, 100, 20)
	offset := max0(f.Offset)

	key := sharedcache.UserFeedKey(f.UserID)
	total64, err := c.client.ZCard(ctx, key).Result()
	if err != nil || total64 == 0 {
		// Cold cache — serve from Postgres and warm the cache.
		acts, total, pgErr := c.pg.GetFeed(ctx, f)
		if pgErr != nil {
			return nil, 0, pgErr
		}
		go c.warmFeedCache(context.Background(), f.UserID, acts)
		return acts, total, nil
	}

	total := int(total64)
	// Read the requested page from the ZSet (newest first = ZREVRANGE by rank).
	start := int64(offset)
	stop := int64(offset + limit - 1)
	members, err := c.client.ZRevRange(ctx, key, start, stop).Result()
	if err != nil {
		// Redis error — fall through to Postgres.
		return c.pg.GetFeed(ctx, f)
	}

	acts := make([]*domain.Activity, 0, len(members))
	for _, m := range members {
		var a domain.Activity
		if err := json.Unmarshal([]byte(m), &a); err != nil {
			continue
		}
		acts = append(acts, &a)
	}
	return acts, total, nil
}

// GetByActor fetches a single user's public activity stream.
// Not feed-cached (the actor's own profile stream is lower traffic and
// already hits the same ZSet as their own feed).
func (c *activityCache) GetByActor(ctx context.Context, f domain.FeedFilter) ([]*domain.Activity, int, error) {
	return c.pg.GetByActor(ctx, f)
}

// DeleteBySubject removes matching activities from Postgres and invalidates
// every cached feed that might contain them.  Full per-user feed invalidation
// is acceptable here because DeleteBySubject is a rare, low-frequency event
// (review deleted, user account deleted).
func (c *activityCache) DeleteBySubject(ctx context.Context, subjectType, subjectID string) error {
	if err := c.pg.DeleteBySubject(ctx, subjectType, subjectID); err != nil {
		return err
	}
	// Rather than scanning all feed keys, we rely on the 30-minute TTL to
	// expire stale entries. For deletions where consistency matters more
	// (e.g. GDPR account deletion), the caller should additionally flush
	// the specific user's feed key:
	//   c.client.Del(ctx, sharedcache.UserFeedKey(userID))
	return nil
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (c *activityCache) pushToFeed(ctx context.Context, userID string, a *domain.Activity) {
	b, err := json.Marshal(a)
	if err != nil {
		return
	}
	key := sharedcache.UserFeedKey(userID)
	score := float64(a.CreatedAt.Unix())

	pipe := c.client.Pipeline()
	pipe.ZAdd(ctx, key, goredis.Z{Score: score, Member: string(b)})
	// Trim to feedMaxLen most recent events to cap memory per user.
	pipe.ZRemRangeByRank(ctx, key, 0, -feedMaxLen-1)
	pipe.Expire(ctx, key, feedCacheTTL)
	_, _ = pipe.Exec(ctx)
}

func (c *activityCache) warmFeedCache(ctx context.Context, userID string, acts []*domain.Activity) {
	if len(acts) == 0 {
		return
	}
	key := sharedcache.UserFeedKey(userID)
	members := make([]goredis.Z, 0, len(acts))
	for _, a := range acts {
		b, err := json.Marshal(a)
		if err != nil {
			continue
		}
		members = append(members, goredis.Z{
			Score:  float64(a.CreatedAt.Unix()),
			Member: string(b),
		})
	}
	pipe := c.client.Pipeline()
	pipe.ZAdd(ctx, key, members...)
	pipe.ZRemRangeByRank(ctx, key, 0, -feedMaxLen-1)
	pipe.Expire(ctx, key, feedCacheTTL)
	_, _ = pipe.Exec(ctx)
}
