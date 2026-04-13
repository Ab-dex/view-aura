package user

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/contract"
	handler "github.com/Ab-dex/view-aura/internal/modules/user/api"
	"github.com/Ab-dex/view-aura/internal/modules/user/repository"
	"github.com/Ab-dex/view-aura/internal/modules/user/service"
)

var _ contract.Module = (*Module)(nil)

type Module struct {
	Handler *handler.UserHandler
	Auth    gin.HandlerFunc
}

func NewModule(h *handler.UserHandler, auth gin.HandlerFunc) *Module {
	return &Module{Handler: h, Auth: auth}
}

func (m *Module) Register(r gin.IRouter) {
	user := r.Group("/users")

	// public routes
	m.Handler.RegisterRoutes(user)

	// protected routes
	protected := user.Group("")
	protected.Use(m.Auth)
	m.Handler.RegisterProtectedRoutes(protected)
}

func ProvideUserModule(
	handler *handler.UserHandler,
	auth contract.AuthMiddleware,
) contract.Module {
	return NewModule(handler, gin.HandlerFunc(auth))
}

var UserModuleSet = wire.NewSet(
	repository.ProviderSet,
	service.ProviderSet,
	handler.ProviderSet,
	ProvideUserModule,
)
