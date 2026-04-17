-- ============================================================
--  ViewAura — Migration 001: Down (Rollback)
--
--  Drops all tables created in 001_initial_schema.up.sql in
--  strict reverse dependency order so no FK constraint is
--  violated during the drop sequence.
--
--  Tables dropped leaf → root:
--    notifications
--    moderation_appeals → moderation_cases
--    media_assets
--    screener_licenses → invoices → subscriptions
--    follows
--    list_collaborators → custom_list_items → custom_lists
--    watchlist_entries
--    review_reactions → reviews
--    ratings
--    filming_locations → streaming_links → credits → persons
--    movies → genres
--    push_tokens
--    user_sessions
--    user_security
--    user_preferences
--    user_profiles
--    user_auth_providers
--    users
--    role_permissions → permissions → roles
--    (extensions left in place — they may be shared)
-- ============================================================

BEGIN;

-- ─── Notifications ───────────────────────────────────────────────────────────
DROP TABLE IF EXISTS notifications;

-- ─── Moderation ──────────────────────────────────────────────────────────────
DROP TABLE IF EXISTS moderation_appeals;
DROP TABLE IF EXISTS moderation_cases;

-- ─── Upload ───────────────────────────────────────────────────────────────────
DROP TABLE IF EXISTS media_assets;

-- ─── Payments ─────────────────────────────────────────────────────────────────
DROP TABLE IF EXISTS screener_licenses;
DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS subscriptions;

-- ─── Social ───────────────────────────────────────────────────────────────────
DROP TABLE IF EXISTS follows;

-- ─── Watchlist ────────────────────────────────────────────────────────────────
DROP TABLE IF EXISTS list_collaborators;
DROP TABLE IF EXISTS custom_list_items;
DROP TABLE IF EXISTS custom_lists;
DROP TABLE IF EXISTS watchlist_entries;

-- ─── Reviews ──────────────────────────────────────────────────────────────────
DROP TABLE IF EXISTS review_reactions;
DROP TABLE IF EXISTS reviews;

-- ─── Ratings ──────────────────────────────────────────────────────────────────
DROP TABLE IF EXISTS ratings;

-- ─── Movies ───────────────────────────────────────────────────────────────────
DROP TABLE IF EXISTS filming_locations;
DROP TABLE IF EXISTS streaming_links;
DROP TABLE IF EXISTS credits;
DROP TABLE IF EXISTS persons;
DROP TABLE IF EXISTS movies;
DROP TABLE IF EXISTS genres;

-- ─── User supporting tables ───────────────────────────────────────────────────
DROP TABLE IF EXISTS push_tokens;
DROP TABLE IF EXISTS user_sessions;
DROP TABLE IF EXISTS user_security;
DROP TABLE IF EXISTS user_preferences;
DROP TABLE IF EXISTS user_profiles;
DROP TABLE IF EXISTS user_auth_providers;

-- ─── Users ────────────────────────────────────────────────────────────────────
DROP TABLE IF EXISTS users;

-- ─── RBAC ─────────────────────────────────────────────────────────────────────
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;

-- ─── Extensions ───────────────────────────────────────────────────────────────
-- Extensions are intentionally NOT dropped here.
-- citext, pg_trgm, pgcrypto, and uuid-ossp may be used by other schemas
-- or future migrations and removing them would require superuser privileges.
-- Drop manually if a full teardown of the database is required.

COMMIT;