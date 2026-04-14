// Package circuit/middleware wires circuit breakers into the HTTP proxy layer.
// Each upstream service path prefix gets its own breaker.  When a breaker is
// OPEN the middleware short-circuits with a 503 and a JSON body that matches
// the platform error envelope — the client never sees a raw "Bad Gateway".
//
// Fallback responses are defined per-service so degraded mode is
// intentional rather than accidental:
//
//	recommendation → 200 with cached popular movies from Redis
//	search         → 200 with an empty result set + "degraded" flag
//	moderation     → 503 (cannot degrade safely)
//	payment        → 503 (cannot degrade safely)
//	all others     → 503 with a clear error body
package circuit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// FallbackFn is called when a circuit is OPEN instead of proxying upstream.
// It should write a valid response that the client can handle gracefully.
type FallbackFn func(w http.ResponseWriter, r *http.Request)

// RouteConfig maps a URL path prefix to a named breaker + optional fallback.
type RouteConfig struct {
	Prefix   string
	Service  string
	Fallback FallbackFn // nil = generic 503
}

// Middleware wraps an http.Handler with per-route circuit breakers.
type Middleware struct {
	manager *Manager
	routes  []RouteConfig
}

// NewMiddleware constructs the middleware.  routes are matched in order; the
// first prefix match wins.
func NewMiddleware(manager *Manager, routes []RouteConfig) *Middleware {
	return &Middleware{manager: manager, routes: routes}
}

// Handler wraps next.  For every request it:
//  1. Finds the matching route config.
//  2. Checks Allow() — if the breaker is OPEN it calls the fallback and returns.
//  3. Proxies to upstream via next, wrapping the ResponseWriter to capture
//     the status code.
//  4. Calls RecordSuccess or RecordFailure based on the status code.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := m.match(r.URL.Path)
		if rc == nil {
			next.ServeHTTP(w, r)
			return
		}

		breaker := m.manager.For(rc.Service)

		if !breaker.Allow() {
			log.Warn().
				Str("service", rc.Service).
				Str("path", r.URL.Path).
				Msg("circuit: open — serving fallback")

			if rc.Fallback != nil {
				rc.Fallback(w, r)
			} else {
				genericFallback(rc.Service)(w, r)
			}
			return
		}

		rw := &statusRecorder{ResponseWriter: w, code: 200}
		next.ServeHTTP(rw, r)

		if isServerError(rw.code) {
			breaker.RecordFailure(r.Context())
		} else {
			breaker.RecordSuccess(r.Context())
		}
	})
}

func (m *Middleware) match(path string) *RouteConfig {
	for i := range m.routes {
		if strings.HasPrefix(path, m.routes[i].Prefix) {
			return &m.routes[i]
		}
	}
	return nil
}

// ─── Built-in fallback factories ─────────────────────────────────────────────

// genericFallback returns a 503 with a standard error envelope.
func genericFallback(service string) FallbackFn {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": map[string]any{
				"code":    "SERVICE_UNAVAILABLE",
				"message": fmt.Sprintf("%s is temporarily unavailable — please retry shortly", service),
			},
		})
	}
}

// RecommendationFallback returns a degraded home feed from Redis.
// If Redis is also unavailable it returns an empty-but-valid payload.
func RecommendationFallback(redis *goredis.Client) FallbackFn {
	return func(w http.ResponseWriter, r *http.Request) {
		var popular []json.RawMessage

		if redis != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
			defer cancel()

			// Try to fetch pre-cached popular movies stored by the movie service.
			raw, err := redis.Get(ctx, "popular:global:top100").Bytes()
			if err == nil {
				_ = json.Unmarshal(raw, &popular)
			}
		}

		if popular == nil {
			popular = []json.RawMessage{}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"degraded": true,
			"rows": []map[string]any{
				{
					"title":  "Popular right now",
					"reason": "personalised_unavailable",
					"items":  popular,
				},
			},
		})
	}
}

// SearchFallback returns an empty result set with a degraded flag so the UI
// can show a "search unavailable" message without a hard error.
func SearchFallback() FallbackFn {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"degraded": true,
			"results":  []any{},
			"total":    0,
			"message":  "Search is temporarily unavailable",
		})
	}
}

// ─── Default route table ─────────────────────────────────────────────────────

// DefaultRoutes returns the standard route → breaker mapping for ViewAura.
// Pass in a Redis client so recommendation fallbacks can serve cached data.
func DefaultRoutes(redis *goredis.Client) []RouteConfig {
	return []RouteConfig{
		// Safety-critical — never degrade, fail hard.
		{Prefix: "/api/v1/webhook/stripe", Service: "payment"},
		{Prefix: "/api/v1/me/subscription", Service: "payment"},
		{Prefix: "/api/v1/moderation", Service: "moderation"},

		// Can degrade gracefully.
		{
			Prefix:   "/api/v1/recommendations",
			Service:  "recommendation",
			Fallback: RecommendationFallback(redis),
		},
		{
			Prefix:   "/api/v1/search",
			Service:  "search",
			Fallback: SearchFallback(),
		},

		// Other core services — generic 503 on open circuit.
		{Prefix: "/api/v1/movies", Service: "movie"},
		{Prefix: "/api/v1/users", Service: "user"},
		{Prefix: "/api/v1/ratings", Service: "rating"},
		{Prefix: "/api/v1/reviews", Service: "review"},
		{Prefix: "/api/v1/watchlist", Service: "watchlist"},
		{Prefix: "/api/v1/social", Service: "social"},
		{Prefix: "/api/v1/uploads", Service: "upload"},
		{Prefix: "/api/v1/notifications", Service: "notification"},
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func isServerError(code int) bool { return code >= 500 }

func writeJSON(w http.ResponseWriter, status int, v any) {
	b, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.Copy(w, bytes.NewReader(b))
}

// statusRecorder captures the upstream HTTP status code.
type statusRecorder struct {
	http.ResponseWriter
	code    int
	written bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.written {
		r.code = code
		r.written = true
	}
	r.ResponseWriter.WriteHeader(code)
}
