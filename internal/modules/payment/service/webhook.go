package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	stripego "github.com/stripe/stripe-go/v76"

	"github.com/Ab-dex/view-aura/internal/events"
	"github.com/Ab-dex/view-aura/internal/modules/payment/domain"
	"github.com/Ab-dex/view-aura/internal/modules/payment/repository"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// WebhookService handles inbound Stripe webhook events.
// It is a separate service from PaymentService because:
//   - The webhook endpoint is unauthenticated (verified by signature).
//   - Idempotency is critical — Stripe delivers events at least once.
//   - The handler must respond within 30 seconds or Stripe retries.
type WebhookService interface {
	Handle(ctx context.Context, payload []byte, sigHeader string) error
}

type webhookService struct {
	subs        repository.SubscriptionRepository
	invoices    repository.InvoiceRepository
	idempotency repository.IdempotencyRepository
	stripe      StripeAdapter
	producer    events.Producer
	secret      string
}

func NewWebhookService(
	subs repository.SubscriptionRepository,
	invoices repository.InvoiceRepository,
	idempotency repository.IdempotencyRepository,
	stripe StripeAdapter,
	producer events.Producer,
	secret string,
) WebhookService {
	return &webhookService{
		subs:        subs,
		invoices:    invoices,
		idempotency: idempotency,
		stripe:      stripe,
		producer:    producer,
		secret:      secret,
	}
}

// Handle verifies the Stripe signature, guards against duplicates, then
// dispatches to the appropriate event handler.
func (s *webhookService) Handle(ctx context.Context, payload []byte, sigHeader string) error {
	log := logger.FromContext(ctx)

	event, err := s.stripe.ConstructWebhookEvent(payload, sigHeader, s.secret)
	if err != nil {
		return apierror.New(400, apierror.CodeWebhookInvalid, "invalid webhook signature")
	}

	// Idempotency: skip if we've already processed this event.
	isNew, err := s.idempotency.MarkSeen(ctx, event.ID)
	if err != nil || !isNew {
		log.Info().Str("stripe_event_id", event.ID).Msg("payment: duplicate webhook — skipped")
		return nil
	}

	log.Info().Str("type", string(event.Type)).Str("event_id", event.ID).Msg("payment: processing webhook")

	switch event.Type {
	case "invoice.payment_succeeded":
		return s.handleInvoicePaid(ctx, event)
	case "invoice.payment_failed":
		return s.handleInvoiceFailed(ctx, event)
	case "customer.subscription.deleted":
		return s.handleSubscriptionDeleted(ctx, event)
	case "customer.subscription.updated":
		return s.handleSubscriptionUpdated(ctx, event)
	default:
		// Acknowledge receipt without processing — prevents Stripe retries.
		return nil
	}
}

// ─── Event handlers ───────────────────────────────────────────────────────────

func (s *webhookService) handleInvoicePaid(ctx context.Context, event *stripego.Event) error {
	var inv stripego.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return err
	}

	existing, err := s.invoices.GetByStripeInvoiceID(ctx, inv.ID)
	if err != nil {
		return err
	}

	now := time.Now()
	if existing != nil {
		existing.Status = domain.InvoicePaid
		existing.PaidAt = &now
		if _, err := s.invoices.Update(ctx, existing); err != nil {
			return err
		}
		_ = s.publishPaymentSucceeded(ctx, existing, now)
		return nil
	}

	// Invoice doesn't exist yet — create it.
	subID := ""
	if inv.Subscription != nil {
		subID = inv.Subscription.ID
	}
	newInv := &domain.Invoice{
		ID:              uuid.New().String(),
		StripeInvoiceID: inv.ID,
		AmountCents:     inv.AmountPaid,
		Currency:        string(inv.Currency),
		Status:          domain.InvoicePaid,
		SubscriptionID:  subID,
		PaidAt:          &now,
	}
	created, err := s.invoices.Create(ctx, newInv)
	if err != nil {
		return err
	}
	_ = s.publishPaymentSucceeded(ctx, created, now)
	return nil
}

func (s *webhookService) handleInvoiceFailed(ctx context.Context, event *stripego.Event) error {
	var inv stripego.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return err
	}
	if inv.Subscription == nil {
		return nil
	}

	sub, err := s.subs.GetByStripeSubID(ctx, inv.Subscription.ID)
	if err != nil {
		return err
	}
	sub.Status = domain.SubStatusPastDue
	if _, err := s.subs.Update(ctx, sub); err != nil {
		return err
	}

	_ = s.producer.Publish(ctx, events.TopicPaymentEvents, "payment.payment_failed", events.PaymentFailed{
		InvoiceID:      inv.ID,
		SubscriptionID: sub.ID,
		UserID:         sub.UserID,
		AmountCents:    inv.AmountDue,
		Currency:       string(inv.Currency),
		FailureReason:  string(inv.LastFinalizationError.Code),
		FailedAt:       time.Now(),
	})
	return nil
}

func (s *webhookService) handleSubscriptionDeleted(ctx context.Context, event *stripego.Event) error {
	var stripeSub stripego.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		return err
	}

	sub, err := s.subs.GetByStripeSubID(ctx, stripeSub.ID)
	if err != nil {
		return err
	}
	sub.Status = domain.SubStatusCancelled
	if _, err := s.subs.Update(ctx, sub); err != nil {
		return err
	}

	_ = s.producer.Publish(ctx, events.TopicPaymentEvents, "payment.subscription_expired", events.SubscriptionExpired{
		SubscriptionID: sub.ID,
		UserID:         sub.UserID,
		Plan:           string(sub.Plan),
		ExpiredAt:      time.Now(),
	})
	return nil
}

func (s *webhookService) handleSubscriptionUpdated(ctx context.Context, event *stripego.Event) error {
	var stripeSub stripego.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		return err
	}

	sub, err := s.subs.GetByStripeSubID(ctx, stripeSub.ID)
	if err != nil {
		return err
	}
	sub.Status = domain.SubscriptionStatus(stripeSub.Status)
	sub.CurrentPeriodEnd = time.Unix(stripeSub.CurrentPeriodEnd, 0)
	_, err = s.subs.Update(ctx, sub)
	return err
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (s *webhookService) publishPaymentSucceeded(ctx context.Context, inv *domain.Invoice, paidAt time.Time) error {
	return s.producer.Publish(ctx, events.TopicPaymentEvents, "payment.payment_succeeded", events.PaymentSucceeded{
		InvoiceID:       inv.ID,
		SubscriptionID:  inv.SubscriptionID,
		UserID:          inv.UserID,
		AmountCents:     inv.AmountCents,
		Currency:        inv.Currency,
		StripeInvoiceID: inv.StripeInvoiceID,
		PaidAt:          paidAt,
	})
}
