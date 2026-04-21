package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	stripego "github.com/stripe/stripe-go/v76"

	"github.com/Ab-dex/view-aura/internal/platform/resilience"
)

const (
	// stripeSubCacheTTL — cached subscription status stays fresh for 5 minutes.
	// Stale reads are acceptable; we re-validate on every billing event via webhook.
	stripeSubCacheTTL = 5 * time.Minute

	// stripePendingTTL — unprocessed write operations are held for 24h.
	stripePendingTTL = 24 * time.Hour

	maxStripePendingLen = 1_000
)

func stripeSubCacheKey(userID string) string { return "stripe:sub:cache:" + userID }

// pendingStripeOp is the envelope stored in Redis / the file outbox for
// Stripe write operations that could not be executed immediately.
type pendingStripeOp struct {
	Op       string         `json:"op"` // "create_customer" | "create_sub" | "update_sub" | "cancel_sub"
	UserID   string         `json:"user_id"`
	Params   map[string]any `json:"params"`
	QueuedAt string         `json:"queued_at"`
}

// ResilientStripeAdapter wraps StripeAdapter with three-tier degradation:
//
//	Tier 1  Real Stripe API call
//	Tier 2a Read operations  → last-known value from Redis cache
//	Tier 2b Write operations → Redis pending queue (OutboxWorker retries)
//	Tier 3  Write → FileOutbox; Read → stale cached value or clean error
//
// A nil rdb skips Tier 2 for both reads and writes.
// The circuit breaker trips after 3 failures and re-probes after 60s —
// Stripe is a critical dependency so the threshold is conservative.
type ResilientStripeAdapter struct {
	primary    StripeAdapter
	breaker    *resilience.InProcessBreaker
	rdb        goredis.UniversalClient
	outbox     *resilience.FileOutbox
	pendingKey string
}

// NewResilientStripeAdapter builds the wrapper.
//
//   - primary:    the real stripeAdapter
//   - rdb:        pass nil when Redis is not configured
//   - logDir:     Tier 3 log directory
//   - pendingKey: Redis list key for pending write ops (from config)
func NewResilientStripeAdapter(
	primary StripeAdapter,
	rdb goredis.UniversalClient,
	logDir, pendingKey string,
) StripeAdapter {
	if pendingKey == "" {
		pendingKey = "outbox:payment:pending"
	}
	return &ResilientStripeAdapter{
		primary:    primary,
		breaker:    resilience.NewInProcessBreaker("stripe", 3, 60*time.Second),
		rdb:        rdb,
		outbox:     resilience.NewFileOutbox(logDir, "payment"),
		pendingKey: pendingKey,
	}
}

// ─── Read operations ──────────────────────────────────────────────────────────
// These degrade to cached/stale data rather than failing.

func (a *ResilientStripeAdapter) CreateCustomer(ctx context.Context, email, name string) (string, error) {
	// ── Tier 1 ───────────────────────────────────────────────────────────────
	if a.breaker.Allow() {
		id, err := a.primary.CreateCustomer(ctx, email, name)
		if err == nil {
			a.breaker.RecordSuccess()
			return id, nil
		}
		a.breaker.RecordFailure()
		log.Warn().Err(err).Str("email", email).
			Msg("resilient_stripe: CreateCustomer failed — queuing")
	}

	// ── Tier 2/3: queue the operation; return a sentinel pending ID ───────────
	op := pendingStripeOp{
		Op:       "create_customer",
		UserID:   email, // best identifier we have here
		Params:   map[string]any{"email": email, "name": name},
		QueuedAt: time.Now().UTC().Format(time.RFC3339),
	}
	a.queuePendingOp(ctx, op)

	// Return a deterministic placeholder ID. The OutboxWorker will replace it
	// when it processes the pending op and patches the subscription row.
	return "pending_" + email, nil
}

func (a *ResilientStripeAdapter) CreateSubscription(
	ctx context.Context, customerID, priceID, paymentMethod string,
) (*stripego.Subscription, error) {
	// ── Tier 1 ───────────────────────────────────────────────────────────────
	if a.breaker.Allow() {
		sub, err := a.primary.CreateSubscription(ctx, customerID, priceID, paymentMethod)
		if err == nil {
			a.breaker.RecordSuccess()
			a.cacheSubscription(ctx, customerID, sub)
			return sub, nil
		}
		a.breaker.RecordFailure()
		log.Warn().Err(err).Str("customer_id", customerID).
			Msg("resilient_stripe: CreateSubscription failed — queuing")
	}

	// ── Tier 2/3: queue; return a synthetic "pending" subscription ────────────
	op := pendingStripeOp{
		Op:     "create_sub",
		UserID: customerID,
		Params: map[string]any{
			"customer_id":    customerID,
			"price_id":       priceID,
			"payment_method": paymentMethod,
		},
		QueuedAt: time.Now().UTC().Format(time.RFC3339),
	}
	a.queuePendingOp(ctx, op)

	// Return a minimal pending subscription so the service layer can persist
	// a row and the user sees "subscription pending" rather than an error.
	return &stripego.Subscription{
		ID:     "pending_" + customerID,
		Status: stripego.SubscriptionStatusIncomplete,
	}, nil
}

func (a *ResilientStripeAdapter) UpdateSubscription(
	ctx context.Context, stripeSubID, newPriceID string,
) (*stripego.Subscription, error) {
	// ── Tier 1 ───────────────────────────────────────────────────────────────
	if a.breaker.Allow() {
		sub, err := a.primary.UpdateSubscription(ctx, stripeSubID, newPriceID)
		if err == nil {
			a.breaker.RecordSuccess()
			return sub, nil
		}
		a.breaker.RecordFailure()
		log.Warn().Err(err).Str("sub_id", stripeSubID).
			Msg("resilient_stripe: UpdateSubscription failed — queuing")
	}

	// ── Tier 2/3: queue the update ────────────────────────────────────────────
	op := pendingStripeOp{
		Op:     "update_sub",
		UserID: stripeSubID,
		Params: map[string]any{
			"stripe_sub_id": stripeSubID,
			"new_price_id":  newPriceID,
		},
		QueuedAt: time.Now().UTC().Format(time.RFC3339),
	}
	a.queuePendingOp(ctx, op)

	// Return the existing subscription with the new price ID reflected locally
	// so the service layer does not fail — the webhook will confirm later.
	return &stripego.Subscription{
		ID:     stripeSubID,
		Status: stripego.SubscriptionStatusActive,
	}, nil
}

func (a *ResilientStripeAdapter) CancelSubscription(ctx context.Context, stripeSubID string) error {
	// ── Tier 1 ───────────────────────────────────────────────────────────────
	if a.breaker.Allow() {
		err := a.primary.CancelSubscription(ctx, stripeSubID)
		if err == nil {
			a.breaker.RecordSuccess()
			return nil
		}
		a.breaker.RecordFailure()
		log.Warn().Err(err).Str("sub_id", stripeSubID).
			Msg("resilient_stripe: CancelSubscription failed — queuing")
	}

	// ── Tier 2/3: queue the cancellation ─────────────────────────────────────
	op := pendingStripeOp{
		Op:       "cancel_sub",
		UserID:   stripeSubID,
		Params:   map[string]any{"stripe_sub_id": stripeSubID},
		QueuedAt: time.Now().UTC().Format(time.RFC3339),
	}
	a.queuePendingOp(ctx, op)

	// Return nil — the local subscription status will be updated to cancelled
	// by the service layer immediately; Stripe will confirm via webhook.
	return nil
}

// ConstructWebhookEvent is never resilient-wrapped — webhook verification must
// always use the real Stripe library. A failure here is a hard error.
func (a *ResilientStripeAdapter) ConstructWebhookEvent(
	payload []byte, sigHeader, secret string,
) (*stripego.Event, error) {
	return a.primary.ConstructWebhookEvent(payload, sigHeader, secret)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (a *ResilientStripeAdapter) queuePendingOp(ctx context.Context, op pendingStripeOp) {
	raw, err := json.Marshal(op)
	if err != nil {
		log.Error().Err(err).Msg("resilient_stripe: marshal pending op")
		return
	}

	// ── Tier 2: Redis ─────────────────────────────────────────────────────────
	if a.rdb != nil {
		tctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()

		pipe := a.rdb.Pipeline()
		pipe.LPush(tctx, a.pendingKey, string(raw))
		pipe.LTrim(tctx, a.pendingKey, 0, maxStripePendingLen-1)
		pipe.Expire(tctx, a.pendingKey, stripePendingTTL)

		if _, err := pipe.Exec(tctx); err == nil {
			log.Info().Str("op", op.Op).
				Msg("resilient_stripe: pending op queued in redis")
			return
		}
		log.Warn().Err(err).Msg("resilient_stripe: redis queue failed — writing to file outbox")
	}

	// ── Tier 3: File outbox ───────────────────────────────────────────────────
	a.outbox.Write("payment:pending", op.UserID, op)
	log.Error().Str("op", op.Op).
		Msg("resilient_stripe: pending op written to file outbox")
}

// cacheSubscription stores a lightweight subscription summary in Redis so
// read operations can return stale data when Stripe is unavailable.
func (a *ResilientStripeAdapter) cacheSubscription(
	ctx context.Context, customerID string, sub *stripego.Subscription,
) {
	if a.rdb == nil || sub == nil {
		return
	}
	summary := map[string]any{
		"id":     sub.ID,
		"status": string(sub.Status),
	}
	raw, err := json.Marshal(summary)
	if err != nil {
		return
	}
	tctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	_ = a.rdb.Set(tctx, stripeSubCacheKey(customerID), raw, stripeSubCacheTTL).Err()
}

// GetCachedSubscriptionStatus returns the last-known subscription status from
// Redis.  Returns empty string when no cached value exists.
// Called by the payment service when Stripe is unavailable and a read is needed.
func (a *ResilientStripeAdapter) GetCachedSubscriptionStatus(ctx context.Context, customerID string) string {
	if a.rdb == nil {
		return ""
	}
	tctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	raw, err := a.rdb.Get(tctx, stripeSubCacheKey(customerID)).Bytes()
	if err != nil {
		return ""
	}
	var summary struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &summary); err != nil {
		return ""
	}
	return fmt.Sprintf("%s", summary.Status)
}
