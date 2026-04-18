package middleware

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/platform/cache"
)

type cacheConfig struct {
	ttl     time.Duration
	private bool // true = Cache-Control: private (authenticated responses)
}

const httpCacheOverrideKey = "http_cache_config"

// HTTPCache is the global response cache middleware.
// Only caches GET requests that complete with 200 OK.
// Authenticated requests (with Authorization header) are never cached.
func HTTPCache(redis *cache.Client, defaultTTL time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only cache GET requests.
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		cfg := cacheConfig{ttl: defaultTTL}
		if override, exists := c.Get(httpCacheOverrideKey); exists {
			cfg = override.(cacheConfig)
		}

		// Never cache private/authenticated responses in Redis.
		isAuthed := c.GetHeader("Authorization") != ""
		if isAuthed || cfg.private {
			setCacheControlPrivate(c)
			c.Next()
			return
		}

		key := httpCacheKey(c)

		// Cache hit.
		if cached, err := redis.Get(c.Request.Context(), key).Bytes(); err == nil {
			var entry cachedResponse
			if json.Unmarshal(cached, &entry) == nil {
				c.Header("X-Cache", "HIT")
				setCacheControlPublic(c, cfg.ttl)
				for k, v := range entry.Headers {
					c.Header(k, v)
				}
				c.Data(entry.Status, entry.ContentType, entry.Body)
				c.Abort()
				return
			}
		}

		// Cache miss — capture response.
		rw := &responseCapturer{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = rw
		c.Next()

		// Only cache successful public responses.
		if rw.status == http.StatusOK {
			entry := cachedResponse{
				Status:      rw.status,
				ContentType: rw.Header().Get("Content-Type"),
				Body:        rw.body.Bytes(),
				Headers: map[string]string{
					"Content-Type": rw.Header().Get("Content-Type"),
				},
			}
			if b, err := json.Marshal(entry); err == nil {
				_ = redis.Set(c.Request.Context(), key, b, cfg.ttl).Err()
			}
		}

		c.Header("X-Cache", "MISS")
		setCacheControlPublic(c, cfg.ttl)
	}
}

// CacheFor overrides the TTL for a specific route.
//
//	r.GET("/movies", middleware.CacheFor(5*time.Minute), handler.List)
func CacheFor(ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(httpCacheOverrideKey, cacheConfig{ttl: ttl})
		c.Next()
	}
}

// NoCache disables caching and sets Cache-Control: no-store for a route.
//
//	r.GET("/users/me", middleware.NoCache(), handler.GetMe)
func NoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(httpCacheOverrideKey, cacheConfig{ttl: 0, private: true})
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

// InvalidateCache deletes cached responses matching a key prefix.
// Use in mutation handlers to bust stale list caches.
//
//	middleware.InvalidateCache(redis, "http:GET:/api/v1/movies*")
func InvalidateCache(redis *cache.Client, patterns ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next() // run handler first

		if c.Writer.Status() >= 400 {
			return // don't invalidate on error
		}

		ctx := c.Request.Context()
		for _, pattern := range patterns {
			keys, err := redis.Keys(ctx, pattern).Result()
			if err != nil || len(keys) == 0 {
				continue
			}
			_ = redis.Del(ctx, keys...).Err()
		}
	}
}

// ─── Cache-Control helpers ────────────────────────────────────────────────────

func setCacheControlPublic(c *gin.Context, ttl time.Duration) {
	maxAge := int(ttl.Seconds())
	c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d, stale-while-revalidate=%d", maxAge, maxAge/2))
	c.Header("Vary", "Accept-Encoding, Accept-Language")
}

func setCacheControlPrivate(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
}

// ─── Internal types ───────────────────────────────────────────────────────────

type cachedResponse struct {
	Status      int               `json:"status"`
	ContentType string            `json:"content_type"`
	Body        []byte            `json:"body"`
	Headers     map[string]string `json:"headers"`
}

type responseCapturer struct {
	gin.ResponseWriter
	body   *bytes.Buffer
	status int
}

func (r *responseCapturer) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *responseCapturer) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func httpCacheKey(c *gin.Context) string {
	// Include query string in cache key so /movies?genre=action
	// is cached separately from /movies?genre=drama.
	raw := c.Request.URL.RequestURI()
	hash := md5.Sum([]byte(raw))
	return fmt.Sprintf("http:GET:%x", hash)
}

// normalizeURL strips sensitive params from cache keys.
func normalizeURL(raw string) string {
	// Remove auth-related query params if accidentally present.
	for _, p := range []string{"token", "access_token", "api_key"} {
		raw = strings.ReplaceAll(raw, p+"=", "_=")
	}
	return raw
}
