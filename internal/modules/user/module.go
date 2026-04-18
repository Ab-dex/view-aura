package user

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/app/middleware"
	"github.com/Ab-dex/view-aura/internal/contract"
	profileapi "github.com/Ab-dex/view-aura/internal/modules/profile/api"
	profilehandler "github.com/Ab-dex/view-aura/internal/modules/profile/api/http"
	profilerepo "github.com/Ab-dex/view-aura/internal/modules/profile/repository"
	profileservice "github.com/Ab-dex/view-aura/internal/modules/profile/service"
	handler "github.com/Ab-dex/view-aura/internal/modules/user/api"
	"github.com/Ab-dex/view-aura/internal/modules/user/repository"
	"github.com/Ab-dex/view-aura/internal/modules/user/service"
)

var _ contract.Module = (*Module)(nil)

type Module struct {
	Handler        *handler.UserHandler
	ProfileHandler *profilehandler.ProfileHandler
	Auth           gin.HandlerFunc
}

func NewModule(h *handler.UserHandler, ph *profilehandler.ProfileHandler, auth gin.HandlerFunc) *Module {
	return &Module{Handler: h, ProfileHandler: ph, Auth: auth}
}

func (m *Module) Register(r gin.IRouter) {
	userGroup := r.Group("/users")
	userGroup.Use(middleware.StrictRateLimit(100, time.Minute))
	m.Handler.RegisterRoutes(userGroup)

	protected := userGroup.Group("")
	protected.Use(m.Auth)

	m.Handler.RegisterProtectedRoutes(protected)
	m.ProfileHandler.RegisterProfileRoutes(protected)
}

func ProvideUserModule(
	handler *handler.UserHandler,
	profileHandler *profilehandler.ProfileHandler,
	auth contract.AuthMiddleware,
) contract.UserModule {
	return contract.UserModule{
		Module: NewModule(handler, profileHandler, gin.HandlerFunc(auth)),
	}
}

var UserModuleSet = wire.NewSet(
	repository.ProviderSet,
	service.ProviderSet,
	handler.ProviderSet,
	profileapi.ProviderSet,
	profilerepo.ProviderSet,
	profileservice.ProviderSet,
	ProvideUserModule,
)
