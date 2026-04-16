package repository

import (
	"github.com/google/wire"

	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
)

// provideMovieRepository builds the Postgres impl and wraps it with the Redis
// read-through cache.  All other repositories are pass-through (no caching).
func ProvideMovieRepository(pool *shareddb.Pool, redis *sharedcache.Client) MovieRepository {
	pg := NewMovieRepository(pool.Pool)
	return NewCachedMovieRepository(pg, redis)
}

func ProvideGenreRepository(pool *shareddb.Pool) GenreRepository {
	return NewGenreRepository(pool.Pool)
}

func ProvidePersonRepository(pool *shareddb.Pool) PersonRepository {
	return NewPersonRepository(pool.Pool)
}

func ProvideCreditRepository(pool *shareddb.Pool) CreditRepository {
	return NewCreditRepository(pool.Pool)
}

func ProvideStreamingLinkRepository(pool *shareddb.Pool) StreamingLinkRepository {
	return NewStreamingLinkRepository(pool.Pool)
}

func ProvideFilmingLocationRepository(pool *shareddb.Pool) FilmingLocationRepository {
	return NewFilmingLocationRepository(pool.Pool)
}

var ProviderSet = wire.NewSet(
	ProvideMovieRepository,
	ProvideGenreRepository,
	ProvidePersonRepository,
	ProvideCreditRepository,
	ProvideStreamingLinkRepository,
	ProvideFilmingLocationRepository,
)
