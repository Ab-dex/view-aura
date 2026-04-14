package events

import "time"

// MovieCreated is published when a new movie record is created (status: draft).
// Consumed by: search-service (index new document), enrichment workflow (fetch TMDB/IMDb metadata).
type MovieCreated struct {
	MovieID     string    `json:"movie_id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	IMDbID      string    `json:"imdb_id,omitempty"`
	TMDbID      int       `json:"tmdb_id,omitempty"`
	Genres      []string  `json:"genres"`
	ReleaseYear *int      `json:"release_year,omitempty"`
	CreatedBy   string    `json:"created_by"` // user_id of the producer/admin
	CreatedAt   time.Time `json:"created_at"`
}

// MoviePublished is published when a movie transitions to status: published.
// Consumed by: search-service (mark as searchable), notification-service
// (notify users who have it on their watchlist), social-service (activity feed).
type MoviePublished struct {
	MovieID     string    `json:"movie_id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	PosterURL   string    `json:"poster_url,omitempty"`
	Genres      []string  `json:"genres"`
	ReleaseDate *string   `json:"release_date,omitempty"` // RFC3339 date
	PublishedAt time.Time `json:"published_at"`
}

// MovieUpdated is published when any core movie fields change.
// Consumed by: search-service (re-index), redis cache invalidation.
type MovieUpdated struct {
	MovieID string `json:"movie_id"`
	Title   string `json:"title"`
	Slug    string `json:"slug"`
	// ChangedFields lists which top-level fields changed so consumers can
	// decide whether to act (e.g. search only re-indexes if title/synopsis/genres changed).
	ChangedFields []string  `json:"changed_fields"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// MovieArchived is published when a movie is soft-deleted (status: archived).
// Consumed by: search-service (remove from index), watchlist-service (mark entries stale).
type MovieArchived struct {
	MovieID    string    `json:"movie_id"`
	Title      string    `json:"title"`
	ArchivedAt time.Time `json:"archived_at"`
}

// MovieMetadataEnriched is published by the enrichment workflow after it has
// successfully fetched and merged TMDB/IMDb metadata.
// Consumed by: movie-service (apply updates), search-service (re-index with enriched data).
type MovieMetadataEnriched struct {
	MovieID        string         `json:"movie_id"`
	Source         string         `json:"source"`          // "tmdb" | "imdb" | "justwatch"
	EnrichedFields map[string]any `json:"enriched_fields"` // field name → new value
	EnrichedAt     time.Time      `json:"enriched_at"`
}

// StreamingAvailabilityChanged is published when streaming links are added,
// updated, or removed for a movie.
// Consumed by: notification-service (notify watchlist users "now on Netflix"),
// search-service (update where-to-watch index).
type StreamingAvailabilityChanged struct {
	MovieID    string    `json:"movie_id"`
	Title      string    `json:"title"`
	Provider   string    `json:"provider"`    // "Netflix", "Prime Video", etc.
	Region     string    `json:"region"`      // ISO-3166 alpha-2; "" = worldwide
	AccessType string    `json:"access_type"` // "stream" | "rent" | "buy"
	LinkURL    string    `json:"link_url"`
	ChangeType string    `json:"change_type"` // "added" | "updated" | "removed"
	ChangedAt  time.Time `json:"changed_at"`
}

// MovieRatingAggregateUpdated is published by the rating-service after it
// recomputes the aggregated score for a movie.
// Consumed by: movie-service (update avg_rating + rating_count columns),
// search-service (re-rank), leaderboard ZSet (Redis).
type MovieRatingAggregateUpdated struct {
	MovieID     string    `json:"movie_id"`
	AvgOverall  float64   `json:"avg_overall"`
	RatingCount int       `json:"rating_count"`
	Popularity  float64   `json:"popularity"` // weighted recency score
	UpdatedAt   time.Time `json:"updated_at"`
}

// MovieSearchIndexRequested is a command event requesting the search service
// to (re)index a specific movie. Produced by movie-service after create/update;
// also produced by the enrichment workflow after metadata enrichment.
// Consumed by: search-service.
type MovieSearchIndexRequested struct {
	MovieID     string    `json:"movie_id"`
	Priority    string    `json:"priority"` // "normal" | "high" (new releases = high)
	RequestedAt time.Time `json:"requested_at"`
}
