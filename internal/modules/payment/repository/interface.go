package repository

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/payment/domain"
)

// SubscriptionRepository persists subscription records to Postgres.
type SubscriptionRepository interface {
	Create(ctx context.Context, s *domain.Subscription) (*domain.Subscription, error)
	// GetByUserID returns the active (non-cancelled) subscription for a user.
	// Returns apierror.ErrSubscriptionNotFound when none exists.
	GetByUserID(ctx context.Context, userID string) (*domain.Subscription, error)
	// GetByStripeSubID is used by the webhook handler to look up a subscription
	// from the Stripe subscription ID carried in webhook events.
	GetByStripeSubID(ctx context.Context, stripeSubID string) (*domain.Subscription, error)
	Update(ctx context.Context, s *domain.Subscription) (*domain.Subscription, error)
}

// InvoiceRepository persists invoice records to Postgres.
type InvoiceRepository interface {
	Create(ctx context.Context, inv *domain.Invoice) (*domain.Invoice, error)
	// GetByStripeInvoiceID looks up an existing invoice row by Stripe ID.
	// Returns nil, nil when not found (used for idempotent create-or-update).
	GetByStripeInvoiceID(ctx context.Context, stripeInvoiceID string) (*domain.Invoice, error)
	Update(ctx context.Context, inv *domain.Invoice) (*domain.Invoice, error)
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.Invoice, int, error)
}

// LicenseRepository persists pay-per-view screener licenses to Postgres.
type LicenseRepository interface {
	Create(ctx context.Context, l *domain.ScreenerLicense) (*domain.ScreenerLicense, error)
	// GetByUserAndMovie returns a valid (non-expired) license, or nil.
	GetByUserAndMovie(ctx context.Context, userID, movieID string) (*domain.ScreenerLicense, error)
	ListByUser(ctx context.Context, userID string) ([]*domain.ScreenerLicense, error)
}

// IdempotencyRepository uses Redis to prevent double-processing of Stripe
// webhook events. Stripe can deliver the same event multiple times.
type IdempotencyRepository interface {
	// MarkSeen stores the Stripe event ID atomically.
	// Returns true if this is the first time the ID is seen (should process).
	// Returns false if it was already seen (skip — duplicate delivery).
	MarkSeen(ctx context.Context, stripeEventID string) (bool, error)
}
