package events

import "time"

// SearchIndexRequested is a command event requesting the search service to
// create or update the index document for a given entity.
// Produced by: movie-service (on create/update/publish), review-service (on publish),
// enrichment workflow (after metadata enrichment completes).
// Consumed by: search-service (MeiliSearch upsert operation).
type SearchIndexRequested struct {
	// EntityType identifies which MeiliSearch index to write to.
	// Supported values: "movie" | "review" | "person" | "genre"
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	// Priority controls queue ordering in the search worker.
	// "high" = new release or trending content; "normal" = routine update.
	Priority    string    `json:"priority"` // "high" | "normal"
	RequestedAt time.Time `json:"requested_at"`
}

// SearchIndexDeleted is a command event requesting the search service to
// remove a document from the index.
// Produced by: movie-service (on archive), review-service (on delete).
// Consumed by: search-service (MeiliSearch delete operation).
type SearchIndexDeleted struct {
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id"`
	DeletedAt  time.Time `json:"deleted_at"`
}

// SearchQueryPerformed is published after every search query for analytics.
// Consumed by: Flink (ClickHouse — query analytics, trending topics, zero-results
// report used to surface content gaps to the editorial team).
// NOTE: This event is published asynchronously and must never block the
// search response path.
type SearchQueryPerformed struct {
	// SessionID is a pseudonymous identifier (not the user_id) for analytics.
	SessionID   string         `json:"session_id"`
	Query       string         `json:"query"`
	Filters     map[string]any `json:"filters,omitempty"` // genre, year, etc.
	ResultCount int            `json:"result_count"`
	// ClickedEntityID is set if the user clicked a result within 10 seconds.
	// Used to compute click-through rate per query.
	ClickedEntityID string    `json:"clicked_entity_id,omitempty"`
	ClickedRank     *int      `json:"clicked_rank,omitempty"` // 0-based position in results
	DurationMs      int       `json:"duration_ms"`
	PerformedAt     time.Time `json:"performed_at"`
}

// SearchSuggestCacheWarmRequested is published when the autocomplete warm-up
// job needs the search service to pre-compute suggestions for a given prefix.
// Produced by: Cloudflare Worker (on cache miss for a prefix with > 100 daily queries).
// Consumed by: search-service (compute + push to Redis ZSet search:suggest:{prefix}).
type SearchSuggestCacheWarmRequested struct {
	Prefix      string    `json:"prefix"`
	RequestedAt time.Time `json:"requested_at"`
}

// SearchRankingUpdated is published when the search service finishes a
// re-ranking run (triggered by RatingAggregateUpdated or a manual admin action).
// Consumed by: analytics (track ranking changes), admin-dashboard.
type SearchRankingUpdated struct {
	EntityType string    `json:"entity_type"` // "movie"
	EntityID   string    `json:"entity_id"`
	OldRank    *int      `json:"old_rank,omitempty"`
	NewRank    *int      `json:"new_rank,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}
