package moderation_saga

import (
	"context"

	"go.temporal.io/sdk/activity"

	"github.com/Ab-dex/view-aura/internal/events"
)

// ─── Input / Output types ─────────────────────────────────────────────────────

type AIScreenInput struct {
	CaseID      string
	ContentType string
	ContentID   string
}

type AIScreeningResult struct {
	Signals        []string
	Confidence     map[string]float64
	Recommendation string // "approve" | "reject" | "escalate_to_human"
}

type DecisionInput struct {
	CaseID    string
	Decision  string
	DeciderID string
	Reason    string
}

type NotifyOwnerInput struct {
	ActorID     string
	ContentType string
	ContentID   string
	Decision    string
}

// ─── Activities ───────────────────────────────────────────────────────────────

// RecordCaseOpenedActivity marks the case as "in_workflow" in the moderation
// module so the admin dashboard shows it as actively being processed.
func RecordCaseOpenedActivity(ctx context.Context, input ModerationSagaInput) error {
	activity.GetLogger(ctx).Info("moderation_saga: recording case open",
		"case_id", input.CaseID,
	)
	// Real implementation: POST /internal/moderation/{case_id}/workflow_started
	return nil
}

// AIScreenContentActivity calls the Python moderation-ai HTTP service.
// Returns screening signals, confidence scores, and a recommendation.
func AIScreenContentActivity(ctx context.Context, input AIScreenInput) (AIScreeningResult, error) {
	activity.GetLogger(ctx).Info("moderation_saga: AI screening",
		"case_id", input.CaseID,
		"content_type", input.ContentType,
	)
	// Real implementation: POST to moderation-ai Python service
	// with {content_type, content_id}, poll for result with heartbeats.
	return AIScreeningResult{
		Signals:        []string{},
		Confidence:     map[string]float64{},
		Recommendation: "approve",
	}, nil
}

// AssignToHumanQueueActivity moves the case to the pending human review status
// and notifies moderators via the notification service.
func AssignToHumanQueueActivity(ctx context.Context, caseID string) error {
	activity.GetLogger(ctx).Info("moderation_saga: assigning to human queue",
		"case_id", caseID,
	)
	// Real implementation: PATCH /internal/moderation/{case_id}/assign
	return nil
}

// EscalateToSeniorActivity escalates an unresolved case to a senior moderator
// and publishes a high-priority notification.
func EscalateToSeniorActivity(ctx context.Context, caseID string) error {
	activity.GetLogger(ctx).Error("moderation_saga: escalating to senior moderator",
		"case_id", caseID,
	)
	// Real implementation: POST /internal/moderation/{case_id}/escalate
	return nil
}

// RecordDecisionActivity writes the final decision to the moderation_cases
// table and publishes a ModerationDecided event.
func RecordDecisionActivity(ctx context.Context, input DecisionInput, producer events.Producer) error {
	activity.GetLogger(ctx).Info("moderation_saga: recording decision",
		"case_id", input.CaseID,
		"decision", input.Decision,
	)
	// Real implementation: PATCH /internal/moderation/{case_id}/decide
	_ = producer.Publish(ctx, events.TopicModerationDecided, "moderation.decided", events.ModerationDecided{
		CaseID:     input.CaseID,
		Decision:   input.Decision,
		ReviewerID: input.DeciderID,
		Reason:     input.Reason,
	})
	return nil
}

// NotifyContentOwnerActivity sends an in-app notification to the content
// creator informing them of the moderation outcome.
func NotifyContentOwnerActivity(ctx context.Context, input NotifyOwnerInput) error {
	activity.GetLogger(ctx).Info("moderation_saga: notifying content owner",
		"actor_id", input.ActorID,
		"decision", input.Decision,
	)
	// Real implementation: POST /internal/notifications
	return nil
}

// RestoreContentActivity re-publishes approved content.
// For uploads: updates media_assets.status = "ready".
// For reviews: updates reviews.is_published = true.
func RestoreContentActivity(ctx context.Context, contentID, contentType string) error {
	activity.GetLogger(ctx).Info("moderation_saga: restoring content",
		"content_id", contentID,
		"content_type", contentType,
	)
	// Real implementation: call the appropriate module's internal endpoint.
	return nil
}

// RemoveContentActivity permanently removes rejected content.
// For uploads: deletes the R2 object and marks the asset as "rejected".
// For reviews: soft-deletes the review row.
func RemoveContentActivity(ctx context.Context, contentID, contentType string) error {
	activity.GetLogger(ctx).Info("moderation_saga: removing content",
		"content_id", contentID,
		"content_type", contentType,
	)
	// Real implementation: call the appropriate module's internal delete endpoint.
	return nil
}

// ─── Registration ─────────────────────────────────────────────────────────────

func Register(w interface {
	RegisterWorkflow(interface{})
	RegisterActivity(interface{})
}) {
	w.RegisterWorkflow(ModerationSagaWorkflow)
	w.RegisterActivity(RecordCaseOpenedActivity)
	w.RegisterActivity(AIScreenContentActivity)
	w.RegisterActivity(AssignToHumanQueueActivity)
	w.RegisterActivity(EscalateToSeniorActivity)
	w.RegisterActivity(RecordDecisionActivity)
	w.RegisterActivity(NotifyContentOwnerActivity)
	w.RegisterActivity(RestoreContentActivity)
	w.RegisterActivity(RemoveContentActivity)
}
