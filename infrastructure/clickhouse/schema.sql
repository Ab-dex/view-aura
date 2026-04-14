-- ClickHouse analytics schema
-- Engine: ReplacingMergeTree for mutable rows, SummingMergeTree for daily stats
-- Partition: by toYYYYMM(date) for efficient time-range scans and TTL drops

-- ─── Events (raw) ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS events (
    event_id      UUID,
    entity_type   LowCardinality(String),  -- 'movie' | 'user' | 'review'
    entity_id     String,
    event_type    LowCardinality(String),  -- 'view' | 'rating' | 'search' | 'upload'
    user_id       String,
    session_id    String,
    properties    Map(String, String),
    ts            DateTime64(3, 'UTC'),
    date          Date MATERIALIZED toDate(ts),
    INDEX idx_entity_id entity_id TYPE bloom_filter GRANULARITY 4,
    INDEX idx_user_id   user_id   TYPE bloom_filter GRANULARITY 4
) ENGINE = ReplacingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (entity_type, entity_id, ts)
TTL date + INTERVAL 90 DAY;

-- ─── Daily movie stats (Flink → ClickHouse, 5s micro-batches) ─────────────────
CREATE TABLE IF NOT EXISTS mv_daily_movie_stats (
    movie_id      String,
    date          Date,
    view_count    UInt64,
    unique_users  UInt64,
    avg_rating    Float32,
    rating_count  UInt64,
    watchlist_adds UInt64,
    review_count  UInt64
) ENGINE = SummingMergeTree((view_count, unique_users, rating_count, watchlist_adds, review_count))
PARTITION BY toYYYYMM(date)
ORDER BY (movie_id, date)
TTL date + INTERVAL 365 DAY;

-- ─── Box office ───────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS box_office (
    movie_id       String,
    date           Date,
    region         LowCardinality(String),
    gross_usd      Int64,
    screens        UInt32,
    weekend_rank   UInt8
) ENGINE = ReplacingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (movie_id, date, region)
TTL date + INTERVAL 730 DAY;

-- ─── Search queries ───────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS search_queries (
    session_id      String,
    query           String,
    result_count    UInt32,
    clicked_rank    Nullable(UInt8),
    duration_ms     UInt32,
    ts              DateTime64(3, 'UTC'),
    date            Date MATERIALIZED toDate(ts)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (date, query)
TTL date + INTERVAL 30 DAY;

-- ─── Upload pipeline metrics ──────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS upload_pipeline_runs (
    upload_id          String,
    asset_id           String,
    user_id            String,
    stage              LowCardinality(String),
    status             LowCardinality(String),
    duration_seconds   Float32,
    size_bytes         UInt64,
    ts                 DateTime64(3, 'UTC'),
    date               Date MATERIALIZED toDate(ts)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (date, upload_id)
TTL date + INTERVAL 90 DAY;

-- ─── Payment analytics ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS payment_events (
    event_id        UUID,
    user_id         String,
    event_type      LowCardinality(String),
    plan            LowCardinality(String),
    amount_cents    Int64,
    currency        LowCardinality(String),
    ts              DateTime64(3, 'UTC'),
    date            Date MATERIALIZED toDate(ts)
) ENGINE = ReplacingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (date, event_type, user_id)
TTL date + INTERVAL 730 DAY;  -- 2 years for revenue compliance