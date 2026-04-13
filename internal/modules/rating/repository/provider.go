package repository

import (
	"github.com/google/wire"

	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
)

func provideRatingRepository(pool *shareddb.Pool, redis *sharedcache.Client) RatingRepository {
	pg := NewRatingRepository(pool.Pool)
	return NewCachedRatingRepository(pg, redis)
}

var ProviderSet = wire.NewSet(provideRatingRepository)
