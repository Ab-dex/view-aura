mod api;
mod config;
mod consumer;
mod indexer;
mod search;
mod suggest;

use std::sync::Arc;

use anyhow::Context;
use redis::aio::ConnectionManager;
use tokio::{net::TcpListener, signal, sync::Mutex};
use tracing::info;
use tracing_subscriber::{fmt, prelude::*, EnvFilter};

use api::{router, AppState};
use config::Settings;
use indexer::MeiliSearchClient;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    // ── Load configuration ────────────────────────────────────────────────────
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

    info!(
        env    = %settings.app.env,
        port   = settings.app.port,
        meili  = %settings.meili.url,
        "search-service starting"
    );

    // ── MeiliSearch client + bootstrap ────────────────────────────────────────
    let meili = MeiliSearchClient::new(&settings.meili)
        .context("creating MeiliSearch client")?;

    meili.bootstrap().await.context("bootstrapping MeiliSearch indexes")?;
    info!("MeiliSearch indexes ready");

    let meili = Arc::new(meili);

    // ── Redis connection ──────────────────────────────────────────────────────
    let redis_client = redis::Client::open(settings.redis.url.as_str())
        .context("opening Redis connection")?;
    let redis_cm = ConnectionManager::new(redis_client)
        .await
        .context("creating Redis connection manager")?;
    let redis = Arc::new(Mutex::new(redis_cm));
    info!("Redis connected");

    // ── Kafka consumer ────────────────────────────────────────────────────────
    consumer::spawn(settings.kafka.clone(), Arc::clone(&meili));

    // ── HTTP server ───────────────────────────────────────────────────────────
    let state = AppState {
        meili:    Arc::clone(&meili),
        redis,
        settings: Arc::new(settings.clone()),
    };

    let app = router(state);
    let addr = format!("0.0.0.0:{}", settings.app.port);
    let listener = TcpListener::bind(&addr)
        .await
        .with_context(|| format!("binding to {addr}"))?;

    info!(addr = %addr, "search-service listening");

    axum::serve(listener, app)
        .with_graceful_shutdown(shutdown_signal(settings.app.shutdown_timeout()))
        .await
        .context("axum server error")?;

    info!("search-service shutdown complete");
    Ok(())
}

/// Wait for SIGINT (Ctrl+C) or SIGTERM, then return to trigger graceful shutdown.
async fn shutdown_signal(timeout: std::time::Duration) {
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

    info!(timeout_secs = timeout.as_secs(), "graceful shutdown initiated");
}