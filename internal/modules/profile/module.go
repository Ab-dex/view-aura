package profile

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/contract"
	api "github.com/Ab-dex/view-aura/internal/modules/profile/api"
	handler "github.com/Ab-dex/view-aura/internal/modules/profile/api/http"
	"github.com/Ab-dex/view-aura/internal/modules/profile/repository"
	"github.com/Ab-dex/view-aura/internal/modules/profile/service"
)

// Ensure Module implements the contract.Module interface
var _ contract.Module = (*Module)(nil)

type Module struct {
	Handler *handler.ProfileHandler
	Auth    gin.HandlerFunc
}

func NewModule(h *handler.ProfileHandler, auth gin.HandlerFunc) *Module {
	return &Module{
		Handler: h,
		Auth:    auth,
	}
}

// Register sets up the profile routes.
// Note: Your handler already creates a "/me/profiles" group internally.
func (m *Module) Register(r gin.IRouter) {
	// Profiles are strictly private/protected in a streaming app
	protected := r.Group("")
	protected.Use(m.Auth)

	m.Handler.RegisterProfileRoutes(protected)
}

// ProvideProfileModule is the wire provider function
func ProvideProfileModule(
	handler *handler.ProfileHandler,
	auth contract.AuthMiddleware,
) contract.Module {
	return NewModule(handler, gin.HandlerFunc(auth))
}

// ProfileModuleSet bundles all dependencies for Google Wire
var ProfileModuleSet = wire.NewSet(
	repository.ProviderSet,
	service.ProviderSet,
	api.ProviderSet,
	ProvideProfileModule,
)
