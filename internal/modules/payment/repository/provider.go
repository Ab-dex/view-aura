package repository

import (
	"github.com/google/wire"

	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
)

func provideSubscriptionRepository(pool *shareddb.Pool) SubscriptionRepository {
	return NewSubscriptionRepository(pool.Pool)
}

func provideInvoiceRepository(pool *shareddb.Pool) InvoiceRepository {
	return NewInvoiceRepository(pool.Pool)
}

func provideLicenseRepository(pool *shareddb.Pool) LicenseRepository {
	return NewLicenseRepository(pool.Pool)
}

func provideIdempotencyRepository(client *sharedcache.Client) IdempotencyRepository {
	return NewIdempotencyRepository(client)
}

var ProviderSet = wire.NewSet(
	provideSubscriptionRepository,
	provideInvoiceRepository,
	provideLicenseRepository,
	provideIdempotencyRepository,
)
