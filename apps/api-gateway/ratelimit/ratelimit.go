// Package ratelimit implements a Redis-backed sliding window rate limiter.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Limiter implements a sliding window rate limiter backed by Redis.
// Each key maps to a sorted set where members are request timestamps.
type Limiter struct {
	client *goredis.Client
}

// New connects to Redis and returns a Limiter.
func New(addr, password string) (*Limiter, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:         addr,
		Password:     password,
		DB:           1, // separate DB from application cache
		DialTimeout:  3 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		MaxRetries:   2,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ratelimit: ping redis: %w", err)
	}
	return &Limiter{client: client}, nil
}

// Allow checks and records a request for the given key.
// Returns (allowed, remaining, error).
// limit is the maximum number of requests in the window.
// window is the duration of the sliding window.
//
// Uses a Lua script for atomicity — the check and increment are a single
// round trip to Redis, preventing race conditions under high concurrency.
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error) {
	now := time.Now().UnixNano()
	windowNs := window.Nanoseconds()

	// Lua script: atomic sliding-window check.
	// 1. Remove members older than (now - window).
	// 2. Count remaining members.
	// 3. If count < limit: add this request and set TTL; return (1, count).
	// 4. Otherwise: return (0, 0).
	script := goredis.NewScript(`
		local key      = KEYS[1]
		local now      = tonumber(ARGV[1])
		local window   = tonumber(ARGV[2])
		local limit    = tonumber(ARGV[3])
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

	result, err := script.Run(ctx, l.client,
		[]string{key},
		now, windowNs, limit,
	).Slice()
	if err != nil {
		return false, 0, fmt.Errorf("ratelimit: lua script: %w", err)
	}

	allowed := result[0].(int64) == 1
	remaining := int(result[1].(int64))
	return allowed, remaining, nil
}
