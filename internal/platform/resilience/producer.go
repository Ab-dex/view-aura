package resilience

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/Ab-dex/view-aura/internal/events"
)

const (
	// redisOutboxPrefix is the list key prefix for Kafka outbox entries.
	// Full key: outbox:kafka:{topic}
	redisOutboxPrefix = "outbox:kafka:"

	// redisOutboxTTL is how long outbox entries survive in Redis.
	// Long enough for Kafka to come back; short enough to not bloat memory.
	redisOutboxTTL = 48 * time.Hour

	// maxRedisOutboxLen caps each list to prevent unbounded memory growth.
	// Oldest entries are trimmed when the cap is exceeded.
	maxRedisOutboxLen = 10_000
)

// ResilientProducer wraps events.Producer with three-tier degradation:
//
//	Tier 1  Kafka (the real producer)
//	Tier 2  Redis LPUSH to outbox:{topic} list (OutboxWorker replays on recovery)
//	Tier 3  FileOutbox JSONL append (ops replays manually via kafka-replay tool)
//
// A nil redis client skips Tier 2 and falls directly to Tier 3.
// The circuit breaker is in-process only when redis is nil.
type ResilientProducer struct {
	primary events.Producer
	breaker *InProcessBreaker
	rdb     goredis.UniversalClient // nil if Redis not configured
	outbox  *FileOutbox
}

// NewResilientProducer constructs the producer wrapper.
//
//   - primary: the real Kafka producer (or NoopProducer in tests)
//   - rdb:     *cache.Client.Client — pass nil when Redis is unconfigured
//   - logDir:  directory for Tier 3 log files (defaults to "logs")
func NewResilientProducer(primary events.Producer, rdb goredis.UniversalClient, logDir string) *ResilientProducer {
	if primary == nil {
		panic("primary producer cannot be nil")
	}

	return &ResilientProducer{
		primary: primary,
		breaker: NewInProcessBreaker("kafka", 5, 30*time.Second),
		rdb:     rdb,
		outbox:  NewFileOutbox(logDir, "kafka"),
	}
}

// Publish implements events.Producer.
func (p *ResilientProducer) Publish(ctx context.Context, topic, eventType string, payload any) error {
	if p == nil || isNilPrimary(p.primary) {
		log.Warn().
			Str("topic", topic).
			Str("event_type", eventType).
			Msg("resilient_producer: primary missing — skipping publish")
		return fmt.Errorf("resilient_producer: all delivery tiers failed for topic=%s event=%s", topic, eventType)
	}
	// ── Tier 1: Kafka ────────────────────────────────────────────────────────
	if p.breaker.Allow() {
		err := p.primary.Publish(ctx, topic, eventType, payload)
		if err == nil {
			p.breaker.RecordSuccess()
			return nil
		}
		p.breaker.RecordFailure()
		log.Warn().
			Err(err).
			Str("topic", topic).
			Str("event_type", eventType).
			Msg("resilient_producer: kafka failed — falling back to redis outbox")
	} else {
		log.Warn().
			Str("topic", topic).
			Str("event_type", eventType).
			Msg("resilient_producer: kafka circuit open — falling back to redis outbox")
	}

	// ── Tier 2: Redis outbox ─────────────────────────────────────────────────
	if p.rdb != nil {
		if err := p.pushToRedis(ctx, topic, eventType, payload); err == nil {
			return nil
		}
		log.Warn().
			Str("topic", topic).
			Msg("resilient_producer: redis outbox failed — falling back to file outbox")
	}

	// ── Tier 3: File outbox ───────────────────────────────────────────────────
	p.outbox.Write(topic, "", map[string]any{
		"event_type": eventType,
		"payload":    payload,
	})
	log.Error().
		Str("topic", topic).
		Str("event_type", eventType).
		Msg("resilient_producer: event written to file outbox (kafka + redis both unavailable)")

	// Return nil — the caller's request should not fail because Kafka is down but for now, it should send direct error especially useful for this case of email.
	// The event will be replayed by the OutboxWorker or by ops using kafka-replay.

	return nil
}

func (p *ResilientProducer) Close() {
	p.primary.Close()
	p.outbox.Close()
}

// pushToRedis serialises the event envelope and LPUSH-es it onto the Redis
// outbox list.  The list is capped at maxRedisOutboxLen via LTRIM.
func (p *ResilientProducer) pushToRedis(ctx context.Context, topic, eventType string, payload any) error {
	raw, err := json.Marshal(map[string]any{
		"topic":      topic,
		"event_type": eventType,
		"payload":    payload,
		"queued_at":  time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("resilient_producer: marshal for redis: %w", err)
	}

	key := redisOutboxPrefix + topic
	pipe := p.rdb.Pipeline()
	pipe.LPush(ctx, key, string(raw))
	pipe.LTrim(ctx, key, 0, maxRedisOutboxLen-1)
	pipe.Expire(ctx, key, redisOutboxTTL)

	tctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	if _, err := pipe.Exec(tctx); err != nil {
		return fmt.Errorf("resilient_producer: redis pipeline: %w", err)
	}
	return nil
}

func isNilPrimary(p events.Producer) bool {
	if p == nil {
		return true
	}

	// detect typed nil inside interface
	switch v := p.(type) {
	case *events.PrimaryProducer:
		return v == nil
	}

	return false
}
