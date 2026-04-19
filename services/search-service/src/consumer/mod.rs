pub mod events;

use anyhow::Context;
use rdkafka::{
    config::ClientConfig,
    consumer::{Consumer, StreamConsumer},
    message::Message,
    util::get_rdkafka_version,
};
use std::sync::Arc;
use tracing::{error, info, warn};

use crate::{config::KafkaConfig, indexer::MeiliSearchClient};
use events::{handle_event, Envelope};

/// Spawn the Kafka consumer loop as a background task.
/// Returns immediately — the loop runs inside a dedicated tokio task.
pub fn spawn(cfg: KafkaConfig, meili: Arc<MeiliSearchClient>) {
    if !cfg.enabled() {
        info!("kafka brokers not configured — consumer disabled");
        return;
    }

    tokio::spawn(async move {
        if let Err(e) = run_consumer(cfg, meili).await {
            error!(error = %e, "kafka consumer crashed");
        }
    });
}

async fn run_consumer(cfg: KafkaConfig, meili: Arc<MeiliSearchClient>) -> anyhow::Result<()> {
    let (version_n, version_s) = get_rdkafka_version();
    info!(rdkafka_version = version_s, version_n, "starting kafka consumer");

    let consumer: StreamConsumer = ClientConfig::new()
        .set("bootstrap.servers", &cfg.brokers)
        .set("group.id", &cfg.group_id)
        .set("auto.offset.reset", &cfg.auto_offset_reset)
        .set("enable.auto.commit", "true")
        .set("auto.commit.interval.ms", "1000")
        .set("session.timeout.ms", "30000")
        .set("max.poll.interval.ms", "300000")
        // Idempotent consumer: exactly once delivery on the read side
        // is handled by MeiliSearch's upsert semantics.
        .create()
        .context("creating kafka consumer")?;

    let topics: Vec<&str> = cfg.topics.iter().map(String::as_str).collect();
    consumer.subscribe(&topics).context("subscribing to topics")?;

    info!(topics = ?topics, "kafka consumer subscribed");

    loop {
        match consumer.recv().await {
            Err(e) => {
                warn!(error = %e, "kafka receive error — retrying");
            }
            Ok(msg) => {
                let payload = match msg.payload_view::<str>() {
                    Some(Ok(s)) => s.to_string(),
                    Some(Err(e)) => {
                        warn!(error = %e, "non-utf8 kafka payload — skipping");
                        continue;
                    }
                    None => {
                        warn!("empty kafka payload — skipping");
                        continue;
                    }
                };

                let topic = msg.topic();
                let partition = msg.partition();
                let offset = msg.offset();

                match serde_json::from_str::<Envelope>(&payload) {
                    Ok(envelope) => {
                        if let Err(e) = handle_event(&envelope, &meili).await {
                            error!(
                                error = %e,
                                topic,
                                partition,
                                offset,
                                event_type = %envelope.event_type,
                                "failed to handle event — skipping (at-least-once)"
                            );
                        }
                    }
                    Err(e) => {
                        warn!(
                            error = %e,
                            topic,
                            partition,
                            offset,
                            "failed to deserialise envelope — skipping"
                        );
                    }
                }
            }
        }
    }
}