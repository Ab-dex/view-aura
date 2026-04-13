package repository

import (
	"github.com/google/wire"

	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
)

func provideFollowRepository(pool *shareddb.Pool) FollowRepository {
	return NewFollowRepository(pool.Pool)
}

// provideActivityRepository wraps the Postgres implementation with the Redis
// feed cache. The FollowRepository is injected so the cache can fan-out new
// activities to all follower feeds on write.
func provideActivityRepository(
	pool *shareddb.Pool,
	follows FollowRepository,
	redis *sharedcache.Client,
) ActivityRepository {
	pg := NewActivityRepository(pool.Pool)
	return NewCachedActivityRepository(pg, follows, redis)
}

func provideThreadRepository(pool *shareddb.Pool) ThreadRepository {
	return NewThreadRepository(pool.Pool)
}

func providePostRepository(pool *shareddb.Pool) PostRepository {
	return NewPostRepository(pool.Pool)
}

func provideChallengeRepository(pool *shareddb.Pool) ChallengeRepository {
	return NewChallengeRepository(pool.Pool)
}

var ProviderSet = wire.NewSet(
	provideFollowRepository,
	provideActivityRepository,
	provideThreadRepository,
	providePostRepository,
	provideChallengeRepository,
)
