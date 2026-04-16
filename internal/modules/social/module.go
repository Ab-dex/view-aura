package social

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/contract"
	"github.com/Ab-dex/view-aura/internal/modules/social/api"
	handler "github.com/Ab-dex/view-aura/internal/modules/social/api/http"
	"github.com/Ab-dex/view-aura/internal/modules/social/repository"
	"github.com/Ab-dex/view-aura/internal/modules/social/service"
)

var _ contract.Module = (*Module)(nil)

type Module struct {
	Handler *handler.SocialHandler
	Auth    gin.HandlerFunc
}

func NewModule(h *handler.SocialHandler, auth gin.HandlerFunc) *Module {
	return &Module{Handler: h, Auth: auth}
}

func (m *Module) Register(r gin.IRouter) {
	// Public routes: follower counts, activity streams, threads, challenges.
	m.Handler.RegisterPublicRoutes(r)

	// Protected routes: follow/unfollow, feed, post, react, join challenge.
	protected := r.Group("")
	protected.Use(m.Auth)
	m.Handler.RegisterProtectedRoutes(protected)
}

func ProvideSocialModule(h *handler.SocialHandler, auth contract.AuthMiddleware) contract.SocialModule {
	return contract.SocialModule{
		Module: NewModule(h, gin.HandlerFunc(auth)),
	}
}

var SocialModuleSet = wire.NewSet(
	repository.ProviderSet,
	service.ProviderSet,
	api.ProviderSet,
	ProvideSocialModule,
)
