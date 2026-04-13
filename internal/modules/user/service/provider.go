package service

import (
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/modules/user/repository"
	"github.com/Ab-dex/view-aura/internal/platform/config"
)

func provideTokenService(cfg *config.Config, sessions repository.SessionRepository) (TokenService, error) {
	return NewTokenService(cfg.JWT, sessions)
}

var ProviderSet = wire.NewSet(
	provideTokenService,
	NewUserService,
)
