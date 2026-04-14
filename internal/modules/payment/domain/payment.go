package domain

import "time"

// ─── Enumerations ─────────────────────────────────────────────────────────────

// Plan mirrors the tier names in the quota module.
type Plan string

const (
	PlanFree   Plan = "free"
	PlanPro    Plan = "pro"
	PlanStudio Plan = "studio"
)

// SubscriptionStatus reflects the Stripe subscription lifecycle.
type SubscriptionStatus string

const (
	SubStatusActive    SubscriptionStatus = "active"
	SubStatusPastDue   SubscriptionStatus = "past_due"
	SubStatusCancelled SubscriptionStatus = "cancelled"
	SubStatusTrialing  SubscriptionStatus = "trialing"
)

// InvoiceStatus reflects the Stripe invoice lifecycle.
type InvoiceStatus string

const (
	InvoicePaid InvoiceStatus = "paid"
	InvoiceOpen InvoiceStatus = "open"
	InvoiceVoid InvoiceStatus = "void"
)

// ─── Entities ─────────────────────────────────────────────────────────────────

// Subscription tracks a user's recurring plan with its Stripe identifiers.
type Subscription struct {
	ID               string
	UserID           string
	Plan             Plan
	Status           SubscriptionStatus
	StripeSubID      string    // Stripe subscription ID, e.g. "sub_1..."
	StripeCustomerID string    // Stripe customer ID, e.g. "cus_1..."
	CurrentPeriodEnd time.Time // when the current billing cycle ends
	CancelAtEnd      bool      // true when cancel_at_period_end is set
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Invoice records a single billing event — both initial charges and renewals.
type Invoice struct {
	ID              string
	UserID          string
	SubscriptionID  string
	StripeInvoiceID string
	AmountCents     int64
	Currency        string // ISO 4217, e.g. "usd"
	Status          InvoiceStatus
	PaidAt          *time.Time
	CreatedAt       time.Time
}

// ScreenerLicense grants time-limited pay-per-view access to a specific movie.
type ScreenerLicense struct {
	ID          string
	UserID      string
	MovieID     string
	AmountCents int64
	Currency    string
	ExpiresAt   time.Time // 48-hour rental window by default
	CreatedAt   time.Time
}

// ─── Commands ─────────────────────────────────────────────────────────────────

type SubscribeCmd struct {
	UserID        string
	Plan          Plan
	PaymentMethod string // Stripe payment method ID, e.g. "pm_1..."
	Email         string // required to create the Stripe customer
	Name          string
}

type UpgradePlanCmd struct {
	UserID  string
	NewPlan Plan
}

type PayPerViewCmd struct {
	UserID        string
	MovieID       string
	PaymentMethod string
	AmountCents   int64
	Currency      string
}
