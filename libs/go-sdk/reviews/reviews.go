// Package reviews provides review CRUD and reaction operations.
package reviews

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

type doer interface {
	Do(ctx context.Context, method, path string, body, out any) error
}

// Service wraps all review endpoints.
type Service struct{ doer doer }

func New(d doer) *Service { return &Service{doer: d} }

// ─── Types ────────────────────────────────────────────────────────────────────

// Review is a user's written or video critique of a movie.
type Review struct {
	ID              string `json:"id"`
	MovieID         string `json:"movie_id"`
	UserID          string `json:"user_id"`
	Type            string `json:"type"` // "short" | "long_form" | "spoiler" | "video"
	Body            string `json:"body,omitempty"`
	VideoURL        string `json:"video_url,omitempty"`
	IsSpoiler       bool   `json:"is_spoiler"`
	IsPublished     bool   `json:"is_published"`
	LikeCount       int    `json:"like_count"`
	HelpfulCount    int    `json:"helpful_count"`
	InsightfulCount int    `json:"insightful_count"`
	FunnyCount      int    `json:"funny_count"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// CreateReviewParams is the request body for Create.
type CreateReviewParams struct {
	Type      string `json:"type"`
	Body      string `json:"body,omitempty"`
	VideoURL  string `json:"video_url,omitempty"`
	IsSpoiler bool   `json:"is_spoiler"`
	Publish   bool   `json:"publish"` // true = publish immediately
}

// ReviewListResult is the paginated response from List.
type ReviewListResult struct {
	Reviews []Review `json:"reviews"`
	Total   int      `json:"total"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// Create submits a new review for a movie.
func (s *Service) Create(ctx context.Context, movieID string, p CreateReviewParams) (*Review, error) {
	var out Review
	return &out, s.doer.Do(ctx, http.MethodPost,
		"/api/v1/movies/"+url.PathEscape(movieID)+"/reviews", p, &out)
}

// List returns paginated reviews for a movie.
func (s *Service) List(ctx context.Context, movieID string, limit, offset int) (*ReviewListResult, error) {
	v := url.Values{}
	if limit > 0 {
		v.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		v.Set("offset", strconv.Itoa(offset))
	}
	var out ReviewListResult
	return &out, s.doer.Do(ctx, http.MethodGet,
		"/api/v1/movies/"+url.PathEscape(movieID)+"/reviews?"+v.Encode(), nil, &out)
}

// Get returns a single review by its UUID.
func (s *Service) Get(ctx context.Context, reviewID string) (*Review, error) {
	var out Review
	return &out, s.doer.Do(ctx, http.MethodGet,
		"/api/v1/reviews/"+url.PathEscape(reviewID), nil, &out)
}

// Delete permanently removes the caller's review.
func (s *Service) Delete(ctx context.Context, reviewID string) error {
	return s.doer.Do(ctx, http.MethodDelete,
		"/api/v1/reviews/"+url.PathEscape(reviewID), nil, nil)
}

// React adds or updates the caller's reaction on a review.
// reaction: "like" | "helpful" | "insightful" | "funny"
func (s *Service) React(ctx context.Context, reviewID, reaction string) error {
	return s.doer.Do(ctx, http.MethodPost,
		"/api/v1/reviews/"+url.PathEscape(reviewID)+"/reactions",
		map[string]string{"reaction": reaction}, nil)
}

// Unreact removes the caller's reaction from a review.
func (s *Service) Unreact(ctx context.Context, reviewID string) error {
	return s.doer.Do(ctx, http.MethodDelete,
		"/api/v1/reviews/"+url.PathEscape(reviewID)+"/reactions", nil, nil)
}

// Report submits an abuse report on a review.
// reason: "spam" | "hate_speech" | "unmarked_spoiler" | "other"
func (s *Service) Report(ctx context.Context, reviewID, reason string) error {
	return s.doer.Do(ctx, http.MethodPost,
		"/api/v1/reviews/"+url.PathEscape(reviewID)+"/reports",
		map[string]string{"reason": reason}, nil)
}
