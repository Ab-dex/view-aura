package service

import (
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/events"
	"github.com/Ab-dex/view-aura/internal/modules/payment/repository"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	stripepkg "github.com/Ab-dex/view-aura/internal/platform/stripe"
)

func ProvideStripeAdapter(client *stripepkg.Client) StripeAdapter {
	return NewStripeAdapter(client)
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
