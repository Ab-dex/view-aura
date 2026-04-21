package service

import (
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/events"
	"github.com/Ab-dex/view-aura/internal/modules/auth/domain"
	authrepo "github.com/Ab-dex/view-aura/internal/modules/auth/repository"
	notifservice "github.com/Ab-dex/view-aura/internal/modules/notification/service"
	userrepo "github.com/Ab-dex/view-aura/internal/modules/user/repository"
	"github.com/Ab-dex/view-aura/internal/platform/config"
)

// ProvideOAuthProviders builds the map of configured social OAuth adapters.
// Providers are only registered when credentials are present (Vault-injected),
// so an unconfigured provider is silently absent rather than causing a panic.
func ProvideOAuthProviders(cfg *config.Config) map[domain.AuthProvider]OAuthProvider {
	providers := map[domain.AuthProvider]OAuthProvider{}
	if cfg.Auth.Google.ClientID != "" {
		providers[domain.AuthProvider(domain.ProviderGoogle)] = NewGoogleProvider(GoogleConfig{
			ClientID:     cfg.Auth.Google.ClientID,
			ClientSecret: cfg.Auth.Google.ClientSecret,
		})
	}
	if cfg.Auth.Apple.ClientID != "" {
		providers[domain.AuthProvider(domain.ProviderApple)] = NewAppleProvider(AppleConfig{
			ClientID:   cfg.Auth.Apple.ClientID,
			TeamID:     cfg.Auth.Apple.TeamID,
			KeyID:      cfg.Auth.Apple.KeyID,
			PrivateKey: cfg.Auth.Apple.PrivateKey,
		})
	}
	return providers
}

// ProvideTokenService constructs the JWT TokenService from config.
func ProvideTokenService(cfg *config.Config) (TokenService, error) {
	return NewTokenService(cfg.JWT)
}

func ProvideAuthService(
	users userrepo.UserRepository,
	profiles userrepo.ProfileRepository,
	prefs userrepo.PreferencesRepository,
	authProviders authrepo.AuthProviderRepository,
	sessions authrepo.SessionRepository,
	security authrepo.SecurityRepository,
	verifyTokens authrepo.VerificationTokenRepository,
	oauthStates authrepo.OAuthStateRepository,
	mfa authrepo.MFARepository,
	oauthProviders map[domain.AuthProvider]OAuthProvider,
	tokens TokenService,
	notifier notifservice.NotificationSender,
	pub events.Producer,
	config *config.Config,
) AuthService {
	return NewAuthService(
		users,
		profiles,
		prefs,
		authProviders,
		sessions,
		security,
		verifyTokens,
		oauthStates,
		mfa,
		oauthProviders,
		tokens,
		notifier,
		pub,
		config,
	)
}

// // ProvideAuthService wires NewAuthService with all scalar config values resolved.
// func ProvideAuthService(
// 	svc interface {
// 		// This indirection lets wire resolve NewAuthService with scalar args
// 		// (appBaseURL, refreshTTL) that cannot be injected as types.
// 	},
// ) AuthService {
// 	panic("use NewAuthService directly via wire.Build")
// }

var ProviderSet = wire.NewSet(
	ProvideOAuthProviders,
	ProvideTokenService,
	NewAuthService,
	// wire.Bind(new(TokenService), new(*tokenService)),
)
