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
            Duration::from_secs(s.trim_end_matches('s').parse().unwrap_or(60))
        } else if s.ends_with('m') {
            Duration::from_secs(s.trim_end_matches('m').parse::<u64>().unwrap_or(1) * 60)
        } else {
            Duration::from_secs(60)
        }
    }
}

#[derive(Debug, Deserialize, Clone)]
pub struct FfmpegConfig {
    pub ffmpeg_bin:          String,
    pub ffprobe_bin:         String,
    pub max_concurrent_jobs: usize,
    pub work_dir:            String,
}

#[derive(Debug, Deserialize, Clone)]
pub struct Rendition {
    pub name:     String,
    pub width:    u32,
    pub height:   u32,
    pub bitrate:  String,
    pub audio_br: String,
}

#[derive(Debug, Deserialize, Clone)]
pub struct ThumbnailConfig {
    pub interval_secs: u32,
    pub width:         u32,
    pub height:        u32,
    pub format:        String,
}

#[derive(Debug, Deserialize, Clone)]
pub struct R2Config {
    pub endpoint:          String,
    pub bucket:            String,
    pub access_key_id:     String,
    pub secret_access_key: String,
    pub region:            String,
}

#[derive(Debug, Deserialize, Clone)]
pub struct LogConfig {
    pub level:  String,
    pub format: String,
}

#[derive(Debug, Deserialize, Clone)]
pub struct Settings {
    pub app:        AppConfig,
    pub ffmpeg:     FfmpegConfig,
    pub renditions: Vec<Rendition>,
    pub thumbnails: ThumbnailConfig,
    pub r2:         R2Config,
    pub log:        LogConfig,
}

impl Settings {
    pub fn load() -> anyhow::Result<Self> {
        let env = std::env::var("VIDEO_APP_ENV").unwrap_or_else(|_| "local".to_string());

        let s = CfgBuilder::builder()
            .add_source(File::with_name("config").required(true))
            .add_source(File::with_name(&format!("config.{env}")).required(false))
            .add_source(
                Environment::with_prefix("VIDEO")
                    .separator("_")
                    .list_separator(","),
            )
            .build()?;

        Ok(s.try_deserialize()?)
    }
}