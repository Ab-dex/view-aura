# Uses the jrottenberg/ffmpeg base image because:
# 1. libvmaf is bundled and linked into this ffmpeg build
# 2. Avoids compiling ffmpeg from source (saves ~20 min build time)
# 3. The same base is used by video-packaging-service for consistency
FROM jrottenberg/ffmpeg:6.1-ubuntu2204 AS base

RUN apt-get update && apt-get install -y --no-install-recommends \
    python3.11 python3.11-venv python3-pip ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN useradd --system --uid 1000 qa

COPY requirements.txt ./
RUN pip3 install --no-cache-dir -r requirements.txt

COPY app ./app
COPY config.yaml ./

USER qa

EXPOSE 9106

CMD ["python3", "-m", "uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "9106", "--workers", "1"]