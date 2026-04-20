# ── Base image ────────────────────────────────────────────────────────────────
FROM python:3.11-slim

WORKDIR /app

# ── System dependencies ───────────────────────────────────────────────────────
RUN apt-get update && apt-get install -y --no-install-recommends \
    libjpeg62-turbo \
    libpng16-16 \
    libssl3 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# ── Create non-root user ──────────────────────────────────────────────────────
RUN useradd --system --uid 1000 --create-home moderation

# ── Python dependencies (cached layer) ────────────────────────────────────────
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# ── Copy application ──────────────────────────────────────────────────────────
COPY app ./app
COPY config.yaml ./

# ── HuggingFace cache (fix permissions issue) ─────────────────────────────────
ENV HF_HOME=/home/moderation/.cache/huggingface

RUN mkdir -p /home/moderation/.cache && \
    chown -R moderation:moderation /home/moderation

# ── Switch user ───────────────────────────────────────────────────────────────
USER moderation

EXPOSE 9102

# ── Run server ────────────────────────────────────────────────────────────────
CMD ["python", "-m", "uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "9102", "--workers", "1", "--log-level", "info"]