package repository

import (
	"github.com/google/wire"

	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
)

func ProvideHouseholdProfileRepository(pool *shareddb.Pool) HouseholdProfileRepository {
	return NewHouseholdProfileRepository(pool.Pool)
}

var ProviderSet = wire.NewSet(
	ProvideHouseholdProfileRepository,
)
