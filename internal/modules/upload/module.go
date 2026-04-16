package upload

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/contract"
	"github.com/Ab-dex/view-aura/internal/modules/upload/api"
	handler "github.com/Ab-dex/view-aura/internal/modules/upload/api/http"
	"github.com/Ab-dex/view-aura/internal/modules/upload/repository"
	"github.com/Ab-dex/view-aura/internal/modules/upload/service"
)

var _ contract.Module = (*Module)(nil)

type Module struct {
	Handler *handler.UploadHandler
	Auth    gin.HandlerFunc
}

func NewModule(h *handler.UploadHandler, auth gin.HandlerFunc) *Module {
	return &Module{Handler: h, Auth: auth}
}

func (m *Module) Register(r gin.IRouter) {
	protected := r.Group("")
	protected.Use(m.Auth)
	m.Handler.RegisterProtectedRoutes(protected)
}

func ProvideUploadModule(h *handler.UploadHandler, auth contract.AuthMiddleware) contract.UploadModule {
	return contract.UploadModule{
		Module: NewModule(h, gin.HandlerFunc(auth)),
	}
}

var UploadModuleSet = wire.NewSet(
	repository.ProviderSet,
	service.ProviderSet,
	api.ProviderSet,
	ProvideUploadModule,
)
