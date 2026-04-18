package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Ab-dex/view-aura/internal/modules/user/service"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
	"github.com/gin-gonic/gin"
)

// Context keys — never use plain strings as map keys across packages.
const (
	ContextKeyUserID       = "user_id"
	ContextKeyUserRole     = "user_role"
	ContextKeyJTI          = "jwt_jti"
	ContextKeySessionID    = "session_id"
	ContextKeyRemainingTTL = "jwt_remaining_ttl"
	ContextKeyRequestID    = "request_id"
)

// Auth validates the Bearer token, checks the JTI blocklist, then injects
// identity fields into the Gin context so downstream handlers can access them
// via c.Get(ContextKeyUserID) without re-parsing the token.
func Auth(tokens service.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		if raw == "" {
			abortUnauth(c, apierror.ErrTokenInvalid)
			return
		}

		parts := strings.SplitN(raw, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			abortUnauth(c, apierror.ErrTokenInvalid)
			return
		}

		tokenStr := strings.TrimSpace(parts[1])
		claims, err := tokens.ValidateAccess(c.Request.Context(), tokenStr)
		if err != nil {
			abortUnauth(c, err)
			return
		}

		// ── Populate context ───
		c.Set(ContextKeyUserID, claims.Subject)
		c.Set(ContextKeyUserRole, claims.Role)
		c.Set(ContextKeyJTI, claims.ID)

		// Session ID sent by the client on logout flows.
		if sid := c.GetHeader("X-Session-ID"); sid != "" {
			c.Set(ContextKeySessionID, sid)
		}

		// Remaining TTL for blocklist entry on logout.
		exp, err := claims.GetExpirationTime()
		if err == nil && exp != nil {
			// exp.Time is already a time.Time object in v5
			ttl := time.Until(exp.Time)

			// Ensure we aren't setting a negative TTL if the token just expired
			if ttl < 0 {
				ttl = 0
			}

			c.Set(ContextKeyRemainingTTL, ttl)
		}

		// Enrich the request-scoped logger with auth fields.
		ctx := logger.WithContext(c.Request.Context(), map[string]string{
			"user_id": claims.Subject,
			"role":    claims.Role,
		})
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// RequireRole aborts with 403 when the authenticated user's role is not in
// the allowed set. Must be placed after Auth in the middleware chain.
func RequireRole(allowed ...string) gin.HandlerFunc {
	permitted := make(map[string]struct{}, len(allowed))
	for _, r := range allowed {
		permitted[r] = struct{}{}
	}

	return func(c *gin.Context) {
		role, exists := c.Get(ContextKeyUserRole)
		if !exists {
			abortForbidden(c)
			return
		}
		if _, ok := permitted[role.(string)]; !ok {
			abortForbidden(c)
			return
		}
		c.Next()
	}
}

// RequestID injects a unique trace/request ID into every request.
// If the upstream gateway (Kong) already set X-Request-ID, that value is
// preserved; otherwise a new random ID is generated.
// Place this as the first middleware in the chain.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = newRequestID()
		}

		c.Set(ContextKeyRequestID, id)
		c.Header("X-Request-ID", id)

		ctx := logger.WithContext(c.Request.Context(), map[string]string{
			"request_id": id,
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// Recovery catches panics, logs the stack, and returns a structured 500 so
// clients always receive a valid JSON body even on unexpected crashes.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log := logger.FromContext(c.Request.Context())
				log.Error().
					Interface("panic", r).
					Str("path", c.FullPath()).
					Str("method", c.Request.Method).
					Msg("recovered from panic")

				ae := apierror.Internal("an unexpected server error occurred", nil)
				c.AbortWithStatusJSON(ae.HTTPStatus, gin.H{
					"error": gin.H{
						"code":    ae.Code,
						"message": ae.Message,
					},
				})
			}
		}()
		c.Next()
	}
}

// StructuredLogger emits one log line per request with method, path, status,
// latency, and the injected request_id.
func StructuredLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		if q := c.Request.URL.RawQuery; q != "" {
			path += "?" + q
		}

		c.Next()

		log := logger.FromContext(c.Request.Context())
		log.Info().
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", c.Writer.Status()).
			Dur("latency_ms", time.Since(start)).
			Int("bytes", c.Writer.Size()).
			Str("client_ip", c.ClientIP()).
			Msg("request")
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func abortUnauth(c *gin.Context, err error) {
	ae, ok := apierror.As(err)
	if !ok {
		ae = apierror.ErrTokenInvalid
	}
	c.AbortWithStatusJSON(ae.HTTPStatus, gin.H{
		"error": gin.H{"code": ae.Code, "message": ae.Message},
	})
}

func abortForbidden(c *gin.Context) {
	ae := apierror.Forbidden("insufficient permissions")
	c.AbortWithStatusJSON(ae.HTTPStatus, gin.H{
		"error": gin.H{"code": ae.Code, "message": ae.Message},
	})
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type rateLimitConfig struct {
	limit  int
	window time.Duration
}

const rateLimitOverrideKey = "rate_limit_override"

// RateLimit is the global Gin rate limit middleware applied at router level.
func RateLimit(rdb *goredis.Client, defaultLimit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := rateLimitConfig{limit: defaultLimit, window: window}

		// Check for route-level override set by StrictRateLimit or RelaxedRateLimit.
		if override, exists := c.Get(rateLimitOverrideKey); exists {
			cfg = override.(rateLimitConfig)
		}

		ip := c.ClientIP()
		key := fmt.Sprintf("rate_limit:%s:%s", ip, c.FullPath())

		allowed, remaining, err := allow(c.Request.Context(), rdb, key, cfg.limit, cfg.window)
		if err != nil {
			// Redis unavailable — fail open.
			c.Next()
			return
		}

		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if !allowed {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "TOO_MANY_REQUESTS",
					"message": "rate limit exceeded",
				},
			})
			return
		}
		c.Next()
	}
}

// StrictRateLimit overrides the global limit for a specific route.
// Place before your handler:
//
//	r.POST("/users/login", middleware.StrictRateLimit(10, time.Minute), handler.Login)
func StrictRateLimit(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(rateLimitOverrideKey, rateLimitConfig{limit: limit, window: window})
		c.Next()
	}
}

// allow is the Redis sliding window check — same Lua script as the gateway.
func allow(ctx context.Context, rdb *goredis.Client, key string, limit int, window time.Duration) (bool, int, error) {
	now := time.Now().UnixNano()
	script := goredis.NewScript(`
		local key        = KEYS[1]
		local now        = tonumber(ARGV[1])
		local window     = tonumber(ARGV[2])
		local limit      = tonumber(ARGV[3])
		local clearBefore = now - window

		redis.call("ZREMRANGEBYSCORE", key, "-inf", clearBefore)
		local count = redis.call("ZCARD", key)

		if count < limit then
			redis.call("ZADD", key, now, now)
			redis.call("PEXPIRE", key, math.ceil(window / 1000000))
			return {1, limit - count - 1}
		end
		return {0, 0}
	`)

	result, err := script.Run(ctx, rdb, []string{key},
		now, window.Nanoseconds(), limit,
	).Slice()
	if err != nil {
		return false, 0, err
	}

	return result[0].(int64) == 1, int(result[1].(int64)), nil
}
