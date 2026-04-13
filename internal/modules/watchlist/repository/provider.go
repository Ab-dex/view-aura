package repository

import (
	"github.com/google/wire"

	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
)

func provideWatchlistRepository(pool *shareddb.Pool, redis *sharedcache.Client) WatchlistRepository {
	pg := NewWatchlistRepository(pool.Pool)
	return NewCachedWatchlistRepository(pg, redis)
}

func provideListRepository(pool *shareddb.Pool) ListRepository {
	return NewListRepository(pool.Pool)
}

var ProviderSet = wire.NewSet(
	provideWatchlistRepository,
	provideListRepository,
)
