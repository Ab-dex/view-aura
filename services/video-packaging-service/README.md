# video-packaging-service

Rust service that transcodes uploaded video files into adaptive HLS streams, generates thumbnail strips, and uploads the output back to Cloudflare R2. Called by the Temporal `TranscodeVideoActivity` and `ExtractMetadataActivity` in the upload pipeline.

Port: **9003**

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Rust | ≥ 1.78 | `curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \| sh` |
| FFmpeg | ≥ 6.0 | see below |

Unlike the search service, this service has **no C build dependencies** in Cargo — it invokes `ffmpeg` and `ffprobe` as subprocess commands. The Rust binary itself builds with a standard `cargo build`.

### Install FFmpeg

```bash
# Ubuntu / Debian
sudo apt install ffmpeg

# macOS
brew install ffmpeg

# Verify
ffmpeg -version
ffprobe -version
```

FFmpeg must have been compiled with **libx264** support (`--enable-libx264`). The package manager versions above include it by default.

For GPU acceleration (NVENC), your system needs an NVIDIA GPU with CUDA drivers and an FFmpeg build compiled with `--enable-nvenc`. The service detects NVENC automatically at runtime and falls back to libx264 if unavailable.

---

## Local setup

### 1. Build

```bash
cargo build
# No cmake, no clang, no native deps — builds in ~45 seconds
```

### 2. Configure

```bash
# Copy and edit
cp config.yaml config.local.yaml
```

For local development without R2, the service can transcode local files and write output to the filesystem. Set `r2.endpoint` to an empty string — the upload step will no-op.

```bash
export VIDEO_APP_ENV=local
export VIDEO_R2_ENDPOINT=https://<account_id>.r2.cloudflarestorage.com
export VIDEO_R2_ACCESS_KEY_ID=your-key
export VIDEO_R2_SECRET_ACCESS_KEY=your-secret
export VIDEO_R2_BUCKET=cinemaos-assets
```

### 3. Run

```bash
cargo run
# or
cargo run --release
```

Service starts on `http://localhost:9003`.

---

## Docker

```bash
docker build -t viewaura/video-packaging-service:local .
docker run -d \
  --name video-packaging \
  -p 9003:9003 \
  -e VIDEO_R2_ENDPOINT=https://<account_id>.r2.cloudflarestorage.com \
  -e VIDEO_R2_ACCESS_KEY_ID=your-key \
  -e VIDEO_R2_SECRET_ACCESS_KEY=your-secret \
  viewaura/video-packaging-service:local
```

---

## API reference

### `GET /health`
```json
{ "status": "ok", "service": "video-packaging-service" }
```

### `POST /api/v1/transcode`

Called by the Temporal `TranscodeVideoActivity`.

```json
{
  "source_key": "uploads/user-id/upload-id/movie.mp4",
  "asset_id":   "550e8400-e29b-41d4-a716-446655440000"
}
```

Response:
```json
{
  "asset_id":       "550e8400...",
  "master_key":     "assets/550e8400/master.m3u8",
  "rendition_keys": ["assets/550e8400/1080p/index.m3u8", "..."],
  "thumbnail_keys": ["assets/550e8400/thumbs/thumb_0000.jpg", "..."],
  "duration_secs":  5400.0,
  "width":          1920,
  "height":         1080,
  "codec_name":     "h264"
}
```

### `POST /api/v1/probe`

Called by the Temporal `ExtractMetadataActivity`. Returns metadata without transcoding.

```json
{ "source_key": "uploads/user-id/upload-id/movie.mp4" }
```

---

## HLS output structure

```
assets/{asset_id}/
├── master.m3u8          ← adaptive manifest (points to all renditions)
├── 1080p/
│   ├── index.m3u8
│   ├── seg000.ts
│   ├── seg001.ts
│   └── ...
├── 720p/  ...
├── 480p/  ...
├── 360p/  ...
└── thumbs/
    ├── thumb_0000.jpg   (0s)
    ├── thumb_0010.jpg   (10s)
    └── ...
```

---

## GPU acceleration

The service probes for NVENC on startup by running `ffmpeg -encoders`. If `h264_nvenc` is listed, hardware encoding is used automatically. No configuration change is needed.

On Kubernetes GPU nodes, add the NVENC toleration and GPU resource request to the worker pod spec:

```yaml
resources:
  limits:
    nvidia.com/gpu: 1
tolerations:
  - key: "workload"
    value: "transcoding"
    effect: "NoSchedule"
```

---

## Module layout

```
src/
├── main.rs        — entrypoint, binary verification, graceful shutdown
├── config/        — Settings from config.yaml + VIDEO_ env vars
├── metadata/      — FFprobe subprocess + VideoMetadata struct
├── transcoder/    — FFmpeg HLS transcode, master playlist generation
├── thumbnailer/   — FFmpeg thumbnail extraction
├── r2/            — AWS SDK v1 configured for Cloudflare R2
└── api/           — Axum router: /transcode + /probe handlers
```

---

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `VIDEO_APP_ENV` | `local` | `local` \| `staging` \| `production` |
| `VIDEO_APP_PORT` | `9003` | HTTP listen port |
| `VIDEO_FFMPEG_FFMPEG_BIN` | `ffmpeg` | Path to ffmpeg binary |
| `VIDEO_FFMPEG_FFPROBE_BIN` | `ffprobe` | Path to ffprobe binary |
| `VIDEO_FFMPEG_MAX_CONCURRENT_JOBS` | `4` | Parallel transcode jobs |
| `VIDEO_FFMPEG_WORK_DIR` | `/tmp/vp-workspace` | Temp dir for intermediate files |
| `VIDEO_R2_ENDPOINT` | — | R2 endpoint URL |
| `VIDEO_R2_BUCKET` | `cinemaos-assets` | Target bucket name |
| `VIDEO_R2_ACCESS_KEY_ID` | — | R2 access key |
| `VIDEO_R2_SECRET_ACCESS_KEY` | — | R2 secret key |
| `VIDEO_LOG_LEVEL` | `debug` | Log level |
| `VIDEO_LOG_FORMAT` | `pretty` | `pretty` \| `json` |