package events

import "time"

// SubscriptionCreated is published when a user successfully subscribes to a
// paid plan (Pro or Studio).
// Consumed by: quota-service (upgrade tier limits), notification-service
// (send welcome-to-pro email), analytics (MRR tracking).
type SubscriptionCreated struct {
	SubscriptionID   string    `json:"subscription_id"`
	UserID           string    `json:"user_id"`
	Plan             string    `json:"plan"` // "pro" | "studio"
	StripeSubID      string    `json:"stripe_sub_id"`
	StripeCustomerID string    `json:"stripe_customer_id"`
	CurrentPeriodEnd time.Time `json:"current_period_end"`
	CreatedAt        time.Time `json:"created_at"`
}

// SubscriptionUpgraded is published when a user moves from one paid plan to
// a higher tier (Pro → Studio).
// Consumed by: quota-service (apply new tier limits immediately),
// notification-service (send upgrade confirmation), analytics.
type SubscriptionUpgraded struct {
	SubscriptionID string    `json:"subscription_id"`
	UserID         string    `json:"user_id"`
	OldPlan        string    `json:"old_plan"`
	NewPlan        string    `json:"new_plan"`
	UpgradedAt     time.Time `json:"upgraded_at"`
}

// SubscriptionCancelled is published when a user cancels their subscription
// (cancel_at_period_end = true; access continues until period end).
// Consumed by: notification-service (send cancellation confirmation with
// "your access continues until X" message), analytics (churn tracking).
type SubscriptionCancelled struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	Plan           string `json:"plan"`
	// AccessUntil is when the user's paid access actually ends.
	AccessUntil time.Time `json:"access_until"`
	CancelledAt time.Time `json:"cancelled_at"`
}

// SubscriptionExpired is published at the end of the billing period when a
// cancelled subscription finally lapses.
// Consumed by: quota-service (downgrade to free tier limits),
// notification-service ("your subscription has ended"), analytics.
type SubscriptionExpired struct {
	SubscriptionID string    `json:"subscription_id"`
	UserID         string    `json:"user_id"`
	Plan           string    `json:"plan"`
	ExpiredAt      time.Time `json:"expired_at"`
}

// PaymentSucceeded is published when an invoice is paid (covers both
// initial subscription and recurring renewals).
// Consumed by: notification-service (send receipt), analytics (revenue event).
type PaymentSucceeded struct {
	InvoiceID       string    `json:"invoice_id"`
	SubscriptionID  string    `json:"subscription_id,omitempty"`
	UserID          string    `json:"user_id"`
	AmountCents     int64     `json:"amount_cents"`
	Currency        string    `json:"currency"`
	StripeInvoiceID string    `json:"stripe_invoice_id"`
	PaidAt          time.Time `json:"paid_at"`
}

// PaymentFailed is published when a charge attempt fails (e.g. expired card).
// Consumed by: notification-service (send "update your payment method" email),
// analytics (failed payment rate).
type PaymentFailed struct {
	InvoiceID      string `json:"invoice_id"`
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	AmountCents    int64  `json:"amount_cents"`
	Currency       string `json:"currency"`
	// FailureReason is the Stripe decline code, e.g. "card_declined", "insufficient_funds".
	FailureReason string     `json:"failure_reason"`
	RetryAt       *time.Time `json:"retry_at,omitempty"` // Stripe's next retry time
	FailedAt      time.Time  `json:"failed_at"`
}

// PayPerViewPurchased is published when a user buys a one-time screener license.
// Consumed by: notification-service (send receipt + access instructions),
// analytics (pay-per-view revenue tracking).
type PayPerViewPurchased struct {
	LicenseID   string    `json:"license_id"`
	UserID      string    `json:"user_id"`
	MovieID     string    `json:"movie_id"`
	AmountCents int64     `json:"amount_cents"`
	Currency    string    `json:"currency"`
	ExpiresAt   time.Time `json:"expires_at"`
	PurchasedAt time.Time `json:"purchased_at"`
}
