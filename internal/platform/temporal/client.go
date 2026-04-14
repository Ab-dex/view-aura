package temporal

import (
	"fmt"

	"go.temporal.io/sdk/client"

	"github.com/Ab-dex/view-aura/internal/platform/config"
)

// Client is a type alias so callers import only this package.
type Client = client.Client

// New dials the Temporal server and returns a ready client.
// Call Close() on shutdown — the app lifecycle hooks handle this.
func New(cfg config.TemporalConfig) (Client, error) {
	c, err := client.Dial(client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("temporal: dial %s/%s: %w", cfg.HostPort, cfg.Namespace, err)
	}
	return c, nil
}
