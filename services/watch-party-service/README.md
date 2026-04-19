# watch-party-service

Rust WebSocket service for real-time synchronized viewing parties. Handles 1M+ concurrent connections at ~2GB RAM using tokio's async runtime.

Port: **9002**

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Rust | ≥ 1.78 | `curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \| sh` |
| Redis | ≥ 7 | see below |

No native C build dependencies — builds cleanly with `cargo build`.

---

## Local setup

### 1. Start Redis

```bash
docker run -d --name redis -p 6379:6379 redis:7-alpine
```

### 2. Build

```bash
cargo build
```

### 3. Configure

```bash
# JWT validation is disabled when public_key_path is empty (default in config.yaml)
# That means any ?token=xxx is accepted in local dev.

export PARTY_APP_ENV=local
export PARTY_REDIS_URL=redis://localhost:6379
# Optional — leave empty to skip JWT validation
export PARTY_JWT_PUBLIC_KEY_PATH=/path/to/public_key.pem
```

### 4. Run

```bash
cargo run
```

Service starts on `http://localhost:9002`.

---

## Quick test with websocat

```bash
# Install websocat
cargo install websocat

# 1. Create a party
curl -s -X POST http://localhost:9002/api/v1/parties \
  -H "Content-Type: application/json" \
  -d '{"movie_id":"movie-uuid-123"}' | jq .

# 2. Connect as host (no real JWT needed in local dev)
websocat "ws://localhost:9002/ws/party/<party_id>?token=any"

# 3. In another terminal, connect as participant
websocat "ws://localhost:9002/ws/party/<party_id>?token=any2"
```

---

## WebSocket protocol

Connect: `ws://host:9002/ws/party/{party_id}?token={jwt}`

The `token` query parameter must be a valid ViewAura access JWT (RS256).
In local dev mode (no public key configured), any non-empty token is accepted.

### Client → Server messages

All messages are JSON with a `type` discriminator.

```jsonc
// Host: broadcast playback position every 500ms
{ "type": "sync", "ts_ms": 47230, "paused": false }

// Any participant: send chat message
{ "type": "chat", "text": "Great scene!" }

// Any participant: send emoji reaction (ephemeral)
{ "type": "reaction", "emoji": "🎬" }

// Host: pause or resume
{ "type": "play_pause", "paused": true, "ts_ms": 47230 }

// Host: seek everyone to a timestamp
{ "type": "seek", "ts_ms": 120000 }

// Host: change the movie
{ "type": "change_movie", "movie_id": "new-movie-id" }

// Host: kick a participant
{ "type": "kick", "user_id": "user-id-to-remove" }

// Any: keepalive ping
{ "type": "ping" }
```

### Server → Client messages

```jsonc
// On successful join
{ "type": "welcome", "party_id": "...", "user_id": "...", "is_host": true,
  "movie_id": "...", "current_ts_ms": 0, "paused": true, "participants": [...] }

// When anyone joins
{ "type": "participant_joined", "user_id": "...", "display_name": "...",
  "is_host": false, "participant_count": 3 }

// When anyone leaves
{ "type": "participant_left", "user_id": "...", "participant_count": 2 }

// Host sync relay (every 500ms)
{ "type": "sync", "ts_ms": 47230, "paused": false }

// Drift correction — sent only to lagging participants
{ "type": "seek_correction", "ts_ms": 47230, "paused": false }

// Chat message
{ "type": "chat", "id": "uuid", "user_id": "...", "display_name": "...",
  "text": "...", "ts_ms": 1720000000000 }

// Ephemeral reaction
{ "type": "reaction", "user_id": "...", "display_name": "...", "emoji": "🎬" }

// Host kicked you
{ "type": "kicked", "reason": "Removed by host" }

// Host ended the party
{ "type": "party_ended" }

// Error
{ "type": "error", "code": "FORBIDDEN", "message": "..." }

// Pong (response to ping)
{ "type": "pong" }
```

---

## Architecture

```
Client A (Host)          Watch Party Service Pod 1          Client B (Participant)
     │                          │                                   │
     │── WS upgrade ──────────▶ │                                   │
     │                          │── JWT validate                     │
     │◀─ Welcome ────────────── │                                   │
     │                          │◀─ WS upgrade ─────────────────── │
     │                          │── JWT validate                     │
     │                          │── Welcome ──────────────────────▶ │
     │                          │                                   │
     │── sync {ts_ms:100} ────▶ │── broadcast ──────────────────▶  │
     │                          │── Redis pub/sub (other pods)       │
     │                          │                                   │
     │── chat "Great!" ───────▶ │── broadcast ──────────────────▶  │
     │                          │── Redis Stream persist             │
```

**Cross-pod fan-out**: When a message needs to reach connections on other pods, it is published to `party:{id}:channel` in Redis pub/sub. Every pod with at least one connection to that party subscribes to the channel and fans out received messages to its local connections.

---

## Sync protocol

- Host sends `sync` every **500ms** with the current timestamp and paused state.
- Server relays the sync to all other participants.
- Server measures drift: if any participant's last timestamp differs from the host by more than **2000ms**, a `seek_correction` is sent to that participant only.
- Reactions are **ephemeral** — they are broadcast but not persisted.
- Chat is persisted to a Redis Stream (last 500 messages, 24h TTL) for replay on join.

---

## Module layout

```
src/
├── main.rs       — entrypoint, wiring, graceful shutdown
├── config/       — Settings from config.yaml + PARTY_ env vars
├── auth/         — RS256 JWT validation (skippable in local dev)
├── protocol/     — ClientMessage + ServerMessage enums (wire format)
├── redis_bus/    — Party state (Hash), members (Set), chat (Stream), pub/sub
├── room/         — In-process DashMap connection registry + drift correction
└── api/          — Axum router: REST endpoints + WebSocket upgrade handler
```

---

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PARTY_APP_ENV` | `local` | `local` \| `staging` \| `production` |
| `PARTY_APP_PORT` | `9002` | HTTP listen port |
| `PARTY_JWT_PUBLIC_KEY_PATH` | `""` | RS256 PEM public key; empty = skip validation |
| `PARTY_JWT_ISSUER` | `https://auth.viewaura.com` | Expected JWT issuer |
| `PARTY_REDIS_URL` | `redis://localhost:6379` | Redis connection URL |
| `PARTY_REDIS_PARTY_TTL_SECS` | `14400` | Party state TTL (4 hours) |
| `PARTY_REDIS_CHAT_TTL_SECS` | `86400` | Chat history TTL (24 hours) |
| `PARTY_PARTY_MAX_PARTICIPANTS` | `50` | Max participants per party |
| `PARTY_PARTY_MAX_DRIFT_MS` | `2000` | Drift threshold for seek correction |
| `PARTY_PARTY_IDLE_TIMEOUT_SECS` | `300` | Idle connection disconnect timeout |
| `PARTY_LOG_LEVEL` | `debug` | Log level |
| `PARTY_LOG_FORMAT` | `pretty` | `pretty` \| `json` |