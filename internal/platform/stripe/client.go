package stripeclient

import (
	"context"

	"github.com/stripe/stripe-go/v76"
	stripeclient "github.com/stripe/stripe-go/v76/client"

	"github.com/Ab-dex/view-aura/internal/platform/config"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// Client wraps the stripe-go API client so callers don't rely on global state.
type Client struct {
	API    *stripeclient.API
	Config config.StripeConfig
}

// New configures and returns a ready Stripe client.
func New(cfg config.StripeConfig) *Client {
	backends := stripe.NewBackends(nil)
	api := &stripeclient.API{}
	api.Init(cfg.SecretKey, &stripe.Backends{
		API:     backends.API,
		Connect: backends.Connect,
		Uploads: backends.Uploads,
	})
	logger.FromContext(context.Background()).Info().
		Str("environment", environmentFromKey(cfg.SecretKey)).
		Msg("stripe: client initialised")

	return &Client{API: api, Config: cfg}
}

func environmentFromKey(key string) string {
	if len(key) > 7 && key[:7] == "sk_live" {
		return "live"
	}
	return "test"
}
