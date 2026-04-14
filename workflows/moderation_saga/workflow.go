package moderation_saga

import (
	"time"

	temporalLogger "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ─── Input / Output ───────────────────────────────────────────────────────────

// ModerationSagaInput is the payload from the Kafka moderation topic.
// Must match events.ModerationSubmitted exactly.
type ModerationSagaInput struct {
	CaseID      string `json:"case_id"`
	ContentType string `json:"content_type"`
	ContentID   string `json:"content_id"`
	ActorID     string `json:"actor_id"`
}

type ModerationSagaOutput struct {
	CaseID    string
	Decision  string // "approved" | "rejected" | "escalated"
	DecidedBy string
}

// ─── Signals ──────────────────────────────────────────────────────────────────

// HumanDecisionSignal is sent by the moderation module API when a moderator
// approves or rejects a case. The workflow blocks on this signal after
// escalating to human review.
const HumanDecisionSignal = "human_decision"

type HumanDecision struct {
	Decision    string // "approved" | "rejected"
	ModeratorID string
	Reason      string
}

// ─── Workflow ─────────────────────────────────────────────────────────────────

// ModerationSagaWorkflow orchestrates the moderation lifecycle for a single
// content item. It coordinates AI screening, optional human review, and
// downstream effects (publish/quarantine/delete).
//
// Flow:
//
//	RecordCaseOpened
//	  → AIScreenContent
//	  → if auto_approve  → RecordDecision(approved) → NotifyOwner → ApplyDecision
//	  → if auto_reject   → RecordDecision(rejected) → NotifyOwner → ApplyDecision
//	  → if escalate      → AssignToHumanQueue → wait for HumanDecisionSignal (72h)
//	                         → RecordDecision → NotifyOwner → ApplyDecision
//	  → timeout (72h)   → EscalateToSenior → continue waiting
func ModerationSagaWorkflow(ctx workflow.Context, input ModerationSagaInput) (*ModerationSagaOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("moderation_saga: started",
		"case_id", input.CaseID,
		"content_type", input.ContentType,
	)

	actOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, actOpts)

	// ── Stage 1: Record case opened in the moderation module ─────────────────
	if err := workflow.ExecuteActivity(ctx, RecordCaseOpenedActivity, input).Get(ctx, nil); err != nil {
		logger.Error("moderation_saga: failed to record case", "error", err)
		return nil, err
	}

	// ── Stage 2: AI screening ────────────────────────────────────────────────
	var aiResult AIScreeningResult
	if err := workflow.ExecuteActivity(ctx, AIScreenContentActivity, AIScreenInput{
		CaseID:      input.CaseID,
		ContentType: input.ContentType,
		ContentID:   input.ContentID,
	}).Get(ctx, &aiResult); err != nil {
		// AI screening failure — escalate to human without signals.
		logger.Warn("moderation_saga: AI screening failed — escalating to human", "error", err)
		aiResult = AIScreeningResult{Recommendation: "escalate_to_human"}
	}

	// ── Stage 3: Auto-decision or human escalation ────────────────────────────
	switch aiResult.Recommendation {
	case "approve":
		return decide(ctx, input, "approved", "ai_auto", "", logger)

	case "reject":
		return decide(ctx, input, "rejected", "ai_auto", "", logger)

	default: // "escalate_to_human"
		logger.Info("moderation_saga: escalating to human review", "case_id", input.CaseID)

		// Assign to the human review queue.
		if err := workflow.ExecuteActivity(ctx, AssignToHumanQueueActivity, input.CaseID).Get(ctx, nil); err != nil {
			logger.Error("moderation_saga: failed to assign to queue", "error", err)
		}

		// Wait up to 72 hours for a human moderator to send the decision signal.
		// If no signal arrives, escalate to a senior moderator.
		return waitForHumanDecision(ctx, input, logger)
	}
}

// waitForHumanDecision blocks on the HumanDecisionSignal with a 72h timeout.
// On timeout it escalates to a senior moderator and waits another 72h before
// forcing an auto-escalate decision.
func waitForHumanDecision(ctx workflow.Context, input ModerationSagaInput, logger temporalLogger.Logger) (*ModerationSagaOutput, error) {
	signalCh := workflow.GetSignalChannel(ctx, HumanDecisionSignal)

	// First 72h window.
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	timer := workflow.NewTimer(timerCtx, 72*time.Hour)

	var decision HumanDecision
	sel := workflow.NewSelector(ctx)

	sel.AddReceive(signalCh, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, &decision)
		cancelTimer()
	})
	sel.AddFuture(timer, func(f workflow.Future) {
		// Timeout — escalate to senior moderator.
		logger.Warn("moderation_saga: 72h timeout — escalating to senior", "case_id", input.CaseID)
		_ = workflow.ExecuteActivity(
			workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 5 * time.Minute}),
			EscalateToSeniorActivity, input.CaseID,
		).Get(ctx, nil)
	})

	sel.Select(ctx)

	// If we got a signal, apply the decision.
	if decision.Decision != "" {
		return decide(ctx, input, decision.Decision, decision.ModeratorID, decision.Reason, logger)
	}

	// After senior escalation, wait another 72h for resolution.
	timerCtx2, cancelTimer2 := workflow.WithCancel(ctx)
	timer2 := workflow.NewTimer(timerCtx2, 72*time.Hour)
	sel2 := workflow.NewSelector(ctx)

	sel2.AddReceive(signalCh, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, &decision)
		cancelTimer2()
	})
	sel2.AddFuture(timer2, func(f workflow.Future) {
		logger.Error("moderation_saga: second timeout — auto-escalating", "case_id", input.CaseID)
		decision = HumanDecision{Decision: "escalated", ModeratorID: "system_timeout"}
	})

	sel2.Select(ctx)

	return decide(ctx, input, decision.Decision, decision.ModeratorID, decision.Reason, logger)
}

// decide records the final decision, notifies the content owner, and
// triggers the appropriate downstream effect (publish or remove content).
func decide(ctx workflow.Context, input ModerationSagaInput, decision, deciderID, reason string, logger temporalLogger.Logger) (*ModerationSagaOutput, error) {
	actOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	ctx = workflow.WithActivityOptions(ctx, actOpts)

	logger.Info("moderation_saga: recording decision",
		"case_id", input.CaseID,
		"decision", decision,
		"decided_by", deciderID,
	)

	// Record decision in the moderation module.
	if err := workflow.ExecuteActivity(ctx, RecordDecisionActivity, DecisionInput{
		CaseID:    input.CaseID,
		Decision:  decision,
		DeciderID: deciderID,
		Reason:    reason,
	}).Get(ctx, nil); err != nil {
		logger.Error("moderation_saga: failed to record decision", "error", err)
		return nil, err
	}

	// Notify content owner.
	notificationFuture := workflow.ExecuteActivity(ctx, NotifyContentOwnerActivity, NotifyOwnerInput{
		ActorID:     input.ActorID,
		ContentType: input.ContentType,
		ContentID:   input.ContentID,
		Decision:    decision,
	})

	// Apply downstream effect based on decision.
	switch decision {
	case "approved":
		_ = workflow.ExecuteActivity(ctx, RestoreContentActivity, input.ContentID, input.ContentType).Get(ctx, nil)
	case "rejected":
		_ = workflow.ExecuteActivity(ctx, RemoveContentActivity, input.ContentID, input.ContentType).Get(ctx, nil)
	}

	// 4. Handle the Notification result AFTER the critical work is done.
	// We check the error here, but we only LOG it. We don't fail the whole Saga.
	if err := notificationFuture.Get(ctx, nil); err != nil {
		logger.Error("moderation_saga: side-effect failed (notification), but core logic succeeded", "error", err)
	}

	return &ModerationSagaOutput{
		CaseID:    input.CaseID,
		Decision:  decision,
		DecidedBy: deciderID,
	}, nil
}
