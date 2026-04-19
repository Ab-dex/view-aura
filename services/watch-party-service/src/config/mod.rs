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
pub struct JwtConfig {
    pub public_key_path: String,
    pub issuer:          String,
}

#[derive(Debug, Deserialize, Clone)]
pub struct RedisConfig {
    pub url:            String,
    pub party_ttl_secs: u64,
    pub chat_ttl_secs:  u64,
}

#[derive(Debug, Deserialize, Clone)]
pub struct PartyConfig {
    pub max_participants:  usize,
    pub sync_interval_ms:  u64,
    pub max_drift_ms:      u64,
    pub idle_timeout_secs: u64,
}

#[derive(Debug, Deserialize, Clone)]
pub struct LogConfig {
    pub level:  String,
    pub format: String,
}

#[derive(Debug, Deserialize, Clone)]
pub struct Settings {
    pub app:   AppConfig,
    pub jwt:   JwtConfig,
    pub redis: RedisConfig,
    pub party: PartyConfig,
    pub log:   LogConfig,
}

impl Settings {
    pub fn load() -> anyhow::Result<Self> {
        let env = std::env::var("PARTY_APP_ENV").unwrap_or_else(|_| "local".to_string());

        let s = CfgBuilder::builder()
            .add_source(File::with_name("config").required(true))
            .add_source(File::with_name(&format!("config.{env}")).required(false))
            .add_source(
                Environment::with_prefix("PARTY")
                    .separator("_")
                    .list_separator(","),
            )
            .build()?;

        Ok(s.try_deserialize()?)
    }
}