package repository

import (
	"github.com/google/wire"

	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
)

func ProvideFollowRepository(pool *shareddb.Pool) FollowRepository {
	return NewFollowRepository(pool.Pool)
}

// ProvideActivityRepository wraps the Postgres implementation with the Redis
// feed cache. The FollowRepository is injected so the cache can fan-out new
// activities to all follower feeds on write.
func ProvideActivityRepository(
	pool *shareddb.Pool,
	follows FollowRepository,
	redis *sharedcache.Client,
) ActivityRepository {
	pg := NewActivityRepository(pool.Pool)
	return NewCachedActivityRepository(pg, follows, redis)
}

func ProvideThreadRepository(pool *shareddb.Pool) ThreadRepository {
	return NewThreadRepository(pool.Pool)
}

func ProvidePostRepository(pool *shareddb.Pool) PostRepository {
	return NewPostRepository(pool.Pool)
}

func ProvideChallengeRepository(pool *shareddb.Pool) ChallengeRepository {
	return NewChallengeRepository(pool.Pool)
}

var ProviderSet = wire.NewSet(
	ProvideFollowRepository,
	ProvideActivityRepository,
	ProvideThreadRepository,
	ProvidePostRepository,
	ProvideChallengeRepository,
)
