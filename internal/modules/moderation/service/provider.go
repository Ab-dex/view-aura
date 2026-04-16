package service

import (
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/platform/config"
)

func ProvideAIClient(cfg *config.Config) AIClient {
	if cfg.ModerationAI.Endpoint == "" {
		return NoopAIClient{}
	}
	return NewAIClient(cfg.ModerationAI)
}

var ProviderSet = wire.NewSet(
	ProvideAIClient,
	NewModerationService,
)
