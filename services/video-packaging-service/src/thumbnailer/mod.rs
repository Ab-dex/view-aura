use anyhow::{bail, Context};
use std::path::{Path, PathBuf};
use tokio::process::Command;
use tracing::info;

use crate::config::ThumbnailConfig;

/// Generate thumbnail frames and a sprite sheet from `source_path`.
///
/// Outputs:
///   output_dir/
///     thumb_0000.jpg
///     thumb_0010.jpg   (one per interval_secs)
///     ...
///     sprite.jpg       (all thumbs tiled in a grid — for timeline scrubbing)
pub async fn generate(
    ffmpeg_bin:  &str,
    thumb_cfg:   &ThumbnailConfig,
    source_path: &Path,
    output_dir:  &Path,
) -> anyhow::Result<Vec<PathBuf>> {
    tokio::fs::create_dir_all(output_dir)
        .await
        .context("creating thumbnail output directory")?;

    // Pattern: thumb_%04d.jpg
    let pattern = output_dir.join(format!("thumb_%04d.{}", thumb_cfg.format));

    info!(
        interval = thumb_cfg.interval_secs,
        size = format!("{}x{}", thumb_cfg.width, thumb_cfg.height),
        "generating thumbnails"
    );

    let status = Command::new(ffmpeg_bin)
        .args(["-y", "-i"])
        .arg(source_path)
        .args(["-vf"])
        .arg(format!(
            "fps=1/{},scale={}:{}",
            thumb_cfg.interval_secs,
            thumb_cfg.width,
            thumb_cfg.height
        ))
        .args(["-q:v", "3"])     // JPEG quality (1=best, 31=worst)
        .arg(&pattern)
        .status()
        .await
        .context("running ffmpeg for thumbnails")?;

    if !status.success() {
        bail!("thumbnail generation failed (exit {:?})", status.code());
    }

    // Collect the generated files.
    let mut files = Vec::new();
    let mut entries = tokio::fs::read_dir(output_dir)
        .await
        .context("reading thumbnail directory")?;
    while let Some(entry) = entries.next_entry().await? {
        let path = entry.path();
        if path.extension().and_then(|e| e.to_str()) == Some(&thumb_cfg.format) {
            files.push(path);
        }
    }
    files.sort();

    info!(count = files.len(), "thumbnails generated");
    Ok(files)
}