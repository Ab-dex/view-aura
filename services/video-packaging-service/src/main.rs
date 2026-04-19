mod api;
mod config;
mod metadata;
mod r2;
mod thumbnailer;
mod transcoder;

use std::sync::Arc;
use anyhow::Context;
use tokio::{net::TcpListener, signal};
use tracing::info;
use tracing_subscriber::{fmt, prelude::*, EnvFilter};

use api::{router, AppState};
use config::Settings;
use r2::R2Client;

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

    info!(
        env  = %settings.app.env,
        port = settings.app.port,
        "video-packaging-service starting"
    );

    // ── Verify FFmpeg and FFprobe are available ────────────────────────────────
    verify_binary(&settings.ffmpeg.ffmpeg_bin, &["-version"]).await?;
    verify_binary(&settings.ffmpeg.ffprobe_bin, &["-version"]).await?;

    // Ensure the working directory exists.
    tokio::fs::create_dir_all(&settings.ffmpeg.work_dir)
        .await
        .context("creating ffmpeg work dir")?;

    // ── R2 client ─────────────────────────────────────────────────────────────
    let r2 = Arc::new(R2Client::new(&settings.r2).context("creating R2 client")?);

    // ── HTTP server ───────────────────────────────────────────────────────────
    let state = AppState {
        settings: Arc::new(settings.clone()),
        r2,
    };

    let addr = format!("0.0.0.0:{}", settings.app.port);
    let listener = TcpListener::bind(&addr)
        .await
        .with_context(|| format!("binding to {addr}"))?;

    info!(addr = %addr, "video-packaging-service listening");

    axum::serve(listener, router(state))
        .with_graceful_shutdown(shutdown_signal())
        .await
        .context("axum server error")?;

    info!("video-packaging-service shutdown complete");
    Ok(())
}

async fn verify_binary(bin: &str, args: &[&str]) -> anyhow::Result<()> {
    let out = tokio::process::Command::new(bin)
        .args(args)
        .output()
        .await
        .with_context(|| format!("'{bin}' not found — install FFmpeg"))?;

    if !out.status.success() {
        anyhow::bail!("'{bin}' returned non-zero exit code");
    }

    let first_line = String::from_utf8_lossy(&out.stdout)
        .lines()
        .next()
        .unwrap_or("")
        .to_string();

    info!(binary = bin, version_line = %first_line, "binary verified");
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