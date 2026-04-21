package service

import (
	"context"
	"strings"
)

// FallbackAIClient is a rule-based content classifier that runs entirely in Go.
// It is used as Tier 2 when the Python AI service (httpAIClient) is unavailable.
//
// It is intentionally conservative:
//   - Hard-coded signal lists catch only obvious violations.
//   - Anything borderline is escalated to a human rather than auto-approved.
//   - It never auto-rejects without a high-confidence keyword hit to avoid
//     false positives when the real AI is down.
//
// This allows the moderation pipeline to keep functioning at reduced accuracy
// without the Python service, while keeping the safety posture intact.
type FallbackAIClient struct{}

// fallbackBlockedKeywords are words/phrases that, if found in text content,
// trigger an immediate "escalate_to_human" recommendation.
// Keep this list short and unambiguous — the goal is catching obvious cases,
// not replacing the ML model.
var fallbackBlockedKeywords = []string{
	// Hate speech signals
	"kill all", "death to", "genocide",
	// Extreme NSFW signals — text only, no image analysis possible in fallback
	"child porn", "cp ", "csam",
	// Spam signals
	"buy now at", "click here to win", "you have been selected",
}

// fallbackEscalateContentTypes are MIME types that always require human review
// when the AI service is unavailable because we cannot analyse them in Go.
var fallbackEscalateContentTypes = []string{
	"video/", // video needs frame analysis
	"image/", // images need CV model
}

func (FallbackAIClient) Screen(ctx context.Context, contentType, contentID string) (*AIScreeningResult, error) {
	// If this is a non-text content type (video, image), we cannot analyse it
	// in Go — escalate to human so nothing bypasses review.
	for _, prefix := range fallbackEscalateContentTypes {
		if strings.HasPrefix(contentType, prefix) {
			return &AIScreeningResult{
				Signals:        []string{"ai_unavailable"},
				Confidence:     map[string]float64{"ai_unavailable": 1.0},
				Recommendation: "escalate_to_human",
			}, nil
		}
	}

	// For text-based content types (review, post, comment), run keyword checks.
	// The contentID here is the text body when pre-fetched by the caller;
	// in practice the moderation service re-fetches from DB.
	lowerID := strings.ToLower(contentID)
	for _, kw := range fallbackBlockedKeywords {
		if strings.Contains(lowerID, kw) {
			return &AIScreeningResult{
				Signals:        []string{"keyword_match"},
				Confidence:     map[string]float64{"keyword_match": 0.9},
				Recommendation: "escalate_to_human",
			}, nil
		}
	}

	// Default: escalate everything when AI is down.
	// We never auto-approve without the real model.
	return &AIScreeningResult{
		Signals:        []string{"ai_unavailable"},
		Confidence:     map[string]float64{"ai_unavailable": 1.0},
		Recommendation: "escalate_to_human",
	}, nil
}
