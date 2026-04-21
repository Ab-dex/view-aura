package repository

import (
	"github.com/google/wire"

	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
)

func ProvideUserRepository(pool *shareddb.Pool) UserRepository {
	return NewUserRepository(pool.Pool)
}

func ProvideProfileRepository(pool *shareddb.Pool) ProfileRepository {
	return NewProfileRepository(pool.Pool)
}

func ProvidePreferencesRepository(pool *shareddb.Pool) PreferencesRepository {
	return NewPreferencesRepository(pool.Pool)
}

var ProviderSet = wire.NewSet(
	ProvideUserRepository,
	ProvideProfileRepository,
	ProvidePreferencesRepository,
)
