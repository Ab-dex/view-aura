package review

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/contract"
	"github.com/Ab-dex/view-aura/internal/modules/review/api"
	handler "github.com/Ab-dex/view-aura/internal/modules/review/api/http"
	"github.com/Ab-dex/view-aura/internal/modules/review/repository"
	"github.com/Ab-dex/view-aura/internal/modules/review/service"
)

var _ contract.Module = (*Module)(nil)

type Module struct {
	Handler *handler.ReviewHandler
	Auth    gin.HandlerFunc
}

func NewModule(h *handler.ReviewHandler, auth gin.HandlerFunc) *Module {
	return &Module{Handler: h, Auth: auth}
}

func (m *Module) Register(r gin.IRouter) {
	m.Handler.RegisterPublicRoutes(r)

	protected := r.Group("")
	protected.Use(m.Auth)
	m.Handler.RegisterProtectedRoutes(protected)
}

func ProvideReviewModule(h *handler.ReviewHandler, auth contract.AuthMiddleware) contract.ReviewModule {
	return contract.ReviewModule{
		Module: NewModule(h, gin.HandlerFunc(auth)),
	}
}

var ReviewModuleSet = wire.NewSet(
	repository.ProviderSet,
	service.ProviderSet,
	api.ProviderSet,
	ProvideReviewModule,
)
