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