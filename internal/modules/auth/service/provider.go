package service

import (
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/modules/auth/domain"
	"github.com/Ab-dex/view-aura/internal/platform/config"
)

// ProvideOAuthProviders builds the map of configured social OAuth adapters.
// Providers are only registered when credentials are present (Vault-injected),
// so an unconfigured provider is silently absent rather than causing a panic.
func ProvideOAuthProviders(cfg *config.Config) map[domain.AuthProvider]OAuthProvider {
	providers := map[domain.AuthProvider]OAuthProvider{}
	if cfg.Auth.Google.ClientID != "" {
		providers[domain.ProviderGoogle] = NewGoogleProvider(GoogleConfig{
			ClientID:     cfg.Auth.Google.ClientID,
			ClientSecret: cfg.Auth.Google.ClientSecret,
		})
	}
	if cfg.Auth.Apple.ClientID != "" {
		providers[domain.ProviderApple] = NewAppleProvider(AppleConfig{
			ClientID:   cfg.Auth.Apple.ClientID,
			TeamID:     cfg.Auth.Apple.TeamID,
			KeyID:      cfg.Auth.Apple.KeyID,
			PrivateKey: cfg.Auth.Apple.PrivateKey,
		})
	}
	return providers
}

// ProvideTokenService constructs the JWT TokenService from config.
// Returns TokenService interface directly so no wire.Bind is needed.
func ProvideTokenService(cfg *config.Config) (TokenService, error) {
	return NewTokenService(cfg.JWT)
}

// ProviderSet wires the auth service and its dependencies.
// NewAuthService is wired directly — Wire resolves all parameters including
// the shareddb.TxManager injected from app.ProvideDB → db.NewTxManager.
var ProviderSet = wire.NewSet(
	ProvideOAuthProviders,
	ProvideTokenService,
	NewAuthService,
)
