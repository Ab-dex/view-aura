package repository

import (
	"github.com/google/wire"

	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
)

func ProvideUploadSessionRepository(client *sharedcache.Client) UploadSessionRepository {
	return NewUploadSessionRepository(client)
}

func ProvideMediaAssetRepository(pool *shareddb.Pool) MediaAssetRepository {
	return NewMediaAssetRepository(pool.Pool)
}

var ProviderSet = wire.NewSet(
	ProvideUploadSessionRepository,
	ProvideMediaAssetRepository,
)
