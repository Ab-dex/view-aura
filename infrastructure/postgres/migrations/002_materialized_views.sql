-- Migration: 002_materialized_views
-- Description: Aggregated views used by the API read path and Flink jobs.
--              Refreshed by a cron job or Temporal workflow every 5 minutes.

BEGIN;

-- Movie rating aggregate — served from Redis cache, this view is the source
-- of truth used when the cache is cold or when the background refresher runs.
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_movie_rating_agg AS
SELECT
    movie_id,
    ROUND(AVG(overall)::NUMERIC, 2)         AS avg_overall,
    ROUND(AVG(acting)::NUMERIC, 2)          AS avg_acting,
    ROUND(AVG(direction)::NUMERIC, 2)       AS avg_direction,
    ROUND(AVG(writing)::NUMERIC, 2)         AS avg_writing,
    ROUND(AVG(cinematography)::NUMERIC, 2)  AS avg_cinematography,
    ROUND(AVG(soundtrack)::NUMERIC, 2)      AS avg_soundtrack,
    COUNT(*)                                AS total_ratings,
    -- Star distribution bucket: rounds to nearest integer
    COUNT(*) FILTER (WHERE ROUND(overall) = 1) AS dist_1,
    COUNT(*) FILTER (WHERE ROUND(overall) = 2) AS dist_2,
    COUNT(*) FILTER (WHERE ROUND(overall) = 3) AS dist_3,
    COUNT(*) FILTER (WHERE ROUND(overall) = 4) AS dist_4,
    COUNT(*) FILTER (WHERE ROUND(overall) = 5) AS dist_5,
    -- Weighted popularity score: recency-dampened
    -- More recent ratings count more toward the score.
    SUM(
        overall * EXP(-0.001 * EXTRACT(EPOCH FROM (NOW() - created_at)) / 86400)
    ) AS weighted_score,
    MAX(updated_at) AS last_updated
FROM ratings
GROUP BY movie_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_mv_rating_agg_movie
    ON mv_movie_rating_agg(movie_id);

-- Genre weekly leaderboard — top 100 movies per genre by weighted_score.
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_genre_leaderboard AS
SELECT
    g.genre,
    m.id         AS movie_id,
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
  AND r.total_ratings >= 10;   -- minimum 10 ratings to appear on leaderboard

CREATE INDEX IF NOT EXISTS idx_mv_leaderboard_genre
    ON mv_genre_leaderboard(genre, rank);

COMMIT;