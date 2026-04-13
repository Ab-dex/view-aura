# 🎬 ViewAura

**A distributed, event-driven media intelligence platform combining streaming, social interaction, and AI-powered discovery.**

---

## 🚀 Overview

ViewAura is a **polyglot, cloud-native media platform** designed for:

* Movie discovery at scale
* Real-time social interaction
* AI-powered recommendations
* High-performance search (Netflix-level)
* Studio-grade upload + processing pipelines

It combines:

* Microservices architecture
* Domain-Driven Design (DDD)
* Event-driven systems (Kafka)
* Edge computing (Cloudflare Workers)
* ML pipelines (Python + GPU inference)

---

## 🧠 Core Capabilities

* 🎥 Movie catalog + metadata enrichment
* ⭐ Ratings + reviews + social graph
* 🔍 Sub-millisecond full-text search
* 🤖 AI recommendations (512-dim taste vectors)
* 📡 Real-time watch parties (WebSockets)
* 📤 Studio upload + video processing pipeline
* 🔔 Multi-channel notifications (push/email/SMS)
* 📊 Real-time analytics (ClickHouse + Flink)

---

## 🏗️ Architecture

ViewAura is built using a **domain-first, event-driven microservices architecture**.

### Layers

* **Edge Layer**

  * Cloudflare Workers (A/B testing, caching, bot detection)

* **API Layer**

  * Go API Gateway (Kong-backed)
  * GraphQL BFF (Apollo Federation)

* **Core Services (Go)**

  * User, Movie, Rating, Review, Watchlist, Social

* **Performance Services (Rust)**

  * Search (MeiliSearch/OpenSearch)
  * Watch Party (WebSockets)
  * Video Packaging (FFmpeg pipeline)

* **AI/ML Services (Python)**

  * Recommendation Engine
  * Content Moderation
  * Scene Understanding
  * Script Analysis

* **Event System**

  * Apache Kafka (primary event backbone)

---

## 📁 Repository Structure

```bash
ViewAura/
│
├── apps/                 # API gateway + BFF + admin
├── services/            # Microservices (domain-owned)
├── internal/            # Shared domain logic (DDD core)
├── edge/                # Cloudflare Workers
├── libs/                # SDKs (Go / TS / Python)
├── workflows/           # Temporal workflows
├── infrastructure/      # Kubernetes + infra config
├── deployments/         # Docker + Helm + Terraform
├── tools/               # Dev + testing utilities
```

---

## ⚙️ Tech Stack

### Backend

* Go — Core microservices
* Rust — Search, real-time systems
* Python — AI/ML pipelines
* Node.js — BFF + edge services

### Infrastructure

* Kubernetes
* Kafka
* PostgreSQL + Citus
* Redis
* ClickHouse
* Cloudflare

### Search

* MeiliSearch / OpenSearch

---

## 🔄 Key Data Flow

### Example: Movie Rating Event

1. User submits rating
2. Rating Service writes to Postgres
3. Event published to Kafka
4. Consumers update:

   * Recommendation engine
   * Social feed
   * Search index
   * Analytics (ClickHouse)

---

## ⚡ Performance Goals

* ⏱ Movie page load: **< 50ms p95**
* 🔍 Search latency: **~1ms p99**
* 📡 Watch party sync: **< 100ms**
* 🚀 System throughput: **100K+ RPS**
* 🌍 Uptime target: **99.99%**

---

## 🧪 Development

### Setup

```bash
go work init
go work use ./internal ./services ./apps
```

### Run API Gateway

```bash
go run apps/api-gateway
```

### Run Search Service

```bash
cd services/search-service
cargo run
```

---

## 📡 Event-Driven Design

ViewAura uses Kafka as its backbone:

* `movie.created`
* `rating.updated`
* `review.published`
* `watch.event`
* `notification.dispatch`

All services are **decoupled via events**, never direct DB access.

---

## 🔐 Security

* JWT + OAuth2 authentication
* mTLS via service mesh
* WAF at edge layer
* Encrypted PII at rest
* Signed uploads (R2)
* DRM for video content

---

## 🧭 Design Principles

* Domain-first architecture
* Event-driven communication
* Database-per-service
* Edge-first optimization
* Async-first AI workloads
* Zero trust networking

---

## 📌 Status

🚧 Early-stage platform architecture
🧱 Core system being built
⚙️ Active development

---

## 📜 License

Private / Proprietary (for now)

---

## 🧠 Vision

ViewAura aims to become:

> A unified intelligence layer for global film discovery, streaming, and social interaction — combining IMDb, Netflix, and Letterboxd into one system.


# ViewAura API Docs

This directory is managed by [swaggo/swag](https://github.com/swaggo/swag).

## Quickstart

```bash
# Install the swag CLI (once)
go install github.com/swaggo/swag/cmd/swag@latest

# Regenerate docs after any annotation change
make swagger

# Run locally with Swagger UI
make swagger-serve
# → http://localhost:8080/swagger/index.html
```

## File layout

| File | Owner | Purpose |
|---|---|---|
| `docs/doc.go` | **hand-written** | `@title`, `@version`, server URLs, security definitions, tag names |
| `docs/docs.go` | **generated** | Go embedding of the OpenAPI spec — do not edit |
| `docs/swagger.json` | **generated** | Raw OpenAPI 3.0 JSON — import into Postman, Insomnia, etc. |
| `docs/swagger.yaml` | **generated** | YAML equivalent |
| `internal/modules/*/api/swagger.go` | **hand-written** | Per-endpoint annotations — one stub function per handler |

> **Rule:** never edit `docs/docs.go`, `swagger.json`, or `swagger.yaml` by hand.
> Always edit annotations and run `make swagger`.

## Annotation conventions

### One file per module handler package

Each handler package gets a `swagger.go` file alongside its `handler.go`.
Annotations live in stub functions with empty bodies:

```go
// List godoc
//
//	@Summary     List movies
//	@Tags        Movies
//	@Produce     json
//	@Param       q query string false "Search query"
//	@Success     200 {object} movieListResponse
//	@Failure     400 {object} errorResponse
//	@Router      /movies [get]
func (h *MovieHandler) swaggerList() {}
```

The stub functions are never called — swag's parser reads the comments.
Place the `//` block immediately above the `func` with no blank line.

### Security

Protected routes must include `@Security BearerAuth`:

```go
//	@Security    BearerAuth
```

This renders a padlock icon in the UI and enables the "Authorize" button.

### Request bodies

Reference the same request struct used in the real handler:

```go
//	@Param body body createMovieRequest true "Movie fields"
```

### Response schemas

Swag infers the JSON shape from Go struct field tags. Define a `*Response`
struct stub in `swagger.go` when the handler returns `gin.H{}` (which swag
cannot inspect):

```go
type movieListResponse struct {
    Movies []movieResponse `json:"movies"`
    Total  int             `json:"total"`
}
```

### Error shape

Every module defines a local `errorResponse` struct in its `swagger.go`.
Swag deduplicates same-named types across packages when `--parseDependency`
is set — if you see conflicts, prefix the type name with the module name
(e.g. `ratingErrorResponse`).

## Using the generated spec

### Postman
File → Import → select `docs/swagger.json`

### Insomnia
Application → Import/Export → Import Data → From File → `docs/swagger.json`

### Client code generation (openapi-generator)
```bash
openapi-generator generate \
  -i docs/swagger.json \
  -g typescript-axios \
  -o clients/ts
```

## Production behaviour

The Swagger UI route (`/swagger/*`) is **only registered when `app.env ≠ production`**.
It is stripped from the production build via the router's environment check in
`internal/app/app.go`. The `docs/` package is still compiled in but never served.