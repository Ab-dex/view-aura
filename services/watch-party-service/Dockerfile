# ── Build stage ────────────────────────────────────────────────────────────────
FROM rust:1.78-bookworm AS builder

WORKDIR /build

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
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libssl3 \
    && rm -rf /var/lib/apt/lists/*

RUN useradd --system --uid 1000 party

WORKDIR /app

COPY --from=builder /build/target/release/watch-party-service /app/watch-party-service
COPY config.yaml /app/config.yaml

USER party

EXPOSE 9002

ENTRYPOINT ["/app/watch-party-service"]