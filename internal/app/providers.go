// package app

// import (
// 	"context"
// 	"fmt"
// 	"net/http"

// 	"github.com/Ab-dex/view-aura/internal/app/middleware"
// 	"github.com/Ab-dex/view-aura/internal/contract"
// 	"github.com/Ab-dex/view-aura/internal/modules/user/service"
// 	"github.com/Ab-dex/view-aura/internal/platform/cache"
// 	"github.com/Ab-dex/view-aura/internal/platform/config"
// 	"github.com/Ab-dex/view-aura/internal/platform/db"
// 	"github.com/gin-gonic/gin"
// )

// func ProvideDB(ctx context.Context, cfg *config.Config) (*db.Pool, error) {
// 	return db.New(ctx, cfg.DB)
// }

// func ProvideHTTPServer(cfg *config.Config, handler http.Handler) *http.Server {
// 	return &http.Server{
// 		Addr:         cfg.HTTP.Host + ":" + fmt.Sprintf("%d", cfg.HTTP.Port),
// 		Handler:      handler,
// 		ReadTimeout:  cfg.HTTP.ReadTimeout,
// 		WriteTimeout: cfg.HTTP.WriteTimeout,
// 		IdleTimeout:  cfg.HTTP.IdleTimeout,
// 	}
// }

// func ProvideHTTPHandler(r *gin.Engine) http.Handler {
// 	return r
// }

// func ProvideRedis(ctx context.Context, cfg *config.Config) (*cache.Client, error) {
// 	return cache.New(ctx, cfg.Redis)
// }

// func ProvideAuthMiddleware(tokens service.TokenService) contract.AuthMiddleware {
// 	return contract.AuthMiddleware(middleware.Auth(tokens))
// }

// func ProvideRequestIDMiddleware() contract.RequestIDMiddleware {
// 	return contract.RequestIDMiddleware(middleware.RequestID())
// }

// func ProvideRecoveryMiddleware() contract.RecoveryMiddleware {
// 	return contract.RecoveryMiddleware(middleware.Recovery())
// }

// func ProvideLoggerMiddleware() contract.LoggerMiddleware {
// 	return contract.LoggerMiddleware(middleware.StructuredLogger())
// }

package app

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/app/middleware"
	"github.com/Ab-dex/view-aura/internal/contract"
	"github.com/Ab-dex/view-aura/internal/modules/user/service"
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

func ProvideAuthMiddleware(tokens service.TokenService) contract.AuthMiddleware {
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
