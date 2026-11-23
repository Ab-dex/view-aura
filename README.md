# ViewAura

**A media-intelligence platform that combines movie discovery (IMDb), social film logging (Letterboxd) and a studio-grade upload/streaming pipeline (Netflix-style) behind one API.**

> Status: active development. The Go core (auth, catalog, ratings, reviews, watchlists, social, payments, uploads, moderation) is implemented. The Rust and Python satellite services and the edge workers are scaffolded and run standalone. The GraphQL BFF and some Kubernetes assets are partially complete. See [§12 Roadmap and known gaps](#12-roadmap-and-known-gaps).

---

## Table of contents

1. [Product requirements (PRD)](#1-product-requirements-prd)
2. [Architecture overview](#2-architecture-overview)
3. [Repository layout](#3-repository-layout)
4. [Core API: modular monolith](#4-core-api-modular-monolith)
5. [Event system (Kafka)](#5-event-system-kafka)
6. [Durable workflows (Temporal)](#6-durable-workflows-temporal)
7. [Satellite services](#7-satellite-services)
8. [Edge and API layers](#8-edge-and-api-layers)
9. [Data architecture](#9-data-architecture)
10. [Cross-cutting concerns](#10-cross-cutting-concerns)
11. [Local development](#11-local-development)
12. [Roadmap and known gaps](#12-roadmap-and-known-gaps)

---

## 1. Product requirements (PRD)

### 1.1 Problem

Film fans use separate products to find a movie (IMDb), log and discuss it (Letterboxd) and watch it (streaming apps). Studios and creators have no simple way to publish content, get it moderated and reach those same fans. ViewAura puts discovery, social interaction, AI personalization and publishing in one platform.

### 1.2 Personas

| Persona | Needs |
|---|---|
| **Viewer** | Find movies fast, rate and review them, keep watchlists and lists, follow friends, get tailored recommendations, watch with friends in sync. |
| **Creator / studio** | Upload video, have it transcoded, screened and published to a CDN, and see processing progress. Quotas and paid tiers apply. |
| **Moderator** | Work a queue of flagged content (reviews, uploads, posts), decide cases, handle appeals. |
| **Admin / catalog editor** | Create and curate movies, people, credits, streaming links and filming locations. |
| **Platform operator** | Observe, scale and deploy the system safely. |

### 1.3 Goals and non-goals

**Goals**
- Fast, accurate movie discovery (search, autocomplete, trending, genre and person pages).
- A social graph around film: ratings, reviews, reactions, follows, activity feed, discussion threads, challenges.
- Personalized recommendations from taste vectors, content features and mood queries.
- Safe user-generated content through automated moderation with human review and appeals.
- A resumable, observable upload and processing pipeline with compensation on failure.
- Monetization through subscriptions, pay-per-view and tier-based quotas.

**Non-goals (for now)**
- Native mobile apps. This repo only ships the backend, BFF and edge.
- Running a full SVOD licensing and DRM stack. DRM is a stated requirement but not implemented.
- Multi-region active-active writes.

### 1.4 Functional requirements

| # | Area | Requirement |
|---|---|---|
| FR-1 | **Auth** | Register with email and password, email verification, login and logout (single and all sessions), refresh tokens, OAuth2 providers (`/auth/oauth/:provider/begin\|callback`), MFA enroll and disable, session listing and revocation. RS256 JWT access (1h) and refresh (30d) tokens. |
| FR-2 | **Users and profiles** | Account details, preferences, multiple profiles per account (create, switch, set default), push-token registration, account deletion. |
| FR-3 | **Catalog** | Movies, genres, people, credits, filming locations and streaming-availability links. Admin CRUD. Public reads are cached. |
| FR-4 | **Ratings** | One rating per user per movie. Upsert, delete, per-movie and per-user lists, aggregates. |
| FR-5 | **Reviews** | CRUD, reactions (like and similar), spoiler flagging, per-user and per-movie lists. |
| FR-6 | **Watchlist and lists** | Per-user watchlist with status and stats. Custom lists with items, collaborators and public slugs. |
| FR-7 | **Social** | Follow and unfollow, followers and following, follow stats, activity feed, discussion threads and posts (replies, likes), movie challenges with leaderboards. |
| FR-8 | **Notifications** | In-app list, unread count, mark read, per-type and per-channel preferences (push, email, SMS), push tokens. |
| FR-9 | **Payments and entitlements** | Subscription plans (create, upgrade, cancel), invoices, pay-per-view, screener licenses, per-movie access checks. Stripe is the provider. |
| FR-10 | **Quota** | Per-tier upload and storage quota, with status and tier endpoints. Quota is reserved at upload start and released on failure. |
| FR-11 | **Uploads** | Signed direct-to-R2 uploads, progress, abort, and asset listing. Completion triggers the processing pipeline. |
| FR-12 | **Moderation** | Case creation from reports and automated screening, moderator queue with locks, decisions, escalation, user appeals. |
| FR-13 | **Search** | Full-text and typo-tolerant search plus autocomplete over movies and people. Index kept fresh from events. |
| FR-14 | **Recommendations** | `home_feed`, `similar:<movie>` and `mood:<text>` contexts. Hybrid collaborative, content and semantic ranking. |
| FR-15 | **Watch party** | Real-time synchronized playback for groups over WebSockets. |
| FR-16 | **Video processing** | HLS adaptive transcode, thumbnails, metadata extraction, quality assessment, subtitles, scene and chapter detection, watermarking. |
| FR-17 | **Email** | Transactional email (verification, notifications) driven by events. |
| FR-18 | **Analytics** | Playback, search and experiment events flow to ClickHouse and Flink-style jobs, including trending score. |
| FR-19 | **Metadata enrichment** | New movies are enriched from TMDb, IMDb and JustWatch, images stored, and the search index triggered. |

### 1.5 Non-functional requirements

| Category | Target |
|---|---|
| Movie page latency | < 50 ms p95 (edge and Redis caching) |
| Search latency | ~1 ms p99 for index queries, with edge-cached autocomplete |
| Watch-party sync | < 100 ms |
| Throughput | 100K+ RPS system-wide through horizontal scale-out |
| Availability | 99.99% target. Kafka, Redis and email failures degrade gracefully (see [§10.3](#103-resilience)). |
| Security | JWT RS256, mTLS in mesh, WAF and bot filtering at the edge, signed uploads, PII encrypted at rest |
| Observability | Structured JSON logs, OpenTelemetry traces, Prometheus metrics, request IDs everywhere |
| Compatibility | Event schemas are append-only, with topics versioned through naming |

### 1.6 Success metrics

Search to click-through rate, recommendation CTR, watch-completion rate (≥ 90% runtime), upload-to-publish time and success rate, moderation time-to-decision, p95 latency and error budget burn, and playback QoE (buffering ratio per ISP and region).

---

## 2. Architecture overview

ViewAura is a **polyglot, event-driven system built around a Go modular monolith**. The product-critical CRUD domains live in one deployable API with strict module boundaries. Workloads that need a different runtime (Rust for latency and concurrency, Python for ML, Node for email and BFF) run as separate services. Everything communicates through HTTP at the edges and Kafka for state changes.

```
                        ┌────────────────────────────────────────────┐
   Browser / Apps ────▶ │  EDGE (Cloudflare Workers, TypeScript)     │
                        │  geo-routing → bot-detection → ab-testing  │
                        │  autocomplete (KV cache)                   │
                        └──────────────────┬─────────────────────────┘
                                           ▼
                        ┌────────────────────────────────────────────┐
                        │  Kong (prod) / Go API Gateway (local)      │
                        │  JWT validate · rate limit · circuit       │
                        │  breaker · request-id · traceparent        │
                        └───────┬──────────────────────┬─────────────┘
                                │ REST /api/v1         │ GraphQL
                                ▼                      ▼
        ┌───────────────────────────────┐   ┌─────────────────────────┐
        │  CORE API  (Go · Gin · Wire)  │◀──│  GraphQL BFF (Apollo,   │
        │  modular monolith, :8081      │   │  TypeScript, DataLoader)│
        │  auth user profile movie      │   └─────────────────────────┘
        │  rating review watchlist      │
        │  social notification payment  │
        │  upload quota moderation      │
        └───┬───────────┬───────────┬───┘
            │ SQL       │ cache     │ produce events
            ▼           ▼           ▼
      ┌──────────┐ ┌─────────┐ ┌───────────────────────────────────────┐
      │PostgreSQL│ │  Redis  │ │               KAFKA                   │
      │ 16       │ │  7      │ │ user.events movie.events ratings ...  │
      └──────────┘ └─────────┘ └──┬────────┬─────────┬──────────┬──────┘
                                  │        │         │          │
              ┌───────────────────┘        │         │          └───────────────┐
              ▼                            ▼         ▼                          ▼
   ┌─────────────────────┐   ┌─────────────────┐ ┌─────────────────┐  ┌─────────────────────┐
   │ WORKER (Go)         │   │ search-service  │ │ recommendation- │  │ email-service       │
   │ Kafka consumer +    │   │ Rust+MeiliSearch│ │ service (Python)│  │ Node + SendGrid     │
   │ Temporal worker     │   └─────────────────┘ └─────────────────┘  └─────────────────────┘
   └────────┬────────────┘
            │ activities
            ▼
   ┌─────────────────────────────────────────────────────────────────────────────┐
   │ Temporal workflows: upload_pipeline · moderation_saga · enrichment_pipeline │
   └───────┬──────────────────┬─────────────────┬──────────────────┬─────────────┘
           ▼                  ▼                 ▼                  ▼
   video-packaging    moderation-service  quality-assessment  scene-understanding
   (Rust, FFmpeg)     (Python, NSFW/hate/ subtitle-generator  watermark-engine
                       spoiler)           (Python)            (Python)
           │
           ▼
   Cloudflare R2 (object storage) → Cloudflare Stream / CDN (HLS)

   watch-party-service (Rust, WebSocket, Redis pub/sub)   ClickHouse (analytics)
```

### 2.1 Key decisions

| Decision | Rationale |
|---|---|
| **Modular monolith for core domains** | The domains share transactions, auth and a database. Going straight to microservices would add network hops and distributed-transaction cost for no benefit. Each module is its own Go module (`go.mod`) behind a small `contract.Module` interface, so a module can be extracted later without a rewrite. |
| **Kafka as the integration backbone** | Side effects (indexing, recommendations, notifications, email, analytics) are consumers, not synchronous calls from the request path. |
| **Temporal for long-running processes** | Uploads and moderation take minutes to days and span many services. Temporal gives durable state, retries, signals (human decision) and saga compensation. |
| **Rust where latency and concurrency matter** | Search, WebSockets and FFmpeg orchestration. |
| **Python for ML** | The model ecosystem (transformers, CLIP, Whisper) lives here. Services expose a small HTTP API. |
| **Edge-first** | Bot filtering, A/B assignment, geo-routing and autocomplete are resolved at the edge to protect the origin and cut latency. |
| **Degrade, don't fail** | Kafka, Redis and email outages must not break user-facing requests (see [§10.3](#103-resilience)). |

---

## 3. Repository layout

```
viewaura/
├── cmd/
│   ├── api/               # Core API binary (Gin + Google Wire), Swagger docs in cmd/api/docs
│   ├── worker/            # Temporal worker + Kafka consumers
│   └── migrator/          # SQL migration runner
├── internal/
│   ├── app/               # HTTP app: router, middleware, lifecycle, providers
│   ├── contract/          # Module interface + typed module wrappers for Wire
│   ├── events/            # Kafka producer/consumer, topic constants, typed event payloads
│   ├── modules/           # Business domains (each its own Go module)
│   │   ├── auth  user  profile  movie  rating  review  watchlist
│   │   └── social  notification  payment  quota  upload  moderation
│   └── platform/          # Shared infrastructure
│       └── auth cache circuit config db error logger r2 resilience stripe temporal tracing
├── workflows/             # Temporal workflows + activities
│   └── upload_pipeline  moderation_saga  enrichment_pipeline
├── apps/
│   ├── api-gateway/       # Go reverse proxy for local/dev (prod uses Kong)
│   └── graphql-bff/       # Apollo GraphQL BFF (TypeScript)
├── services/              # Satellite services
│   ├── search-service/            (Rust)
│   ├── watch-party-service/       (Rust)
│   ├── video-packaging-service/   (Rust)
│   ├── recommendation-service/    (Python)
│   ├── moderation-service/        (Python)
│   ├── quality-assessment-service/(Python)
│   ├── scene-understanding-service/(Python)
│   ├── subtitle-generator-service/(Python)
│   ├── watermark-engine-service/  (Python)
│   └── email-service/             (Node/TypeScript)
├── edge/                  # Cloudflare Workers: autocomplete, bot-detection, geo-routing, ab-testing
├── libs/go-sdk/           # Typed Go client SDK for the REST API
├── infrastructure/        # Postgres schema+migrations, ClickHouse, Kafka topics, Kong, Istio, Redis, K8s, Cloudflare
├── deployments/           # Dockerfiles, docker-compose, Helm chart
├── viewaura/              # API-client collection (Auth register / verify-email test cases)
├── secrets/               # Local dev JWT keys (never use outside local)
├── go.work*               # Go workspace files (full, api-only, worker-only)
└── Makefile               # Build, codegen, migrate, test, deploy targets
```

---

## 4. Core API: modular monolith

**Stack:** Go 1.25, Gin, Google Wire (compile-time DI), pgx/Postgres, go-redis, zerolog, Swag for OpenAPI.

### 4.1 Module structure

Every module under `internal/modules/<name>` follows the same layering:

```
module.go      # Register(r gin.IRouter): mounts routes, applies auth middleware
api/http/      # Gin handlers + swagger.go (annotations only)
service/       # Business logic (no HTTP, no SQL)
domain/        # Entities, value objects, domain errors
repository/    # Interface + Postgres (and Redis/in-memory where relevant) implementations
```

Modules implement `contract.Module`. Wire builds each one into a typed wrapper (`AuthModule`, `MovieModule`, ...) and `internal/app` mounts them all under **`/api/v1`**. Dependencies point inward (handler → service → repository). Modules must not read each other's tables. They call each other through service interfaces or communicate through events.

### 4.2 Module catalogue

| Module | Responsibility | Representative routes (under `/api/v1`) |
|---|---|---|
| **auth** | Registration, verification, login, JWT, refresh, sessions, OAuth, MFA | `POST /auth/register`, `/auth/login` (strict rate limit 10/min), `/auth/logout`, `/auth/logout/all`, `GET /auth/oauth/:provider/begin\|callback`, `DELETE /auth/mfa`, `GET/DELETE /me/sessions` |
| **user** | Account details, preferences, deletion | `GET/PATCH /me/account`, `/me/preferences`, `DELETE /me` |
| **profile** | Multiple profiles per account | `GET/POST /me/profiles`, `PATCH/DELETE /me/profiles/:id`, `POST …/:id/switch`, `PUT …/:id/default`, `GET/PATCH /me/profile` |
| **movie** | Catalog, genres, people, credits, locations, streaming links. Mounts rating, review, thread and payment sub-routes under `/movies/:id/...` | `GET /movies`, `/movies/:id`, `/movies/:id/credits` (10 min cache), `/movies/:id/locations` (24 h), `/movies/:id/streaming` (6 h), `/genres` (1 h), `/people/:id`, `/people/:id/filmography`, `/people/search`. Admin `POST/PATCH/DELETE`. |
| **rating** | Star ratings and aggregates | `/movies/:id/ratings`, `/movies/:id/rating`, `/users/:user_id/ratings` |
| **review** | Reviews and reactions | `/movies/:id/reviews`, `/reviews/:id`, `/reviews/:id/reactions`, `/users/:user_id/reviews` |
| **watchlist** | Watchlist, custom lists, collaborators | `/me/watchlist`, `/me/watchlist/stats`, `/me/lists`, `/me/lists/:id/items`, `/me/lists/:id/collaborators`, `/lists/:id_or_slug` |
| **social** | Follows, feed, threads, posts, challenges | `/users/:user_id/follow`, `/followers`, `/following`, `/me/feed`, `/threads`, `/threads/:id/posts`, `/posts/:id/like`, `/challenges/*` |
| **notification** | In-app notifications and channel preferences | `/me/notifications`, `/me/notifications/unread-count`, `/me/notifications/:id/read`, `/me/notification-preferences/:type/:channel`, `/me/push-tokens` |
| **payment** | Subscriptions, invoices, PPV, licenses, access checks | `/me/subscription`, `/me/invoices`, `/me/pay-per-view`, `/me/licenses`, `/movies/:id/payment` |
| **quota** | Tier-based usage limits | `GET /me/quota`, `/me/quota/tier` |
| **upload** | Signed upload sessions, progress, abort, asset listing | `/uploads`, `/uploads/:id`, `/me/assets`, `/me/assets/:asset_id` |
| **moderation** | Case queue, locks, decisions, appeals | `/moderation/queue`, `/moderation/:id`, `/moderation/:id/lock`, `/me/moderation`, `/me/moderation/:id/appeal` |

Public GET endpoints use `middleware.CacheFor(ttl)` (Redis-backed response cache). Auth-sensitive endpoints use `StrictRateLimit`. Admin routes sit behind an admin middleware chain. A `/health` endpoint is exposed for probes.

### 4.3 Runtime processes

| Binary | Role |
|---|---|
| `cmd/api` | HTTP server. Wire-built. Graceful shutdown (30s). Swagger UI at `/swagger/index.html`, **only when `app.env ≠ production`**. |
| `cmd/worker` | Starts a Temporal worker on task queue `viewaura-main` (10 concurrent activities and workflow tasks) and a Kafka consumer. `uploads.completed`, `moderation.submitted` and movie-created events start the corresponding workflows. |
| `cmd/migrator` | Applies SQL migrations from `infrastructure/postgres/migrations`. |

---

## 5. Event system (Kafka)

All topic names are constants in [internal/events/topics.go](internal/events/topics.go). Payload types live beside them (`*_events.go`). Partitions, replication and retention are defined in [infrastructure/kafka/topics.yaml](infrastructure/kafka/topics.yaml) (Strimzi `KafkaTopic` CRDs; hot topics 64–128 partitions, standard 16–32, retention 3–90 days).

**Rule:** services never read each other's databases. State changes are published as events, and other services react.

| Domain | Topics |
|---|---|
| Identity | `user.events` |
| Catalog | `movie.events`, `search_index` |
| Engagement | `ratings`, `reviews`, `watchlist.events`, `social.events`, `notifications` |
| Uploads | `uploads.initiated`, `.progress`, `.completed`, `.failed` |
| Billing | `payment.events`, `subscription.created`, `subscription.canceled`, `payment.failed` |
| Moderation | `moderation.submitted`, `.created`, `.decided`, `.escalated`, `.appeal_decided` |
| Playback behaviour | `user.playback.session_started`, `.heartbeat` (every 10 s), `.abandoned`, `.completed` (≥ 90%), `.qoe` |
| Search behaviour | `user.search.query`, `.click`, `.abandoned` |
| Content / experiments | `content.trending_score_updated`, `experiment.exposure` |
| Entitlements | `entitlement.granted`, `entitlement.revoked` |
| Orchestration | `workflow_events`; legacy `box_office`, `watch_events` |

### 5.1 Example: a rating is submitted

1. `POST /api/v1/movies/:id/rating` hits the **rating** module, which writes to Postgres.
2. The module publishes to `ratings`.
3. Consumers react independently:
   - **search-service** updates rating fields in the MeiliSearch document.
   - **recommendation-service** updates the user's taste vector (positive or negative signal).
   - **social** records an activity entry for followers' feeds.
   - **ClickHouse and Flink** aggregate it into analytics and trending score, which is published back as `content.trending_score_updated`.

---

## 6. Durable workflows (Temporal)

Located in [workflows/](workflows/), run by `cmd/worker`.

### 6.1 `upload_pipeline`
Starts from Kafka `uploads.completed` (raw object already in R2 via a signed upload).

```
ValidateFile
  ├─ TranscodeVideo ─┐ (parallel)       → video-packaging-service (HLS ladder, thumbnails)
  └─ ExtractMetadata ┘
ScreenContent                           → moderation-service (AI screening)
  ├─ approved → PublishToCDN → NotifyUploader(success)         status: published
  ├─ flagged  → QuarantineAsset → NotifyUploader(pending)      status: quarantined
  └─ error    → CompensateAndCleanup                           status: failed
                (delete transcoded segments, mark asset failed, release quota)
```

Retries and timeouts are set per activity, and a pod restart mid-pipeline resumes rather than restarts.

### 6.2 `moderation_saga`
Triggered by `moderation.submitted`. Runs automated classification. If it is clearly safe or clearly unsafe it decides automatically. Otherwise it escalates and **blocks on the `human_decision` signal**, which the moderation API sends when a moderator approves or rejects. It then emits `moderation.decided` and applies the outcome (hide content, notify the author, open the appeal window).

### 6.3 `enrichment_pipeline`
Triggered when a movie is created. TMDb, IMDb and JustWatch fetches run **in parallel**. Results are merged and written to the movie (`MergeAndUpdateMovie`), images are downloaded to R2, and a search re-index is triggered.

---

## 7. Satellite services

| Service | Lang | Port | Purpose | Interfaces |
|---|---|---|---|---|
| **search-service** | Rust (Axum, MeiliSearch, rdkafka) | – | Typo-tolerant search + autocomplete. Maintains the index from events. | HTTP `GET` search/suggest. Consumes `search_index`, `movie.events`, `ratings`. |
| **watch-party-service** | Rust (tokio, WebSockets, Redis) | 9002 | Synchronized playback rooms. Designed for very high connection counts. Validates JWT via public key (disabled locally when no key set). | WebSocket `?token=`. Redis pub/sub for fan-out across instances. |
| **video-packaging-service** | Rust (FFmpeg subprocess) | 9003 | HLS adaptive transcoding, thumbnail strips, metadata probe, upload results to R2. Uses NVENC when available, otherwise libx264. | HTTP, called by Temporal activities. |
| **recommendation-service** | Python (FastAPI) | 9101 | Hybrid recommender: 512-dim collaborative taste vectors, content embeddings (genre and crew), mood-semantic matching with sentence-transformers. Redis feature store. | `POST /recommend` with `context`: `home_feed`, `mood:<text>` or `similar:<movie_id>`. Kafka consumer for behaviour events. |
| **moderation-service** | Python (HF models) | 9102 | NSFW, hate-speech and spoiler classification for text and images. GPU optional. | HTTP, called by the moderation module and workflows. |
| **scene-understanding-service** | Python (CLIP, Whisper) | 9103 | Frame sampling, scene-change detection, chapters, emotion scoring. | HTTP. |
| **quality-assessment-service** | Python | – | Technical quality scoring of uploaded video. | HTTP. |
| **subtitle-generator-service** | Python | – | Speech-to-text subtitle generation. | HTTP. |
| **watermark-engine-service** | Python | – | Forensic and visible watermarking for screeners. | HTTP. |
| **email-service** | Node/TS (SendGrid, React Email) | – | Renders and sends transactional email from Kafka events. HTTP-only mode when `KAFKA_BROKERS` is empty. | Kafka consumer and HTTP. |

Each service has its own `config.yaml` (or `.env`), `Dockerfile` and, for most, a README with run instructions.

---

## 8. Edge and API layers

### 8.1 Cloudflare Workers ([edge/](edge/))
Execution order: **geo-routing → bot-detection → ab-testing → autocomplete**.

| Worker | Route | Function |
|---|---|---|
| geo-routing | `viewaura.com/*` | Route to the nearest healthy origin |
| bot-detection | `viewaura.com/*` | Bot score, ASN block, rate limit |
| ab-testing | `viewaura.com/*` | Sticky variant assignment (emits `experiment.exposure`) |
| autocomplete | `viewaura.com/api/v1/search/suggest*` | KV-cached suggestions, terminal handler |

### 8.2 API gateway ([apps/api-gateway/](apps/api-gateway/))
Production uses **Kong** ([infrastructure/kong/](infrastructure/kong/)). The Go gateway reproduces its behavior for local development and integration tests. It validates Bearer JWTs, applies per-user (or per-IP) rate limits in Redis, wraps each upstream in a circuit breaker with typed fallbacks, adds `X-Request-ID`, propagates W3C `traceparent`, proxies `/api/v1/*` to the core API, and exposes `/health` and `/metrics`. Without Redis it degrades to in-process state.

### 8.3 GraphQL BFF ([apps/graphql-bff/](apps/graphql-bff/))
Apollo Server in TypeScript that aggregates REST calls into screen-shaped queries (`homeFeed`, `movie`, `searchMovies`, `trendingMovies`, `recommendedMovies`, `me`, ratings and reviews connections). It uses DataLoader batching, mappers from REST to GraphQL types, and subscriptions through a pub/sub layer.

---

## 9. Data architecture

| Store | Use |
|---|---|
| **PostgreSQL 16** | System of record. Schema in [infrastructure/postgres/migrations](infrastructure/postgres/migrations) (`001_init`, `002_materialized_views`, `003_auth`). |
| **Redis 7** | Response cache, rate-limit and circuit state, sessions, outbox fallback, recommendation feature store, watch-party fan-out. |
| **Kafka** | Event backbone (see [§5](#5-event-system-kafka)). |
| **MeiliSearch** | Search index (owned by search-service). |
| **ClickHouse 24.3** | Analytics and behavioral events ([infrastructure/clickhouse/schema.sql](infrastructure/clickhouse/schema.sql)). |
| **Cloudflare R2 / Stream** | Raw uploads, transcoded segments, images, CDN delivery. |
| **Temporal** | Workflow state. |

**Postgres schema groups** (from `001_init`):
- *Identity and access:* `users`, `user_auth_providers`, `user_profiles`, `user_preferences`, `user_security`, `user_sessions`, `push_tokens`, `roles`, `permissions`, `role_permissions`, `auth_verification_tokens`, `mfa_enrollments`
- *Catalog:* `movies`, `genres`, `persons`, `credits`, `streaming_links`, `filming_locations`
- *Engagement:* `ratings`, `reviews`, `review_reactions`, `watchlist_entries`, `custom_lists`, `custom_list_items`, `list_collaborators`, `follows`
- *Commerce:* `subscriptions`, `invoices`, `screener_licenses`
- *Media and safety:* `media_assets`, `moderation_cases`, `moderation_appeals`
- *Messaging:* `notifications`

Design principle: **database-per-service for satellites; one logical schema for the monolith**, with module-owned tables. Cross-service data flows through events, not shared tables.

---

## 10. Cross-cutting concerns

### 10.1 Security
- **Authn:** RS256 JWTs (issuer `https://auth.viewaura.com`), access 1h, refresh 720h. Keys are mounted as secrets (`/run/secrets/jwt_*`).
- **Authz:** role and permission tables, admin middleware chain, per-resource ownership checks in services.
- **Network:** WAF and bot filtering at the edge, mTLS via Istio ([peer-authentication.yaml](infrastructure/istio/peer-authentication.yaml)), zero-trust service mesh.
- **Data:** PII encrypted at rest, signed R2 upload URLs, Stripe handles card data.
- **Hygiene:** `/swagger` is disabled in production. The checked-in `secrets/` keys are **local-dev only**. Rotate and move to a secret manager before any shared environment, and confirm they are covered by `.gitignore`.

### 10.2 Observability
zerolog JSON logs, request-ID middleware, OpenTelemetry tracing ([internal/platform/tracing](internal/platform/tracing)) with W3C propagation across gateway, API and services, Prometheus metrics from the gateway, and K8s observability manifests under [infrastructure/kubernetes/observability](infrastructure/kubernetes/observability).

### 10.3 Resilience
- **Circuit breakers** per upstream ([internal/platform/circuit](internal/platform/circuit)).
- **Three-tier Kafka publishing** ([internal/platform/resilience](internal/platform/resilience)): Kafka → Redis outbox (48 h TTL, capped at 10k per topic) → local file outbox. An `OutboxWorker` drains and republishes on recovery (30 s poll).
- **Resilient mailer** with the same fallback approach.
- **Graceful degradation:** the gateway and cache work without Redis, and the API keeps serving when Kafka is down.
- **Temporal** retries and compensation for multi-step processes.

### 10.4 Configuration
[internal/platform/config](internal/platform/config) loads YAML (`config.yaml`, `config.local.yaml`) with environment overrides. Satellite services use their own prefixed env vars (for example `REC_*`, `PARTY_*`).

---

## 11. Local development

### 11.1 Prerequisites
Go 1.25+, Docker, `make`. Tools: `swag`, `wire`, `golangci-lint`, `migrate` (`make tools`). For satellites: Rust ≥ 1.78 (+ cmake, clang for search), Python ≥ 3.11, Node ≥ 20, FFmpeg ≥ 6.

### 11.2 Start the core stack

```bash
make docker-up        # Postgres, Redis, Kafka (+topics), Temporal (+UI), ClickHouse
make migrate-up       # apply SQL migrations
make dev              # swagger + wire (api, worker) + build both binaries
APP_ENV=local bin/viewaura-api      # → http://localhost:8081
bin/viewaura-worker                 # Temporal worker + Kafka consumers
```

Swagger UI: `http://localhost:8081/swagger/index.html` (`make swagger-serve`).
A convenience target, `make app`, brings up infra, migrates, generates docs and runs the API.

### 11.3 Run a satellite service

```bash
# Search (Rust)
cd services/search-service && cargo run          # needs MeiliSearch :7700 + Kafka
# Recommendations (Python)
cd services/recommendation-service && python -m venv .venv && . .venv/bin/activate \
  && pip install -r requirements.txt && python -m app.main     # :9101/docs
# Email (Node)
cd services/email-service && npm ci && npm run dev
```

### 11.4 Makefile reference

| Target | Purpose |
|---|---|
| `swagger` | Regenerate `cmd/api/docs/{docs.go,swagger.json,swagger.yaml}` from annotations. **Never hand-edit generated files.** |
| `wire`, `wire-worker` | Regenerate DI graphs for API and worker |
| `build`, `build-worker` | Compile binaries into `bin/` |
| `docker-build`, `docker-push` | Build and push API and worker images (`ghcr.io/viewaura/*`) |
| `docker-up`, `docker-down`, `docker-logs` | Local stack lifecycle |
| `migrate-up`, `migrate-down`, `migrate-status` | Database migrations |
| `kafka-topics` | Manually (re)create topics |
| `test`, `test-coverage`, `test-integration`, `lint` | Quality gates (race detector on) |
| `infra-init\|plan\|apply` | Terraform |
| `deploy-staging`, `deploy-prod`, `rollback` | Helm release management |

### 11.5 API documentation conventions
Annotations live in `internal/modules/*/api/http/swagger.go` as empty stub functions placed directly above their descriptions. Protected routes carry `@Security BearerAuth`. Handlers that return `gin.H` need a `*Response` struct stub so swag can infer the schema. Import `cmd/api/docs/swagger.json` into Postman, Insomnia or `openapi-generator`. The `viewaura/` folder holds an API-client collection (register and verify-email suites: positive, validation and business cases such as duplicate email and expired token) targeting `http://localhost:8081`.

### 11.6 Deployment
Dockerfiles in [deployments/docker](deployments/docker), a Helm chart in [deployments/helm/viewaura](deployments/helm/viewaura), Kubernetes base, ingress, service and observability manifests in [infrastructure/kubernetes](infrastructure/kubernetes), and Kong and Istio configuration. Environments: local, staging, production.

---

## 12. Roadmap and known gaps

This list is based on reading the repository, not on a formal backlog.

**Implemented:** Go modules for auth, user, profile, movie, rating, review, watchlist, social, notification, payment, quota, upload and moderation. Kafka producer and consumer with outbox fallback. Three Temporal workflows. Postgres schema and migrations. Swagger generation. Dev gateway. Local compose stack.

**Scaffolded or partial:**
- Python services (scene-understanding, quality-assessment, subtitle-generator, watermark-engine) have app skeletons and configs. Their analyzers are mostly stubs.
- Several Temporal activities that call satellites (for example `ScreenContentActivity`, `TranscodeVideoActivity`) need wiring to real service endpoints.
- GraphQL BFF subscription and federation setup is incomplete. The original design called for Apollo Federation across subgraphs.
- DRM, mTLS rollout and Flink analytics jobs are specified but not present in the repo.
- Test coverage is thin. Auth has register and verify-email API cases and little else.
- `tmp/` and local build artifacts exist in the tree and should be ignored. The `tools/` directory is empty.

**Next up (suggested order):**
1. Complete the auth suite (login, refresh, OAuth, MFA) in the API collection and add integration tests per module.
2. Wire real satellite endpoints into the upload and moderation workflows and add workflow replay tests.
3. Stand up search indexing end to end (movie.created → search_index → query) and expose `/search` through the core API or the BFF.
4. Finish BFF resolvers and decide between federation and a single BFF.
5. Build the ClickHouse ingestion and trending-score loop.
6. Production hardening: secret management, SLO dashboards and alerts, load tests against the stated performance goals.

---

## License

Private / Proprietary.

## Vision

> A unified intelligence layer for global film discovery, streaming and social interaction, combining IMDb, Netflix and Letterboxd in one system.
