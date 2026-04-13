package repository

import (
	"github.com/google/wire"

	sharedredis "github.com/Ab-dex/view-aura/internal/platform/cache"
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
)

func provideUserRepository(pool *shareddb.Pool) UserRepository {
	return NewUserRepository(pool.Pool)
}

func provideProfileRepository(pool *shareddb.Pool) ProfileRepository {
	return NewProfileRepository(pool.Pool)
}

func providePreferencesRepository(pool *shareddb.Pool) PreferencesRepository {
	return NewPreferencesRepository(pool.Pool)
}

func provideSecurityRepository(pool *shareddb.Pool) SecurityRepository {
	return NewSecurityRepository(pool.Pool)
}

func provideSessionRepository(client *sharedredis.Client) SessionRepository {
	return NewSessionRepository(client)
}
func provideAuthProviderRepository(pool *shareddb.Pool) AuthProviderRepository {
	return NewAuthProviderRepository(pool.Pool)
}

var ProviderSet = wire.NewSet(
	provideUserRepository,
	provideProfileRepository,
	providePreferencesRepository,
	provideSecurityRepository,
	provideSessionRepository,
	provideAuthProviderRepository,
)
