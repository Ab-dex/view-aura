package rating

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/contract"
	api "github.com/Ab-dex/view-aura/internal/modules/rating/api"
	handler "github.com/Ab-dex/view-aura/internal/modules/rating/api/http"
	"github.com/Ab-dex/view-aura/internal/modules/rating/repository"
	"github.com/Ab-dex/view-aura/internal/modules/rating/service"
)

var _ contract.Module = (*Module)(nil)

type Module struct {
	Handler *handler.RatingHandler
	Auth    gin.HandlerFunc
}

func NewModule(h *handler.RatingHandler, auth gin.HandlerFunc) *Module {
	return &Module{Handler: h, Auth: auth}
}

// Register attaches rating routes under the shared router.
// Public stats/list routes are open; write routes require auth.
func (m *Module) Register(r gin.IRouter) {
	m.Handler.RegisterPublicRoutes(r)

	protected := r.Group("")
	protected.Use(m.Auth)
	m.Handler.RegisterProtectedRoutes(protected)
}

func ProvideRatingModule(h *handler.RatingHandler, auth contract.AuthMiddleware) contract.Module {
	return NewModule(h, gin.HandlerFunc(auth))
}

var RatingModuleSet = wire.NewSet(
	repository.ProviderSet,
	service.ProviderSet,
	api.ProviderSet,
	ProvideRatingModule,
)
