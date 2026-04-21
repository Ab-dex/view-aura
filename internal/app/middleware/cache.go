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
	private bool
}

const httpCacheOverrideKey = "http_cache_config"

// HTTPCache is the global response cache middleware.
//
// When redis is nil (not configured) the middleware is a transparent passthrough —
// every request is a cache miss and responses are never stored.
// This allows the app to serve correctly without Redis; just without the
// performance benefit of cached responses.
func HTTPCache(redis *cache.Client, defaultTTL time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only cache GET requests.
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		// Without Redis: always miss, never store.
		if redis == nil {
			c.Header("X-Cache", "BYPASS")
			c.Next()
			return
		}

		cfg := cacheConfig{ttl: defaultTTL}
		if override, exists := c.Get(httpCacheOverrideKey); exists {
			cfg = override.(cacheConfig)
		}

		// Never cache authenticated responses in shared Redis.
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

		// Cache miss — capture the response.
		rw := &responseCapturer{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = rw
		c.Next()

		// Only persist successful public responses.
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
func CacheFor(ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(httpCacheOverrideKey, cacheConfig{ttl: ttl})
		c.Next()
	}
}

// NoCache disables caching and sets Cache-Control: no-store.
func NoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(httpCacheOverrideKey, cacheConfig{ttl: 0, private: true})
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

// InvalidateCache deletes cached responses matching a key prefix.
// Safe to call when redis is nil — no-ops silently.
func InvalidateCache(redis *cache.Client, patterns ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if redis == nil || c.Writer.Status() >= 400 {
			return
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
	raw := c.Request.URL.RequestURI()
	hash := md5.Sum([]byte(raw))
	return fmt.Sprintf("http:GET:%x", hash)
}

// normalizeURL strips sensitive params from cache keys.
func normalizeURL(raw string) string {
	for _, p := range []string{"token", "access_token", "api_key"} {
		raw = strings.ReplaceAll(raw, p+"=", "_=")
	}
	return raw
}
