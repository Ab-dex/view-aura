-- Migration: 002_materialized_views (down)
-- Description: Drops the materialized views and their indexes created in
--              002_materialized_views.up.sql.
--
-- Drop order: mv_genre_leaderboard before mv_movie_rating_agg because
-- the leaderboard view depends on mv_movie_rating_agg as a source.
-- Dropping the dependency first avoids a "view depends on materialized
-- view" error even though Postgres does not enforce inter-view FKs —
-- it is safer and documents the dependency explicitly.

BEGIN;

-- ─── Genre leaderboard ───────────────────────────────────────────────────────
-- Drop the index first (Postgres drops it automatically with the view,
-- but being explicit is safer and documents the relationship).
DROP INDEX IF EXISTS idx_mv_leaderboard_genre;
DROP MATERIALIZED VIEW IF EXISTS mv_genre_leaderboard;

-- ─── Movie rating aggregate ───────────────────────────────────────────────────
DROP INDEX IF EXISTS idx_mv_rating_agg_movie;
DROP MATERIALIZED VIEW IF EXISTS mv_movie_rating_agg;

COMMIT;