package user

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/contract"
	// Rename to avoid collision with the 'profile' package
	profileapi "github.com/Ab-dex/view-aura/internal/modules/profile/api"
	profilehandler "github.com/Ab-dex/view-aura/internal/modules/profile/api/http"
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
	// 1. Create the base /users group
	userGroup := r.Group("/users")

	// 2. Register Public Routes (Login/Register)
	m.Handler.RegisterRoutes(userGroup)

	// 3. Create Protected Group
	protected := userGroup.Group("")
	protected.Use(m.Auth)

	// 4. Register User Profile management (/users/me)
	m.Handler.RegisterProtectedRoutes(protected)

	// 5. Register Household Profiles (/users/me/profiles)
	m.ProfileHandler.RegisterProfileRoutes(protected)
}

func ProvideUserModule(
	handler *handler.UserHandler,
	profileHandler *profilehandler.ProfileHandler,
	auth contract.AuthMiddleware,
) contract.Module {
	return NewModule(handler, profileHandler, gin.HandlerFunc(auth))
}

var UserModuleSet = wire.NewSet(
	repository.ProviderSet,
	service.ProviderSet,
	handler.ProviderSet,
	profileapi.ProviderSet,
	ProvideUserModule,
)
