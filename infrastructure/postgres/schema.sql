-- ============================================================
--  ViewAura — Postgres schema
--  Applies to: PostgreSQL 15+ with Citus extension
--  Run with: psql $DB_DSN -f schema.sql
-- ============================================================

-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";   -- trigram indexes for LIKE search
CREATE EXTENSION IF NOT EXISTS "pgcrypto";  -- encrypt PII columns
CREATE EXTENSION IF NOT EXISTS "citus";     -- distributed tables

-- ─── Users ───────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT UNIQUE NOT NULL,
    email_verified  BOOLEAN NOT NULL DEFAULT FALSE,
    username        TEXT UNIQUE,
    display_name    TEXT NOT NULL,
    password_hash   TEXT,
    role            TEXT NOT NULL DEFAULT 'user'
                    CHECK (role IN ('user','producer','critic','admin')),
    status          TEXT NOT NULL DEFAULT 'pending_verification'
                    CHECK (status IN ('active','suspended','deleted','pending_verification')),
    auth_provider   TEXT NOT NULL DEFAULT 'email',
    provider_id     TEXT,
    locale          TEXT DEFAULT 'en',
    timezone        TEXT DEFAULT 'UTC',
    country         TEXT,
    is_deleted      BOOLEAN NOT NULL DEFAULT FALSE,
    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_username ON users(username) WHERE username IS NOT NULL;
CREATE INDEX idx_users_status ON users(status) WHERE is_deleted = FALSE;

CREATE TABLE IF NOT EXISTS user_profiles (
    user_id     UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    avatar_url  TEXT,
    banner_url  TEXT,
    bio         TEXT,
    website     TEXT,
    birthdate   DATE,
    gender      TEXT,
    visibility  TEXT NOT NULL DEFAULT 'public'
                CHECK (visibility IN ('public','private','friends')),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_sessions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id           TEXT,
    ip_address          INET,
    user_agent          TEXT,
    refresh_token_hash  TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ NOT NULL,
    revoked_at          TIMESTAMPTZ
);

CREATE INDEX idx_sessions_user ON user_sessions(user_id);
CREATE INDEX idx_sessions_hash ON user_sessions(refresh_token_hash);

-- ─── Movies ──────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS genres (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT UNIQUE NOT NULL,
    slug        TEXT UNIQUE NOT NULL,
    description TEXT
);

CREATE TABLE IF NOT EXISTS movies (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title            TEXT NOT NULL,
    original_title   TEXT,
    slug             TEXT UNIQUE NOT NULL,
    synopsis         TEXT,
    tagline          TEXT,
    release_date     DATE,
    runtime_mins     INT,
    content_rating   TEXT,
    status           TEXT NOT NULL DEFAULT 'draft'
                     CHECK (status IN ('draft','published','archived')),
    original_lang    TEXT DEFAULT 'en',
    countries        TEXT[],
    genres           TEXT[],   -- denormalised for fast reads
    poster_url       TEXT,
    backdrop_url     TEXT,
    trailer_url      TEXT,
    imdb_id          TEXT UNIQUE,
    tmdb_id          INT UNIQUE,
    avg_rating       NUMERIC(4,2) DEFAULT 0,
    rating_count     INT DEFAULT 0,
    popularity_score NUMERIC(10,4) DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_movies_status    ON movies(status);
CREATE INDEX idx_movies_slug      ON movies(slug);
CREATE INDEX idx_movies_imdb      ON movies(imdb_id) WHERE imdb_id IS NOT NULL;
CREATE INDEX idx_movies_genres    ON movies USING GIN(genres);
CREATE INDEX idx_movies_title_trgm ON movies USING GIN(title gin_trgm_ops);

CREATE TABLE IF NOT EXISTS persons (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT NOT NULL,
    slug         TEXT UNIQUE NOT NULL,
    birth_date   DATE,
    death_date   DATE,
    birthplace   TEXT,
    bio          TEXT,
    profile_url  TEXT,
    imdb_id      TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS credits (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    movie_id      UUID NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    person_id     UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    role          TEXT NOT NULL,
    character     TEXT,
    billing_order INT DEFAULT 999,
    UNIQUE (movie_id, person_id, role)
);

CREATE INDEX idx_credits_movie  ON credits(movie_id);
CREATE INDEX idx_credits_person ON credits(person_id);

CREATE TABLE IF NOT EXISTS streaming_links (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    movie_id     UUID NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    provider     TEXT NOT NULL,
    link_url     TEXT NOT NULL,
    access_type  TEXT NOT NULL CHECK (access_type IN ('stream','rent','buy')),
    price_cents  INT,
    region       TEXT DEFAULT '',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (movie_id, provider, region)
);

CREATE TABLE IF NOT EXISTS filming_locations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    movie_id    UUID NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    latitude    NUMERIC(9,6),
    longitude   NUMERIC(9,6),
    description TEXT
);

-- ─── Ratings ─────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS ratings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    movie_id        UUID NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    overall         NUMERIC(3,1) NOT NULL CHECK (overall BETWEEN 1 AND 5),
    acting          NUMERIC(3,1) CHECK (acting BETWEEN 1 AND 5),
    direction       NUMERIC(3,1) CHECK (direction BETWEEN 1 AND 5),
    writing         NUMERIC(3,1) CHECK (writing BETWEEN 1 AND 5),
    cinematography  NUMERIC(3,1) CHECK (cinematography BETWEEN 1 AND 5),
    soundtrack      NUMERIC(3,1) CHECK (soundtrack BETWEEN 1 AND 5),
    reaction        TEXT,
    is_verified     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, movie_id)
);

CREATE INDEX idx_ratings_movie ON ratings(movie_id);
CREATE INDEX idx_ratings_user  ON ratings(user_id);

-- ─── Reviews ─────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS reviews (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    movie_id           UUID NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    user_id            UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type               TEXT NOT NULL
                       CHECK (type IN ('short','long_form','spoiler','video')),
    body               TEXT,
    video_url          TEXT,
    is_spoiler         BOOLEAN NOT NULL DEFAULT FALSE,
    is_published       BOOLEAN NOT NULL DEFAULT FALSE,
    like_count         INT NOT NULL DEFAULT 0,
    helpful_count      INT NOT NULL DEFAULT 0,
    insightful_count   INT NOT NULL DEFAULT 0,
    funny_count        INT NOT NULL DEFAULT 0,
    credibility_score  NUMERIC(5,4) DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reviews_movie ON reviews(movie_id) WHERE is_published = TRUE;
CREATE INDEX idx_reviews_user  ON reviews(user_id);

CREATE TABLE IF NOT EXISTS review_reactions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    review_id   UUID NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        TEXT NOT NULL CHECK (type IN ('like','helpful','insightful','funny')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (review_id, user_id)
);

-- ─── Watchlist ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS watchlist_entries (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    movie_id    UUID NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    status      TEXT NOT NULL
                CHECK (status IN ('to_watch','watching','watched','dropped')),
    progress    INT DEFAULT 0,
    added_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, movie_id)
);

CREATE INDEX idx_watchlist_user  ON watchlist_entries(user_id);
CREATE INDEX idx_watchlist_movie ON watchlist_entries(movie_id);

CREATE TABLE IF NOT EXISTS custom_lists (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT,
    is_public   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS custom_list_items (
    list_id     UUID NOT NULL REFERENCES custom_lists(id) ON DELETE CASCADE,
    movie_id    UUID NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    position    INT NOT NULL DEFAULT 0,
    added_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (list_id, movie_id)
);

-- ─── Social ───────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS follows (
    follower_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followee_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (follower_id, followee_id)
);

CREATE INDEX idx_follows_followee ON follows(followee_id);

-- ─── Payments ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS subscriptions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan                TEXT NOT NULL CHECK (plan IN ('free','pro','studio')),
    status              TEXT NOT NULL
                        CHECK (status IN ('active','past_due','cancelled','trialing')),
    stripe_sub_id       TEXT UNIQUE NOT NULL,
    stripe_customer_id  TEXT NOT NULL,
    current_period_end  TIMESTAMPTZ NOT NULL,
    cancel_at_end       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_user ON subscriptions(user_id);

CREATE TABLE IF NOT EXISTS invoices (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID REFERENCES users(id),
    subscription_id   TEXT,
    stripe_invoice_id TEXT UNIQUE NOT NULL,
    amount_cents      BIGINT NOT NULL,
    currency          TEXT NOT NULL DEFAULT 'usd',
    status            TEXT NOT NULL CHECK (status IN ('paid','open','void')),
    paid_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS screener_licenses (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    movie_id     UUID NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    amount_cents BIGINT NOT NULL,
    currency     TEXT NOT NULL DEFAULT 'usd',
    expires_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_licenses_user_movie ON screener_licenses(user_id, movie_id);

-- ─── Upload ───────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS media_assets (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    upload_id    TEXT NOT NULL,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    movie_id     UUID REFERENCES movies(id),
    storage_key  TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes   BIGINT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'uploading'
                 CHECK (status IN ('uploading','processing','ready','failed','moderated')),
    source_info  JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_media_assets_user   ON media_assets(user_id);
CREATE INDEX idx_media_assets_upload ON media_assets(upload_id);
CREATE INDEX idx_media_assets_movie  ON media_assets(movie_id) WHERE movie_id IS NOT NULL;

-- ─── Moderation ──────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS moderation_cases (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content_type       TEXT NOT NULL,
    content_id         TEXT NOT NULL,
    owner_id           UUID REFERENCES users(id),
    submitted_by       TEXT NOT NULL,
    trigger_reason     TEXT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending','ai_reviewed','in_review','decided','appealed')),
    ai_signals         TEXT[],
    ai_confidence      JSONB,
    ai_recommendation  TEXT,
    decision           TEXT CHECK (decision IN ('approved','rejected','escalated')),
    decided_by         TEXT,
    reason             TEXT,
    decided_at         TIMESTAMPTZ,
    locked_by          TEXT,
    locked_until       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mod_cases_status       ON moderation_cases(status);
CREATE INDEX idx_mod_cases_content      ON moderation_cases(content_type, content_id);

CREATE TABLE IF NOT EXISTS moderation_appeals (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id      UUID NOT NULL REFERENCES moderation_cases(id) ON DELETE CASCADE,
    appellant_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    statement    TEXT NOT NULL,
    outcome      TEXT CHECK (outcome IN ('upheld','overturned')),
    decided_by   TEXT,
    decided_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Notifications ────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS notifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        TEXT NOT NULL,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL,
    data        JSONB,
    channel     TEXT NOT NULL CHECK (channel IN ('push','email','sms','in_app')),
    is_read     BOOLEAN NOT NULL DEFAULT FALSE,
    sent_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user ON notifications(user_id, is_read);