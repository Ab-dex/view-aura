package app

import (
	"github.com/google/wire"

	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
)

var ProviderSet = wire.NewSet(
	ProvideDB,
	ProvideRedis,
	ProvideAuthMiddleware,
	ProvideRequestIDMiddleware,
	ProvideRecoveryMiddleware,
	ProvideLoggerMiddleware,
	NewRouter,
	// ProvideHTTPHandler,
	// ProvideHTTPServer,
	New,
	shareddb.NewTxManager,
)
