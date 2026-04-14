package cache

// keys.go — single source of truth for every Redis key pattern.
// TTLs are documented inline; enforcement is done at the call site.
//
// Pattern conventions:
//   {entity}:{id}:{concern}   — entity-scoped data
//   {concern}:{entity}:{id}   — concern-scoped lookups (e.g. rate_limit:ip:...)
//   {entity}:{id}             — when there is only one concern per entity

// ─── Session / auth keys ──────────────────────────────────────────────────────

// SessionKey stores a refresh-token hash → session record (TTL: refresh TTL).
func SessionKey(token string) string { return "session:" + token }

// UserSessionsKey is the set of active session IDs for a user (TTL: unbounded, cleaned on logout).
func UserSessionsKey(userID string) string { return "user:" + userID + ":sessions" }

// BlocklistJTIKey stores a revoked JWT jti to prevent reuse (TTL: remaining token TTL).
func BlocklistJTIKey(jti string) string { return "blocklist:jti:" + jti }

// ─── Rate-limit keys ──────────────────────────────────────────────────────────

// RateLimitKey is the sliding-window counter for a specific IP + route (TTL: 1 min).
func RateLimitKey(ip, route string) string { return "rate_limit:" + ip + ":" + route }

// UserRateLimitKey is the per-authenticated-user rate-limit counter (TTL: 1 min).
func UserRateLimitKey(userID, route string) string {
	return "rate_limit:user:" + userID + ":" + route
}

// ─── Movie / catalog keys ─────────────────────────────────────────────────────

// MovieMetaKey caches the hot movie metadata (TTL: 1h).
func MovieMetaKey(movieID string) string { return "movie:" + movieID + ":meta" }

// MovieAvailabilityKey caches JustWatch streaming availability per region (TTL: 6h).
func MovieAvailabilityKey(movieID, region string) string {
	return "movie:" + movieID + ":availability:" + region
}

// ─── Rating / scoring keys ────────────────────────────────────────────────────

// MovieRatingAggKey caches the aggregated avg+count hash (TTL: 5min).
func MovieRatingAggKey(movieID string) string { return "movie:" + movieID + ":rating:agg" }

// UserRatedKey is the set of movie IDs the user has rated — idempotency guard (TTL: 1h).
func UserRatedKey(userID string) string { return "user:" + userID + ":rated" }

// GenreLeaderboardKey is the sorted set of movie_id → weighted score for a genre (TTL: 1h).
func GenreLeaderboardKey(genre string) string { return "leaderboard:" + genre + ":weekly" }

// TrendingKey is the global sorted set of movie_id → trending score (TTL: 5min, refreshed by Flink).
func TrendingKey(window string) string { return "trending:" + window } // window: "1h", "24h", "7d"

// PopularGlobalKey caches the pre-serialised top-100 popular movies list for
// recommendation fallback responses (TTL: 15min).
func PopularGlobalKey() string { return "popular:global:top100" }

// ─── Watchlist keys ───────────────────────────────────────────────────────────

// UserWatchlistKey caches the watchlist hash of movie_id → status JSON (TTL: 30min).
func UserWatchlistKey(userID string) string { return "user:" + userID + ":watchlist" }

// ─── Social / feed keys ───────────────────────────────────────────────────────

// UserFeedKey is the sorted set of serialised activity events (score = unix ts, TTL: 30min).
func UserFeedKey(userID string) string { return "user:" + userID + ":feed" }

// FollowCountKey stores follower/following counts as a Hash (TTL: 10min).
func FollowCountKey(userID string) string { return "user:" + userID + ":follow_counts" }

// ─── Recommendation / AI keys ─────────────────────────────────────────────────

// UserEmbeddingKey stores the 512-dim taste vector (msgpack-encoded float32, TTL: 1h).
func UserEmbeddingKey(userID string) string { return "user:" + userID + ":embedding" }

// ProfileEmbeddingKey stores the per-profile taste vector (TTL: 1h).
func ProfileEmbeddingKey(profileID string) string { return "profile:" + profileID + ":embedding" }

// ─── Notification keys ────────────────────────────────────────────────────────

// NotifRateKey counts push notifications sent this hour to enforce the 20/hr cap (TTL: 1h).
func NotifRateKey(userID string) string { return "notif:rate:" + userID }

// ─── Upload / quota keys ──────────────────────────────────────────────────────

// UploadStateKey stores the in-progress upload session hash (TTL: 24h).
func UploadStateKey(uploadID string) string { return "upload:" + uploadID + ":state" }

// UploadProgressKey stores transcoding progress (TTL: 4h).
func UploadProgressKey(uploadID string) string { return "upload:" + uploadID + ":progress" }

// QuotaStorageKey tracks bytes used (TTL: 24h, refreshed on every upload event).
func QuotaStorageKey(userID string) string { return "quota:storage:" + userID }

// QuotaDailyKey counts uploads today (TTL: end of UTC day).
func QuotaDailyKey(userID string) string { return "quota:daily:" + userID }

// ─── Playback / viewing-history keys ─────────────────────────────────────────

// PlaybackResumeKey stores the last-known position_ms for a user+movie (TTL: 90 days).
// Written by the heartbeat endpoint every 10 seconds.
func PlaybackResumeKey(userID, movieID string) string {
	return "playback:resume:" + userID + ":" + movieID
}

// PlaybackSessionKey stores the active playback session metadata (TTL: 8h).
func PlaybackSessionKey(sessionID string) string { return "playback:session:" + sessionID }

// PlaybackManifestTokenKey stores the short-lived signed token for CDN segment auth (TTL: 4h).
func PlaybackManifestTokenKey(token string) string { return "playback:token:" + token }

// ContinueWatchingKey stores the sorted set of (movie_id → last_position_ms) for the
// "Continue Watching" row (score = last_watched_unix, TTL: 7 days).
func ContinueWatchingKey(userID string) string { return "user:" + userID + ":continue_watching" }

// ─── Profile keys ─────────────────────────────────────────────────────────────

// UserProfilesKey stores the list of profile IDs for a user account (TTL: 1h).
func UserProfilesKey(userID string) string { return "user:" + userID + ":profiles" }

// ProfileKey caches a single profile record (TTL: 30min).
func ProfileKey(profileID string) string { return "profile:" + profileID + ":meta" }

// ─── Experiment / feature-flag keys ──────────────────────────────────────────

// ExperimentAssignmentKey stores a user's treatment assignments as a Hash
// (experiment_id → variant, TTL: 24h).
func ExperimentAssignmentKey(userID string) string { return "experiment:assignment:" + userID }

// ─── Circuit-breaker keys ─────────────────────────────────────────────────────

// CircuitKey stores the serialised circuit state for a named service (TTL: managed by breaker).
func CircuitKey(namespace, service string) string {
	return "circuit:" + namespace + ":" + service
}

// ─── Watch party keys ─────────────────────────────────────────────────────────

// WatchPartyKey stores the watch-party session state hash (TTL: 4h).
func WatchPartyKey(partyID string) string { return "watch_party:" + partyID + ":state" }

// ─── Search autocomplete keys ─────────────────────────────────────────────────

// SearchSuggestKey is the sorted set of title suggestions for a prefix (TTL: 1h).
func SearchSuggestKey(prefix string) string { return "search:suggest:" + prefix }
