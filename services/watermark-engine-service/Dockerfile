FROM python:3.11-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends \
    libgl1 libglib2.0-0 ffmpeg ca-certificates \
    && rm -rf /var/lib/apt/lists/*
RUN useradd --system --uid 1000 watermark
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY app ./app
COPY config.yaml ./
USER watermark
EXPOSE 9105
# 2 workers — CPU-only, no GPU required
CMD ["python", "-m", "uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "9105", "--workers", "2"]