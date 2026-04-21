package service

import (
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/modules/upload/repository"
	"github.com/Ab-dex/view-aura/internal/platform/cache"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	r2pkg "github.com/Ab-dex/view-aura/internal/platform/r2"
	temporalpkg "github.com/Ab-dex/view-aura/internal/platform/temporal"
)

// ProvideResilientR2 builds the three-tier R2 client:
//
//	Tier 1  R2 presigned URL (real Cloudflare R2)
//	Tier 2  Local temp path  (monolith serves the file during R2 outage)
//	Tier 3  Hard 503         (no silent data loss on upload bytes)
//
// The returned *ResilientR2Client satisfies r2pkg.ObjectStore so it
// drops in wherever *r2pkg.Client was used previously.
func ProvideResilientR2(real *r2pkg.Client, cfg *config.Config) *ResilientR2Client {
	return NewResilientR2Client(
		real,
		cfg.Resilience.R2TempUploadDir,
		cfg.Resilience.R2TempMaxBytes,
		cfg.Auth.BaseURL+"/internal/temp-uploads",
	)
}

// ProvideResilientTemporal builds the three-tier Temporal client:
//
//	Tier 1  Temporal SDK  — starts upload_pipeline workflow
//	Tier 2  Redis queue   — deferred start on recovery
//	Tier 3  File outbox   — observable, manually replayable
//
// temporalClient may be nil when Temporal is not configured.
// rdb may be nil when Redis is not configured.
func ProvideResilientTemporal(
	temporalClient temporalpkg.Client,
	rdb *cache.Client,
	assets repository.MediaAssetRepository,
	cfg *config.Config,
) *ResilientTemporalClient {
	logDir := cfg.Resilience.LogDir
	deferredKey := cfg.Resilience.TemporalDeferredKey
	taskQueue := cfg.Temporal.TaskQueue

	var rdbClient interface{ Pipeline() interface{} }
	_ = rdbClient

	if rdb != nil {
		return NewResilientTemporalClient(
			temporalClient, rdb.Client, assets, logDir, deferredKey, taskQueue,
		)
	}
	return NewResilientTemporalClient(
		temporalClient, nil, assets, logDir, deferredKey, taskQueue,
	)
}

var ProviderSet = wire.NewSet(
	ProvideResilientR2,
	wire.Bind(new(r2pkg.ObjectStore), new(*ResilientR2Client)),
	ProvideResilientTemporal,
	NewUploadService,
)
