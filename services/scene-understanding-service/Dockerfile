FROM python:3.11-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends \
    libgl1 libglib2.0-0 ffmpeg ca-certificates \
    && rm -rf /var/lib/apt/lists/*
RUN useradd --system --uid 1000 scene
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY app ./app
COPY config.yaml ./
ENV HF_HOME=/root/.cache/huggingface
USER scene
EXPOSE 9103
CMD ["python", "-m", "uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "9103", "--workers", "1"]