package resilience

import (
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/events"
	"github.com/Ab-dex/view-aura/internal/platform/cache"
	"github.com/Ab-dex/view-aura/internal/platform/config"
)

// ProvideResilientProducer builds the three-tier Kafka producer.
//
// rdb may be nil when Redis is not configured — the producer degrades directly
// to Tier 3 (file outbox) in that case.
//
// The caller must ensure that the events.Producer passed as `primary` is the
// raw Kafka producer (or NoopProducer) — not another ResilientProducer —
// so we don't create a circular fallback chain.
func ProvideResilientProducer(
	primary *events.PrimaryProducer,
	rdb *cache.Client,
	cfg *config.Config,
) *ResilientProducer {

	logDir := cfg.Resilience.LogDir
	if rdb != nil {
		return NewResilientProducer(primary, rdb.Client, logDir)
	}
	return NewResilientProducer(primary, nil, logDir)
}

// ProvideResilientMailer builds the three-tier email sender.
//
// rdb may be nil — mailer degrades directly to file outbox without Redis.
// emailServiceURL is pulled from cfg.Resilience.EmailServiceURL; leave empty
// to disable Tier 1 in local dev without the Node.js email service running.
func ProvideResilientMailer(
	rdb *cache.Client,
	cfg *config.Config,
) *ResilientMailer {
	emailURL := cfg.Resilience.EmailServiceURL
	logDir := cfg.Resilience.LogDir
	if rdb != nil {
		return NewResilientMailer(emailURL, rdb.Client, logDir)
	}
	return NewResilientMailer(emailURL, nil, logDir)
}

// ProvideOutboxWorker builds the background drain worker.
//
// rdb may be nil — the worker runs but no-ops on every tick when there is
// no Redis to drain from.  This is intentional: the worker must always be
// registered so it is ready to drain as soon as Redis becomes available.
func ProvideOutboxWorker(
	rdb *cache.Client,
	prod *ResilientProducer,
	mailer *ResilientMailer,
) *OutboxWorker {
	if rdb != nil {
		return NewOutboxWorker(rdb.Client, prod, mailer)
	}
	return NewOutboxWorker(nil, prod, mailer)
}

// ProviderSet wires the entire resilience layer into the application.
//
// Wire binding declared here:
//   - *ResilientProducer satisfies events.Producer everywhere so all modules
//     automatically get three-tier Kafka fallback without any module changes.
//
// Modules that consume events.Producer (rating, review, social, auth, moderation,
// upload, payment) receive the ResilientProducer transparently.
var ProviderSet = wire.NewSet(
	ProvideResilientProducer,
	wire.Bind(new(events.Producer), new(*ResilientProducer)),
	ProvideResilientMailer,
	ProvideOutboxWorker,
)

// func ProvideResilientProducer(
// 	primary *events.PrimaryProducer,
// 	rdb *cache.Client,
// 	cfg *config.Config,
// ) *ResilientProducer {
// 	return NewResilientProducer(primary, rdb.Client, cfg.Resilience.LogDir)
// }
// var ProviderSet = wire.NewSet(
// 	events.ProducerSet,
// 	ProvideResilientProducer,
// 	wire.Bind(new(events.Producer), new(*ResilientProducer)),
// )
