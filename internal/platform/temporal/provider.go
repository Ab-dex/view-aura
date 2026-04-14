package temporal

import (
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/platform/config"
)

func provideTemporalClient(cfg *config.Config) (Client, error) {
	return New(cfg.Temporal)
}

var ProviderSet = wire.NewSet(provideTemporalClient)
