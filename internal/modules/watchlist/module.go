package watchlist

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/contract"
	"github.com/Ab-dex/view-aura/internal/modules/watchlist/api"
	handler "github.com/Ab-dex/view-aura/internal/modules/watchlist/api/http"
	"github.com/Ab-dex/view-aura/internal/modules/watchlist/repository"
	"github.com/Ab-dex/view-aura/internal/modules/watchlist/service"
)

var _ contract.Module = (*Module)(nil)

type Module struct {
	Handler *handler.WatchlistHandler
	Auth    gin.HandlerFunc
}

func NewModule(h *handler.WatchlistHandler, auth gin.HandlerFunc) *Module {
	return &Module{Handler: h, Auth: auth}
}

func (m *Module) Register(r gin.IRouter) {
	// Public — list detail by share slug (no auth needed for public lists).
	m.Handler.RegisterPublicRoutes(r)

	// All write operations and personal watchlist require auth.
	protected := r.Group("")
	protected.Use(m.Auth)
	m.Handler.RegisterProtectedRoutes(protected)
}

func ProvideWatchlistModule(h *handler.WatchlistHandler, auth contract.AuthMiddleware) contract.Module {
	return NewModule(h, gin.HandlerFunc(auth))
}

var WatchlistModuleSet = wire.NewSet(
	repository.ProviderSet,
	service.ProviderSet,
	api.ProviderSet,
	ProvideWatchlistModule,
)
