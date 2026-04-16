package repository

import (
	"github.com/google/wire"

	sharedredis "github.com/Ab-dex/view-aura/internal/platform/cache"
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

func ProvideSecurityRepository(pool *shareddb.Pool) SecurityRepository {
	return NewSecurityRepository(pool.Pool)
}

func ProvideSessionRepository(client *sharedredis.Client) SessionRepository {
	return NewSessionRepository(client)
}
func ProvideAuthProviderRepository(pool *shareddb.Pool) AuthProviderRepository {
	return NewAuthProviderRepository(pool.Pool)
}

var ProviderSet = wire.NewSet(
	ProvideUserRepository,
	ProvideProfileRepository,
	ProvidePreferencesRepository,
	ProvideSecurityRepository,
	ProvideSessionRepository,
	ProvideAuthProviderRepository,
)
