package moderation

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/contract"
	"github.com/Ab-dex/view-aura/internal/modules/moderation/api"
	handler "github.com/Ab-dex/view-aura/internal/modules/moderation/api/http"
	"github.com/Ab-dex/view-aura/internal/modules/moderation/repository"
	"github.com/Ab-dex/view-aura/internal/modules/moderation/service"
)

var _ contract.Module = (*Module)(nil)

type Module struct {
	Handler *handler.ModerationHandler
	Auth    gin.HandlerFunc
	Admin   gin.HandlersChain
}

func NewModule(h *handler.ModerationHandler, auth gin.HandlerFunc, admin gin.HandlersChain) *Module {
	return &Module{Handler: h, Auth: auth, Admin: admin}
}

func (m *Module) Register(r gin.IRouter) {
	// User-facing: authenticated users can view their own cases and appeal.
	protected := r.Group("")
	protected.Use(m.Auth)
	m.Handler.RegisterProtectedRoutes(protected)

	// Admin queue: requires moderator or admin role.
	admin := r.Group("")
	admin.Use(m.Auth)
	admin.Use(m.Admin...)
	m.Handler.RegisterAdminRoutes(admin)
}

func ProvideModerationModule(
	h *handler.ModerationHandler,
	auth contract.AuthMiddleware,
	admin contract.AdminMiddleware,
) contract.ModerationModule {
	return contract.ModerationModule{
		Module: NewModule(h, gin.HandlerFunc(auth), gin.HandlersChain(admin)),
	}
}

var ModerationModuleSet = wire.NewSet(
	repository.ProviderSet,
	service.ProviderSet,
	api.ProviderSet,
	ProvideModerationModule,
)
