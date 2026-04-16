package repository

import (
	"github.com/google/wire"

	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
)

func ProvideQuotaRepository(client *sharedcache.Client) QuotaRepository {
	return NewQuotaRepository(client)
}

var ProviderSet = wire.NewSet(ProvideQuotaRepository)
