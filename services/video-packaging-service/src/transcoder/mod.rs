use anyhow::{bail, Context};
use serde::Serialize;
use std::path::{Path, PathBuf};
use tokio::process::Command;
use tracing::{info, warn};

use crate::config::{FfmpegConfig, Rendition};

/// TranscodeOutput describes the files produced for one asset.
#[derive(Debug, Serialize, Clone)]
pub struct TranscodeOutput {
    /// Path to the HLS master playlist file.
    pub master_playlist: PathBuf,
    /// All files produced (playlists + .ts segments + thumbnails).
    pub all_files:       Vec<PathBuf>,
    /// Per-rendition info.
    pub renditions:      Vec<RenditionOutput>,
}

#[derive(Debug, Serialize, Clone)]
pub struct RenditionOutput {
    pub name:     String,
    pub playlist: PathBuf,
    pub width:    u32,
    pub height:   u32,
    pub bitrate:  String,
}

/// Transcode `source_path` into an adaptive HLS ladder under `output_dir`.
///
/// Produces:
///   output_dir/
///     master.m3u8
///     1080p/
///       index.m3u8
///       seg000.ts  seg001.ts  ...
///     720p/  ...
///     480p/  ...
///     360p/  ...
///
/// Each rendition is encoded independently using libx264 + AAC.
/// GPU acceleration (h264_nvenc) is automatically tried first and falls
/// back to libx264 when unavailable.
pub async fn transcode(
    cfg: &FfmpegConfig,
    renditions: &[Rendition],
    source_path: &Path,
    output_dir:  &Path,
) -> anyhow::Result<TranscodeOutput> {
    // Detect GPU availability once.
    let use_gpu = has_nvenc(&cfg.ffmpeg_bin).await;
    if use_gpu {
        info!("GPU encoder (h264_nvenc) available — using hardware acceleration");
    } else {
        info!("GPU encoder not available — using libx264 (CPU)");
    }

    tokio::fs::create_dir_all(output_dir)
        .await
        .context("creating output directory")?;

    let mut rendition_outputs = Vec::new();
    let mut all_files: Vec<PathBuf> = Vec::new();

    for r in renditions {
        let rdir = output_dir.join(&r.name);
        tokio::fs::create_dir_all(&rdir)
            .await
            .context("creating rendition directory")?;

        let playlist = rdir.join("index.m3u8");
        let segment_pattern = rdir.join("seg%03d.ts");

        let vcodec = if use_gpu { "h264_nvenc" } else { "libx264" };
        let preset = if use_gpu { "p4" } else { "veryfast" };

        info!(
            rendition = %r.name,
            vcodec,
            "transcoding rendition"
        );

        let status = Command::new(&cfg.ffmpeg_bin)
            .args([
                "-y",                    // overwrite without asking
                "-i",
            ])
            .arg(source_path)
            .args([
                "-vf",
            ])
            // Scale: maintain aspect ratio, pad to even dimensions
            .arg(format!("scale={}:{},setsar=1", r.width, r.height))
            .args([
                "-c:v",   vcodec,
                "-preset", preset,
                "-b:v",   &r.bitrate,
                "-maxrate", &r.bitrate,
                "-bufsize",
            ])
            .arg(double_bitrate(&r.bitrate))
            .args([
                "-c:a",   "aac",
                "-b:a",   &r.audio_br,
                "-ac",    "2",        // force stereo
                // HLS output options
                "-f",     "hls",
                "-hls_time", "4",        // 4-second segments
                "-hls_playlist_type", "vod",
                "-hls_segment_filename",
            ])
            .arg(&segment_pattern)
            .arg(&playlist)
            .status()
            .await
            .with_context(|| format!("running ffmpeg for rendition {}", r.name))?;

        if !status.success() {
            bail!("ffmpeg failed for rendition {} (exit {:?})", r.name, status.code());
        }

        // Collect produced files.
        let mut entries = tokio::fs::read_dir(&rdir)
            .await
            .context("reading rendition dir")?;
        while let Some(entry) = entries.next_entry().await? {
            all_files.push(entry.path());
        }

        rendition_outputs.push(RenditionOutput {
            name:     r.name.clone(),
            playlist: playlist.clone(),
            width:    r.width,
            height:   r.height,
            bitrate:  r.bitrate.clone(),
        });
    }

    // Write the master playlist.
    let master = write_master_playlist(output_dir, renditions, &rendition_outputs).await?;
    all_files.push(master.clone());

    info!(
        renditions = rendition_outputs.len(),
        files = all_files.len(),
        "transcoding complete"
    );

    Ok(TranscodeOutput {
        master_playlist: master,
        all_files,
        renditions: rendition_outputs,
    })
}

async fn write_master_playlist(
    output_dir:   &Path,
    renditions:   &[Rendition],
    outputs:      &[RenditionOutput],
) -> anyhow::Result<PathBuf> {
    let mut m3u8 = String::from("#EXTM3U\n#EXT-X-VERSION:3\n\n");

    for (r, o) in renditions.iter().zip(outputs.iter()) {
        let bw = parse_bitrate_bps(&r.bitrate);
        m3u8.push_str(&format!(
            "#EXT-X-STREAM-INF:BANDWIDTH={bw},RESOLUTION={}x{}\n{}/index.m3u8\n\n",
            r.width, r.height, r.name
        ));
    }

    let path = output_dir.join("master.m3u8");
    tokio::fs::write(&path, &m3u8)
        .await
        .context("writing master playlist")?;

    Ok(path)
}

/// Probe for NVENC support by asking ffmpeg to list encoders.
async fn has_nvenc(ffmpeg_bin: &str) -> bool {
    match Command::new(ffmpeg_bin)
        .args(["-encoders", "-v", "quiet"])
        .output()
        .await
    {
        Ok(out) => String::from_utf8_lossy(&out.stdout).contains("h264_nvenc"),
        Err(_)  => false,
    }
}

/// Double a bitrate string: "4000k" → "8000k"
fn double_bitrate(bitrate: &str) -> String {
    if let Some(stripped) = bitrate.strip_suffix('k') {
        if let Ok(n) = stripped.parse::<u64>() {
            return format!("{}k", n * 2);
        }
    }
    bitrate.to_string()
}

/// Parse "4000k" → 4_000_000 bps
fn parse_bitrate_bps(bitrate: &str) -> u64 {
    if let Some(s) = bitrate.strip_suffix('k') {
        s.parse::<u64>().unwrap_or(0) * 1_000
    } else if let Some(s) = bitrate.strip_suffix('M') {
        s.parse::<u64>().unwrap_or(0) * 1_000_000
    } else {
        bitrate.parse().unwrap_or(0)
    }
}