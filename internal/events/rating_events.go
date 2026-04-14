package events

import "time"

// RatingUpserted is published whenever a user creates or updates their rating
// for a movie (the Upsert endpoint is idempotent at the DB level).
// Consumed by: social-service (activity feed + fan-out), notification-service
// (review reaction nudge), Flink (real-time aggregate pipeline → ClickHouse).
type RatingUpserted struct {
	RatingID string `json:"rating_id"`
	MovieID  string `json:"movie_id"`
	UserID   string `json:"user_id"`
	// Overall is always present; dimension scores are nil when not provided.
	Overall        float64  `json:"overall"`
	Acting         *float64 `json:"acting,omitempty"`
	Direction      *float64 `json:"direction,omitempty"`
	Writing        *float64 `json:"writing,omitempty"`
	Cinematography *float64 `json:"cinematography,omitempty"`
	Soundtrack     *float64 `json:"soundtrack,omitempty"`
	// Reaction is the quick-tap emoji shorthand (can coexist with scores).
	Reaction   *string `json:"reaction,omitempty"`
	IsVerified bool    `json:"is_verified"`
	// IsNew distinguishes a brand-new rating from an update so consumers
	// can decide whether to emit a "first rating" activity or a "re-rated" one.
	IsNew      bool      `json:"is_new"`
	UpsertedAt time.Time `json:"upserted_at"`
}

// RatingDeleted is published when a user removes their rating.
// Consumed by: social-service (remove from feed), Flink (update aggregates).
type RatingDeleted struct {
	MovieID   string    `json:"movie_id"`
	UserID    string    `json:"user_id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// RatingAggregateUpdated is published after the aggregate pipeline recomputes
// the per-movie rolled-up scores. This is the event that drives cache busting
// and search re-ranking — NOT RatingUpserted (which fires too frequently).
// Published by: Flink job (every 5 minutes) or rating-service after every upsert
// when the movie has < 100 ratings (low-volume early adopter phase).
// Consumed by: movie-service (UPDATE movies SET avg_rating, rating_count),
// search-service (re-rank document), Redis cache (bust movie:{id}:rating:agg).
type RatingAggregateUpdated struct {
	MovieID    string  `json:"movie_id"`
	AvgOverall float64 `json:"avg_overall"`
	// Dimension averages are nil when fewer than 10 users have scored that axis.
	AvgActing         *float64 `json:"avg_acting,omitempty"`
	AvgDirection      *float64 `json:"avg_direction,omitempty"`
	AvgWriting        *float64 `json:"avg_writing,omitempty"`
	AvgCinematography *float64 `json:"avg_cinematography,omitempty"`
	AvgSoundtrack     *float64 `json:"avg_soundtrack,omitempty"`
	TotalRatings      int      `json:"total_ratings"`
	// Distribution maps rounded star score → count, e.g. {"1":10,"2":30,"5":80}.
	// Used by the frontend histogram widget.
	Distribution map[string]int `json:"distribution"`
	// WeightedScore is the leaderboard score: AvgOverall × log-dampening factor.
	WeightedScore float64   `json:"weighted_score"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ReactionAdded is published when a user quick-taps a reaction on a movie
// without giving a numeric score (standalone reaction use case).
// Consumed by: social-service (activity feed), analytics.
type ReactionAdded struct {
	MovieID   string    `json:"movie_id"`
	UserID    string    `json:"user_id"`
	Reaction  string    `json:"reaction"` // "masterpiece" | "good" | "meh" | "bad" | "terrible"
	ReactedAt time.Time `json:"reacted_at"`
}
