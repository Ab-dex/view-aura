package events

import (
	"fmt"

	"github.com/Ab-dex/view-aura/internal/platform/config"
	"github.com/google/wire"
)

// KafkaConfig holds all connection details for both producer and consumer.
// Populated from internal/platform/config.Config.Kafka.
type KafkaConfig struct {
	Brokers          string // comma-separated, e.g. "broker1:9092,broker2:9092"
	GroupID          string // consumer group ID, e.g. "viewaura-worker"
	SecurityProtocol string // "PLAINTEXT" | "SASL_SSL"
	SASLMechanism    string // "PLAIN" | "SCRAM-SHA-256"
	SASLUsername     string
	SASLPassword     string
}

// Validate returns an error if any required field is missing.
func (c KafkaConfig) Validate() error {
	if c.Brokers == "" {
		return fmt.Errorf("kafka: brokers must not be empty")
	}
	return nil
}

// ProvideProducer creates a Kafka Producer from config.
// Bind to NoopProducer in tests: wire.Bind(new(Producer), new(*NoopProducer))
func ProvideProducer(cfg KafkaConfig) (Producer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return NewProducer(
		config.KafkaConfig{
			Brokers:          cfg.Brokers,
			GroupID:          cfg.GroupID,
			SecurityProtocol: cfg.SecurityProtocol,
			SASLMechanism:    cfg.SASLMechanism,
			SASLUsername:     cfg.SASLUsername,
			SASLPassword:     cfg.SASLPassword,
		},
	)
}

// ProvideConsumer creates a Kafka Consumer from config.
func ProvideConsumer(cfg KafkaConfig) (Consumer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return NewConsumer(cfg)
}

// ProducerSet wires the Kafka producer. Consumer is wired separately only in
// cmd/worker — the HTTP server does not need a consumer.
var ProducerSet = wire.NewSet(ProvideProducer)

// WorkerProviderSet wires both producer and consumer for cmd/worker.
var WorkerProviderSet = wire.NewSet(ProvideProducer, ProvideConsumer)
