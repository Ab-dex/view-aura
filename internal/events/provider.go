package events

import (
	"github.com/Ab-dex/view-aura/internal/platform/config"
	"github.com/google/wire"
)

// PrimaryProducer is a named wrapper around the raw Kafka producer.
// It gives Wire a distinct concrete type so it can differentiate:
//
//	*PrimaryProducer  — the unwrapped Kafka producer (this type)
//	events.Producer   — the resilient wrapper (provided by resilience.ProviderSet)
//
// Without this distinction, ProvideResilientProducer(Producer) → Producer
// creates an unresolvable self-referential cycle in the Wire graph.
type PrimaryProducer struct{ Producer }

// ProvideKafkaConfig extracts the Kafka sub-config from the root Config.
func ProvideKafkaConfig(cfg *config.Config) config.KafkaConfig {
	return cfg.Kafka
}

// ProvidePrimaryProducer constructs the raw Kafka producer as *PrimaryProducer.
func ProvidePrimaryProducer(cfg config.KafkaConfig) (*PrimaryProducer, error) {
	if cfg.Brokers == "" {
		// return nil, fmt.Errorf("kafka: brokers must not be empty")
		return nil, nil
	}
	p, err := NewProducer(cfg)
	if err != nil {
		return nil, err
	}
	return &PrimaryProducer{p}, nil
}

// ProvideConsumer constructs the Kafka consumer.
func ProvideConsumer(cfg config.KafkaConfig) (Consumer, error) {
	if cfg.Brokers == "" {
		// return nil, fmt.Errorf("kafka: brokers must not be empty")
		return nil, nil
	}
	return NewConsumer(cfg)
}

// ProducerSet provides *PrimaryProducer only.
// The events.Producer binding is owned by resilience.ProviderSet.
var ProviderSet = wire.NewSet(ProvideKafkaConfig, ProvidePrimaryProducer)

// WorkerProviderSet provides *PrimaryProducer and Consumer for cmd/worker.
// The worker publishes directly via *PrimaryProducer — no resilience wrapper.
var WorkerProviderSet = wire.NewSet(
	ProvideKafkaConfig,
	ProvidePrimaryProducer,
	ProvideConsumer,
	// Bind *PrimaryProducer → Producer so the worker's constructors that
	// take events.Producer are satisfied without needing a resilience layer.
	wire.Bind(new(Producer), new(*PrimaryProducer)),
)
