mod api;
mod auth;
mod config;
mod protocol;
mod redis_bus;
mod room;

use std::sync::Arc;
use anyhow::Context;
use tokio::{net::TcpListener, signal};
use tracing::info;
use tracing_subscriber::{fmt, prelude::*, EnvFilter};

use api::{router, AppState};
use auth::JwtValidator;
use config::Settings;
use redis_bus::RedisBus;
use room::RoomRegistry;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    // ── Config ────────────────────────────────────────────────────────────────
    let settings = Settings::load().context("loading configuration")?;

    // ── Logging ───────────────────────────────────────────────────────────────
    let filter = EnvFilter::try_from_default_env()
        .unwrap_or_else(|_| EnvFilter::new(&settings.log.level));

    if settings.log.format == "json" {
        tracing_subscriber::registry()
            .with(filter)
            .with(fmt::layer().json())
            .init();
    } else {
        tracing_subscriber::registry()
            .with(filter)
            .with(fmt::layer().pretty())
            .init();
    }

    info!(env = %settings.app.env, port = settings.app.port, "watch-party-service starting");

    // ── JWT validator ─────────────────────────────────────────────────────────
    let jwt = Arc::new(JwtValidator::new(&settings.jwt)?);

    // ── Redis bus ─────────────────────────────────────────────────────────────
    let redis = RedisBus::new(
        &settings.redis.url,
        settings.redis.party_ttl_secs,
        settings.redis.chat_ttl_secs,
    )
    .await
    .context("connecting to Redis")?;

    info!("Redis connected");

    // ── Room registry ─────────────────────────────────────────────────────────
    let registry = RoomRegistry::new();

    // ── HTTP/WebSocket server ─────────────────────────────────────────────────
    let state = AppState {
        settings: Arc::new(settings.clone()),
        registry,
        redis,
        jwt,
    };

    let addr = format!("0.0.0.0:{}", settings.app.port);
    let listener = TcpListener::bind(&addr)
        .await
        .with_context(|| format!("binding to {addr}"))?;

    info!(addr = %addr, "watch-party-service listening");

    axum::serve(listener, router(state))
        .with_graceful_shutdown(shutdown_signal())
        .await
        .context("axum server error")?;

    info!("watch-party-service shutdown complete");
    Ok(())
}

async fn shutdown_signal() {
    let ctrl_c = async {
        signal::ctrl_c().await.expect("failed to install Ctrl+C handler");
    };

    #[cfg(unix)]
    let terminate = async {
        signal::unix::signal(signal::unix::SignalKind::terminate())
            .expect("failed to install SIGTERM handler")
            .recv()
            .await;
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        _ = ctrl_c    => info!("received Ctrl+C"),
        _ = terminate => info!("received SIGTERM"),
    }
}