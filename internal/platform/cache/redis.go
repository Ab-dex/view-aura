package cache

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/Ab-dex/view-aura/internal/platform/config"
)

// Client wraps the go-redis client with project-specific helpers.
type Client struct {
	*goredis.Client
}

// New creates a Redis client and verifies connectivity.
func New(ctx context.Context, cfg config.RedisConfig) (*Client, error) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,

		// Retry transient errors (e.g. LOADING) up to 3 times.
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("redis: ping failed: %w", err)
	}

	log.Info().Str("addr", cfg.Addr).Msg("redis: connected")
	return &Client{Client: rdb}, nil
}

// Close shuts down the connection pool gracefully.
func (c *Client) Close() {
	if err := c.Client.Close(); err != nil {
		log.Error().Err(err).Msg("redis: close error")
	}
	log.Info().Msg("redis: connection closed")
}

// Centralising key construction prevents typos and makes TTL audits easy.

func SessionKey(token string) string        { return "session:" + token }
func UserSessionsKey(userID string) string  { return "user:" + userID + ":sessions" }
func BlocklistJTIKey(jti string) string     { return "blocklist:jti:" + jti }
func UserEmbeddingKey(userID string) string { return "user:" + userID + ":embedding" }
func RateLimitKey(ip, route string) string  { return "rate_limit:" + ip + ":" + route }
func NotifRateKey(userID string) string     { return "notif:rate:" + userID }
