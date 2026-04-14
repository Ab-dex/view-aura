package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Ab-dex/view-aura/internal/platform/logger"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/rs/zerolog/log"
)

// HandlerFunc processes one message. Returning a non-nil error causes the
// consumer to log the error and continue — messages are not retried by
// default; use a dead-letter topic pattern for retry semantics.
type HandlerFunc func(ctx context.Context, msg Envelope) error

// Consumer is the interface cmd/worker uses to read events.
type Consumer interface {
	// Subscribe registers the topics this consumer wants to receive.
	Subscribe(topics []string) error
	// Run blocks until ctx is cancelled, calling handler for every message.
	Run(ctx context.Context, handler HandlerFunc) error
	// Close commits offsets and shuts down.
	Close()
}

// kafkaConsumer wraps a confluent consumer group.
type kafkaConsumer struct {
	c *kafka.Consumer
}

// NewConsumer creates a new Kafka consumer group member.
func NewConsumer(cfg KafkaConfig) (Consumer, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":     cfg.Brokers,
		"group.id":              cfg.GroupID,
		"auto.offset.reset":     "earliest",
		"enable.auto.commit":    false, // We manage commits manually for safety
		"max.poll.interval.ms":  300000,
		"session.timeout.ms":    30000,
		"heartbeat.interval.ms": 3000,
		"security.protocol":     cfg.SecurityProtocol,
		"sasl.mechanism":        cfg.SASLMechanism,
		"sasl.username":         cfg.SASLUsername,
		"sasl.password":         cfg.SASLPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("kafka: create consumer: %w", err)
	}

	return &kafkaConsumer{c: c}, nil
}

// Subscribe joins the group, listens to topics, and dispatches to the handler.
func (kc *kafkaConsumer) Subscribe(topics []string) error {
	// Note: rebalanceCb is nil here
	return kc.c.SubscribeTopics(topics, nil)
}

func (kc *kafkaConsumer) Run(ctx context.Context, handler HandlerFunc) error {
	log := logger.FromContext(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msg, err := kc.c.ReadMessage(100 * time.Millisecond)
		if err != nil {
			if kerr, ok := err.(kafka.Error); ok && kerr.Code() == kafka.ErrTimedOut {
				continue
			}
			return fmt.Errorf("kafka read error: %w", err)
		}

		var env Envelope
		if err := json.Unmarshal(msg.Value, &env); err != nil {
			log.Error().Err(err).Msg("kafka: malformed envelope — skipping")
			_, _ = kc.c.CommitMessage(msg)
			continue
		}

		// Map Metadata
		env.Topic = *msg.TopicPartition.Topic
		env.Partition = msg.TopicPartition.Partition
		env.Offset = int64(msg.TopicPartition.Offset)

		if err := handler(ctx, env); err != nil {
			log.Error().Err(err).Str("id", env.ID).Msg("handler error — retrying")
			time.Sleep(1 * time.Second) // Prevent CPU thrashing
			continue
		}

		if _, err := kc.c.CommitMessage(msg); err != nil {
			log.Error().Err(err).Msg("kafka commit failed")
		}
	}
}
func (kc *kafkaConsumer) Close() {
	_ = kc.c.Close()
	log.Info().Msg("kafka: consumer closed")
}

// ─── Decode helpers ───────────────────────────────────────────────────────────

// Decode unmarshals a message payload into dest.
func Decode(msg Envelope, dest any) error {
	if err := json.Unmarshal(msg.Payload, dest); err != nil {
		return fmt.Errorf("events: decode %s: %w", msg.Topic, err)
	}
	return nil
}
