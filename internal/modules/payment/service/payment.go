package service

import (
	"context"
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
	// HasAccess returns true if the user has a Pro/Studio subscription or an
	// active pay-per-view license for the given movie.
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
	// Guard: no double-subscription.
	if _, err := s.subs.GetByUserID(ctx, cmd.UserID); err == nil {
		return nil, apierror.ErrSubscriptionActive
	}

	priceID := s.priceID(cmd.Plan)
	if priceID == "" {
		return nil, apierror.Validation("invalid plan: "+string(cmd.Plan), nil)
	}

	customerID, err := s.stripe.CreateCustomer(ctx, cmd.Email, cmd.Name)
	if err != nil {
		return nil, apierror.UpstreamError("stripe", err)
	}

	stripeSub, err := s.stripe.CreateSubscription(ctx, customerID, priceID, cmd.PaymentMethod)
	if err != nil {
		return nil, apierror.UpstreamError("stripe", err)
	}

	sub := &domain.Subscription{
		ID:               uuid.New().String(),
		UserID:           cmd.UserID,
		Plan:             cmd.Plan,
		Status:           domain.SubStatusActive,
		StripeSubID:      stripeSub.ID,
		StripeCustomerID: customerID,
		CurrentPeriodEnd: time.Unix(stripeSub.CurrentPeriodEnd, 0),
	}
	created, err := s.subs.Create(ctx, sub)
	if err != nil {
		return nil, err
	}

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
	updated, err := s.stripe.UpdateSubscription(ctx, sub.StripeSubID, priceID)
	if err != nil {
		return nil, apierror.UpstreamError("stripe", err)
	}

	sub.Plan = cmd.NewPlan
	sub.CurrentPeriodEnd = time.Unix(updated.CurrentPeriodEnd, 0)
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
	// Idempotent — return existing valid license if one exists.
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
