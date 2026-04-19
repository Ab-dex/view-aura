# search-service

Rust service wrapping MeiliSearch. Consumes `search_index`, `movie.events`, and `ratings` Kafka topics and exposes a REST search + autocomplete API.

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Rust | ≥ 1.78 | `curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \| sh` |
| cmake | any | `apt install cmake` / `brew install cmake` |
| clang | any | `apt install clang` / `brew install llvm` |
| Docker | ≥ 24 | [docs.docker.com](https://docs.docker.com/get-docker/) |
| MeiliSearch | ≥ 1.8 | see below |

`rdkafka` (the Kafka client crate) wraps `librdkafka` via CMake at build time. `cmake` and `clang` must be on `PATH` before `cargo build` runs.

---

## Local setup (without Docker)

### 1. Start MeiliSearch

```bash
# Docker (easiest)
docker run -d \
  --name meilisearch \
  -p 7700:7700 \
  -e MEILI_MASTER_KEY="" \
  -v $(pwd)/meili_data:/meili_data \
  getmeili/meilisearch:v1.8

# Verify
curl http://localhost:7700/health
# {"status":"available"}
```

### 2. Start Kafka (use the monolith docker-compose)

```bash
cd ../../  # project root
docker compose -f deployments/docker/docker-compose.yml up -d kafka zookeeper kafka-init
```

Or set `kafka.brokers: ""` in `config.yaml` to disable the consumer and run the HTTP API only.

### 3. Start Redis

```bash
docker run -d --name redis -p 6379:6379 redis:7-alpine
```

### 4. Configure

Copy and edit the config:

```bash
cp config.yaml config.local.yaml
# Edit config.local.yaml — set meili.master_key if you set MEILI_MASTER_KEY above
```

Environment variables override config file values. Prefix: `SEARCH_`.

```bash
export SEARCH_APP_ENV=local
export SEARCH_MEILI_URL=http://localhost:7700
export SEARCH_KAFKA_BROKERS=localhost:29092
export SEARCH_REDIS_URL=redis://localhost:6379
```

### 5. Build and run

```bash
# First build (compiles librdkafka — takes ~3 min)
cargo build

# Run
cargo run

# Or with release optimisations
cargo run --release
```

The service starts on `http://localhost:9001`.

---

## Local setup (Docker)

```bash
docker build -t viewaura/search-service:local .
docker run -d \
  --name search-service \
  --network host \
  -e SEARCH_MEILI_URL=http://localhost:7700 \
  -e SEARCH_KAFKA_BROKERS=localhost:29092 \
  -e SEARCH_REDIS_URL=redis://localhost:6379 \
  viewaura/search-service:local
```

---

## API reference

### `GET /health`
```json
{ "status": "ok", "service": "search-service" }
```

### `GET /api/v1/search`

| Parameter | Type | Description |
|-----------|------|-------------|
| `q` | string | Free-text query |
| `genres` | string (repeated) | Genre filter |
| `countries` | string (repeated) | Country filter |
| `min_rating` | float | Minimum avg_rating |
| `year_from` | int | Earliest release year |
| `year_to` | int | Latest release year |
| `content_rating` | string | e.g. `PG-13` |
| `lang` | string | ISO 639-1 language code |
| `provider` | string | Streaming provider name |
| `sort_by` | string | `popularity_score` \| `avg_rating` \| `release_date` |
| `sort_dir` | string | `asc` \| `desc` (default: `desc`) |
| `limit` | int | Max 100 (default 20) |
| `offset` | int | Pagination offset |

```bash
curl "http://localhost:9001/api/v1/search?q=inception&genres=Thriller&min_rating=4.0&limit=5"
```

### `GET /api/v1/suggest?q=<prefix>`

Returns up to 8 autocomplete suggestions. Results are cached in Redis for 1 hour.

```bash
curl "http://localhost:9001/api/v1/suggest?q=inc"
```

### `GET /api/v1/search/stats`

Returns the current state of the MeiliSearch movies index.

---

## How indexing works

```
movie.created (Kafka)
  └─> consumer/events.rs::handle_movie_upsert()
        └─> Only indexes status="published" movies
        └─> Builds MovieDocument with denormalised cast/director names
        └─> indexer::upsert_movie() → MeiliSearch

movie.rating_aggregate_updated (Kafka)
  └─> consumer/events.rs::handle_rating_update()
        └─> Patches avg_rating + popularity_score only
        └─> Uses MeiliSearch partial update (add_or_update)

movie.archived (Kafka)
  └─> indexer::delete_movie()
```

---

## Module layout

```
src/
├── main.rs          — entrypoint, wiring, graceful shutdown
├── config/          — Settings loaded from config.yaml + env
├── indexer/
│   ├── mod.rs       — MeiliSearchClient, bootstrap, upsert/delete
│   ├── documents.rs — MovieDocument, PersonDocument (search schemas)
│   └── settings.rs  — MeiliSearch index settings (searchable, filterable, ranking)
├── consumer/
│   ├── mod.rs       — Kafka StreamConsumer loop
│   └── events.rs    — Envelope dispatch + per-event handlers
├── search/
│   └── mod.rs       — search_movies() — filter builder + MeiliSearch query
├── suggest/
│   └── mod.rs       — suggest() — Redis ZSet cache + MeiliSearch fallback
└── api/
    └── mod.rs       — Axum router, handlers, AppState
```

---

## Testing

```bash
# Unit tests
cargo test

# Integration test (requires local MeiliSearch on :7700)
SEARCH_MEILI_URL=http://localhost:7700 cargo test --test integration
```

---

## Environment variables reference

| Variable | Default | Description |
|----------|---------|-------------|
| `SEARCH_APP_ENV` | `local` | `local` \| `staging` \| `production` |
| `SEARCH_APP_PORT` | `9001` | HTTP listen port |
| `SEARCH_MEILI_URL` | `http://localhost:7700` | MeiliSearch base URL |
| `SEARCH_MEILI_MASTER_KEY` | `""` | MeiliSearch master key |
| `SEARCH_KAFKA_BROKERS` | `localhost:29092` | Comma-separated brokers; empty disables consumer |
| `SEARCH_KAFKA_GROUP_ID` | `search-service` | Consumer group ID |
| `SEARCH_REDIS_URL` | `redis://localhost:6379` | Redis connection URL |
| `SEARCH_LOG_LEVEL` | `debug` | `trace` \| `debug` \| `info` \| `warn` \| `error` |
| `SEARCH_LOG_FORMAT` | `pretty` | `pretty` (local) \| `json` (production) |
| `SEARCH_TRACING_ENDPOINT` | `""` | OTLP gRPC endpoint; empty disables tracing |


## Setup

# 1. Install Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# 2. Install C build deps (rdkafka needs cmake + clang)
apt install cmake clang libssl-dev      # Ubuntu/Debian
brew install cmake llvm                 # macOS

# 3. Start MeiliSearch + Redis (Docker)
docker run -d -p 7700:7700 getmeili/meilisearch:v1.8
docker run -d -p 6379:6379 redis:7-alpine

# 4. Start Kafka (via the monolith compose file)
docker compose -f deployments/docker/docker-compose.yml up -d kafka kafka-init

# 5. Build and run
cargo build   # ~3 min first time (compiles librdkafka from C source)
cargo run