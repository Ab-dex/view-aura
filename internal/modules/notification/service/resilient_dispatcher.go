package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/Ab-dex/view-aura/internal/modules/notification/domain"
	"github.com/Ab-dex/view-aura/internal/platform/resilience"
)

const (
	// pushOutboxTTL — undelivered push notifications expire after 72h.
	// On next app open the client fetches its in-app notifications instead.
	pushOutboxTTL = 72 * time.Hour

	// maxPushOutboxPerUser caps the Redis list to prevent per-user bloat.
	maxPushOutboxPerUser = 200
)

// ResilientDispatcher wraps the real push dispatcher with three-tier degradation:
//
//	Tier 1  Primary dispatcher (FCM / APNs via the real Dispatcher implementation)
//	Tier 2  Redis per-user push outbox (delivered on next app open via polling)
//	Tier 3  FileOutbox JSONL append (observable; ops can replay via admin tooling)
//
// A nil rdb skips Tier 2.
// A nil primary falls straight to Tier 2/3 (useful in local dev).
//
// In-app notifications are always persisted by the notification service before
// Dispatch is called, so Tier 2/3 degradation only affects push/email channels,
// never the in-app record.
type ResilientDispatcher struct {
	primary       Dispatcher
	breaker       *resilience.InProcessBreaker
	rdb           goredis.UniversalClient // nil if Redis not configured
	outbox        *resilience.FileOutbox
	pushKeyPrefix string
}

// NewResilientDispatcher builds the dispatcher.
//
//   - primary: the real FCM/APNs dispatcher (or NoopDispatcher in local dev)
//   - rdb:     pass nil when Redis is not configured
//   - logDir:  Tier 3 log directory (defaults to "logs")
//   - pushKeyPrefix: Redis key prefix, e.g. "outbox:push:" (from config)
func NewResilientDispatcher(
	primary Dispatcher,
	rdb goredis.UniversalClient,
	logDir, pushKeyPrefix string,
) Dispatcher {
	if pushKeyPrefix == "" {
		pushKeyPrefix = "outbox:push:"
	}
	return &ResilientDispatcher{
		primary:       primary,
		breaker:       resilience.NewInProcessBreaker("push-dispatcher", 5, 30*time.Second),
		rdb:           rdb,
		outbox:        resilience.NewFileOutbox(logDir, "push"),
		pushKeyPrefix: pushKeyPrefix,
	}
}

// Dispatch implements the Dispatcher interface.
func (d *ResilientDispatcher) Dispatch(
	ctx context.Context,
	ch domain.Channel,
	n *domain.Notification,
	tokens []string,
) error {
	// ── Tier 1: primary dispatcher (FCM / APNs) ───────────────────────────────
	if d.primary != nil && d.breaker.Allow() {
		err := d.primary.Dispatch(ctx, ch, n, tokens)
		if err == nil {
			d.breaker.RecordSuccess()
			return nil
		}
		d.breaker.RecordFailure()
		log.Warn().
			Err(err).
			Str("channel", string(ch)).
			Str("user_id", n.UserID).
			Msg("resilient_dispatcher: push failed — queuing in redis outbox")
	} else if d.breaker.IsOpen() {
		log.Warn().
			Str("channel", string(ch)).
			Str("user_id", n.UserID).
			Msg("resilient_dispatcher: push circuit open — queuing in redis outbox")
	}

	// ── Tier 2: Redis per-user push outbox ────────────────────────────────────
	if d.rdb != nil {
		if err := d.pushToRedis(ctx, ch, n, tokens); err == nil {
			return nil
		}
		log.Warn().
			Str("user_id", n.UserID).
			Msg("resilient_dispatcher: redis outbox failed — writing to file outbox")
	}

	// ── Tier 3: File outbox ───────────────────────────────────────────────────
	d.outbox.Write(string(ch), n.UserID, map[string]any{
		"notification_id": n.ID.String(),
		"type":            string(n.Type),
		"title":           n.Title,
		"body":            n.Body,
		"tokens":          tokens,
	})
	log.Error().
		Str("user_id", n.UserID).
		Str("notification_id", n.ID.String()).
		Msg("resilient_dispatcher: push written to file outbox (all tiers failed)")

	// Return nil — the in-app record is already persisted; a failed push is
	// non-critical and must not bubble up as a 500 to the caller.
	return nil
}

// pushToRedis stores the push payload in a per-user Redis list.
// On next app open the client can poll for pending in-app notifications;
// the push message is delivered then if the device re-registers.
func (d *ResilientDispatcher) pushToRedis(
	ctx context.Context,
	ch domain.Channel,
	n *domain.Notification,
	tokens []string,
) error {
	entry, err := json.Marshal(map[string]any{
		"channel":         string(ch),
		"notification_id": n.ID.String(),
		"type":            string(n.Type),
		"title":           n.Title,
		"body":            n.Body,
		"tokens":          tokens,
		"queued_at":       time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("resilient_dispatcher: marshal: %w", err)
	}

	key := d.pushKeyPrefix + n.UserID
	tctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	pipe := d.rdb.Pipeline()
	pipe.LPush(tctx, key, string(entry))
	pipe.LTrim(tctx, key, 0, maxPushOutboxPerUser-1)
	pipe.Expire(tctx, key, pushOutboxTTL)

	if _, err := pipe.Exec(tctx); err != nil {
		return fmt.Errorf("resilient_dispatcher: redis pipeline: %w", err)
	}
	return nil
}
