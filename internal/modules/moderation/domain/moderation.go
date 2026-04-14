package domain

import "time"

// ContentType identifies what kind of content is under review.
type ContentType string

const (
	ContentTypeReview  ContentType = "review"
	ContentTypeUpload  ContentType = "upload"
	ContentTypePost    ContentType = "post"
	ContentTypeProfile ContentType = "profile"
)

// Decision is the outcome recorded on a case.
type Decision string

const (
	DecisionApproved  Decision = "approved"
	DecisionRejected  Decision = "rejected"
	DecisionEscalated Decision = "escalated"
)

// CaseStatus tracks the lifecycle of a moderation case.
type CaseStatus string

const (
	StatusPending    CaseStatus = "pending"     // awaiting AI screening
	StatusAIReviewed CaseStatus = "ai_reviewed" // AI has made a recommendation
	StatusInReview   CaseStatus = "in_review"   // assigned to a human moderator
	StatusDecided    CaseStatus = "decided"     // final decision recorded
	StatusAppealed   CaseStatus = "appealed"    // under appeal review
)

// ModerationCase is the central entity — one case per flagged piece of content.
type ModerationCase struct {
	ID            string
	ContentType   ContentType
	ContentID     string
	OwnerID       string // user_id who created the content
	SubmittedBy   string // "system" | user_id of reporter
	TriggerReason string // "auto_screen" | "user_report" | "admin_flag" | "ai_flag"
	Status        CaseStatus
	// AISignals is populated after the Python AI service screens the content.
	AISignals        []string           // e.g. ["nsfw", "hate_speech"]
	AIConfidence     map[string]float64 // signal → confidence score
	AIRecommendation string             // "approve" | "reject" | "escalate_to_human"
	// Decision fields — set when a human moderator acts.
	Decision  *Decision
	DecidedBy string // moderator user_id or "ai_auto"
	Reason    string // moderator note or AI classification label
	DecidedAt *time.Time
	// Lock prevents two moderators from acting on the same case simultaneously.
	LockedBy    string
	LockedUntil *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Appeal is submitted by content owners contesting a rejection.
type Appeal struct {
	ID          string
	CaseID      string
	AppellantID string
	Statement   string
	Outcome     string // "upheld" | "overturned" — set after senior review
	DecidedBy   string
	DecidedAt   *time.Time
	CreatedAt   time.Time
}

// ─── Commands ─────────────────────────────────────────────────────────────────

type SubmitCaseCmd struct {
	ContentType   ContentType
	ContentID     string
	OwnerID       string
	SubmittedBy   string
	TriggerReason string
}

type RecordAIScreeningCmd struct {
	CaseID         string
	Signals        []string
	Confidence     map[string]float64
	Recommendation string
}

type DecideCmd struct {
	CaseID      string
	ModeratorID string
	Decision    Decision
	Reason      string
}

type SubmitAppealCmd struct {
	CaseID      string
	AppellantID string
	Statement   string
}

type DecideAppealCmd struct {
	AppealID    string
	ModeratorID string
	Outcome     string // "upheld" | "overturned"
}

// ─── Filters ─────────────────────────────────────────────────────────────────

type CaseFilter struct {
	Status      *CaseStatus
	ContentType *ContentType
	Limit       int
	Offset      int
}
