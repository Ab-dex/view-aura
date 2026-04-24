package domain

import (
	"time"

	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/google/uuid"
)

// ─── IDs ─────────────────────────────────────────────────────────────────────

type MovieID uuid.UUID

func (id MovieID) String() string {
	return uuid.UUID(id).String()
}

type PersonID string

func (id PersonID) String() string { return string(id) }

type GenreID string

// ─── Enumerations ─────────────────────────────────────────────────────────────

type MovieStatus string

const (
	MovieStatusDraft     MovieStatus = "draft"
	MovieStatusPublished MovieStatus = "published"
	MovieStatusArchived  MovieStatus = "archived"
)

type ContentRating string

const (
	ContentRatingG    ContentRating = "G"
	ContentRatingPG   ContentRating = "PG"
	ContentRatingPG13 ContentRating = "PG-13"
	ContentRatingR    ContentRating = "R"
	ContentRatingNC17 ContentRating = "NC-17"
	ContentRatingNR   ContentRating = "NR"
)

func (c ContentRating) IsValid() bool {
	switch c {
	case ContentRatingG, ContentRatingPG, ContentRatingPG13, ContentRatingR, ContentRatingNC17, ContentRatingNR:
		return true
	default:
		return false
	}
}

func (c ContentRating) String() string {
	return string(c)
}

func ParseContentRating(s string) (ContentRating, error) {
	switch s {
	case "G", "PG", "PG-13", "R", "NC-17", "NR":
		return ContentRating(s), nil
	default:
		return "", apierror.Validation("invalid content rating", map[string]any{"value": s})
	}
}

type CreditRole string

const (
	CreditRoleDirector        CreditRole = "director"
	CreditRoleWriter          CreditRole = "writer"
	CreditRoleProducer        CreditRole = "producer"
	CreditRoleCinematographer CreditRole = "cinematographer"
	CreditRoleComposer        CreditRole = "composer"
	CreditRoleEditor          CreditRole = "editor"
	CreditRoleActor           CreditRole = "actor"
)

// ─── Core entities ─────────────────────────────────────────────────────────────

// Movie is the canonical film record.
type Movie struct {
	ID            MovieID
	Title         string
	OriginalTitle string // title in the film's original language
	Slug          string // url-safe unique identifier, e.g. "the-godfather-1972"
	Synopsis      string
	Tagline       string
	ReleaseDate   *time.Time
	RuntimeMins   int
	ContentRating ContentRating
	Status        MovieStatus
	OriginalLang  string   // BCP-47, e.g. "en", "ko", "fr"
	Countries     []string // ISO-3166 alpha-2
	Genres        []string // denormalized genre names for fast reads
	PosterURL     string
	BackdropURL   string
	TrailerURL    string
	IMDbID        string // "tt0068646"
	TMDbID        int    // 238
	// Aggregated scores — updated asynchronously, never computed inline.
	AvgRating       float64
	RatingCount     int
	PopularityScore float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Genre is the taxonomy entry.
type Genre struct {
	ID          GenreID
	Name        string
	Slug        string
	Description string
}

// Person represents any individual who worked on films (actor, director, etc.).
type Person struct {
	ID         PersonID
	Name       string
	Slug       string
	BirthDate  *time.Time
	DeathDate  *time.Time
	Birthplace string
	Bio        string
	ProfileURL string
	IMDbID     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Credit links a Person to a Movie with a specific role and optional character name.
type Credit struct {
	ID           string
	MovieID      MovieID
	PersonID     PersonID
	Role         CreditRole
	Character    string // filled for actor credits
	BillingOrder int    // cast list position
}

// StreamingLink tells users where to watch or rent/buy a movie.
type StreamingLink struct {
	ID         string
	MovieID    MovieID
	Provider   string // "Netflix", "Prime Video", "Apple TV+"
	LinkURL    string
	AccessType string // "stream", "rent", "buy"
	PriceCents *int   // nil for subscription titles
	Region     string // ISO-3166 alpha-2; "" = worldwide
	UpdatedAt  time.Time
}

// FilmingLocation records where a movie was physically shot.
type FilmingLocation struct {
	ID          string
	MovieID     MovieID
	Name        string // "Pinewood Studios, UK"
	Latitude    *float64
	Longitude   *float64
	Description string
}

// ─── Filter / pagination types ────────────────────────────────────────────────

// MovieFilter drives the search/browse query.
type MovieFilter struct {
	Query          string // full-text search
	Genres         []string
	Countries      []string
	Languages      []string
	ContentRatings []ContentRating
	YearFrom       *int
	YearTo         *int
	RuntimeMax     *int // minutes
	MinRating      *float64
	Status         *MovieStatus
	SortBy         string // "popularity", "release_date", "avg_rating", "title"
	SortDir        string // "asc", "desc"
	Limit          int
	Offset         int
}

// ─── Commands ─────────────────────────────────────────────────────────────────

type CreateMovieCmd struct {
	Title         string
	OriginalTitle string
	Synopsis      string
	Tagline       string
	ReleaseDate   *time.Time
	RuntimeMins   int
	ContentRating ContentRating
	OriginalLang  string
	Countries     []string
	Genres        []string
	PosterURL     string
	BackdropURL   string
	TrailerURL    string
	IMDbID        string
	TMDbID        int
}

type UpdateMovieCmd struct {
	ID            MovieID
	Title         *string
	OriginalTitle *string
	Synopsis      *string
	Tagline       *string
	ReleaseDate   *time.Time
	RuntimeMins   *int
	ContentRating *ContentRating
	OriginalLang  *string
	Countries     []string
	Genres        []string
	PosterURL     *string
	BackdropURL   *string
	TrailerURL    *string
	Status        *MovieStatus
}

type AddCreditCmd struct {
	MovieID      MovieID
	PersonID     PersonID
	Role         CreditRole
	Character    string
	BillingOrder int
}

type UpsertStreamingLinkCmd struct {
	MovieID    MovieID
	Provider   string
	LinkURL    string
	AccessType string
	PriceCents *int
	Region     string
}
