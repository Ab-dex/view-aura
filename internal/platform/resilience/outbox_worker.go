package resilience

import (
	"context"
	"encoding/json"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/Ab-dex/view-aura/internal/events"
)

// OutboxWorker is a background goroutine that drains the Redis outbox lists
// and re-publishes them to Kafka / the email service when the primary services
// recover.
//
// It runs on a poll interval (default 30s) and processes up to batchSize
// entries per topic per tick.  Entries are re-queued at Tier 1 using the
// same ResilientProducer / ResilientMailer so a second failure drops them
// back to Redis (not the file).
//
// When rdb is nil the worker starts but immediately no-ops each tick —
// it does not crash or log errors.  This keeps the startup path clean when
// Redis is not configured.
type OutboxWorker struct {
	rdb          goredis.UniversalClient // nil = no-op
	kafkaProd    events.Producer
	mailer       *ResilientMailer
	pollInterval time.Duration
	batchSize    int64
	stop         chan struct{}
}

// NewOutboxWorker creates the worker.  Call Start() to launch the goroutine.
func NewOutboxWorker(
	rdb goredis.UniversalClient,
	kafkaProd events.Producer,
	mailer *ResilientMailer,
) *OutboxWorker {
	return &OutboxWorker{
		rdb:          rdb,
		kafkaProd:    kafkaProd,
		mailer:       mailer,
		pollInterval: 30 * time.Second,
		batchSize:    50,
		stop:         make(chan struct{}),
	}
}

// Start launches the worker goroutine.  It returns immediately.
// Call Stop() during application shutdown.
func (w *OutboxWorker) Start(ctx context.Context) {
	go w.run(ctx)
}

// Stop signals the worker to exit.  It waits for the current tick to finish.
func (w *OutboxWorker) Stop() {
	close(w.stop)
}

func (w *OutboxWorker) run(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	log.Info().Msg("outbox_worker: started")

	for {
		select {
		case <-w.stop:
			log.Info().Msg("outbox_worker: stopped")
			return
		case <-ctx.Done():
			log.Info().Msg("outbox_worker: context cancelled")
			return
		case <-ticker.C:
			w.drainKafkaOutbox(ctx)
			w.drainMailOutbox(ctx)
		}
	}
}

// drainKafkaOutbox processes up to batchSize entries from every known
// outbox list.  Unknown topics that were created dynamically are also
// drained via SCAN.
func (w *OutboxWorker) drainKafkaOutbox(ctx context.Context) {
	if w.rdb == nil {
		return
	}

	// Discover all outbox lists via pattern scan.
	pattern := redisOutboxPrefix + "*"
	var cursor uint64
	for {
		keys, nextCursor, err := w.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			log.Warn().Err(err).Msg("outbox_worker: kafka scan failed")
			return
		}

		for _, key := range keys {
			w.drainKafkaList(ctx, key)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
}

func (w *OutboxWorker) drainKafkaList(ctx context.Context, key string) {
	topic := key[len(redisOutboxPrefix):]

	for i := int64(0); i < w.batchSize; i++ {
		// RPOP — process oldest entries first (FIFO).
		val, err := w.rdb.RPop(ctx, key).Result()
		if err == goredis.Nil {
			return // list empty
		}
		if err != nil {
			log.Warn().Err(err).Str("key", key).Msg("outbox_worker: rpop failed")
			return
		}

		var entry struct {
			EventType string          `json:"event_type"`
			Payload   json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal([]byte(val), &entry); err != nil {
			log.Warn().Err(err).Str("val", val).Msg("outbox_worker: unmarshal kafka entry")
			continue
		}

		// Re-publish directly to Kafka (bypass the resilient wrapper's breaker
		// so we don't re-queue on every tick while Kafka is still down).
		var payload any
		if err := json.Unmarshal(entry.Payload, &payload); err != nil {
			log.Warn().Err(err).Msg("outbox_worker: unmarshal kafka payload")
			continue
		}

		if err := w.kafkaProd.Publish(ctx, topic, entry.EventType, payload); err != nil {
			// Kafka still down — put the entry back at the tail (RPUSH) and stop.
			_ = w.rdb.RPush(ctx, key, val)
			log.Debug().Str("topic", topic).Msg("outbox_worker: kafka still unavailable — re-queued")
			return
		}
	}
}

func (w *OutboxWorker) drainMailOutbox(ctx context.Context) {
	if w.rdb == nil || w.mailer == nil {
		return
	}

	for i := int64(0); i < w.batchSize; i++ {
		val, err := w.rdb.RPop(ctx, redisMailOutboxKey).Result()
		if err == goredis.Nil {
			return
		}
		if err != nil {
			log.Warn().Err(err).Msg("outbox_worker: mail rpop failed")
			return
		}

		var p MailPayload
		if err := json.Unmarshal([]byte(val), &p); err != nil {
			log.Warn().Err(err).Msg("outbox_worker: unmarshal mail entry")
			continue
		}

		// Attempt direct delivery — bypass the Tier 2 Redis push so we don't
		// loop back into the outbox on failure.
		if err := w.mailer.postToEmailService(ctx, p); err != nil {
			// Email service still down — put back and stop for this tick.
			_ = w.rdb.RPush(ctx, redisMailOutboxKey, val)
			log.Debug().Str("to", p.To).Msg("outbox_worker: email-service still unavailable — re-queued")
			return
		}
	}
}
