# ── Build stage ────────────────────────────────────────────────────────────────
FROM rust:1.78-bookworm AS builder

WORKDIR /build

# No rdkafka here — no cmake needed. Only standard Rust toolchain.
RUN apt-get update && apt-get install -y --no-install-recommends \
    pkg-config libssl-dev \
    && rm -rf /var/lib/apt/lists/*

# Cache dependency layer
COPY Cargo.toml Cargo.lock ./
RUN mkdir -p src && echo "fn main(){}" > src/main.rs
RUN cargo build --release && rm -rf src

COPY src ./src
RUN touch src/main.rs && cargo build --release

# ── Runtime stage ───────────────────────────────────────────────────────────────
# Use the official FFmpeg image as the runtime base — it ships with
# all the codec libraries the ffmpeg binary needs at runtime.
FROM jrottenberg/ffmpeg:6.1-ubuntu2204

# Install Rust runtime deps
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libssl3 \
    && rm -rf /var/lib/apt/lists/*

RUN useradd --system --uid 1000 video

WORKDIR /app

COPY --from=builder /build/target/release/video-packaging-service /app/video-packaging-service
COPY config.yaml /app/config.yaml

# Temp workspace for FFmpeg intermediate files
RUN mkdir -p /tmp/vp-workspace && chown video /tmp/vp-workspace

USER video

EXPOSE 9003

ENTRYPOINT ["/app/video-packaging-service"]