package app

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

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
	router *gin.Engine,
	modules []contract.Module,
) (*App, error) {
	// Register all module routes onto the router before wrapping in http.Server
	api := router.Group("/api/v1")
	fmt.Printf("All modules: %v", modules)
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
		DB:      db,
		Redis:   redis,
		Server:  server,
		Modules: modules,
		Hooks:   &Hooks{},
	}, nil
}

func NewRouter(
	cfg *config.Config,
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

	if cfg.App.Env != "production" {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(
			swaggerFiles.Handler,
			ginSwagger.URL("/swagger/doc.json"),
			ginSwagger.DefaultModelsExpandDepth(-1),
		))
	}

	return r
}
