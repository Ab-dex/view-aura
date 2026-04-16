package payment

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/contract"
	"github.com/Ab-dex/view-aura/internal/modules/payment/api"
	handler "github.com/Ab-dex/view-aura/internal/modules/payment/api/http"
	"github.com/Ab-dex/view-aura/internal/modules/payment/repository"
	"github.com/Ab-dex/view-aura/internal/modules/payment/service"
)

var _ contract.Module = (*Module)(nil)

type Module struct {
	Handler *handler.PaymentHandler
	Auth    gin.HandlerFunc
}

func NewModule(h *handler.PaymentHandler, auth gin.HandlerFunc) *Module {
	return &Module{Handler: h, Auth: auth}
}

func (m *Module) Register(r gin.IRouter) {
	// Stripe webhook is unauthenticated — registered on the public router
	// before the auth middleware group.
	m.Handler.RegisterPublicRoutes(r)

	// All other payment routes require a valid JWT.
	protected := r.Group("")
	protected.Use(m.Auth)
	m.Handler.RegisterProtectedRoutes(protected)
}

func ProvidePaymentModule(h *handler.PaymentHandler, auth contract.AuthMiddleware) contract.PaymentModule {
	return contract.PaymentModule{
		Module: NewModule(h, gin.HandlerFunc(auth)),
	}
}

var PaymentModuleSet = wire.NewSet(
	repository.ProviderSet,
	service.ProviderSet,
	api.ProviderSet,
	ProvidePaymentModule,
)
