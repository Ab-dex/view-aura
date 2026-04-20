# moderation-service

Python AI service for content screening. Classifies text and images for NSFW content, hate speech, and spoilers. Called by the Go moderation module and the Temporal upload pipeline.

Port: **9102**

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Python | ≥ 3.11 | [python.org](https://python.org) |
| pip | ≥ 23 | bundled with Python |

GPU acceleration requires an NVIDIA GPU with CUDA 12.1 drivers. The service falls back to CPU automatically.

---

## Local setup

### 1. Create virtual environment

```bash
python -m venv .venv
source .venv/bin/activate    # Windows: .venv\Scripts\activate
```

### 2. Install dependencies

```bash
pip install -r requirements.txt
```

### 3. First run — model download

Models are downloaded from HuggingFace Hub on first startup (~2–4 GB total). Set `HF_HOME` to a persistent directory to avoid re-downloading on restart:

```bash
export HF_HOME=/path/to/model/cache
```

On Kubernetes, mount a PVC at `/root/.cache/huggingface`.

### 4. Configure

```bash
# Override config values via environment variables (prefix: MODERATION_)
export MODERATION_APP_ENV=local
export MODERATION_MODELS__DEVICE=cpu       # or "cuda" on GPU nodes
export MODERATION_LOG_LEVEL=DEBUG
```

### 5. Run

```bash
python -m app.main
# or
uvicorn app.main:app --host 0.0.0.0 --port 9102 --reload
```

Swagger UI: `http://localhost:9102/docs`

---

## API

### `GET /health`

```json
{
  "status": "ok",
  "models": {
    "nsfw":        "ready",
    "hate_speech": "ready",
    "spoiler":     "ready"
  }
}
```

Returns `503` while models are loading on startup. Used as the Kubernetes readiness probe.

### `POST /screen`

Called by the Go `httpAIClient.Screen()` and by `ScreenContentActivity` in the Temporal upload pipeline.

Request:
```json
{
  "content_type": "review",
  "content_id":   "uuid",
  "text":         "The butler did it in the final scene when...",
  "frame_urls":   [],
  "image_url":    null
}
```

Response:
```json
{
  "signals":        ["spoiler"],
  "confidence":     { "spoiler": 0.87 },
  "recommendation": "escalate_to_human"
}
```

Recommendation values:
| Value | Meaning |
|-------|---------|
| `approve` | No signals detected |
| `reject` | Hard-reject threshold exceeded (e.g. nsfw > 0.90) |
| `escalate_to_human` | Signals detected but below auto-reject threshold |

---

## What gets screened per content type

| `content_type` | NSFW | Hate speech | Spoiler |
|----------------|------|-------------|---------|
| `review`       | ✗    | ✓           | ✓       |
| `upload`       | ✓ (frames) | ✓ (caption) | ✗  |
| `post`         | ✓ (image) | ✓          | ✓      |
| `profile`      | ✓ (avatar) | ✓ (bio)   | ✗      |
| `comment`      | ✗    | ✓           | ✗       |

---

## Models

| Classifier | HuggingFace model | Size |
|-----------|-------------------|------|
| NSFW image | `Falconsai/nsfw_image_detection` | ~350MB |
| Hate speech | `Hate-speech-CNERG/dehatebert-mono-english` | ~440MB |
| Spoiler (zero-shot) | `facebook/bart-large-mnli` | ~1.6GB |

To swap in a different model, change `models.nsfw_image`, `models.hate_speech`, or `models.spoiler_zsl` in `config.yaml` or via environment variable.

---

## Module layout

```
app/
├── main.py           — FastAPI app + uvicorn entrypoint + lifespan (model loading)
├── config/
│   └── __init__.py   — Pydantic Settings (config.yaml + MODERATION_ env vars)
├── models/
│   └── __init__.py   — Pydantic request/response schemas
├── classifiers/
│   ├── __init__.py   — Screening engine: orchestrates classifiers + decision logic
│   ├── nsfw.py       — NSFW image classifier (ViT pipeline)
│   ├── hate_speech.py — Hate speech text classifier (BERT pipeline)
│   └── spoiler.py    — Zero-shot spoiler detector (BART-NLI pipeline)
└── api/
    └── __init__.py   — FastAPI router: /health + /screen
```

---

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MODERATION_APP_ENV` | `local` | `local` \| `staging` \| `production` |
| `MODERATION_APP_PORT` | `9102` | HTTP listen port |
| `MODERATION_APP_WORKERS` | `1` | uvicorn workers (keep at 1 with GPU) |
| `MODERATION_MODELS__DEVICE` | `cpu` | `cpu` \| `cuda` \| `mps` |
| `MODERATION_MODELS__NSFW_IMAGE` | `Falconsai/nsfw_image_detection` | HuggingFace model ID |
| `MODERATION_MODELS__HATE_SPEECH` | `Hate-speech-CNERG/dehatebert-mono-english` | HuggingFace model ID |
| `MODERATION_MODELS__SPOILER_ZSL` | `facebook/bart-large-mnli` | HuggingFace model ID |
| `MODERATION_THRESHOLDS__NSFW_REJECT` | `0.90` | Auto-reject above this |
| `MODERATION_THRESHOLDS__ESCALATE_THRESHOLD` | `0.70` | Escalate to human above this |
| `MODERATION_LOG_LEVEL` | `DEBUG` | Log level |
| `HF_HOME` | `~/.cache/huggingface` | Model cache directory |

---

## GPU setup (Kubernetes)

```yaml
resources:
  requests:
    memory: 6Gi
    cpu: 2000m
    nvidia.com/gpu: 1
  limits:
    memory: 8Gi
    nvidia.com/gpu: 1
tolerations:
  - key: "nvidia.com/gpu"
    operator: "Exists"
    effect: "NoSchedule"
env:
  - name: MODERATION_MODELS__DEVICE
    value: "cuda"
```

Replace the base image in `Dockerfile` with `nvidia/cuda:12.1.0-cudnn8-runtime-ubuntu22.04` and install `torch+cu121` instead of the CPU `torch` build.