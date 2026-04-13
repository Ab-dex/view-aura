package domain

import "time"

// ─── IDs ──────────────────────────────────────────────────────────────────────

type ReviewID string

func (id ReviewID) String() string { return string(id) }

type ReactionID string

// ─── Enumerations ─────────────────────────────────────────────────────────────

type ReviewType string

const (
	ReviewTypeShort    ReviewType = "short"     // tweet-length
	ReviewTypeLongForm ReviewType = "long_form" // essay
	ReviewTypeSpoiler  ReviewType = "spoiler"   // hidden by default
	ReviewTypeVideo    ReviewType = "video"
)

// ReviewReactionType maps to the PRD reaction options on a review.
type ReviewReactionType string

const (
	ReviewReactionLike       ReviewReactionType = "like"
	ReviewReactionHelpful    ReviewReactionType = "helpful"
	ReviewReactionInsightful ReviewReactionType = "insightful"
	ReviewReactionFunny      ReviewReactionType = "funny"
)

// ReportReason is used when a user reports a review for abuse.
type ReportReason string

const (
	ReportReasonSpam       ReportReason = "spam"
	ReportReasonHateSpeech ReportReason = "hate_speech"
	ReportReasonSpoiler    ReportReason = "unmarked_spoiler"
	ReportReasonOther      ReportReason = "other"
)

// ─── Core entities ────────────────────────────────────────────────────────────

// Review is a user-written critique of a movie.
type Review struct {
	ID          ReviewID
	MovieID     string
	UserID      string
	Type        ReviewType
	Body        string // text content; empty for video reviews
	VideoURL    string // filled only for ReviewTypeVideo
	IsSpoiler   bool
	IsPublished bool
	// Aggregated counters — incremented by the reaction write path, never
	// computed by COUNT(*) on the hot read path.
	LikeCount       int
	HelpfulCount    int
	InsightfulCount int
	FunnyCount      int
	// CredibilityScore is computed asynchronously from helpful votes + genre
	// expertise + match with critic consensus.
	CredibilityScore float64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ReviewReaction records a user's reaction to a specific review.
// A user may react exactly once per (user_id, review_id) pair,
// but can change their reaction type by upserting.
type ReviewReaction struct {
	ID        ReactionID
	ReviewID  ReviewID
	UserID    string
	Type      ReviewReactionType
	CreatedAt time.Time
}

// ReviewReport records an abuse report submitted by a user.
type ReviewReport struct {
	ID        string
	ReviewID  ReviewID
	UserID    string
	Reason    ReportReason
	Note      string
	CreatedAt time.Time
}

// ─── Commands ─────────────────────────────────────────────────────────────────

type CreateReviewCmd struct {
	MovieID   string
	UserID    string
	Type      ReviewType
	Body      string
	VideoURL  string
	IsSpoiler bool
}

type UpdateReviewCmd struct {
	ID        ReviewID
	UserID    string // ownership guard
	Body      *string
	VideoURL  *string
	IsSpoiler *bool
	Publish   *bool
}

type ReactToReviewCmd struct {
	ReviewID ReviewID
	UserID   string
	Type     ReviewReactionType
}

type ReportReviewCmd struct {
	ReviewID ReviewID
	UserID   string
	Reason   ReportReason
	Note     string
}

type ReviewFilter struct {
	MovieID         string
	UserID          string
	Type            *ReviewType
	IncludeSpoilers bool
	Limit           int
	Offset          int
	SortBy          string // "created_at", "helpful_count", "credibility_score"
}
