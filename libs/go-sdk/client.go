// Package sdk is the official Go client library for the ViewAura API.
//
// # Quick start
//
//	client := sdk.New("https://api.cinemaos.com", sdk.WithAPIKey("your-token"))
//
//	// List movies
//	movies, err := client.Movies.List(ctx, sdk.MovieListParams{Limit: 20})
//
//	// Rate a movie
//	err = client.Ratings.Upsert(ctx, "movie-uuid", sdk.RatingParams{Overall: 4.5})
//
// # Authentication
//
// Pass a Bearer token via WithAPIKey. The token is attached to every request
// as Authorization: Bearer <token>. Use the Auth service to obtain tokens.
package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Ab-dex/view-aura/libs/go-sdk/auth"
	"github.com/Ab-dex/view-aura/libs/go-sdk/movies"
	"github.com/Ab-dex/view-aura/libs/go-sdk/payment"
	"github.com/Ab-dex/view-aura/libs/go-sdk/quota"
	"github.com/Ab-dex/view-aura/libs/go-sdk/ratings"
	"github.com/Ab-dex/view-aura/libs/go-sdk/reviews"
	"github.com/Ab-dex/view-aura/libs/go-sdk/social"
	"github.com/Ab-dex/view-aura/libs/go-sdk/upload"
	"github.com/Ab-dex/view-aura/libs/go-sdk/watchlist"
)

const defaultTimeout = 30 * time.Second

// Client is the root ViewAura API client.
// Create one with New() and reuse it across your application.
// It is safe for concurrent use from multiple goroutines.
type Client struct {
	base       string
	httpClient *http.Client
	token      string

	// Service clients — access each domain through these fields.
	Auth      *auth.Service
	Movies    *movies.Service
	Ratings   *ratings.Service
	Reviews   *reviews.Service
	Watchlist *watchlist.Service
	Social    *social.Service
	Upload    *upload.Service
	Payment   *payment.Service
	Quota     *quota.Service
}

// Option configures the Client at construction time.
type Option func(*Client)

// WithAPIKey sets the Bearer token attached to every request.
func WithAPIKey(token string) Option {
	return func(c *Client) { c.token = token }
}

// WithHTTPClient replaces the default HTTP client (useful for testing or
// when you need custom transport/TLS config).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithTimeout overrides the default 30-second request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// New creates a configured ViewAura API client.
//
//	client := sdk.New("https://api.cinemaos.com", sdk.WithAPIKey(token))
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		base: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
	for _, o := range opts {
		o(c)
	}

	// Construct a transport that injects the auth header on every call.
	transport := &authTransport{
		base:  http.DefaultTransport,
		token: &c.token,
	}
	c.httpClient.Transport = transport

	// Initialise service clients with a shared doer so token rotation
	// propagates automatically (c.token is a pointer).
	doer := &httpDoer{client: c.httpClient, base: c.base}

	c.Auth = auth.New(doer)
	c.Movies = movies.New(doer)
	c.Ratings = ratings.New(doer)
	c.Reviews = reviews.New(doer)
	c.Watchlist = watchlist.New(doer)
	c.Social = social.New(doer)
	c.Upload = upload.New(doer)
	c.Payment = payment.New(doer)
	c.Quota = quota.New(doer)

	return c
}

// SetToken replaces the authentication token at runtime.
// Useful when a token is refreshed after construction.
func (c *Client) SetToken(token string) { c.token = token }

// ─── Internal HTTP plumbing ───────────────────────────────────────────────────

// Doer is the interface that service clients use to make HTTP calls.
// All service packages depend on this interface, not on *http.Client directly,
// so they can be tested by injecting a mock.
type Doer interface {
	Do(ctx context.Context, method, path string, body, out any) error
}

// httpDoer implements Doer using a real *http.Client.
type httpDoer struct {
	client *http.Client
	base   string
}

func (d *httpDoer) Do(ctx context.Context, method, path string, body, out any) error {
	return doRequest(ctx, d.client, method, d.base+path, body, out)
}

// authTransport injects the Authorization header on every outbound request.
type authTransport struct {
	base  http.RoundTripper
	token *string
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if *t.token != "" {
		req = req.Clone(req.Context())
		req.Header.Set("Authorization", "Bearer "+*t.token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return t.base.RoundTrip(req)
}

// ─── Shared request / response helpers ───────────────────────────────────────

func doRequest(ctx context.Context, hc *http.Client, method, url string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("sdk: marshal request: %w", err)
		}
		reqBody = bytesReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("sdk: build request: %w", err)
	}

	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("sdk: http %s %s: %w", method, url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return parseAPIError(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("sdk: decode response: %w", err)
	}
	return nil
}

// ─── Error type ───────────────────────────────────────────────────────────────

// APIError is returned when the server responds with a 4xx or 5xx status.
type APIError struct {
	StatusCode int
	Code       string `json:"code"`
	Message    string `json:"message"`
	Details    any    `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("viewaura: %s (%d): %s", e.Code, e.StatusCode, e.Message)
}

func parseAPIError(resp *http.Response) error {
	ae := &APIError{StatusCode: resp.StatusCode}
	var wrapper struct {
		Error *APIError `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err == nil && wrapper.Error != nil {
		wrapper.Error.StatusCode = resp.StatusCode
		return wrapper.Error
	}
	ae.Code = "UNKNOWN"
	ae.Message = resp.Status
	return ae
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

type bytesReaderT []byte

func (b bytesReaderT) Read(p []byte) (int, error) { return copy(p, b), io.EOF }

func bytesReader(b []byte) io.Reader { return bytesReaderT(b) }
