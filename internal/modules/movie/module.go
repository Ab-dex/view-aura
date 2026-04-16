package movie

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/contract"
	"github.com/Ab-dex/view-aura/internal/modules/movie/api"
	handler "github.com/Ab-dex/view-aura/internal/modules/movie/api/http"
	"github.com/Ab-dex/view-aura/internal/modules/movie/repository"
	"github.com/Ab-dex/view-aura/internal/modules/movie/service"

	// ratingApi "github.com/Ab-dex/view-aura/internal/modules/rating/api"
	ratingHandler "github.com/Ab-dex/view-aura/internal/modules/rating/api/http"
	reviewHandler "github.com/Ab-dex/view-aura/internal/modules/review/api/http"

	socialHandler "github.com/Ab-dex/view-aura/internal/modules/social/api/http"

	paymentHandler "github.com/Ab-dex/view-aura/internal/modules/payment/api/http"
)

var _ contract.Module = (*Module)(nil)

type Module struct {
	Handler        *handler.MovieHandler
	Auth           gin.HandlerFunc
	RatingHandler  *ratingHandler.RatingHandler
	ReviewHandler  *reviewHandler.ReviewHandler
	SocialHandler  *socialHandler.SocialHandler
	PaymentHandler *paymentHandler.PaymentHandler
	AdminAuth      gin.HandlersChain
}

func NewModule(h *handler.MovieHandler, auth gin.HandlerFunc, rh *ratingHandler.RatingHandler, revh *reviewHandler.ReviewHandler, sh *socialHandler.SocialHandler, ph *paymentHandler.PaymentHandler, adminAuth gin.HandlersChain) *Module {
	return &Module{Handler: h, Auth: auth, RatingHandler: rh, ReviewHandler: revh, SocialHandler: sh, PaymentHandler: ph, AdminAuth: adminAuth}
}

func (m *Module) Register(r gin.IRouter) {
	movies := r.Group("/movies")

	// Public read-only routes — no auth required.
	m.Handler.RegisterRoutes(movies)

	// Rating routes are nested under movies and have a mix of public and protected routes.
	ratingGrp := movies.Group("/:id/ratings")
	m.RatingHandler.RegisterPublicMovieRatingRoutes(ratingGrp)

	// Protected rating routes require auth.
	pratingGrp := movies.Group("/:id/rating")
	pratingGrp.Use(m.Auth)
	m.RatingHandler.RegisterProtectedRoutes(pratingGrp)

	// Review routes are also nested under movies, with a similar pattern.
	reviewGrp := movies.Group("/:id/reviews")
	m.ReviewHandler.RegisterPublicMovieRoutes(reviewGrp)

	// Protected review routes require auth.
	previewGrp := movies.Group("/:id/reviews")
	previewGrp.Use(m.Auth)
	m.ReviewHandler.RegisterProtectedMoviesRoutes(previewGrp)

	// Social routes related to movies (e.g. threads) would be registered here.
	// For example:
	socialGrp := movies.Group("/:id/threads")
	m.SocialHandler.RegisterPublicMovieRoutes(socialGrp)

	// Protected social routes require auth.
	psocialGrp := movies.Group("/:id/threads")
	psocialGrp.Use(m.Auth)
	m.SocialHandler.RegisterProtectedMovieRoutes(psocialGrp)

	// Protected payment routes related to movies (e.g. checking access) would be registered here.
	// For example:
	paymentGrp := movies.Group("/:id/payment")
	paymentGrp.Use(m.Auth)
	m.PaymentHandler.RegisterProtectedMovieRoutes(paymentGrp)

	// Write routes require admin or producer role.
	admin := movies.Group("")
	admin.Use(m.Auth)

	admin.Use(m.AdminAuth...)
	m.Handler.RegisterAdminRoutes(admin)
}

// ProvideMovieModule is the Wire provider that assembles the Module from its
// dependencies. AdminAuth receives a pre-built HandlersChain from the app layer
// (e.g. RequireRole("admin", "producer")).
func ProvideMovieModule(
	h *handler.MovieHandler,
	rh *ratingHandler.RatingHandler,
	revh *reviewHandler.ReviewHandler,
	sh *socialHandler.SocialHandler,
	ph *paymentHandler.PaymentHandler,
	auth contract.AuthMiddleware,
	adminAuth contract.AdminMiddleware,

) contract.MovieModule {
	return contract.MovieModule{
		Module: NewModule(h, gin.HandlerFunc(auth), rh, revh, sh, ph, gin.HandlersChain(adminAuth)),
	}
}

var MovieModuleSet = wire.NewSet(
	repository.ProviderSet,
	service.ProviderSet,
	api.ProviderSet,
	ProvideMovieModule,
)
