// Package ratings provides rating operations — upsert, delete, list, stats.
package ratings

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

type doer interface {
	Do(ctx context.Context, method, path string, body, out any) error
}

// Service wraps all rating endpoints.
type Service struct{ doer doer }

func New(d doer) *Service { return &Service{doer: d} }

// ─── Types ────────────────────────────────────────────────────────────────────

// Rating is a user's dimensional score for a movie.
type Rating struct {
	ID             string   `json:"id"`
	MovieID        string   `json:"movie_id"`
	UserID         string   `json:"user_id"`
	Overall        float64  `json:"overall"`
	Acting         *float64 `json:"acting,omitempty"`
	Direction      *float64 `json:"direction,omitempty"`
	Writing        *float64 `json:"writing,omitempty"`
	Cinematography *float64 `json:"cinematography,omitempty"`
	Soundtrack     *float64 `json:"soundtrack,omitempty"`
	Reaction       *string  `json:"reaction,omitempty"`
	IsVerified     bool     `json:"is_verified"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

// RatingParams is the request body for Upsert.
// Overall is required (1.0–5.0); all dimension scores are optional.
type RatingParams struct {
	Overall        float64  `json:"overall"`
	Acting         *float64 `json:"acting,omitempty"`
	Direction      *float64 `json:"direction,omitempty"`
	Writing        *float64 `json:"writing,omitempty"`
	Cinematography *float64 `json:"cinematography,omitempty"`
	Soundtrack     *float64 `json:"soundtrack,omitempty"`
	// Reaction is a quick-tap alternative: "masterpiece" | "good" | "meh" | "bad" | "terrible"
	Reaction *string `json:"reaction,omitempty"`
}

// RatingStats holds aggregated per-movie stats.
type RatingStats struct {
	MovieID           string         `json:"movie_id"`
	AvgOverall        float64        `json:"avg_overall"`
	AvgActing         *float64       `json:"avg_acting,omitempty"`
	AvgDirection      *float64       `json:"avg_direction,omitempty"`
	AvgWriting        *float64       `json:"avg_writing,omitempty"`
	AvgCinematography *float64       `json:"avg_cinematography,omitempty"`
	AvgSoundtrack     *float64       `json:"avg_soundtrack,omitempty"`
	TotalRatings      int            `json:"total_ratings"`
	Distribution      map[string]int `json:"distribution"` // "1"→count, "2"→count, …
}

// RatingListResult is the paginated response from ListByMovie / ListByUser.
type RatingListResult struct {
	Ratings []Rating `json:"ratings"`
	Total   int      `json:"total"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// Upsert submits or updates the caller's rating for a movie.
//
//	rating, err := client.Ratings.Upsert(ctx, "movie-uuid", sdk.RatingParams{
//	    Overall: 4.5,
//	    Acting:  ptr(5.0),
//	})
func (s *Service) Upsert(ctx context.Context, movieID string, p RatingParams) (*Rating, error) {
	var out Rating
	return &out, s.doer.Do(ctx, http.MethodPut,
		"/api/v1/movies/"+url.PathEscape(movieID)+"/rating", p, &out)
}

// GetMine returns the caller's rating for a movie, or nil if not yet rated.
func (s *Service) GetMine(ctx context.Context, movieID string) (*Rating, error) {
	var out struct {
		Rating *Rating `json:"rating"`
	}
	if err := s.doer.Do(ctx, http.MethodGet,
		"/api/v1/movies/"+url.PathEscape(movieID)+"/rating", nil, &out); err != nil {
		return nil, err
	}
	return out.Rating, nil
}

// Delete removes the caller's rating for a movie.
func (s *Service) Delete(ctx context.Context, movieID string) error {
	return s.doer.Do(ctx, http.MethodDelete,
		"/api/v1/movies/"+url.PathEscape(movieID)+"/rating", nil, nil)
}

// ListByMovie returns paginated ratings for a movie.
func (s *Service) ListByMovie(ctx context.Context, movieID string, limit, offset int) (*RatingListResult, error) {
	q := paginationQuery(limit, offset)
	var out RatingListResult
	return &out, s.doer.Do(ctx, http.MethodGet,
		"/api/v1/movies/"+url.PathEscape(movieID)+"/ratings?"+q, nil, &out)
}

// GetStats returns aggregated rating stats for a movie.
func (s *Service) GetStats(ctx context.Context, movieID string) (*RatingStats, error) {
	var out RatingStats
	return &out, s.doer.Do(ctx, http.MethodGet,
		"/api/v1/movies/"+url.PathEscape(movieID)+"/ratings/stats", nil, &out)
}

// ListByUser returns all movies a user has rated.
func (s *Service) ListByUser(ctx context.Context, userID string, limit, offset int) (*RatingListResult, error) {
	q := paginationQuery(limit, offset)
	var out RatingListResult
	return &out, s.doer.Do(ctx, http.MethodGet,
		"/api/v1/users/"+url.PathEscape(userID)+"/ratings?"+q, nil, &out)
}

func paginationQuery(limit, offset int) string {
	v := url.Values{}
	if limit > 0 {
		v.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		v.Set("offset", strconv.Itoa(offset))
	}
	return v.Encode()
}
