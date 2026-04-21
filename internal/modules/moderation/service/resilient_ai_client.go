package service

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/Ab-dex/view-aura/internal/platform/resilience"
)

// ResilientAIClient wraps the Python AI service with three-tier degradation:
//
//	Tier 1  httpAIClient  — full ML screening (Python service)
//	Tier 2  FallbackAIClient — rule-based Go classifier (runs in-process)
//	Tier 3  escalate_to_human — nothing approved/rejected blindly
//
// The circuit breaker is in-process only; it does not require Redis.
// Any state is lost on pod restart which is acceptable — the breaker re-closes
// after a probe succeeds on the next request.
type ResilientAIClient struct {
	primary  AIClient
	fallback AIClient
	breaker  *resilience.InProcessBreaker
}

// NewResilientAIClient constructs the wrapper.
//
//   - primary:  httpAIClient (or NoopAIClient in tests)
//   - fallback: FallbackAIClient{} — always pass the rule-based implementation
func NewResilientAIClient(primary, fallback AIClient) AIClient {
	return &ResilientAIClient{
		primary:  primary,
		fallback: fallback,
		// Open after 3 consecutive failures; re-probe after 60s.
		// Python GPU services are slower to recover than HTTP services.
		breaker: resilience.NewInProcessBreaker("moderation-ai", 3, 60*time.Second),
	}
}

func (c *ResilientAIClient) Screen(ctx context.Context, contentType, contentID string) (*AIScreeningResult, error) {
	// ── Tier 1: Python AI service ─────────────────────────────────────────────
	if c.breaker.Allow() {
		result, err := c.primary.Screen(ctx, contentType, contentID)
		if err == nil {
			c.breaker.RecordSuccess()
			return result, nil
		}
		c.breaker.RecordFailure()
		log.Warn().
			Err(err).
			Str("content_type", contentType).
			Str("content_id", contentID).
			Msg("resilient_ai_client: python service failed — falling back to rule-based classifier")
	} else {
		log.Warn().
			Str("content_type", contentType).
			Msg("resilient_ai_client: python service circuit open — using rule-based fallback")
	}

	// ── Tier 2: Go rule-based classifier ─────────────────────────────────────
	result, err := c.fallback.Screen(ctx, contentType, contentID)
	if err == nil {
		log.Info().
			Str("content_type", contentType).
			Str("recommendation", result.Recommendation).
			Msg("resilient_ai_client: rule-based fallback served")
		return result, nil
	}

	// ── Tier 3: safe default — never silently approve ─────────────────────────
	log.Error().
		Err(err).
		Str("content_type", contentType).
		Msg("resilient_ai_client: all tiers failed — returning escalate_to_human")

	return &AIScreeningResult{
		Signals:        []string{"screening_unavailable"},
		Confidence:     map[string]float64{"screening_unavailable": 1.0},
		Recommendation: "escalate_to_human",
	}, nil
}
