package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/google/uuid"

	"github.com/Ab-dex/view-aura/internal/platform/config"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// ─── Envelope ─────────────────────────────────────────────────────────────────

// Envelope is the standard message wrapper for every event published to Kafka.
// The payload is a JSON-encoded domain event struct.
type Envelope struct {
	ID          string          `json:"id"`
	Topic       string          `json:"topic"`
	Partition   int32           `json:"partition"`
	Offset      int64           `json:"offset"`
	EventType   string          `json:"event_type"`
	PublishedAt time.Time       `json:"published_at"`
	Payload     json.RawMessage `json:"payload"`
}

// ─── Producer ─────────────────────────────────────────────────────────────────

// Producer is the interface for publishing events to Kafka.
// The implementation is confluent-kafka-go; tests can inject a mock.
type Producer interface {
	// Publish serialises the payload and sends it to the given topic.
	Publish(ctx context.Context, topic, eventType string, payload any) error
	// Close flushes pending messages and closes the underlying producer.
	Close()
}

type kafkaProducer struct {
	p *kafka.Producer
}

// NewProducer creates a confluent-kafka-go producer from config.
func NewProducer(cfg config.KafkaConfig) (Producer, error) {
	cm := kafka.ConfigMap{
		"bootstrap.servers":  cfg.Brokers,
		"acks":               "all", // wait for all in-sync replicas
		"enable.idempotence": true,  // exactly-once producer semantics
		"retries":            10,
		"retry.backoff.ms":   200,
		"linger.ms":          5,
		"compression.type":   "snappy",
		"message.max.bytes":  1048576,
		"security.protocol":  cfg.SecurityProtocol,
		"sasl.mechanism":     cfg.SASLMechanism,
		"sasl.username":      cfg.SASLUsername,
		"sasl.password":      cfg.SASLPassword,
	}
	p, err := kafka.NewProducer(&cm)
	if err != nil {
		return nil, fmt.Errorf("kafka: new producer: %w", err)
	}

	// Start delivery-report goroutine — logs permanent failures.
	go func() {
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					logger.FromContext(context.Background()).Error().
						Err(ev.TopicPartition.Error).
						Str("topic", *ev.TopicPartition.Topic).
						Msg("kafka: permanent delivery failure")
				}
			case kafka.Error:
				logger.FromContext(context.Background()).Error().
					Err(ev).
					Msg("kafka: producer error")
			}
		}
	}()

	return &kafkaProducer{p: p}, nil
}

func (kp *kafkaProducer) Publish(ctx context.Context, topic, eventType string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("events: marshal payload for %s: %w", eventType, err)
	}

	env := Envelope{
		ID:          uuid.New().String(),
		Topic:       topic,
		EventType:   eventType,
		PublishedAt: time.Now().UTC(),
		Payload:     raw,
	}
	envBytes, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("events: marshal envelope for %s: %w", eventType, err)
	}

	// Use the eventType as the message key so events for the same logical
	// entity land on the same partition (order preserved per entity).
	return kp.p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: kafka.PartitionAny,
		},
		Key:   []byte(eventType),
		Value: envBytes,
	}, nil) // nil = async delivery report via Events() channel
}

func (kp *kafkaProducer) Close() {
	// Flush waits up to 10 seconds for all outstanding messages to be delivered.
	remaining := kp.p.Flush(10_000)
	if remaining > 0 {
		logger.FromContext(context.Background()).Warn().
			Int("remaining", remaining).
			Msg("kafka: producer closed with undelivered messages")
	}
	kp.p.Close()
}

// NoopProducer discards all events. Used in tests and local dev without Kafka.
type NoopProducer struct{}

func (NoopProducer) Publish(_ context.Context, _, _ string, _ any) error { return nil }
func (NoopProducer) Close()
