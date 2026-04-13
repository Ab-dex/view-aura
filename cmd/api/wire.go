package main

import (
	"context"

	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/app"
	"github.com/Ab-dex/view-aura/internal/contract"
	"github.com/Ab-dex/view-aura/internal/modules/movie"
	"github.com/Ab-dex/view-aura/internal/modules/notification"
	notifservice "github.com/Ab-dex/view-aura/internal/modules/notification/service"
	"github.com/Ab-dex/view-aura/internal/modules/rating"
	"github.com/Ab-dex/view-aura/internal/modules/review"
	"github.com/Ab-dex/view-aura/internal/modules/social"
	"github.com/Ab-dex/view-aura/internal/modules/user"
	"github.com/Ab-dex/view-aura/internal/modules/watchlist"
	"github.com/Ab-dex/view-aura/internal/platform/config"
)

func ProvideModules(
	userMod contract.Module,
	movieMod contract.Module,
	ratingMod contract.Module,
	reviewMod contract.Module,
	watchlistMod contract.Module,
	socialMod contract.Module,
	notifMod contract.Module,
) []contract.Module {
	return []contract.Module{
		userMod,
		movieMod,
		ratingMod,
		reviewMod,
		watchlistMod,
		socialMod,
		notifMod,
	}
}

// ProvideAdminMiddleware builds the HandlersChain injected into modules that
// restrict certain routes to admin / producer roles.
func ProvideAdminMiddleware() contract.AdminMiddleware {
	return contract.AdminMiddleware(nil) // replaced by RequireRole at wire-gen time
}

// ProvideNoopDispatcher satisfies the service.Dispatcher interface for local
// dev and CI. Swap for a real FCM/APNs adapter in production.
func ProvideNoopDispatcher() notifservice.Dispatcher {
	return notifservice.NoopDispatcher{}
}

// InitializeApp builds the full application object via Wire.
func InitializeApp(ctx context.Context, cfg *config.Config) (*app.App, error) {
	wire.Build(
		// Core app infrastructure (DB, Redis, HTTP server, router, middlewares).
		app.ProviderSet,

		// Feature modules — each module's ProviderSet is listed here so the
		// graph is self-contained and auditable in one place.
		user.UserModuleSet,
		movie.MovieModuleSet,
		rating.RatingModuleSet,
		review.ReviewModuleSet,
		watchlist.WatchlistModuleSet,
		social.SocialModuleSet,
		notification.NotificationModuleSet,

		ProvideAdminMiddleware,
		ProvideNoopDispatcher,

		// Module aggregator — converts the individual contract.Module bindings
		// into the []contract.Module slice that app.New requires.
		ProvideModules,
	)

	return nil, nil
}
