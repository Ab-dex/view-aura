package movie

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/contract"
	"github.com/Ab-dex/view-aura/internal/modules/movie/api"
	handler "github.com/Ab-dex/view-aura/internal/modules/movie/api/http"
	"github.com/Ab-dex/view-aura/internal/modules/movie/repository"
	"github.com/Ab-dex/view-aura/internal/modules/movie/service"
)

var _ contract.Module = (*Module)(nil)

type Module struct {
	Handler *handler.MovieHandler
	Auth    gin.HandlerFunc
	// AdminAuth is a stricter middleware chain (auth + role=admin/producer).
	// Passed in from the app layer so the module stays dependency-free.
	AdminAuth gin.HandlersChain
}

func NewModule(h *handler.MovieHandler, auth gin.HandlerFunc, adminAuth gin.HandlersChain) *Module {
	return &Module{Handler: h, Auth: auth, AdminAuth: adminAuth}
}

func (m *Module) Register(r gin.IRouter) {
	movies := r.Group("/movies")

	// Public read-only routes — no auth required.
	m.Handler.RegisterRoutes(movies)

	// Write routes require admin or producer role.
	admin := movies.Group("")
	admin.Use(m.Auth)
	admin.Use(m.AdminAuth...)
	m.Handler.RegisterAdminRoutes(admin)
}

// ProvideMovieModule is the Wire provider that assembles the Module from its
// dependencies. AdminAuth receives a pre-built HandlersChain from the app layer
// (e.g. RequireRole("admin", "producer")).
func ProvideMovieModule(
	h *handler.MovieHandler,
	auth contract.AuthMiddleware,
	adminAuth contract.AdminMiddleware,
) contract.Module {
	return NewModule(h, gin.HandlerFunc(auth), gin.HandlersChain(adminAuth))
}

var MovieModuleSet = wire.NewSet(
	repository.ProviderSet,
	service.ProviderSet,
	api.ProviderSet,
	ProvideMovieModule,
)
