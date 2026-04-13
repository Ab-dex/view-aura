package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	// Side-effect import: registers the generated docs with the swagger runtime.
	// _ "github.com/Ab-dex/view-aura/docs"

	"github.com/Ab-dex/view-aura/internal/contract"
	"github.com/Ab-dex/view-aura/internal/platform/cache"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	"github.com/Ab-dex/view-aura/internal/platform/db"
)

type App struct {
	Config  *config.Config
	DB      *db.Pool
	Redis   *cache.Client
	Server  *http.Server
	Modules []contract.Module
	Hooks   *Hooks
}

func New(
	cfg *config.Config,
	db *db.Pool,
	redis *cache.Client,
	server *http.Server,
	modules []contract.Module,
) (*App, error) {
	return &App{
		Config:  cfg,
		DB:      db,
		Redis:   redis,
		Server:  server,
		Modules: modules,
		Hooks:   &Hooks{},
	}, nil
}

func NewRouter(
	cfg *config.Config,
	modules []contract.Module,
	reqID contract.RequestIDMiddleware,
	log contract.LoggerMiddleware,
	recover contract.RecoveryMiddleware,
) *gin.Engine {
	r := gin.New()
	r.Use(gin.HandlerFunc(recover))
	r.Use(gin.HandlerFunc(reqID))
	r.Use(gin.HandlerFunc(log))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": cfg.App.Name,
			"version": cfg.App.Version,
			"env":     cfg.App.Env,
		})
	})

	// ── Swagger UI ────────────────────────────────────────────────────────────
	// Exposed on local and staging only. Never in production — it reveals
	// the full API surface and is not rate-limited.
	if cfg.App.Env != "production" {
		// GET /swagger/index.html  → interactive Swagger UI
		// GET /swagger/doc.json    → raw OpenAPI JSON (useful for code-gen tools)
		r.GET("/swagger/*any", ginSwagger.WrapHandler(
			swaggerFiles.Handler,
			ginSwagger.URL("/swagger/doc.json"),
			ginSwagger.DefaultModelsExpandDepth(-1), // collapse schema models by default
		))
	}

	api := r.Group("/api/v1")
	for _, m := range modules {
		m.Register(api)
	}

	return r
}
