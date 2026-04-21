package app

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/app/middleware"
	"github.com/Ab-dex/view-aura/internal/contract"
	authservice "github.com/Ab-dex/view-aura/internal/modules/auth/service"
	"github.com/Ab-dex/view-aura/internal/platform/cache"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	"github.com/Ab-dex/view-aura/internal/platform/db"
)

func ProvideDB(ctx context.Context, cfg *config.Config) (*db.Pool, error) {
	return db.New(ctx, cfg.DB)
}

func ProvideRedis(ctx context.Context, cfg *config.Config) (*cache.Client, error) {
	return cache.New(ctx, cfg.Redis)
}

func ProvideAuthMiddleware(tokens authservice.TokenService) contract.AuthMiddleware {
	return contract.AuthMiddleware(middleware.Auth(tokens))
}

func ProvideRequestIDMiddleware() contract.RequestIDMiddleware {
	return contract.RequestIDMiddleware(middleware.RequestID())
}

func ProvideRecoveryMiddleware() contract.RecoveryMiddleware {
	return contract.RecoveryMiddleware(middleware.Recovery())
}

func ProvideLoggerMiddleware() contract.LoggerMiddleware {
	return contract.LoggerMiddleware(middleware.StructuredLogger())
}
