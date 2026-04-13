package domain

import "time"

// ─── IDs ──────────────────────────────────────────────────────────────────────

type RatingID string

func (id RatingID) String() string { return string(id) }

// ─── Enumerations ─────────────────────────────────────────────────────────────

// Reaction is the single-tap quick-rate value (emoji shorthand in the PRD).
type Reaction string

const (
	ReactionMasterpiece Reaction = "masterpiece" // 🔥
	ReactionGood        Reaction = "good"        // 👍
	ReactionMeh         Reaction = "meh"         // 😐
	ReactionBad         Reaction = "bad"         // 👎
	ReactionTerrible    Reaction = "terrible"    // 💀
)

// ─── Core entity ──────────────────────────────────────────────────────────────

// Rating is a user's dimensional score for a single movie.
// Overall and each dimension are nullable — a user may rate overall only,
// or fill all dimensions. A nil score means "not rated for this dimension".
type Rating struct {
	ID             RatingID
	MovieID        string   // domain.MovieID as string to avoid import cycle
	UserID         string   // domain.UserID as string
	Overall        float64  // 1.0–5.0, required
	Acting         *float64 // 1.0–5.0, optional
	Direction      *float64
	Writing        *float64
	Cinematography *float64
	Soundtrack     *float64
	// Reaction is the quick-tap alternative to dimensional scoring.
	// Both can coexist: a user may quick-react AND give a score.
	Reaction   *Reaction
	IsVerified bool // "Verified Watch" badge — set by the streaming/ticketing layer
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (r *Rating) IsValid() bool {
	return r.Overall >= 1.0 && r.Overall <= 5.0
}

// MovieRatingStats holds the pre-aggregated per-movie stats written by the
// ratings worker. Stored in the movies table but computed here.
type MovieRatingStats struct {
	MovieID           string
	AvgOverall        float64
	AvgActing         *float64
	AvgDirection      *float64
	AvgWriting        *float64
	AvgCinematography *float64
	AvgSoundtrack     *float64
	TotalRatings      int
	Distribution      map[int]int // star bucket → count, e.g. {1:10, 2:20, 3:50, 4:80, 5:40}
}

// ─── Commands ─────────────────────────────────────────────────────────────────

type UpsertRatingCmd struct {
	MovieID        string
	UserID         string
	Overall        float64
	Acting         *float64
	Direction      *float64
	Writing        *float64
	Cinematography *float64
	Soundtrack     *float64
	Reaction       *Reaction
}

type RatingFilter struct {
	MovieID string
	UserID  string
	Limit   int
	Offset  int
}
