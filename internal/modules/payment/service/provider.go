package service

import (
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/events"
	"github.com/Ab-dex/view-aura/internal/modules/payment/repository"
	"github.com/Ab-dex/view-aura/internal/platform/cache"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	stripepkg "github.com/Ab-dex/view-aura/internal/platform/stripe"
)

// ProvideStripeAdapter builds the three-tier Stripe adapter:
//
//	Tier 1  Real Stripe API call
//	Tier 2a Read  → last-known Redis cache
//	Tier 2b Write → Redis pending queue (OutboxWorker retries)
//	Tier 3  Write → FileOutbox; Read → clean error with retry guidance
//
// rdb is nullable — Tier 2 is skipped and all degraded ops go to file.
func ProvideStripeAdapter(
	client *stripepkg.Client,
	rdb *cache.Client,
	cfg *config.Config,
) StripeAdapter {
	real := NewStripeAdapter(client)
	logDir := cfg.Resilience.LogDir
	pendingKey := cfg.Resilience.StripePendingKey

	if rdb != nil {
		return NewResilientStripeAdapter(real, rdb.Client, logDir, pendingKey)
	}
	return NewResilientStripeAdapter(real, nil, logDir, pendingKey)
}

func ProvideWebhookService(
	subs repository.SubscriptionRepository,
	invoices repository.InvoiceRepository,
	idempotency repository.IdempotencyRepository,
	stripe StripeAdapter,
	producer events.Producer,
	cfg *config.Config,
) WebhookService {
	return NewWebhookService(subs, invoices, idempotency, stripe, producer, cfg.Stripe.WebhookSecret)
}

var ProviderSet = wire.NewSet(
	ProvideStripeAdapter,
	NewPaymentService,
	ProvideWebhookService,
)
