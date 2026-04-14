// Package payment provides subscription and pay-per-view operations.
package payment

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

type doer interface {
	Do(ctx context.Context, method, path string, body, out any) error
}

// Service wraps all payment endpoints.
type Service struct{ doer doer }

func New(d doer) *Service { return &Service{doer: d} }

// ─── Types ────────────────────────────────────────────────────────────────────

// Subscription is the caller's active plan.
type Subscription struct {
	ID               string `json:"id"`
	Plan             string `json:"plan"`   // "free" | "pro" | "studio"
	Status           string `json:"status"` // "active" | "past_due" | "cancelled"
	CurrentPeriodEnd string `json:"current_period_end"`
	CancelAtEnd      bool   `json:"cancel_at_end"`
}

// Invoice is a billing event record.
type Invoice struct {
	ID          string  `json:"id"`
	AmountCents int64   `json:"amount_cents"`
	Currency    string  `json:"currency"`
	Status      string  `json:"status"`
	PaidAt      *string `json:"paid_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

// ScreenerLicense grants pay-per-view access to a specific movie.
type ScreenerLicense struct {
	ID          string `json:"id"`
	MovieID     string `json:"movie_id"`
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
	ExpiresAt   string `json:"expires_at"`
	CreatedAt   string `json:"created_at"`
}

// InvoiceListResult is the paginated invoice response.
type InvoiceListResult struct {
	Invoices []Invoice `json:"invoices"`
	Total    int       `json:"total"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
}

// SubscribeParams is the request body for Subscribe.
type SubscribeParams struct {
	Plan          string `json:"plan"`           // "pro" | "studio"
	PaymentMethod string `json:"payment_method"` // Stripe payment method ID
}

// UpgradeParams is the request body for UpgradePlan.
type UpgradeParams struct {
	Plan string `json:"plan"`
}

// PayPerViewParams is the request body for PayPerView.
type PayPerViewParams struct {
	MovieID       string `json:"movie_id"`
	PaymentMethod string `json:"payment_method"`
	AmountCents   int64  `json:"amount_cents"`
	Currency      string `json:"currency"`
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// Subscribe creates a new Pro or Studio subscription.
func (s *Service) Subscribe(ctx context.Context, p SubscribeParams) (*Subscription, error) {
	var out Subscription
	return &out, s.doer.Do(ctx, http.MethodPost, "/api/v1/me/subscription", p, &out)
}

// GetSubscription returns the caller's current subscription.
func (s *Service) GetSubscription(ctx context.Context) (*Subscription, error) {
	var out Subscription
	return &out, s.doer.Do(ctx, http.MethodGet, "/api/v1/me/subscription", nil, &out)
}

// UpgradePlan changes the caller's plan (e.g. pro → studio).
func (s *Service) UpgradePlan(ctx context.Context, p UpgradeParams) (*Subscription, error) {
	var out Subscription
	return &out, s.doer.Do(ctx, http.MethodPatch, "/api/v1/me/subscription", p, &out)
}

// CancelSubscription sets cancel_at_period_end = true.
// Access continues until current_period_end.
func (s *Service) CancelSubscription(ctx context.Context) error {
	return s.doer.Do(ctx, http.MethodDelete, "/api/v1/me/subscription", nil, nil)
}

// ListInvoices returns the caller's billing history.
func (s *Service) ListInvoices(ctx context.Context, limit, offset int) (*InvoiceListResult, error) {
	v := url.Values{}
	if limit > 0 {
		v.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		v.Set("offset", strconv.Itoa(offset))
	}
	var out InvoiceListResult
	return &out, s.doer.Do(ctx, http.MethodGet, "/api/v1/me/invoices?"+v.Encode(), nil, &out)
}

// PayPerView purchases a 48-hour screener license for a specific movie.
func (s *Service) PayPerView(ctx context.Context, p PayPerViewParams) (*ScreenerLicense, error) {
	var out ScreenerLicense
	return &out, s.doer.Do(ctx, http.MethodPost, "/api/v1/me/pay-per-view", p, &out)
}

// ListLicenses returns all active screener licenses for the caller.
func (s *Service) ListLicenses(ctx context.Context) ([]ScreenerLicense, error) {
	var out struct {
		Licenses []ScreenerLicense `json:"licenses"`
	}
	return out.Licenses, s.doer.Do(ctx, http.MethodGet, "/api/v1/me/licenses", nil, &out)
}

// HasAccess checks whether the caller has access to a specific movie
// (via subscription or pay-per-view license).
func (s *Service) HasAccess(ctx context.Context, movieID string) (bool, error) {
	var out struct {
		HasAccess bool `json:"has_access"`
	}
	return out.HasAccess, s.doer.Do(ctx, http.MethodGet,
		"/api/v1/movies/"+url.PathEscape(movieID)+"/access", nil, &out)
}
