package resilience

import (
	"sync"
	"time"
)

// InProcessBreaker is a minimal thread-safe circuit breaker used inside the
// resilient wrappers.  It is intentionally simpler than platform/circuit.Breaker
// because it does not need Redis persistence — each wrapper manages its own
// local state.  The platform/circuit.Manager is for the API gateway layer;
// this is for service-level fallback decisions inside the monolith.
//
// Exported so module packages (moderation, notification, payment, upload) can
// construct breakers for their own resilient wrappers.
//
// States: closed → open (after FailureThreshold failures) → closed (after OpenFor elapses)
type InProcessBreaker struct {
	name             string
	failureThreshold int
	openFor          time.Duration

	mu       sync.RWMutex
	failures int
	openedAt time.Time
	open     bool
}

// NewInProcessBreaker constructs a breaker.
//   - name:             label used in log messages
//   - failureThreshold: consecutive failures before opening
//   - openFor:          how long the circuit stays open before a probe is allowed
func NewInProcessBreaker(name string, failureThreshold int, openFor time.Duration) *InProcessBreaker {
	return &InProcessBreaker{
		name:             name,
		failureThreshold: failureThreshold,
		openFor:          openFor,
	}
}

// newInProcessBreaker is the package-private alias used by producer.go / mailer.go
// which live in the same package and don't need the exported constructor.
func newInProcessBreaker(name string, failureThreshold int, openFor time.Duration) *InProcessBreaker {
	return NewInProcessBreaker(name, failureThreshold, openFor)
}

// Allow returns true when the breaker is closed or the open window has elapsed
// (transitions to half-open — the next call is a probe).
func (b *InProcessBreaker) Allow() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if !b.open {
		return true
	}
	return time.Since(b.openedAt) >= b.openFor
}

// RecordSuccess resets the failure counter and closes the breaker.
func (b *InProcessBreaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.open = false
}

// RecordFailure increments the failure counter and opens the breaker when the
// threshold is exceeded.
func (b *InProcessBreaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if !b.open && b.failures >= b.failureThreshold {
		b.open = true
		b.openedAt = time.Now()
	}
}

// IsOpen returns whether the circuit is currently open.
func (b *InProcessBreaker) IsOpen() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.open
}
