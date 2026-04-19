use config::{Config as CfgBuilder, Environment, File};
use serde::Deserialize;
use std::time::Duration;

#[derive(Debug, Deserialize, Clone)]
pub struct AppConfig {
    pub env:              String,
    pub port:             u16,
    pub shutdown_timeout: String,
}

impl AppConfig {
    pub fn shutdown_timeout(&self) -> Duration {
        // Parse "30s", "1m", etc. — simple suffix parser.
        let s = &self.shutdown_timeout;
        if s.ends_with('s') {
            Duration::from_secs(s.trim_end_matches('s').parse().unwrap_or(30))
        } else if s.ends_with('m') {
            Duration::from_secs(s.trim_end_matches('m').parse::<u64>().unwrap_or(1) * 60)
        } else {
            Duration::from_secs(30)
        }
    }
}

#[derive(Debug, Deserialize, Clone)]
pub struct MeiliConfig {
    pub url:                  String,
    pub master_key:           String,
    pub movie_index:          String,
    pub person_index:         String,
    pub max_values_per_facet: usize,
}

#[derive(Debug, Deserialize, Clone)]
pub struct KafkaConfig {
    pub brokers:           String,
    pub group_id:          String,
    pub auto_offset_reset: String,
    pub topics:            Vec<String>,
}

impl KafkaConfig {
    /// Returns true when Kafka is configured (brokers non-empty).
    pub fn enabled(&self) -> bool {
        !self.brokers.trim().is_empty()
    }
}

#[derive(Debug, Deserialize, Clone)]
pub struct RedisConfig {
    pub url:               String,
    pub suggest_ttl_secs:  u64,
}

#[derive(Debug, Deserialize, Clone)]
pub struct LogConfig {
    pub level:  String,
    pub format: String,
}

#[derive(Debug, Deserialize, Clone)]
pub struct TracingConfig {
    pub endpoint:     String,
    pub service_name: String,
}

#[derive(Debug, Deserialize, Clone)]
pub struct Settings {
    pub app:     AppConfig,
    pub meili:   MeiliConfig,
    pub kafka:   KafkaConfig,
    pub redis:   RedisConfig,
    pub log:     LogConfig,
    pub tracing: TracingConfig,
}

impl Settings {
    /// Load configuration from:
    ///   1. config.yaml (base)
    ///   2. config.{env}.yaml (environment overlay, optional)
    ///   3. Environment variables prefixed SEARCH_ (highest priority)
    pub fn load() -> anyhow::Result<Self> {
        let env = std::env::var("SEARCH_APP_ENV").unwrap_or_else(|_| "local".to_string());

        let settings = CfgBuilder::builder()
            .add_source(File::with_name("config").required(true))
            .add_source(File::with_name(&format!("config.{env}")).required(false))
            .add_source(
                Environment::with_prefix("SEARCH")
                    .separator("_")
                    .list_separator(","),
            )
            .build()?;

        Ok(settings.try_deserialize()?)
    }
}