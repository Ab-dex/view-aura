// Package auth provides authentication operations: register, login, refresh, logout.
package auth

import (
	"context"
	"net/http"
)

// Service wraps all auth endpoints.
type Service struct{ doer doer }

type doer interface {
	Do(ctx context.Context, method, path string, body, out any) error
}

func New(d doer) *Service { return &Service{doer: d} }

// ─── Request / Response types ─────────────────────────────────────────────────

// RegisterParams is the request body for POST /users/register.
type RegisterParams struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Locale      string `json:"locale,omitempty"`
	Country     string `json:"country,omitempty"`
}

// LoginParams is the request body for POST /users/login.
type LoginParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	DeviceID string `json:"device_id,omitempty"`
}

// TokenPair is returned by Register and Login.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// AuthResponse wraps the user and token pair returned after successful auth.
type AuthResponse struct {
	User         UserInfo `json:"user"`
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int64    `json:"expires_in"`
}

// UserInfo is the condensed user representation embedded in auth responses.
type UserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Username      string `json:"username,omitempty"`
	DisplayName   string `json:"display_name"`
	Role          string `json:"role"`
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// Register creates a new account and returns auth tokens.
//
//	resp, err := client.Auth.Register(ctx, sdk.RegisterParams{
//	    Email: "alex@example.com", Password: "secret", DisplayName: "Alex",
//	})
func (s *Service) Register(ctx context.Context, p RegisterParams) (*AuthResponse, error) {
	var out AuthResponse
	return &out, s.doer.Do(ctx, http.MethodPost, "/api/v1/users/register", p, &out)
}

// Login authenticates with email/password and returns auth tokens.
func (s *Service) Login(ctx context.Context, p LoginParams) (*AuthResponse, error) {
	var out AuthResponse
	return &out, s.doer.Do(ctx, http.MethodPost, "/api/v1/users/login", p, &out)
}

// Refresh exchanges a refresh token for a new token pair.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	var out TokenPair
	return &out, s.doer.Do(ctx, http.MethodPost, "/api/v1/users/refresh",
		map[string]string{"refresh_token": refreshToken}, &out)
}

// Logout revokes the current session.
func (s *Service) Logout(ctx context.Context) error {
	return s.doer.Do(ctx, http.MethodPost, "/api/v1/users/logout", nil, nil)
}
