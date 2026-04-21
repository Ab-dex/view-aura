//go:build wireinject

// *build wireinject

package main

import (
	"context"

	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/app"
	"github.com/Ab-dex/view-aura/internal/contract"

	events "github.com/Ab-dex/view-aura/internal/events"
	auth "github.com/Ab-dex/view-aura/internal/modules/auth"
	moderation "github.com/Ab-dex/view-aura/internal/modules/moderation"
	movie "github.com/Ab-dex/view-aura/internal/modules/movie"
	notification "github.com/Ab-dex/view-aura/internal/modules/notification"
	notifservice "github.com/Ab-dex/view-aura/internal/modules/notification/service"
	payment "github.com/Ab-dex/view-aura/internal/modules/payment"
	quota "github.com/Ab-dex/view-aura/internal/modules/quota"
	rating "github.com/Ab-dex/view-aura/internal/modules/rating"
	review "github.com/Ab-dex/view-aura/internal/modules/review"
	social "github.com/Ab-dex/view-aura/internal/modules/social"
	upload "github.com/Ab-dex/view-aura/internal/modules/upload"
	user "github.com/Ab-dex/view-aura/internal/modules/user"
	watchlist "github.com/Ab-dex/view-aura/internal/modules/watchlist"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	r2 "github.com/Ab-dex/view-aura/internal/platform/r2"
	resilience "github.com/Ab-dex/view-aura/internal/platform/resilience"
	stripeclient "github.com/Ab-dex/view-aura/internal/platform/stripe"
)

func ProvideModules(
	authMod contract.AuthModule,
	userMod contract.UserModule,
	movieMod contract.MovieModule,
	ratingMod contract.RatingModule,
	reviewMod contract.ReviewModule,
	watchlistMod contract.WatchlistModule,
	socialMod contract.SocialModule,
	notifMod contract.NotificationModule,
	paymentMod contract.PaymentModule,
	uploadMod contract.UploadModule,
	quotaMod contract.QuotaModule,
	moderationMod contract.ModerationModule,
) []contract.Module {
	return []contract.Module{
		authMod.Module,
		userMod.Module,
		movieMod.Module,
		ratingMod.Module,
		reviewMod.Module,
		watchlistMod.Module,
		socialMod.Module,
		notifMod.Module,
		paymentMod.Module,
		uploadMod.Module,
		quotaMod.Module,
		moderationMod.Module,
	}
}

// ProvideAdminMiddleware builds the HandlersChain injected into modules that
// restrict certain routes to admin / producer roles.
func ProvideAdminMiddleware() contract.AdminMiddleware {
	return contract.AdminMiddleware(nil) // replaced by RequireRole at wire-gen time
}

// ProvideNoopDispatcher satisfies the service.Dispatcher interface for local
// dev and CI. Swap for a real FCM/APNs adapter in production.
func ProvideNoopDispatcher() notifservice.NoopDispatcher {
	return notifservice.NoopDispatcher{}
}

// InitializeApp builds the full application object via Wire.
func InitializeApp(ctx context.Context, cfg *config.Config) (*app.App, error) {
	wire.Build(
		app.ProviderSet,

		stripeclient.ProviderSet,
		r2.ProviderSet,
		// temporal.ProviderSet,
		events.ProviderSet,
		resilience.ProviderSet,

		auth.AuthModuleSet,
		user.UserModuleSet,
		movie.MovieModuleSet,
		rating.RatingModuleSet,
		review.ReviewModuleSet,
		watchlist.WatchlistModuleSet,
		social.SocialModuleSet,
		notification.NotificationModuleSet,
		payment.PaymentModuleSet,
		upload.UploadModuleSet,
		quota.QuotaModuleSet,
		moderation.ModerationModuleSet,

		ProvideAdminMiddleware,
		wire.Bind(new(notifservice.Dispatcher), new(notifservice.NoopDispatcher)),
		ProvideNoopDispatcher,

		ProvideModules,
	)
	return nil, nil
}
