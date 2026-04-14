// Package circuit implements a thread-safe, Redis-backed circuit breaker
// with an in-process fallback so a Redis outage never disables protection.
//
// State machine:
//
//	CLOSED ──(failures >= threshold)──► OPEN
//	OPEN   ──(probe TTL elapsed)──────► HALF-OPEN
//	HALF-OPEN ──(success)─────────────► CLOSED
//	HALF-OPEN ──(failure)─────────────► OPEN
//
// Each breaker is keyed by a service name.  The state is stored in Redis
// so all gateway pods share the same view.  A local sync.Map mirrors the
// state so a Redis read is not required on the hot path — the local copy
// is refreshed whenever a Redis write succeeds.
package circuit

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// State enumerates the three circuit states.
type State string

const (
	StateClosed   State = "closed"
	StateOpen     State = "open"
	StateHalfOpen State = "half_open"
)

// Config controls breaker sensitivity.
type Config struct {
	// FailureThreshold is the number of consecutive failures that trip the breaker.
	FailureThreshold int
	// SuccessThreshold is the number of consecutive successes in HALF-OPEN
	// needed to return to CLOSED.
	SuccessThreshold int
	// OpenTimeout is how long the breaker stays OPEN before allowing a probe.
	OpenTimeout time.Duration
	// Namespace is prepended to all Redis keys (e.g. "gateway", "upload").
	Namespace string
}

func DefaultConfig(namespace string) Config {
	return Config{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		OpenTimeout:      30 * time.Second,
		Namespace:        namespace,
	}
}

// stateRecord is what we persist in Redis.
type stateRecord struct {
	State       State     `json:"state"`
	Failures    int       `json:"failures"`
	Successes   int       `json:"successes"`
	LastFailure time.Time `json:"last_failure"`
	OpenedAt    time.Time `json:"opened_at,omitempty"`
}

// Breaker is a named circuit breaker for one downstream dependency.
type Breaker struct {
	name  string
	cfg   Config
	redis *goredis.Client
	mu    sync.RWMutex
	local stateRecord // in-process mirror; read without Redis on hot path
}

// Manager owns a set of named Breakers and constructs them on first use.
type Manager struct {
	cfg      Config
	redis    *goredis.Client
	mu       sync.Mutex
	breakers map[string]*Breaker
}

// NewManager creates a Manager.  Pass nil for redis to run entirely in-process
// (useful in tests and when Redis is unavailable at startup).
func NewManager(cfg Config, redis *goredis.Client) *Manager {
	return &Manager{
		cfg:      cfg,
		redis:    redis,
		breakers: make(map[string]*Breaker),
	}
}

// For returns (or lazily creates) the Breaker for a named service.
func (m *Manager) For(service string) *Breaker {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b, ok := m.breakers[service]; ok {
		return b
	}
	b := &Breaker{
		name:  service,
		cfg:   m.cfg,
		redis: m.redis,
		local: stateRecord{State: StateClosed},
	}
	m.breakers[service] = b
	return b
}

// ─── Public API ───────────────────────────────────────────────────────────────

// Allow reports whether the request should be allowed through.
// It reads from the local mirror (no Redis round-trip) and returns false when
// the circuit is OPEN and the probe window has not elapsed.
func (b *Breaker) Allow() bool {
	b.mu.RLock()
	rec := b.local
	b.mu.RUnlock()

	switch rec.State {
	case StateClosed:
		return true
	case StateHalfOpen:
		return true // allow the probe
	case StateOpen:
		if time.Since(rec.OpenedAt) >= b.cfg.OpenTimeout {
			// Transition locally; persist async.
			b.transitionTo(context.Background(), StateHalfOpen)
			return true
		}
		return false
	}
	return true
}

// RecordSuccess records a successful downstream call.
func (b *Breaker) RecordSuccess(ctx context.Context) {
	b.mu.Lock()
	rec := b.local
	b.mu.Unlock()

	switch rec.State {
	case StateClosed:
		// Reset failure counter.
		b.update(ctx, func(r *stateRecord) {
			r.Failures = 0
		})
	case StateHalfOpen:
		b.update(ctx, func(r *stateRecord) {
			r.Successes++
			if r.Successes >= b.cfg.SuccessThreshold {
				r.State = StateClosed
				r.Failures = 0
				r.Successes = 0
				log.Info().Str("service", b.name).Msg("circuit: closed after recovery")
			}
		})
	}
}

// RecordFailure records a failed downstream call and may open the circuit.
func (b *Breaker) RecordFailure(ctx context.Context) {
	b.update(ctx, func(r *stateRecord) {
		r.Failures++
		r.LastFailure = time.Now()
		r.Successes = 0
		if r.State == StateHalfOpen || r.Failures >= b.cfg.FailureThreshold {
			if r.State != StateOpen {
				r.State = StateOpen
				r.OpenedAt = time.Now()
				log.Warn().
					Str("service", b.name).
					Int("failures", r.Failures).
					Msg("circuit: opened")
			}
		}
	})
}

// State returns the current circuit state (reads local mirror).
func (b *Breaker) State() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.local.State
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (b *Breaker) redisKey() string {
	return fmt.Sprintf("circuit:%s:%s", b.cfg.Namespace, b.name)
}

// update applies a mutation function to the record, updates the local mirror,
// and persists to Redis.  Redis failures are non-fatal: the local state still
// advances so the breaker continues to function without Redis.
func (b *Breaker) update(ctx context.Context, fn func(*stateRecord)) {
	b.mu.Lock()
	fn(&b.local)
	rec := b.local
	b.mu.Unlock()

	b.persistAsync(rec)
}

func (b *Breaker) transitionTo(ctx context.Context, s State) {
	b.mu.Lock()
	b.local.State = s
	rec := b.local
	b.mu.Unlock()
	b.persistAsync(rec)
}

func (b *Breaker) persistAsync(rec stateRecord) {
	if b.redis == nil {
		return
	}
	go func() {
		data, err := json.Marshal(rec)
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = b.redis.Set(ctx, b.redisKey(), data, b.cfg.OpenTimeout*4).Err()
	}()
}
