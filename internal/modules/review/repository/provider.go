package repository

import (
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
	"github.com/google/wire"
)

func ProvideReviewRepository(pool *shareddb.Pool) ReviewRepository {
	return NewReviewRepository(pool.Pool)
}

func ProvideReviewReactionRepository(pool *shareddb.Pool) ReviewReactionRepository {
	return NewReviewReactionRepository(pool.Pool)
}

func ProvideReviewReportRepository(pool *shareddb.Pool) ReviewReportRepository {
	return NewReviewReportRepository(pool.Pool)
}

var ProviderSet = wire.NewSet(
	ProvideReviewRepository,
	ProvideReviewReactionRepository,
	ProvideReviewReportRepository,
)
