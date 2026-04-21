package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Ab-dex/view-aura/internal/events"
	"github.com/Ab-dex/view-aura/internal/modules/payment/domain"
	"github.com/Ab-dex/view-aura/internal/modules/payment/repository"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// PaymentService handles subscription and pay-per-view operations.
type PaymentService interface {
	Subscribe(ctx context.Context, cmd domain.SubscribeCmd) (*domain.Subscription, error)
	GetSubscription(ctx context.Context, userID string) (*domain.Subscription, error)
	UpgradePlan(ctx context.Context, cmd domain.UpgradePlanCmd) (*domain.Subscription, error)
	CancelSubscription(ctx context.Context, userID string) error
	GetInvoices(ctx context.Context, userID string, limit, offset int) ([]*domain.Invoice, int, error)
	PayPerView(ctx context.Context, cmd domain.PayPerViewCmd) (*domain.ScreenerLicense, error)
	HasAccess(ctx context.Context, userID, movieID string) (bool, error)
	GetLicenses(ctx context.Context, userID string) ([]*domain.ScreenerLicense, error)
}

type paymentService struct {
	subs     repository.SubscriptionRepository
	invoices repository.InvoiceRepository
	licenses repository.LicenseRepository
	stripe   StripeAdapter
	producer events.Producer
	cfg      config.StripeConfig
}

func NewPaymentService(
	subs repository.SubscriptionRepository,
	invoices repository.InvoiceRepository,
	licenses repository.LicenseRepository,
	stripe StripeAdapter,
	producer events.Producer,
	cfg *config.Config,
) PaymentService {
	return &paymentService{
		subs:     subs,
		invoices: invoices,
		licenses: licenses,
		stripe:   stripe,
		producer: producer,
		cfg:      cfg.Stripe,
	}
}

// ─── Subscribe ────────────────────────────────────────────────────────────────

func (s *paymentService) Subscribe(ctx context.Context, cmd domain.SubscribeCmd) (*domain.Subscription, error) {
	if _, err := s.subs.GetByUserID(ctx, cmd.UserID); err == nil {
		return nil, apierror.ErrSubscriptionActive
	}

	priceID := s.priceID(cmd.Plan)
	if priceID == "" {
		return nil, apierror.Validation("invalid plan: "+string(cmd.Plan), nil)
	}

	// ResilientStripeAdapter never returns a hard error — on Stripe outage it
	// returns a "pending_*" sentinel ID and queues the op for replay.
	// We record the subscription locally regardless so the user sees feedback.
	customerID, err := s.stripe.CreateCustomer(ctx, cmd.Email, cmd.Name)
	if err != nil {
		// This should not happen with ResilientStripeAdapter but guard defensively.
		return nil, apierror.UpstreamError("stripe", err)
	}

	stripeSub, err := s.stripe.CreateSubscription(ctx, customerID, priceID, cmd.PaymentMethod)
	if err != nil {
		return nil, apierror.UpstreamError("stripe", err)
	}

	// Determine the effective subscription status.
	// When Stripe is degraded, stripeSub.ID starts with "pending_" and the
	// status is Incomplete — we store that and the webhook will update it later.
	status := domain.SubStatusActive
	if strings.HasPrefix(stripeSub.ID, "pending_") {
		status = domain.SubscriptionStatus(string(stripeSub.Status)) // "incomplete"
	}

	// CurrentPeriodEnd is 0 for pending subs — default to 30 days from now
	// so the UI shows a meaningful date while the webhook hasn't confirmed yet.
	periodEnd := time.Unix(stripeSub.CurrentPeriodEnd, 0)
	if periodEnd.IsZero() || stripeSub.CurrentPeriodEnd == 0 {
		periodEnd = time.Now().Add(30 * 24 * time.Hour)
	}

	sub := &domain.Subscription{
		ID:               uuid.New().String(),
		UserID:           cmd.UserID,
		Plan:             cmd.Plan,
		Status:           status,
		StripeSubID:      stripeSub.ID,
		StripeCustomerID: customerID,
		CurrentPeriodEnd: periodEnd,
	}
	created, err := s.subs.Create(ctx, sub)
	if err != nil {
		return nil, err
	}

	// ResilientProducer handles Kafka fallback — never propagates publish errors.
	_ = s.producer.Publish(ctx, events.TopicPaymentEvents, "payment.subscription_created", events.SubscriptionCreated{
		SubscriptionID:   created.ID,
		UserID:           cmd.UserID,
		Plan:             string(cmd.Plan),
		StripeSubID:      stripeSub.ID,
		StripeCustomerID: customerID,
		CurrentPeriodEnd: created.CurrentPeriodEnd,
		CreatedAt:        created.CreatedAt,
	})

	logger.FromContext(ctx).Info().
		Str("user_id", cmd.UserID).
		Str("plan", string(cmd.Plan)).
		Bool("pending", strings.HasPrefix(stripeSub.ID, "pending_")).
		Msg("payment: subscription created")

	return created, nil
}

// ─── GetSubscription ──────────────────────────────────────────────────────────

func (s *paymentService) GetSubscription(ctx context.Context, userID string) (*domain.Subscription, error) {
	return s.subs.GetByUserID(ctx, userID)
}

// ─── UpgradePlan ──────────────────────────────────────────────────────────────

func (s *paymentService) UpgradePlan(ctx context.Context, cmd domain.UpgradePlanCmd) (*domain.Subscription, error) {
	sub, err := s.subs.GetByUserID(ctx, cmd.UserID)
	if err != nil {
		return nil, err
	}

	priceID := s.priceID(cmd.NewPlan)
	if priceID == "" {
		return nil, apierror.Validation("invalid plan: "+string(cmd.NewPlan), nil)
	}

	oldPlan := sub.Plan
	// ResilientStripeAdapter returns the existing sub on Stripe outage so the
	// local row is updated optimistically; webhook confirms the change later.
	updated, err := s.stripe.UpdateSubscription(ctx, sub.StripeSubID, priceID)
	if err != nil {
		return nil, apierror.UpstreamError("stripe", err)
	}

	sub.Plan = cmd.NewPlan
	if updated.CurrentPeriodEnd > 0 {
		sub.CurrentPeriodEnd = time.Unix(updated.CurrentPeriodEnd, 0)
	}
	result, err := s.subs.Update(ctx, sub)
	if err != nil {
		return nil, err
	}

	_ = s.producer.Publish(ctx, events.TopicPaymentEvents, "payment.subscription_upgraded", events.SubscriptionUpgraded{
		SubscriptionID: result.ID,
		UserID:         cmd.UserID,
		OldPlan:        string(oldPlan),
		NewPlan:        string(cmd.NewPlan),
		UpgradedAt:     time.Now(),
	})

	return result, nil
}

// ─── CancelSubscription ───────────────────────────────────────────────────────

func (s *paymentService) CancelSubscription(ctx context.Context, userID string) error {
	sub, err := s.subs.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}

	// ResilientStripeAdapter queues the cancel op when Stripe is down and
	// returns nil so the local record is updated immediately.
	if err := s.stripe.CancelSubscription(ctx, sub.StripeSubID); err != nil {
		return apierror.UpstreamError("stripe", err)
	}

	sub.Status = domain.SubStatusCancelled
	sub.CancelAtEnd = true
	if _, err := s.subs.Update(ctx, sub); err != nil {
		return err
	}

	_ = s.producer.Publish(ctx, events.TopicPaymentEvents, "payment.subscription_cancelled", events.SubscriptionCancelled{
		SubscriptionID: sub.ID,
		UserID:         userID,
		Plan:           string(sub.Plan),
		AccessUntil:    sub.CurrentPeriodEnd,
		CancelledAt:    time.Now(),
	})

	return nil
}

// ─── Invoices ─────────────────────────────────────────────────────────────────

func (s *paymentService) GetInvoices(ctx context.Context, userID string, limit, offset int) ([]*domain.Invoice, int, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.invoices.ListByUser(ctx, userID, limit, offset)
}

// ─── PayPerView ───────────────────────────────────────────────────────────────

func (s *paymentService) PayPerView(ctx context.Context, cmd domain.PayPerViewCmd) (*domain.ScreenerLicense, error) {
	if existing, err := s.licenses.GetByUserAndMovie(ctx, cmd.UserID, cmd.MovieID); err == nil && existing != nil {
		return existing, nil
	}

	lic := &domain.ScreenerLicense{
		ID:          uuid.New().String(),
		UserID:      cmd.UserID,
		MovieID:     cmd.MovieID,
		AmountCents: cmd.AmountCents,
		Currency:    cmd.Currency,
		ExpiresAt:   time.Now().Add(48 * time.Hour),
	}
	created, err := s.licenses.Create(ctx, lic)
	if err != nil {
		return nil, err
	}

	_ = s.producer.Publish(ctx, events.TopicPaymentEvents, "payment.pay_per_view_purchased", events.PayPerViewPurchased{
		LicenseID:   created.ID,
		UserID:      cmd.UserID,
		MovieID:     cmd.MovieID,
		AmountCents: cmd.AmountCents,
		Currency:    cmd.Currency,
		ExpiresAt:   created.ExpiresAt,
		PurchasedAt: time.Now(),
	})

	return created, nil
}

// ─── HasAccess ────────────────────────────────────────────────────────────────

func (s *paymentService) HasAccess(ctx context.Context, userID, movieID string) (bool, error) {
	sub, err := s.subs.GetByUserID(ctx, userID)
	if err == nil && sub != nil {
		if sub.Plan == domain.PlanPro || sub.Plan == domain.PlanStudio {
			return true, nil
		}
	}
	lic, err := s.licenses.GetByUserAndMovie(ctx, userID, movieID)
	if err != nil {
		return false, err
	}
	return lic != nil, nil
}

func (s *paymentService) GetLicenses(ctx context.Context, userID string) ([]*domain.ScreenerLicense, error) {
	return s.licenses.ListByUser(ctx, userID)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (s *paymentService) priceID(plan domain.Plan) string {
	switch plan {
	case domain.PlanPro:
		return s.cfg.PriceIDPro
	case domain.PlanStudio:
		return s.cfg.PriceIDStudio
	default:
		return ""
	}
}
