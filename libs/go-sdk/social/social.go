// Package social provides follow graph and activity feed operations.
package social

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

type doer interface {
	Do(ctx context.Context, method, path string, body, out any) error
}

// Service wraps all social endpoints.
type Service struct{ doer doer }

func New(d doer) *Service { return &Service{doer: d} }

// ─── Types ────────────────────────────────────────────────────────────────────

// FeedEntry is a single activity item in the user's feed.
type FeedEntry struct {
	ID          string         `json:"id"`
	ActorID     string         `json:"actor_id"`
	ActorName   string         `json:"actor_name"`
	ActorAvatar string         `json:"actor_avatar,omitempty"`
	EventType   string         `json:"event_type"` // "rated" | "reviewed" | "watchlisted" | "followed"
	MovieID     string         `json:"movie_id,omitempty"`
	MovieTitle  string         `json:"movie_title,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	OccurredAt  string         `json:"occurred_at"`
}

// UserSummary is a condensed user representation used in follow lists.
type UserSummary struct {
	ID          string `json:"id"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// FeedResult is the paginated feed response.
type FeedResult struct {
	Entries []FeedEntry `json:"entries"`
	Total   int         `json:"total"`
	Limit   int         `json:"limit"`
	Offset  int         `json:"offset"`
}

// FollowListResult is the paginated follow list response.
type FollowListResult struct {
	Users  []UserSummary `json:"users"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// Follow follows a user.
func (s *Service) Follow(ctx context.Context, userID string) error {
	return s.doer.Do(ctx, http.MethodPost,
		"/api/v1/users/"+url.PathEscape(userID)+"/follow", nil, nil)
}

// Unfollow unfollows a user.
func (s *Service) Unfollow(ctx context.Context, userID string) error {
	return s.doer.Do(ctx, http.MethodDelete,
		"/api/v1/users/"+url.PathEscape(userID)+"/follow", nil, nil)
}

// GetFollowers returns paginated followers for a user.
func (s *Service) GetFollowers(ctx context.Context, userID string, limit, offset int) (*FollowListResult, error) {
	q := paginationQuery(limit, offset)
	var out FollowListResult
	return &out, s.doer.Do(ctx, http.MethodGet,
		"/api/v1/users/"+url.PathEscape(userID)+"/followers?"+q, nil, &out)
}

// GetFollowing returns paginated accounts a user follows.
func (s *Service) GetFollowing(ctx context.Context, userID string, limit, offset int) (*FollowListResult, error) {
	q := paginationQuery(limit, offset)
	var out FollowListResult
	return &out, s.doer.Do(ctx, http.MethodGet,
		"/api/v1/users/"+url.PathEscape(userID)+"/following?"+q, nil, &out)
}

// GetFeed returns the caller's personalised activity feed.
func (s *Service) GetFeed(ctx context.Context, limit, offset int) (*FeedResult, error) {
	q := paginationQuery(limit, offset)
	var out FeedResult
	return &out, s.doer.Do(ctx, http.MethodGet, "/api/v1/me/feed?"+q, nil, &out)
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
