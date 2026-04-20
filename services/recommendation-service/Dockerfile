# ── Base ──────────────────────────────────────────────────────────────────────
# CPU build. For GPU: swap for nvidia/cuda:12.1.0-cudnn8-runtime-ubuntu22.04
# and install torch+cu121 instead.
FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libssl3 \
    && rm -rf /var/lib/apt/lists/*

RUN useradd --system --uid 1000 rec

COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY app ./app
COPY config.yaml ./

# Persist HuggingFace model downloads across pod restarts (mount a PVC here).
ENV HF_HOME=/root/.cache/huggingface

USER rec

EXPOSE 9101

CMD ["python", "-m", "uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "9101", "--workers", "1", "--log-level", "info"]