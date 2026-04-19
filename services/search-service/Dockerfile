# ── Build stage ────────────────────────────────────────────────────────────────
FROM rust:1.78-bookworm AS builder

WORKDIR /build

# Install build deps for rdkafka (needs cmake + clang for librdkafka)
RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake clang libssl-dev pkg-config \
    && rm -rf /var/lib/apt/lists/*

# Cache dependency build layer
COPY Cargo.toml Cargo.lock ./
# Create a dummy main so cargo can compile dependencies without the real source.
RUN mkdir -p src && echo "fn main(){}" > src/main.rs
RUN cargo build --release && rm -rf src

# Build the real binary
COPY src ./src
RUN touch src/main.rs && cargo build --release

# ── Runtime stage ───────────────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libssl3 \
    && rm -rf /var/lib/apt/lists/*

RUN useradd --system --uid 1000 search

WORKDIR /app

COPY --from=builder /build/target/release/search-service /app/search-service
COPY config.yaml /app/config.yaml

USER search

EXPOSE 9001

ENTRYPOINT ["/app/search-service"]