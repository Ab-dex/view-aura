package app

import (
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	ProvideDB,
	ProvideRedis,
	ProvideAuthMiddleware,
	ProvideRequestIDMiddleware,
	ProvideRecoveryMiddleware,
	ProvideLoggerMiddleware,
	NewRouter,
	ProvideHTTPServer,
	New,
)
