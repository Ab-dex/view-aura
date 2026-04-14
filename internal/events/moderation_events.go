package events

import "time"

// SubscriptionCanceled is published when a subscription is canceled or lapses.
type SubscriptionCanceled struct {
	UserID         string    `json:"user_id"`
	SubscriptionID string    `json:"subscription_id"`
	EffectiveAt    time.Time `json:"effective_at"` // end of billing period
	Reason         string    `json:"reason"`
	OccurredAt     time.Time `json:"occurred_at"`
}

// ─── Moderation events ────────────────────────────────────────────────────────

// ModerationSubmitted is published when new content needs review.
// cmd/worker's moderation_saga workflow consumes this.
type ModerationSubmitted struct {
	CaseID      string `json:"case_id"`
	ContentType string `json:"content_type"` // "review" | "upload" | "post" | "profile"
	ContentID   string `json:"content_id"`
	ActorID     string `json:"actor_id"`
	// AISignals carries pre-computed scores from earlier automated checks.
	AISignals  map[string]float64 `json:"ai_signals,omitempty"`
	OccurredAt time.Time          `json:"occurred_at"`
}

// ModerationDecided is published after a human or AI decision is recorded.
// upload_pipeline workflow listens to this to proceed with publishing.
type ModerationDecided struct {
	CaseID      string    `json:"case_id"`
	ContentType string    `json:"content_type"`
	ContentID   string    `json:"content_id"`
	ActorID     string    `json:"actor_id"`
	Decision    string    `json:"decision"` // "approved" | "rejected" | "escalated"
	ReviewerID  string    `json:"reviewer_id,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// ModerationAppealDecided is published when a senior moderator resolves an appeal.
// Consumed by: notification-service (inform the appellant),
// review/upload-service (restore or permanently remove content).
type ModerationAppealDecided struct {
	AppealID    string `json:"appeal_id"`
	CaseID      string `json:"case_id"`
	ContentID   string `json:"content_id"`
	AppellantID string `json:"appellant_id"`
	// Outcome: "upheld" (original decision stands) | "overturned" (content restored)
	Outcome   string    `json:"outcome"`
	DecidedBy string    `json:"decided_by"` // senior moderator user_id
	DecidedAt time.Time `json:"decided_at"`
}
