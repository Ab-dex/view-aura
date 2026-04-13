package cache

// This file extends the existing key helpers in redis.go with every key
// pattern specified in the PRD Redis schema that was not yet defined.

// ─── Movie keys ───────────────────────────────────────────────────────────────

// MovieMetaKey caches the hot movie metadata hash (TTL: 1h).
func MovieMetaKey(movieID string) string { return "movie:" + movieID + ":meta" }

// ─── Rating keys ──────────────────────────────────────────────────────────────

// MovieRatingAggKey caches the aggregated avg+count hash (TTL: 5min).
func MovieRatingAggKey(movieID string) string { return "movie:" + movieID + ":rating:agg" }

// UserRatedKey is the set of movie IDs the user has rated, used for
// idempotency checks on the write path (TTL: 1h).
func UserRatedKey(userID string) string { return "user:" + userID + ":rated" }

// GenreLeaderboardKey is the weekly sorted set of movie_id → weighted score
// for a given genre (TTL: 1h).
func GenreLeaderboardKey(genre string) string { return "leaderboard:" + genre + ":weekly" }

// ─── Watchlist keys ───────────────────────────────────────────────────────────

// UserWatchlistKey caches the watchlist hash of movie_id → status JSON (TTL: 30min).
func UserWatchlistKey(userID string) string { return "user:" + userID + ":watchlist" }

// ─── Social / feed keys ───────────────────────────────────────────────────────

// UserFeedKey is the sorted set of serialised activity events for a user's
// feed (score = unix timestamp, TTL: 30min).
func UserFeedKey(userID string) string { return "user:" + userID + ":feed" }
