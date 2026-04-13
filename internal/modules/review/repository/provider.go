package repository

import (
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
	"github.com/google/wire"
)

func provideReviewRepository(pool *shareddb.Pool) ReviewRepository {
	return NewReviewRepository(pool.Pool)
}

func provideReviewReactionRepository(pool *shareddb.Pool) ReviewReactionRepository {
	return NewReviewReactionRepository(pool.Pool)
}

func provideReviewReportRepository(pool *shareddb.Pool) ReviewReportRepository {
	return NewReviewReportRepository(pool.Pool)
}

var ProviderSet = wire.NewSet(
	provideReviewRepository,
	provideReviewReactionRepository,
	provideReviewReportRepository,
)
