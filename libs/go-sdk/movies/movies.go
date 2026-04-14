// Package movies provides movie catalog operations.
package movies

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type doer interface {
	Do(ctx context.Context, method, path string, body, out any) error
}

// Service wraps all movie catalog endpoints.
type Service struct{ doer doer }

func New(d doer) *Service { return &Service{doer: d} }

// ─── Types ────────────────────────────────────────────────────────────────────

// Movie is the canonical movie representation.
type Movie struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	OriginalTitle   string   `json:"original_title,omitempty"`
	Slug            string   `json:"slug"`
	Synopsis        string   `json:"synopsis,omitempty"`
	Tagline         string   `json:"tagline,omitempty"`
	ReleaseDate     *string  `json:"release_date,omitempty"`
	RuntimeMins     int      `json:"runtime_mins,omitempty"`
	ContentRating   string   `json:"content_rating,omitempty"`
	Status          string   `json:"status"`
	Genres          []string `json:"genres"`
	Countries       []string `json:"countries,omitempty"`
	PosterURL       string   `json:"poster_url,omitempty"`
	BackdropURL     string   `json:"backdrop_url,omitempty"`
	TrailerURL      string   `json:"trailer_url,omitempty"`
	IMDbID          string   `json:"imdb_id,omitempty"`
	TMDbID          int      `json:"tmdb_id,omitempty"`
	AvgRating       float64  `json:"avg_rating"`
	RatingCount     int      `json:"rating_count"`
	PopularityScore float64  `json:"popularity_score"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

// MovieDetail is the enriched response for a single-movie GET, including
// credits, streaming links, and filming locations.
type MovieDetail struct {
	Movie
	Credits          []Credit          `json:"credits"`
	StreamingLinks   []StreamingLink   `json:"streaming_links"`
	FilmingLocations []FilmingLocation `json:"filming_locations"`
}

// Credit links a person to a movie with a role.
type Credit struct {
	ID           string  `json:"id"`
	Role         string  `json:"role"`
	Character    string  `json:"character,omitempty"`
	BillingOrder int     `json:"billing_order"`
	Person       *Person `json:"person,omitempty"`
}

// Person is the condensed person representation embedded in credits.
type Person struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	ProfileURL string `json:"profile_url,omitempty"`
}

// StreamingLink tells clients where to watch, rent, or buy a movie.
type StreamingLink struct {
	ID         string `json:"id"`
	Provider   string `json:"provider"`
	LinkURL    string `json:"link_url"`
	AccessType string `json:"access_type"` // "stream" | "rent" | "buy"
	PriceCents *int   `json:"price_cents,omitempty"`
	Region     string `json:"region,omitempty"`
}

// FilmingLocation is where a movie was shot.
type FilmingLocation struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Genre is a taxonomy entry.
type Genre struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
}

// MovieListParams controls filtering and pagination for List.
type MovieListParams struct {
	Query     string
	Genres    []string
	Countries []string
	YearFrom  *int
	YearTo    *int
	MinRating *float64
	SortBy    string // "popularity" | "release_date" | "avg_rating" | "title"
	SortDir   string // "asc" | "desc"
	Limit     int
	Offset    int
}

// MovieListResult is the paginated response from List.
type MovieListResult struct {
	Movies []Movie `json:"movies"`
	Total  int     `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// List returns a paginated, filtered list of movies.
//
//	result, err := client.Movies.List(ctx, sdk.MovieListParams{
//	    Genres: []string{"Drama"}, SortBy: "avg_rating", Limit: 20,
//	})
func (s *Service) List(ctx context.Context, p MovieListParams) (*MovieListResult, error) {
	q := buildMovieQuery(p)
	var out MovieListResult
	return &out, s.doer.Do(ctx, http.MethodGet, "/api/v1/movies?"+q, nil, &out)
}

// Get returns a fully enriched movie by its UUID or slug.
func (s *Service) Get(ctx context.Context, idOrSlug string) (*MovieDetail, error) {
	var out MovieDetail
	return &out, s.doer.Do(ctx, http.MethodGet,
		"/api/v1/movies/"+url.PathEscape(idOrSlug), nil, &out)
}

// ListGenres returns the full genre taxonomy.
func (s *Service) ListGenres(ctx context.Context) ([]Genre, error) {
	var out struct {
		Genres []Genre `json:"genres"`
	}
	return out.Genres, s.doer.Do(ctx, http.MethodGet, "/api/v1/movies/genres", nil, &out)
}

// ListStreamingLinks returns where to watch a movie, optionally filtered by region.
func (s *Service) ListStreamingLinks(ctx context.Context, movieID, region string) ([]StreamingLink, error) {
	path := "/api/v1/movies/" + url.PathEscape(movieID) + "/streaming"
	if region != "" {
		path += "?region=" + url.QueryEscape(region)
	}
	var out struct {
		Links []StreamingLink `json:"links"`
	}
	return out.Links, s.doer.Do(ctx, http.MethodGet, path, nil, &out)
}

// ListCredits returns all cast and crew for a movie.
func (s *Service) ListCredits(ctx context.Context, movieID string) ([]Credit, error) {
	var out struct {
		Credits []Credit `json:"credits"`
	}
	return out.Credits, s.doer.Do(ctx, http.MethodGet,
		"/api/v1/movies/"+url.PathEscape(movieID)+"/credits", nil, &out)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func buildMovieQuery(p MovieListParams) string {
	v := url.Values{}
	if p.Query != "" {
		v.Set("q", p.Query)
	}
	for _, g := range p.Genres {
		v.Add("genres", g)
	}
	for _, c := range p.Countries {
		v.Add("countries", c)
	}
	if p.YearFrom != nil {
		v.Set("year_from", strconv.Itoa(*p.YearFrom))
	}
	if p.YearTo != nil {
		v.Set("year_to", strconv.Itoa(*p.YearTo))
	}
	if p.MinRating != nil {
		v.Set("min_rating", fmt.Sprintf("%.1f", *p.MinRating))
	}
	if p.SortBy != "" {
		v.Set("sort_by", p.SortBy)
	}
	if p.SortDir != "" {
		v.Set("sort_dir", p.SortDir)
	}
	if p.Limit > 0 {
		v.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Offset > 0 {
		v.Set("offset", strconv.Itoa(p.Offset))
	}
	return v.Encode()
}
