package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Ab-dex/view-aura/internal/events"
	"github.com/Ab-dex/view-aura/internal/modules/moderation/domain"
	"github.com/Ab-dex/view-aura/internal/modules/moderation/repository"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

type ModerationService interface {
	// ── Case lifecycle ────────────────────────────────────────────────────────
	SubmitCase(ctx context.Context, cmd domain.SubmitCaseCmd) (*domain.ModerationCase, error)
	GetCase(ctx context.Context, id string) (*domain.ModerationCase, error)
	ListPendingCases(ctx context.Context, filter domain.CaseFilter) ([]*domain.ModerationCase, int, error)
	RecordAIScreening(ctx context.Context, cmd domain.RecordAIScreeningCmd) (*domain.ModerationCase, error)
	Decide(ctx context.Context, cmd domain.DecideCmd) (*domain.ModerationCase, error)

	// ── Locking (prevents double-processing by two moderators) ────────────────
	AcquireLock(ctx context.Context, caseID, moderatorID string) (bool, error)
	ReleaseLock(ctx context.Context, caseID string) error

	// ── Appeals ───────────────────────────────────────────────────────────────
	SubmitAppeal(ctx context.Context, cmd domain.SubmitAppealCmd) (*domain.Appeal, error)
	GetAppeal(ctx context.Context, caseID string) (*domain.Appeal, error)
	DecideAppeal(ctx context.Context, cmd domain.DecideAppealCmd) (*domain.Appeal, error)

	// ── User-facing ───────────────────────────────────────────────────────────
	GetMyCases(ctx context.Context, ownerID string, limit, offset int) ([]*domain.ModerationCase, int, error)
}

type moderationService struct {
	cases   repository.CaseRepository
	appeals repository.AppealRepository
	locks   repository.LockRepository
	ai      AIClient
	pub     events.Producer
}

func NewModerationService(
	cases repository.CaseRepository,
	appeals repository.AppealRepository,
	locks repository.LockRepository,
	ai AIClient,
	pub events.Producer,
) ModerationService {
	return &moderationService{
		cases:   cases,
		appeals: appeals,
		locks:   locks,
		ai:      ai,
		pub:     pub,
	}
}

// ─── Case lifecycle ───────────────────────────────────────────────────────────

func (s *moderationService) SubmitCase(ctx context.Context, cmd domain.SubmitCaseCmd) (*domain.ModerationCase, error) {
	// Guard: prevent duplicate cases for the same content.
	existing, err := s.cases.GetByContentID(ctx, cmd.ContentType, cmd.ContentID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil // idempotent
	}

	c := &domain.ModerationCase{
		ID:            uuid.New().String(),
		ContentType:   cmd.ContentType,
		ContentID:     cmd.ContentID,
		OwnerID:       cmd.OwnerID,
		SubmittedBy:   cmd.SubmittedBy,
		TriggerReason: cmd.TriggerReason,
		Status:        domain.StatusPending,
	}
	created, err := s.cases.Create(ctx, c)
	if err != nil {
		return nil, err
	}

	// Publish event so the Python AI service picks it up.
	// ActorID carries the content owner — the party whose content is under review.
	_ = s.pub.Publish(ctx, events.TopicModeratonCreated, "moderation.submitted", events.ModerationSubmitted{
		CaseID:      created.ID,
		ContentType: string(cmd.ContentType),
		ContentID:   cmd.ContentID,
		ActorID:     cmd.OwnerID,
		OccurredAt:  created.CreatedAt,
	})

	logger.FromContext(ctx).Info().
		Str("case_id", created.ID).
		Str("content_type", string(cmd.ContentType)).
		Str("content_id", cmd.ContentID).
		Msg("moderation: case submitted")

	return created, nil
}

func (s *moderationService) GetCase(ctx context.Context, id string) (*domain.ModerationCase, error) {
	return s.cases.GetByID(ctx, id)
}

func (s *moderationService) ListPendingCases(ctx context.Context, filter domain.CaseFilter) ([]*domain.ModerationCase, int, error) {
	if filter.Status == nil {
		pending := domain.StatusPending
		filter.Status = &pending
	}
	return s.cases.List(ctx, filter)
}

func (s *moderationService) RecordAIScreening(ctx context.Context, cmd domain.RecordAIScreeningCmd) (*domain.ModerationCase, error) {
	c, err := s.cases.GetByID(ctx, cmd.CaseID)
	if err != nil {
		return nil, err
	}

	c.AISignals = cmd.Signals
	c.AIConfidence = cmd.Confidence
	c.AIRecommendation = cmd.Recommendation
	c.Status = domain.StatusAIReviewed

	// Auto-decide on high-confidence recommendations.
	if cmd.Recommendation == "approve" && s.maxConfidence(cmd.Confidence) < 0.3 {
		dec := domain.DecisionApproved
		c.Decision = &dec
		c.DecidedBy = "ai_auto"
		c.Status = domain.StatusDecided
		now := time.Now()
		c.DecidedAt = &now
	} else if cmd.Recommendation == "reject" && s.maxConfidence(cmd.Confidence) > 0.95 {
		dec := domain.DecisionRejected
		c.Decision = &dec
		c.DecidedBy = "ai_auto"
		c.Status = domain.StatusDecided
		now := time.Now()
		c.DecidedAt = &now
	}

	updated, err := s.cases.Update(ctx, c)
	if err != nil {
		return nil, err
	}

	if updated.Status == domain.StatusDecided {
		s.publishDecision(ctx, updated)
	}

	return updated, nil
}

func (s *moderationService) Decide(ctx context.Context, cmd domain.DecideCmd) (*domain.ModerationCase, error) {
	c, err := s.cases.GetByID(ctx, cmd.CaseID)
	if err != nil {
		return nil, err
	}
	if c.Status == domain.StatusDecided {
		return nil, apierror.ErrModerationAlreadyDecided
	}

	// Verify the lock is held by this moderator.
	holder, err := s.locks.IsLocked(ctx, cmd.CaseID)
	if err != nil {
		return nil, err
	}
	if holder != "" && holder != cmd.ModeratorID {
		return nil, apierror.ErrModerationLocked
	}

	now := time.Now()
	c.Decision = &cmd.Decision
	c.DecidedBy = cmd.ModeratorID
	c.Reason = cmd.Reason
	c.DecidedAt = &now
	c.Status = domain.StatusDecided
	c.LockedBy = ""
	c.LockedUntil = nil

	updated, err := s.cases.Update(ctx, c)
	if err != nil {
		return nil, err
	}

	_ = s.locks.Release(ctx, cmd.CaseID)
	s.publishDecision(ctx, updated)

	return updated, nil
}

// ─── Locking ──────────────────────────────────────────────────────────────────

func (s *moderationService) AcquireLock(ctx context.Context, caseID, moderatorID string) (bool, error) {
	acquired, err := s.locks.Acquire(ctx, caseID, moderatorID)
	if err != nil {
		return false, err
	}
	if acquired {
		// Update the case row so the lock is visible in the admin UI.
		c, err := s.cases.GetByID(ctx, caseID)
		if err != nil {
			return true, nil // lock acquired but case fetch failed — non-fatal
		}
		now := time.Now()
		expiry := now.Add(5 * time.Minute)
		c.LockedBy = moderatorID
		c.LockedUntil = &expiry
		c.Status = domain.StatusInReview
		_, _ = s.cases.Update(ctx, c)
	}
	return acquired, nil
}

func (s *moderationService) ReleaseLock(ctx context.Context, caseID string) error {
	return s.locks.Release(ctx, caseID)
}

// ─── Appeals ─────────────────────────────────────────────────────────────────

func (s *moderationService) SubmitAppeal(ctx context.Context, cmd domain.SubmitAppealCmd) (*domain.Appeal, error) {
	c, err := s.cases.GetByID(ctx, cmd.CaseID)
	if err != nil {
		return nil, err
	}
	if c.OwnerID != cmd.AppellantID {
		return nil, apierror.Forbidden("only the content owner can appeal")
	}
	if c.Status != domain.StatusDecided {
		return nil, apierror.Validation("can only appeal a decided case", nil)
	}

	appeal := &domain.Appeal{
		ID:          uuid.New().String(),
		CaseID:      cmd.CaseID,
		AppellantID: cmd.AppellantID,
		Statement:   cmd.Statement,
	}
	created, err := s.appeals.Create(ctx, appeal)
	if err != nil {
		return nil, err
	}

	c.Status = domain.StatusAppealed
	_, _ = s.cases.Update(ctx, c)

	return created, nil
}

func (s *moderationService) GetAppeal(ctx context.Context, caseID string) (*domain.Appeal, error) {
	return s.appeals.GetByCaseID(ctx, caseID)
}

func (s *moderationService) DecideAppeal(ctx context.Context, cmd domain.DecideAppealCmd) (*domain.Appeal, error) {
	appeal, err := s.appeals.GetByID(ctx, cmd.AppealID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	appeal.Outcome = cmd.Outcome
	appeal.DecidedBy = cmd.ModeratorID
	appeal.DecidedAt = &now

	updated, err := s.appeals.Update(ctx, appeal)
	if err != nil {
		return nil, err
	}

	// Publish appeal decision so affected services can restore/keep content.
	_ = s.pub.Publish(ctx, events.TopicModerationAppealDecided, "moderation.appeal_decided", events.ModerationAppealDecided{
		AppealID:    appeal.ID,
		CaseID:      appeal.CaseID,
		AppellantID: appeal.AppellantID,
		Outcome:     cmd.Outcome,
		DecidedBy:   cmd.ModeratorID,
		DecidedAt:   now,
	})

	return updated, nil
}

func (s *moderationService) GetMyCases(ctx context.Context, ownerID string, limit, offset int) ([]*domain.ModerationCase, int, error) {
	// List all statuses for a given owner — used on the user-facing "my content" page.
	return s.cases.List(ctx, domain.CaseFilter{Limit: limit, Offset: offset})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (s *moderationService) publishDecision(ctx context.Context, c *domain.ModerationCase) {
	dec := ""
	if c.Decision != nil {
		dec = string(*c.Decision)
	}
	_ = s.pub.Publish(ctx, events.TopicModerationDecided, "moderation.decided", events.ModerationDecided{
		CaseID:      c.ID,
		ContentType: string(c.ContentType),
		ContentID:   c.ContentID,
		ActorID:     c.OwnerID,
		Decision:    dec,
		ReviewerID:  c.DecidedBy,
		Reason:      c.Reason,
		OccurredAt:  time.Now(),
	})
}

func (s *moderationService) maxConfidence(m map[string]float64) float64 {
	max := 0.0
	for _, v := range m {
		if v > max {
			max = v
		}
	}
	return max
}
