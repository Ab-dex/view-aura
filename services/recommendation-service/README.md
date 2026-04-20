# recommendation-service

Python hybrid recommendation engine. Combines collaborative filtering (512-dim taste vectors), content-based filtering (genre/crew embeddings), and mood-semantic matching (sentence-transformer) to produce personalised movie rankings.

Port: **9101**

---

## Prerequisites

| Tool | Version |
|------|---------|
| Python | >= 3.11 |
| Redis | >= 7 |

---

## Local setup

```bash
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
docker run -d --name redis -p 6379:6379 redis:7-alpine
export REC_FEATURE_STORE__REDIS_URL=redis://localhost:6379
python -m app.main
```

Swagger UI: http://localhost:9101/docs

---

## API

### POST /recommend

```json
{ "user_id": "uuid", "limit": 20, "context": "home_feed", "exclude_watched": true }
```

Context variants: `home_feed`, `mood:<text>`, `similar:<movie_id>`

Response includes `movie_id`, `score`, `reason`, and `is_cold_start`.

### POST /vectors/update

Immediate taste vector update without Kafka.

```json
{ "user_id": "uuid", "movie_id": "uuid", "rating": 4.5, "event": "rating" }
```

---

## Hybrid scoring weights

| Context | Collaborative | Content | Mood |
|---------|--------------|---------|------|
| Normal user (>= 10 ratings) | 0.5 | 0.3 | 0.2 |
| Cold-start (< 10 ratings) | 0.1 | 0.2 | 0.7 |
| similar:{movie_id} | 0.0 | 1.0 | 0.0 |

---

## Module layout

```
app/
├── main.py           — FastAPI app, lifespan
├── config/           — Pydantic Settings
├── models/           — Request/response schemas
├── feature_store/    — Redis taste vector (msgpack float32)
├── recommender/
│   ├── __init__.py   — Hybrid scoring engine
│   └── mood.py       — Sentence-transformer + 40 mood clusters
├── consumer/         — aiokafka: ratings + watch_events -> vector update
└── api/              — FastAPI router
```

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| REC_APP_PORT | 9101 | HTTP port |
| REC_FEATURE_STORE__REDIS_URL | redis://localhost:6379 | Redis URL |
| REC_FEATURE_STORE__VECTOR_DIM | 512 | Taste vector dimension |
| REC_MOOD__DEVICE | cpu | cpu or cuda |
| REC_KAFKA__BROKERS | "" | Empty disables consumer |
| REC_SCORING__COLD_START_THRESHOLD | 10 | Min ratings before full model |