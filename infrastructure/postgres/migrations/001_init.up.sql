-- ============================================================
--  ViewAura — Migration 001: Initial Schema
--
--  Design principles:
--    • RBAC via roles + permissions + role_permissions.
--      No TEXT CHECK on users.role. New permissions require
--      INSERT, not ALTER TABLE.
--    • Auth credentials live in user_auth_providers, not users.
--      One user may have email + Google + Apple simultaneously.
--    • Password hash is a row in user_auth_providers
--      (provider='email'), not a column on users.
--    • Security state (lockout, 2FA, risk score) is its own
--      table so login writes don't touch the hot users row.
--    • Preferences (genres, DND, language) are their own table
--      for independent caching and write isolation.
--    • Push tokens are their own table — one user, many devices.
--    • Collaborative lists via list_collaborators junction.
--    • media_assets.upload_id is UUID for hash joins.
--    • invoices.subscription_id is a proper UUID FK.
--    • All FK columns are indexed.
--    • Partial indexes wherever the WHERE clause is predictable.
-- ============================================================

BEGIN;

-- ─── Extensions ───────────────────────────────────────────────────────────────

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "citext";   -- case-insensitive text for email

-- ─── RBAC: roles ─────────────────────────────────────────────────────────────

CREATE TABLE roles (
    id          UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT    UNIQUE NOT NULL,
    description TEXT,
    is_system   BOOLEAN NOT NULL DEFAULT FALSE,  -- system roles cannot be deleted
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── RBAC: permissions ────────────────────────────────────────────────────────

CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT UNIQUE NOT NULL,  -- e.g. 'movie:write', 'upload:create'
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── RBAC: role <-> permission junction ──────────────────────────────────────

CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id)       ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- ─── Seed: system roles ───────────────────────────────────────────────────────

INSERT INTO roles (name, description, is_system) VALUES
    ('user',      'Standard authenticated user',        TRUE),
    ('producer',  'Content producer — can upload',      TRUE),
    ('critic',    'Verified critic — weighted reviews', TRUE),
    ('moderator', 'Content moderator',                  TRUE),
    ('admin',     'Platform administrator',             TRUE);

-- ─── Seed: permissions ────────────────────────────────────────────────────────

INSERT INTO permissions (name, description) VALUES
    ('movie:read',         'Read published movie catalog'),
    ('movie:write',        'Create and edit movie records'),
    ('movie:publish',      'Publish or archive movies'),
    ('upload:create',      'Initiate file uploads'),
    ('upload:admin',       'View and manage all uploads'),
    ('rating:write',       'Submit and update ratings'),
    ('review:write',       'Submit and update reviews'),
    ('review:publish',     'Publish without moderation hold'),
    ('moderation:read',    'View the moderation queue'),
    ('moderation:decide',  'Approve or reject cases'),
    ('moderation:appeal',  'Decide appeals'),
    ('user:admin',         'Manage user accounts and roles'),
    ('payment:admin',      'View and manage all payments');

-- ─── Seed: role <-> permission assignments ────────────────────────────────────

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'user'
  AND p.name IN ('movie:read','rating:write','review:write','upload:create');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'producer'
  AND p.name IN ('movie:read','movie:write','rating:write',
                 'review:write','upload:create','review:publish');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'critic'
  AND p.name IN ('movie:read','rating:write','review:write','review:publish');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'moderator'
  AND p.name IN ('movie:read','moderation:read','moderation:decide','upload:admin');

-- Admin gets every permission.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'admin';

-- ─── Users ────────────────────────────────────────────────────────────────────
-- Core identity only. No auth credential, no preferences, no security state —
-- those live in their own tables.

CREATE TABLE users (
    id             UUID   PRIMARY KEY DEFAULT gen_random_uuid(),
    email          CITEXT UNIQUE NOT NULL,   -- CITEXT: case-insensitive uniqueness
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    username       TEXT    UNIQUE,
    display_name   TEXT    NOT NULL,
    -- FK to roles table — no TEXT CHECK, new roles need INSERT only.
    role_id        UUID    NOT NULL REFERENCES roles(id),
    status         TEXT    NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('active','suspended','deleted','pending')),
    locale         TEXT    NOT NULL DEFAULT 'en',
    timezone       TEXT    NOT NULL DEFAULT 'UTC',
    country        TEXT,
    is_deleted     BOOLEAN NOT NULL DEFAULT FALSE,
    deleted_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email      ON users(email);
CREATE INDEX idx_users_username   ON users(username)  WHERE username IS NOT NULL;
CREATE INDEX idx_users_role       ON users(role_id);
CREATE INDEX idx_users_status     ON users(status)    WHERE is_deleted = FALSE;
CREATE INDEX idx_users_created_at ON users(created_at DESC);

-- ─── Auth providers ───────────────────────────────────────────────────────────
-- One row per (user × provider) pair.
--
-- provider = 'email':
--   provider_user_id = email address
--   access_token     = bcrypt hash of the password
--   refresh_token    = NULL
--
-- provider = 'google' | 'apple' | 'github':
--   provider_user_id = external sub / oid
--   access_token     = OAuth access token
--   refresh_token    = OAuth refresh token

CREATE TABLE user_auth_providers (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider         TEXT NOT NULL
                     CHECK (provider IN ('email','google','apple','github')),
    provider_user_id TEXT NOT NULL,
    access_token     TEXT,
    refresh_token    TEXT,
    linked_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at     TIMESTAMPTZ,
    UNIQUE (provider, provider_user_id)
);

CREATE INDEX idx_auth_providers_user     ON user_auth_providers(user_id);
CREATE INDEX idx_auth_providers_provider ON user_auth_providers(provider, provider_user_id);

-- ─── User profiles ────────────────────────────────────────────────────────────

CREATE TABLE user_profiles (
    user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    avatar_url TEXT,
    banner_url TEXT,
    bio        TEXT,
    website    TEXT,
    birthdate  DATE,
    gender     TEXT,
    visibility TEXT NOT NULL DEFAULT 'public'
               CHECK (visibility IN ('public','private','friends')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── User preferences ─────────────────────────────────────────────────────────
-- Separate table: independent cache key, no write amplification on users row.

CREATE TABLE user_preferences (
    user_id             UUID    PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    preferred_genres    TEXT[]  NOT NULL DEFAULT '{}',
    disliked_genres     TEXT[]  NOT NULL DEFAULT '{}',
    preferred_languages TEXT[]  NOT NULL DEFAULT '{}',
    adult_content       BOOLEAN NOT NULL DEFAULT FALSE,
    dark_mode           BOOLEAN NOT NULL DEFAULT TRUE,
    autoplay_trailers   BOOLEAN NOT NULL DEFAULT TRUE,
    -- Notification DND window as local hour 0-23. NULL = DND disabled.
    dnd_start_hour      SMALLINT CHECK (dnd_start_hour BETWEEN 0 AND 23),
    dnd_end_hour        SMALLINT CHECK (dnd_end_hour   BETWEEN 0 AND 23),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── User security ────────────────────────────────────────────────────────────
-- Updated on every login attempt — isolated to avoid hot-row write contention.

CREATE TABLE user_security (
    user_id            UUID         PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    failed_login_count INT          NOT NULL DEFAULT 0,
    last_failed_at     TIMESTAMPTZ,
    locked_until       TIMESTAMPTZ,
    risk_score         NUMERIC(5,4) NOT NULL DEFAULT 0,
    two_factor_enabled BOOLEAN      NOT NULL DEFAULT FALSE,
    two_factor_secret  TEXT,        -- TOTP secret, encrypted at rest by the app layer
    last_login_at      TIMESTAMPTZ,
    last_login_ip      INET,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Sessions ─────────────────────────────────────────────────────────────────

CREATE TABLE user_sessions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id          TEXT,
    ip_address         INET,
    user_agent         TEXT,
    refresh_token_hash TEXT NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at         TIMESTAMPTZ NOT NULL,
    revoked_at         TIMESTAMPTZ
);

CREATE INDEX idx_sessions_user ON user_sessions(user_id);
CREATE INDEX idx_sessions_hash ON user_sessions(refresh_token_hash);
CREATE INDEX idx_sessions_exp  ON user_sessions(expires_at) WHERE revoked_at IS NULL;

-- ─── Push tokens ──────────────────────────────────────────────────────────────
-- One user -> many devices. Cleared when push provider returns "token invalid"
-- or when the linked session is revoked.

CREATE TABLE push_tokens (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id)         ON DELETE CASCADE,
    session_id   UUID          REFERENCES user_sessions(id) ON DELETE SET NULL,
    provider     TEXT NOT NULL CHECK (provider IN ('fcm','apns')),
    token        TEXT UNIQUE NOT NULL,
    device_type  TEXT CHECK (device_type IN ('ios','android','web')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

CREATE INDEX idx_push_tokens_user ON push_tokens(user_id);

-- ─── Movies ───────────────────────────────────────────────────────────────────

CREATE TABLE genres (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT UNIQUE NOT NULL,
    slug        TEXT UNIQUE NOT NULL,
    description TEXT
);

CREATE TABLE movies (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    title            TEXT         NOT NULL,
    original_title   TEXT,
    slug             TEXT         UNIQUE NOT NULL,
    synopsis         TEXT,
    tagline          TEXT,
    release_date     DATE,
    runtime_mins     INT          CHECK (runtime_mins > 0),
    content_rating   TEXT,
    status           TEXT         NOT NULL DEFAULT 'draft'
                     CHECK (status IN ('draft','published','archived')),
    original_lang    TEXT         NOT NULL DEFAULT 'en',
    countries        TEXT[]       NOT NULL DEFAULT '{}',
    genres           TEXT[]       NOT NULL DEFAULT '{}',
    poster_url       TEXT,
    backdrop_url     TEXT,
    trailer_url      TEXT,
    imdb_id          TEXT         UNIQUE,
    tmdb_id          INT          UNIQUE,
    -- NOT NULL DEFAULT 0 — aggregates are never NULL on the read path.
    avg_rating       NUMERIC(4,2) NOT NULL DEFAULT 0,
    rating_count     INT          NOT NULL DEFAULT 0,
    popularity_score NUMERIC(10,4) NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_movies_status     ON movies(status);
CREATE INDEX idx_movies_slug       ON movies(slug);
CREATE INDEX idx_movies_imdb       ON movies(imdb_id) WHERE imdb_id IS NOT NULL;
CREATE INDEX idx_movies_tmdb       ON movies(tmdb_id) WHERE tmdb_id IS NOT NULL;
CREATE INDEX idx_movies_genres     ON movies USING GIN(genres);
CREATE INDEX idx_movies_countries  ON movies USING GIN(countries);
CREATE INDEX idx_movies_trgm_title ON movies USING GIN(title gin_trgm_ops);
CREATE INDEX idx_movies_popularity ON movies(popularity_score DESC, avg_rating DESC)
    WHERE status = 'published';
CREATE INDEX idx_movies_release    ON movies(release_date DESC)
    WHERE status = 'published';

CREATE TABLE persons (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    slug        TEXT UNIQUE NOT NULL,
    birth_date  DATE,
    death_date  DATE,
    birthplace  TEXT,
    bio         TEXT,
    profile_url TEXT,
    imdb_id     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_persons_name ON persons USING GIN(name gin_trgm_ops);

CREATE TABLE credits (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    movie_id      UUID NOT NULL REFERENCES movies(id)  ON DELETE CASCADE,
    person_id     UUID NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
    role          TEXT NOT NULL,
    character     TEXT,
    billing_order INT  NOT NULL DEFAULT 999,
    UNIQUE (movie_id, person_id, role)
);

CREATE INDEX idx_credits_movie  ON credits(movie_id);
CREATE INDEX idx_credits_person ON credits(person_id);

CREATE TABLE streaming_links (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    movie_id    UUID NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    provider    TEXT NOT NULL,
    link_url    TEXT NOT NULL,
    access_type TEXT NOT NULL CHECK (access_type IN ('stream','rent','buy')),
    price_cents INT,
    region      TEXT NOT NULL DEFAULT '',   -- '' = worldwide
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- access_type in the unique key: same provider can offer stream AND rent.
    UNIQUE (movie_id, provider, region, access_type)
);

CREATE INDEX idx_streaming_movie  ON streaming_links(movie_id);
CREATE INDEX idx_streaming_region ON streaming_links(movie_id, region);

CREATE TABLE filming_locations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    movie_id    UUID NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    latitude    NUMERIC(9,6),
    longitude   NUMERIC(9,6),
    description TEXT
);

CREATE INDEX idx_filming_movie ON filming_locations(movie_id);

-- ─── Ratings ──────────────────────────────────────────────────────────────────

CREATE TABLE ratings (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    movie_id        UUID         NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    user_id         UUID         NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    overall         NUMERIC(3,1) NOT NULL CHECK (overall        BETWEEN 1 AND 5),
    acting          NUMERIC(3,1)          CHECK (acting         BETWEEN 1 AND 5),
    direction       NUMERIC(3,1)          CHECK (direction      BETWEEN 1 AND 5),
    writing         NUMERIC(3,1)          CHECK (writing        BETWEEN 1 AND 5),
    cinematography  NUMERIC(3,1)          CHECK (cinematography BETWEEN 1 AND 5),
    soundtrack      NUMERIC(3,1)          CHECK (soundtrack     BETWEEN 1 AND 5),
    reaction        TEXT,
    is_verified     BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, movie_id)
);

CREATE INDEX idx_ratings_movie    ON ratings(movie_id);
CREATE INDEX idx_ratings_user     ON ratings(user_id);
-- Partial index for verified-only aggregate (leaderboard + credibility score).
CREATE INDEX idx_ratings_verified ON ratings(movie_id, overall) WHERE is_verified = TRUE;

-- ─── Reviews ──────────────────────────────────────────────────────────────────

CREATE TABLE reviews (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    movie_id         UUID         NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    user_id          UUID         NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    type             TEXT         NOT NULL
                     CHECK (type IN ('short','long_form','spoiler','video')),
    body             TEXT,
    video_url        TEXT,
    is_spoiler       BOOLEAN      NOT NULL DEFAULT FALSE,
    is_published     BOOLEAN      NOT NULL DEFAULT FALSE,
    -- Reaction counters denormalised for O(1) reads.
    -- Incremented atomically by the write path; never recomputed by COUNT(*).
    like_count       INT          NOT NULL DEFAULT 0,
    helpful_count    INT          NOT NULL DEFAULT 0,
    insightful_count INT          NOT NULL DEFAULT 0,
    funny_count      INT          NOT NULL DEFAULT 0,
    credibility_score NUMERIC(5,4) NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reviews_movie ON reviews(movie_id, created_at DESC)
    WHERE is_published = TRUE;
CREATE INDEX idx_reviews_user  ON reviews(user_id);
CREATE INDEX idx_reviews_type  ON reviews(movie_id, type)
    WHERE is_published = TRUE;

CREATE TABLE review_reactions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    review_id   UUID NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    type        TEXT NOT NULL CHECK (type IN ('like','helpful','insightful','funny')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (review_id, user_id)
);

CREATE INDEX idx_review_reactions_review ON review_reactions(review_id);

-- ─── Watchlist ────────────────────────────────────────────────────────────────

CREATE TABLE watchlist_entries (
    id         UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID    NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    movie_id   UUID    NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    status     TEXT    NOT NULL
               CHECK (status IN ('to_watch','watching','watched','dropped')),
    progress   INT     NOT NULL DEFAULT 0 CHECK (progress >= 0),
    added_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, movie_id)
);

CREATE INDEX idx_watchlist_user        ON watchlist_entries(user_id);
CREATE INDEX idx_watchlist_movie       ON watchlist_entries(movie_id);
CREATE INDEX idx_watchlist_user_status ON watchlist_entries(user_id, status);

CREATE TABLE custom_lists (
    id          UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT    NOT NULL,
    description TEXT,
    is_public   BOOLEAN NOT NULL DEFAULT TRUE,
    cover_url   TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_custom_lists_user   ON custom_lists(user_id);
CREATE INDEX idx_custom_lists_public ON custom_lists(created_at DESC)
    WHERE is_public = TRUE;

CREATE TABLE custom_list_items (
    list_id  UUID NOT NULL REFERENCES custom_lists(id) ON DELETE CASCADE,
    movie_id UUID NOT NULL REFERENCES movies(id)       ON DELETE CASCADE,
    position INT  NOT NULL DEFAULT 0,
    note     TEXT,   -- per-item annotation ("great car chase in Act 2")
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (list_id, movie_id)
);

CREATE INDEX idx_list_items_list  ON custom_list_items(list_id, position);
CREATE INDEX idx_list_items_movie ON custom_list_items(movie_id);

-- Collaborative lists: other users can be granted read or write access.
CREATE TABLE list_collaborators (
    list_id    UUID NOT NULL REFERENCES custom_lists(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id)        ON DELETE CASCADE,
    access     TEXT NOT NULL DEFAULT 'read'
               CHECK (access IN ('read','write')),
    invited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (list_id, user_id)
);

CREATE INDEX idx_list_collaborators_user ON list_collaborators(user_id);

-- ─── Social ───────────────────────────────────────────────────────────────────

CREATE TABLE follows (
    follower_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (follower_id, followee_id),
    CHECK (follower_id != followee_id)
);

CREATE INDEX idx_follows_follower ON follows(follower_id);
CREATE INDEX idx_follows_followee ON follows(followee_id);
-- Covering index for fan-out follower-count query — avoids a heap fetch.
CREATE INDEX idx_follows_followee_count ON follows(followee_id) INCLUDE (follower_id);

-- ─── Payments ─────────────────────────────────────────────────────────────────

CREATE TABLE subscriptions (
    id                 UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan               TEXT    NOT NULL CHECK (plan IN ('free','pro','studio')),
    status             TEXT    NOT NULL
                       CHECK (status IN ('active','past_due','cancelled','trialing')),
    stripe_sub_id      TEXT    UNIQUE NOT NULL,
    stripe_customer_id TEXT    NOT NULL,
    current_period_end TIMESTAMPTZ NOT NULL,
    cancel_at_end      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_user   ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_status ON subscriptions(status);
CREATE INDEX idx_subscriptions_stripe ON subscriptions(stripe_sub_id);

CREATE TABLE invoices (
    id                UUID   PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID   REFERENCES users(id)         ON DELETE SET NULL,
    -- UUID FK (was TEXT) — referential integrity, enables join with subscriptions.
    subscription_id   UUID   REFERENCES subscriptions(id) ON DELETE SET NULL,
    stripe_invoice_id TEXT   UNIQUE NOT NULL,
    amount_cents      BIGINT NOT NULL CHECK (amount_cents >= 0),
    currency          TEXT   NOT NULL DEFAULT 'usd',
    status            TEXT   NOT NULL CHECK (status IN ('paid','open','void')),
    paid_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invoices_user         ON invoices(user_id);
CREATE INDEX idx_invoices_subscription ON invoices(subscription_id);
CREATE INDEX idx_invoices_stripe       ON invoices(stripe_invoice_id);

CREATE TABLE screener_licenses (
    id           UUID   PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID   NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    movie_id     UUID   NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    amount_cents BIGINT NOT NULL CHECK (amount_cents >= 0),
    currency     TEXT   NOT NULL DEFAULT 'usd',
    expires_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_licenses_user_movie ON screener_licenses(user_id, movie_id);
-- Partial index: only non-expired licenses appear on the access-check hot path.
CREATE INDEX idx_licenses_expiry ON screener_licenses(expires_at);

-- ─── Upload ───────────────────────────────────────────────────────────────────

CREATE TABLE media_assets (
    id             UUID   PRIMARY KEY DEFAULT gen_random_uuid(),
    -- UUID (not TEXT): matches the Redis session key format, enables hash
    -- joins, and prevents accidental string comparison bugs.
    upload_id      UUID   NOT NULL UNIQUE,
    user_id        UUID   NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    movie_id       UUID            REFERENCES movies(id) ON DELETE SET NULL,
    storage_key    TEXT   NOT NULL,
    content_type   TEXT   NOT NULL,
    size_bytes     BIGINT NOT NULL CHECK (size_bytes > 0),
    status         TEXT   NOT NULL DEFAULT 'uploading'
                   CHECK (status IN ('uploading','processing','ready','failed','moderated')),
    failure_reason TEXT,           -- set by saga compensation activities
    source_info    JSONB,          -- FFprobe output: codec, resolution, duration, bitrate
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_media_assets_user   ON media_assets(user_id);
CREATE INDEX idx_media_assets_movie  ON media_assets(movie_id)  WHERE movie_id IS NOT NULL;
-- Partial: processing queue scan skips the majority of rows (status='ready').
CREATE INDEX idx_media_assets_status ON media_assets(status)    WHERE status != 'ready';

-- ─── Moderation ──────────────────────────────────────────────────────────────

CREATE TABLE moderation_cases (
    id                UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    content_type      TEXT    NOT NULL
                      CHECK (content_type IN ('review','upload','post','profile','comment')),
    content_id        TEXT    NOT NULL,
    owner_id          UUID    REFERENCES users(id) ON DELETE SET NULL,
    submitted_by      TEXT    NOT NULL,
    trigger_reason    TEXT    NOT NULL
                      CHECK (trigger_reason IN ('auto_screen','user_report','admin_flag','ai_flag')),
    status            TEXT    NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending','ai_reviewed','in_review','decided','appealed')),
    -- NOT NULL DEFAULT '{}' — application code never needs to check for NULL array.
    ai_signals        TEXT[]  NOT NULL DEFAULT '{}',
    ai_confidence     JSONB,
    ai_recommendation TEXT    CHECK (ai_recommendation IN ('approve','reject','escalate_to_human')),
    decision          TEXT    CHECK (decision IN ('approved','rejected','escalated')),
    decided_by        TEXT,
    reason            TEXT,
    decided_at        TIMESTAMPTZ,
    locked_by         TEXT,
    locked_until      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mod_cases_status  ON moderation_cases(status);
CREATE INDEX idx_mod_cases_content ON moderation_cases(content_type, content_id);
CREATE INDEX idx_mod_cases_owner   ON moderation_cases(owner_id);
-- Queue index: moderator dashboard polls pending + ai_reviewed by age.
CREATE INDEX idx_mod_cases_queue   ON moderation_cases(created_at ASC)
    WHERE status IN ('pending','ai_reviewed');

CREATE TABLE moderation_appeals (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id      UUID NOT NULL REFERENCES moderation_cases(id) ON DELETE CASCADE,
    appellant_id UUID NOT NULL REFERENCES users(id)            ON DELETE CASCADE,
    statement    TEXT NOT NULL,
    outcome      TEXT CHECK (outcome IN ('upheld','overturned')),
    decided_by   TEXT,
    decided_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Enforce one appeal per case at the DB level.
CREATE UNIQUE INDEX idx_mod_appeals_case      ON moderation_appeals(case_id);
CREATE        INDEX idx_mod_appeals_appellant ON moderation_appeals(appellant_id);

-- ─── Notifications ───────────────────────────────────────────────────────────
-- One row per delivery attempt per channel.
-- A single event (e.g. new follower) produces multiple rows:
-- one 'push', one 'email', one 'in_app'.

CREATE TABLE notifications (
    id         UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT    NOT NULL,
    title      TEXT    NOT NULL,
    body       TEXT    NOT NULL,
    data       JSONB,              -- deep-link payload for mobile clients
    channel    TEXT    NOT NULL
               CHECK (channel IN ('push','email','sms','in_app')),
    is_read    BOOLEAN NOT NULL DEFAULT FALSE,
    sent_at    TIMESTAMPTZ,        -- NULL = not yet dispatched
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user   ON notifications(user_id, created_at DESC);
CREATE INDEX idx_notifications_unread ON notifications(user_id)
    WHERE is_read = FALSE AND channel = 'in_app';
-- Dispatch worker scans unsent rows ordered by age — partial avoids full scan.
CREATE INDEX idx_notifications_unsent ON notifications(created_at ASC)
    WHERE sent_at IS NULL;

COMMIT;