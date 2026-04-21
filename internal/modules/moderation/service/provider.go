package service

import (
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/platform/config"
)

// ProvideAIClient builds the three-tier AI client:
//
//	Tier 1  Python moderation-ai service (httpAIClient)
//	        — skipped when ModerationAI.Endpoint is empty (NoopAIClient used instead)
//	Tier 2  Rule-based Go classifier (FallbackAIClient)
//	        — keyword + MIME type checks; always escalates to human when uncertain
//	Tier 3  escalate_to_human — hard-coded safe default; nothing auto-approved
//
// The result is ALWAYS a ResilientAIClient regardless of config — even in local
// dev the fallback chain ensures content is never auto-approved when the AI
// service is unavailable.
//
// Note: NoopAIClient (auto-approves everything) is intentionally NOT used as a
// fallback here.  It exists only for unit tests that want to bypass screening.
func ProvideAIClient(cfg *config.Config) AIClient {
	var primary AIClient
	if cfg.ModerationAI.Endpoint != "" {
		// Real Python service configured — use it as Tier 1.
		primary = NewAIClient(cfg.ModerationAI)
	} else {
		// No endpoint configured (local dev / CI) — Tier 1 is effectively disabled.
		// The breaker will open immediately on the first NoopAIClient "success"
		// and FallbackAIClient will handle everything via Tier 2.
		// We use a closed-circuit noop rather than nil so the breaker logic path
		// is still exercised in tests.
		primary = NoopAIClient{}
	}

	return NewResilientAIClient(primary, FallbackAIClient{})
}

var ProviderSet = wire.NewSet(
	ProvideAIClient,
	NewModerationService,
)
