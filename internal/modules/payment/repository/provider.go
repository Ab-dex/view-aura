package repository

import (
	"github.com/google/wire"

	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
)

func ProvideSubscriptionRepository(pool *shareddb.Pool) SubscriptionRepository {
	return NewSubscriptionRepository(pool.Pool)
}

func ProvideInvoiceRepository(pool *shareddb.Pool) InvoiceRepository {
	return NewInvoiceRepository(pool.Pool)
}

func ProvideLicenseRepository(pool *shareddb.Pool) LicenseRepository {
	return NewLicenseRepository(pool.Pool)
}

func ProvideIdempotencyRepository(client *sharedcache.Client) IdempotencyRepository {
	return NewIdempotencyRepository(client)
}

var ProviderSet = wire.NewSet(
	ProvideSubscriptionRepository,
	ProvideInvoiceRepository,
	ProvideLicenseRepository,
	ProvideIdempotencyRepository,
)
