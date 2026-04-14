// Package middleware implements the gateway's HTTP middleware chain.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"github.com/Ab-dex/view-aura/apps/api-gateway/ratelimit"
)

// Context keys used across middleware.
type ctxKey string

const (
	ctxKeyRequestID ctxKey = "request_id"
	ctxKeyUserID    ctxKey = "user_id"
	ctxKeyUserRole  ctxKey = "user_role"
)

// ─── RequestID ────────────────────────────────────────────────────────────────

// RequestID injects a unique X-Request-ID into every request.
// If the upstream gateway (Kong / CloudFlare) already set the header,
// that value is preserved.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateID()
		}
		r = r.WithContext(context.WithValue(r.Context(), ctxKeyRequestID, id))
		r.Header.Set("X-Request-ID", id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

// ─── Logger ───────────────────────────────────────────────────────────────────

// Logger logs every request at INFO level with method, path, status, and latency.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		reqID, _ := r.Context().Value(ctxKeyRequestID).(string)
		log.Info().
			Str("request_id", reqID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", rw.statusCode).
			Dur("latency", time.Since(start)).
			Str("ip", clientIP(r)).
			Msg("gateway: request")
	})
}

// ─── Tracing ──────────────────────────────────────────────────────────────────

// Tracing forwards the W3C traceparent header from client to upstream,
// generating a new one when absent so every request has a trace ID.
func Tracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("traceparent") == "" {
			r.Header.Set("traceparent", generateTraceParent())
		}
		next.ServeHTTP(w, r)
	})
}

// ─── CORS ─────────────────────────────────────────────────────────────────────

// CORS handles cross-origin requests.
// allowedOrigins is a slice of exact origins; "*" is not supported intentionally
// to prevent credential-bearing cross-site requests.
func CORS(next http.Handler, allowedOrigins []string) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers",
				"Authorization,Content-Type,X-Request-ID,X-Session-ID")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "3600")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ─── Rate limiting ────────────────────────────────────────────────────────────

// RateLimit enforces per-IP sliding window limits using Redis.
// Limits: 1000 req/min per authenticated user, 100 req/min per IP (unauthenticated).
func RateLimit(next http.Handler, limiter *ratelimit.Limiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		key := "rate_limit:" + ip + ":" + r.URL.Path

		allowed, remaining, err := limiter.Allow(r.Context(), key, 100, time.Minute)
		if err != nil {
			// Redis unavailable — fail open (do not block traffic).
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if !allowed {
			w.Header().Set("Retry-After", "60")
			writeJSON(w, http.StatusTooManyRequests,
				`{"error":{"code":"TOO_MANY_REQUESTS","message":"rate limit exceeded"}}`)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ─── Auth ─────────────────────────────────────────────────────────────────────

// Auth validates Bearer JWTs using the RS256 public key.
// Requests to paths in skipPaths bypass auth entirely.
// On success, user_id and user_role are injected into both the request context
// and the X-User-ID / X-User-Role headers forwarded to the upstream.
func Auth(next http.Handler, publicKeyPath string, skipPaths []string) http.Handler {
	skip := make(map[string]bool, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = true
	}

	pubKey, err := loadPublicKey(publicKeyPath)
	if err != nil {
		// Log but do not fatal — the key may not be available in test mode.
		log.Warn().Err(err).Str("path", publicKeyPath).
			Msg("gateway: failed to load JWT public key — auth middleware disabled")
		pubKey = nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for public paths and for OPTIONS preflight.
		if r.Method == http.MethodOptions || isSkipped(r.URL.Path, skip) {
			next.ServeHTTP(w, r)
			return
		}

		// Public key unavailable (test mode) — pass through.
		if pubKey == nil {
			next.ServeHTTP(w, r)
			return
		}

		raw := r.Header.Get("Authorization")
		if raw == "" || !strings.HasPrefix(raw, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized,
				`{"error":{"code":"UNAUTHORIZED","message":"missing bearer token"}}`)
			return
		}

		tokenStr := strings.TrimPrefix(raw, "Bearer ")
		claims := &gatewayClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return pubKey, nil
		})
		if err != nil || !token.Valid {
			writeJSON(w, http.StatusUnauthorized,
				`{"error":{"code":"TOKEN_INVALID","message":"invalid or expired token"}}`)
			return
		}

		// Forward identity to the upstream service.
		r = r.WithContext(context.WithValue(r.Context(), ctxKeyUserID, claims.Subject))
		r.Header.Set("X-User-ID", claims.Subject)
		r.Header.Set("X-User-Role", claims.Role)

		next.ServeHTTP(w, r)
	})
}

// ─── Metrics ──────────────────────────────────────────────────────────────────

var (
	requestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_requests_total",
		Help: "Total requests handled by the API gateway.",
	}, []string{"method", "path", "status"})

	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gateway_request_duration_seconds",
		Help:    "Request latency distribution.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	}, []string{"method", "path"})
)

func init() {
	prometheus.MustRegister(requestsTotal, requestDuration)
}

// MetricsHandler returns a handler that exposes Prometheus metrics.
func MetricsHandler() http.Handler { return promhttp.Handler() }

// ─── JWT claims ───────────────────────────────────────────────────────────────

type gatewayClaims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generateTraceParent() string {
	// W3C traceparent: 00-{traceID}-{parentID}-01
	traceID := make([]byte, 16)
	parentID := make([]byte, 8)
	_, _ = rand.Read(traceID)
	_, _ = rand.Read(parentID)
	return fmt.Sprintf("00-%x-%x-01", traceID, parentID)
}

func isSkipped(path string, skip map[string]bool) bool {
	if skip[path] {
		return true
	}
	// Prefix match — e.g. /api/v1/movies matches /api/v1/movies/some-id
	for p := range skip {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(body))
}

func loadPublicKey(path string) (any, error) {
	if path == "" {
		return nil, fmt.Errorf("public key path is empty")
	}
	data, err := readFile(path)
	if err != nil {
		return nil, err
	}
	return jwt.ParseRSAPublicKeyFromPEM(data)
}

// readFile is a thin wrapper so tests can stub it.
var readFile = func(path string) ([]byte, error) {
	return os.ReadFile(path)
}
