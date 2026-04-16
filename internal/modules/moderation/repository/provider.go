package repository

import (
	"github.com/google/wire"

	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
)

func ProvideCaseRepository(pool *shareddb.Pool) CaseRepository {
	return NewCaseRepository(pool.Pool)
}

func ProvideAppealRepository(pool *shareddb.Pool) AppealRepository {
	return NewAppealRepository(pool.Pool)
}

func ProvideLockRepository(client *sharedcache.Client) LockRepository {
	return NewLockRepository(client)
}

var ProviderSet = wire.NewSet(
	ProvideCaseRepository,
	ProvideAppealRepository,
	ProvideLockRepository,
)
