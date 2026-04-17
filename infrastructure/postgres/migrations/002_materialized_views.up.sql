-- Migration: 002_materialized_views
-- Description: Aggregated views used by API + background jobs

BEGIN;

-- Ensure dependency exists (fails early if migration order is wrong)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_tables WHERE tablename = 'ratings') THEN
        RAISE EXCEPTION 'ratings table does not exist — run 001_initial_schema first';
    END IF;
END $$;

-- ─────────────────────────────────────────────────────────────
-- Movie rating aggregate
-- ─────────────────────────────────────────────────────────────

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_movie_rating_agg AS
SELECT
    movie_id,

    ROUND(AVG(overall)::NUMERIC, 2)        AS avg_overall,
    ROUND(AVG(acting)::NUMERIC, 2)         AS avg_acting,
    ROUND(AVG(direction)::NUMERIC, 2)      AS avg_direction,
    ROUND(AVG(writing)::NUMERIC, 2)        AS avg_writing,
    ROUND(AVG(cinematography)::NUMERIC, 2) AS avg_cinematography,
    ROUND(AVG(soundtrack)::NUMERIC, 2)     AS avg_soundtrack,

    COUNT(*) AS total_ratings,

    COUNT(*) FILTER (WHERE ROUND(overall)::INT = 1) AS dist_1,
    COUNT(*) FILTER (WHERE ROUND(overall)::INT = 2) AS dist_2,
    COUNT(*) FILTER (WHERE ROUND(overall)::INT = 3) AS dist_3,
    COUNT(*) FILTER (WHERE ROUND(overall)::INT = 4) AS dist_4,
    COUNT(*) FILTER (WHERE ROUND(overall)::INT = 5) AS dist_5,

    SUM(
        overall * EXP(-0.001 * EXTRACT(EPOCH FROM (CURRENT_TIMESTAMP - created_at)) / 86400)
    ) AS weighted_score,

    MAX(updated_at) AS last_updated

FROM ratings
GROUP BY movie_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_mv_rating_agg_movie
ON mv_movie_rating_agg(movie_id);

-- ─────────────────────────────────────────────────────────────
-- Genre leaderboard
-- ─────────────────────────────────────────────────────────────

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_genre_leaderboard AS
SELECT
    g.genre,
    m.id AS movie_id,
    m.title,
    m.poster_url,
    r.weighted_score,
    r.total_ratings,
    r.avg_overall,

    ROW_NUMBER() OVER (
        PARTITION BY g.genre
        ORDER BY r.weighted_score DESC
    ) AS rank

FROM mv_movie_rating_agg r
JOIN movies m ON m.id = r.movie_id
CROSS JOIN LATERAL UNNEST(m.genres) AS g(genre)
WHERE m.status = 'published'
  AND r.total_ratings >= 10;

CREATE INDEX IF NOT EXISTS idx_mv_leaderboard_genre
ON mv_genre_leaderboard(genre, rank);

COMMIT;