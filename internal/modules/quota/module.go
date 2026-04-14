package quota

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/contract"
	"github.com/Ab-dex/view-aura/internal/modules/quota/api"
	handler "github.com/Ab-dex/view-aura/internal/modules/quota/api/http"
	"github.com/Ab-dex/view-aura/internal/modules/quota/repository"
	"github.com/Ab-dex/view-aura/internal/modules/quota/service"
)

var _ contract.Module = (*Module)(nil)

type Module struct {
	Handler *handler.QuotaHandler
	Auth    gin.HandlerFunc
}

func NewModule(h *handler.QuotaHandler, auth gin.HandlerFunc) *Module {
	return &Module{Handler: h, Auth: auth}
}

func (m *Module) Register(r gin.IRouter) {
	protected := r.Group("")
	protected.Use(m.Auth)
	m.Handler.RegisterProtectedRoutes(protected)
}

func ProvideQuotaModule(h *handler.QuotaHandler, auth contract.AuthMiddleware) contract.Module {
	return NewModule(h, gin.HandlerFunc(auth))
}

var QuotaModuleSet = wire.NewSet(
	repository.ProviderSet,
	service.ProviderSet,
	api.ProviderSet,
	ProvideQuotaModule,
)
