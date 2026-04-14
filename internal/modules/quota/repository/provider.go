package repository

import (
	"github.com/google/wire"

	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
)

func provideQuotaRepository(client *sharedcache.Client) QuotaRepository {
	return NewQuotaRepository(client)
}

var ProviderSet = wire.NewSet(provideQuotaRepository)
