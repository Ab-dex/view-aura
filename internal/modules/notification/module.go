package notification

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/contract"
	"github.com/Ab-dex/view-aura/internal/modules/notification/api"
	handler "github.com/Ab-dex/view-aura/internal/modules/notification/api/http"
	"github.com/Ab-dex/view-aura/internal/modules/notification/repository"
	"github.com/Ab-dex/view-aura/internal/modules/notification/service"
)

var _ contract.Module = (*Module)(nil)

type Module struct {
	Handler *handler.NotificationHandler
	Auth    gin.HandlerFunc
}

func NewModule(h *handler.NotificationHandler, auth gin.HandlerFunc) *Module {
	return &Module{Handler: h, Auth: auth}
}

// Register attaches all notification routes under the shared router.
// Every route requires authentication — there are no public notification endpoints.
func (m *Module) Register(r gin.IRouter) {
	protected := r.Group("")
	protected.Use(m.Auth)
	m.Handler.RegisterProtectedRoutes(protected)
}

func ProvideNotificationModule(h *handler.NotificationHandler, auth contract.AuthMiddleware) contract.NotificationModule {
	return contract.NotificationModule{
		Module: NewModule(h, gin.HandlerFunc(auth)),
	}
}

// NotificationModuleSet is the full wire provider set for the notification module.
// The caller must additionally bind a concrete service.Dispatcher implementation:
//
//	wire.Bind(new(service.Dispatcher), new(*MyFCMDispatcher))
//
// or use the no-op for local dev / tests:
//
//	wire.Bind(new(service.Dispatcher), new(*service.NoopDispatcher))
var NotificationModuleSet = wire.NewSet(
	repository.ProviderSet,
	service.ProviderSet,
	api.ProviderSet,
	ProvideNotificationModule,
)
