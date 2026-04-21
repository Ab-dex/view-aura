package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/Ab-dex/view-aura/internal/app/middleware"
	"github.com/Ab-dex/view-aura/internal/contract"
	"github.com/Ab-dex/view-aura/internal/platform/cache"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	"github.com/Ab-dex/view-aura/internal/platform/db"
)

// App is the root application object that owns all live resources.
type App struct {
	Config  *config.Config
	DB      *db.Pool
	Redis   *cache.Client // nil when Redis is not configured — all code must guard
	Server  *http.Server
	Modules []contract.Module
	Hooks   *Hooks
}

// New assembles the HTTP server and registers all module routes.
// redis may be nil — the app degrades gracefully without it.
func New(
	cfg *config.Config,
	dbPool *db.Pool,
	redis *cache.Client,
	router *gin.Engine,
	modules []contract.Module,
) (*App, error) {
	api := router.Group("/api/v1")
	for _, m := range modules {
		m.Register(api)
	}

	server := &http.Server{
		Addr:         cfg.HTTP.Host + ":" + fmt.Sprintf("%d", cfg.HTTP.Port),
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	return &App{
		Config:  cfg,
		DB:      dbPool,
		Redis:   redis,
		Server:  server,
		Modules: modules,
		Hooks:   &Hooks{},
	}, nil
}

// NewRouter builds the Gin engine with all platform middleware applied.
// redis is nullable — rate limiting and HTTP caching degrade gracefully when nil.
func NewRouter(
	cfg *config.Config,
	redis *cache.Client,
	reqID contract.RequestIDMiddleware,
	log contract.LoggerMiddleware,
	recover contract.RecoveryMiddleware,
) *gin.Engine {
	r := gin.New()
	r.Use(gin.HandlerFunc(recover))
	r.Use(gin.HandlerFunc(reqID))
	r.Use(gin.HandlerFunc(log))

	// Rate limiting — degrades to no-op when Redis is nil (fail open).
	var rdbClient interface { /* goredis.UniversalClient */
	} = nil
	if redis != nil {
		r.Use(middleware.RateLimit(redis.Client, 1000, time.Minute))
		r.Use(middleware.HTTPCache(redis, 2*time.Minute))
	} else {
		// Without Redis: skip distributed rate limit and HTTP cache.
		// StrictRateLimit on individual routes still works via the in-process
		// token bucket fallback added to middleware.RateLimit below.
		_ = rdbClient
	}

	r.GET("/health", func(c *gin.Context) {
		status := gin.H{
			"status":  "ok",
			"service": cfg.App.Name,
			"version": cfg.App.Version,
			"env":     cfg.App.Env,
		}
		if redis == nil {
			status["degraded"] = []string{"redis_unavailable"}
		}
		c.JSON(http.StatusOK, status)
	})

	if cfg.App.Env != "production" {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(
			swaggerFiles.Handler,
			ginSwagger.URL("/swagger/doc.json"),
			ginSwagger.DefaultModelsExpandDepth(-1),
		))
	}

	return r
}
