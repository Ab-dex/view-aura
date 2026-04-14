// Package watchlist provides watchlist entry and custom list operations.
package watchlist

import (
	"context"
	"net/http"
	"net/url"
)

type doer interface {
	Do(ctx context.Context, method, path string, body, out any) error
}

// Service wraps all watchlist endpoints.
type Service struct{ doer doer }

func New(d doer) *Service { return &Service{doer: d} }

// ─── Types ────────────────────────────────────────────────────────────────────

// Entry is a single watchlist status record.
type Entry struct {
	ID        string `json:"id"`
	MovieID   string `json:"movie_id"`
	UserID    string `json:"user_id"`
	Status    string `json:"status"` // "to_watch" | "watching" | "watched" | "dropped"
	Progress  int    `json:"progress"`
	AddedAt   string `json:"added_at"`
	UpdatedAt string `json:"updated_at"`
}

// SetStatusParams is the request body for SetStatus.
type SetStatusParams struct {
	Status   string `json:"status"`
	Progress int    `json:"progress,omitempty"`
}

// CustomList is a user-curated movie list.
type CustomList struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsPublic    bool   `json:"is_public"`
	CreatedAt   string `json:"created_at"`
}

// CreateListParams is the request body for CreateList.
type CreateListParams struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsPublic    bool   `json:"is_public"`
}

// ─── Watch status methods ─────────────────────────────────────────────────────

// SetStatus creates or updates the caller's watch status for a movie.
//
//	err := client.Watchlist.SetStatus(ctx, "movie-uuid", sdk.SetStatusParams{
//	    Status: "watched",
//	})
func (s *Service) SetStatus(ctx context.Context, movieID string, p SetStatusParams) (*Entry, error) {
	var out Entry
	return &out, s.doer.Do(ctx, http.MethodPut,
		"/api/v1/watchlist/"+url.PathEscape(movieID), p, &out)
}

// GetStatus returns the caller's watch status for a movie, or nil if not set.
func (s *Service) GetStatus(ctx context.Context, movieID string) (*Entry, error) {
	var out struct {
		Entry *Entry `json:"entry"`
	}
	if err := s.doer.Do(ctx, http.MethodGet,
		"/api/v1/watchlist/"+url.PathEscape(movieID), nil, &out); err != nil {
		return nil, err
	}
	return out.Entry, nil
}

// RemoveStatus removes the caller's watch entry for a movie.
func (s *Service) RemoveStatus(ctx context.Context, movieID string) error {
	return s.doer.Do(ctx, http.MethodDelete,
		"/api/v1/watchlist/"+url.PathEscape(movieID), nil, nil)
}

// ListEntries returns all watchlist entries for the caller, optionally
// filtered by status.
func (s *Service) ListEntries(ctx context.Context, status string) ([]Entry, error) {
	path := "/api/v1/me/watchlist"
	if status != "" {
		path += "?status=" + url.QueryEscape(status)
	}
	var out struct {
		Entries []Entry `json:"entries"`
	}
	return out.Entries, s.doer.Do(ctx, http.MethodGet, path, nil, &out)
}

// ─── Custom list methods ──────────────────────────────────────────────────────

// CreateList creates a new custom movie list.
func (s *Service) CreateList(ctx context.Context, p CreateListParams) (*CustomList, error) {
	var out CustomList
	return &out, s.doer.Do(ctx, http.MethodPost, "/api/v1/lists", p, &out)
}

// GetList returns a single custom list by ID.
func (s *Service) GetList(ctx context.Context, listID string) (*CustomList, error) {
	var out CustomList
	return &out, s.doer.Do(ctx, http.MethodGet,
		"/api/v1/lists/"+url.PathEscape(listID), nil, &out)
}

// AddToList adds a movie to a custom list.
func (s *Service) AddToList(ctx context.Context, listID, movieID string) error {
	return s.doer.Do(ctx, http.MethodPost,
		"/api/v1/lists/"+url.PathEscape(listID)+"/movies",
		map[string]string{"movie_id": movieID}, nil)
}

// RemoveFromList removes a movie from a custom list.
func (s *Service) RemoveFromList(ctx context.Context, listID, movieID string) error {
	return s.doer.Do(ctx, http.MethodDelete,
		"/api/v1/lists/"+url.PathEscape(listID)+"/movies/"+url.PathEscape(movieID),
		nil, nil)
}

// DeleteList permanently deletes a custom list.
func (s *Service) DeleteList(ctx context.Context, listID string) error {
	return s.doer.Do(ctx, http.MethodDelete,
		"/api/v1/lists/"+url.PathEscape(listID), nil, nil)
}
