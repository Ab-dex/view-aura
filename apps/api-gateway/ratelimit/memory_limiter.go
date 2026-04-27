package ratelimit

import "time"

type MemoryLimiter struct {
	limiters map[string]*tokenBucket
}

type tokenBucket struct {
	tokens     int
	lastRefill time.Time
}

func NewMemoryLimiter() *MemoryLimiter {
	return &MemoryLimiter{
		limiters: make(map[string]*tokenBucket),
	}
}

func (m *MemoryLimiter) Allow(key string) bool {
	b, ok := m.limiters[key]
	now := time.Now()

	if !ok {
		m.limiters[key] = &tokenBucket{
			tokens:     10,
			lastRefill: now,
		}
		return true
	}

	// simple refill logic
	if now.Sub(b.lastRefill) > time.Second {
		b.tokens = 10
		b.lastRefill = now
	}

	if b.tokens <= 0 {
		return false
	}

	b.tokens--
	return true
}
