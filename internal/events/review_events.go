package events

import "time"

// ReviewCreated is published when a new review is submitted (status: draft or published).
// Consumed by: moderation-service (auto-screen for spoilers/hate speech),
// social-service (activity feed on publish), notification-service (critic alert).
type ReviewCreated struct {
	ReviewID    string `json:"review_id"`
	MovieID     string `json:"movie_id"`
	UserID      string `json:"user_id"`
	Type        string `json:"type"` // "short" | "long_form" | "spoiler" | "video"
	IsPublished bool   `json:"is_published"`
	IsSpoiler   bool   `json:"is_spoiler"`
	// BodyPreview is the first 280 chars of the body — enough for the feed card
	// without transmitting the full text in the event.
	BodyPreview string    `json:"body_preview,omitempty"`
	VideoURL    string    `json:"video_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ReviewPublished is published when a draft review transitions to published.
// Separate from ReviewCreated because moderation may hold the review in draft
// before approving it — consumers should only fan-out to feeds on this event.
// Consumed by: social-service (fan-out to follower feeds),
// notification-service (notify followers), search-service (index review text).
type ReviewPublished struct {
	ReviewID    string    `json:"review_id"`
	MovieID     string    `json:"movie_id"`
	UserID      string    `json:"user_id"`
	Type        string    `json:"type"`
	IsSpoiler   bool      `json:"is_spoiler"`
	BodyPreview string    `json:"body_preview,omitempty"`
	PublishedAt time.Time `json:"published_at"`
}

// ReviewUpdated is published when an author edits a published review.
// NOTE: the business rule is that the body cannot be edited after publish —
// only non-body fields (is_spoiler, video_url) can change. This event carries
// the changed fields list so consumers know what actually changed.
// Consumed by: search-service (re-index if spoiler flag changed).
type ReviewUpdated struct {
	ReviewID      string    `json:"review_id"`
	MovieID       string    `json:"movie_id"`
	ChangedFields []string  `json:"changed_fields"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ReviewDeleted is published when a review is permanently removed.
// Consumed by: social-service (remove from feeds), search-service (de-index),
// notification-service (cancel pending reaction notifications).
type ReviewDeleted struct {
	ReviewID  string    `json:"review_id"`
	MovieID   string    `json:"movie_id"`
	UserID    string    `json:"user_id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// ReviewReacted is published when a user reacts to a review.
// Consumed by: notification-service (notify the review author),
// social-service (activity feed entry), analytics.
type ReviewReacted struct {
	ReviewID string `json:"review_id"`
	MovieID  string `json:"movie_id"`
	// AuthorID is the review owner — needed by notification-service to
	// route the "someone reacted to your review" notification.
	AuthorID  string `json:"author_id"`
	ReactorID string `json:"reactor_id"` // user who reacted
	Reaction  string `json:"reaction"`   // "like" | "helpful" | "insightful" | "funny"
	// IsChange is true when the user switched from one reaction type to another.
	IsChange  bool      `json:"is_change"`
	ReactedAt time.Time `json:"reacted_at"`
}

// ReviewUnreacted is published when a user removes their reaction.
// Consumed by: analytics.
type ReviewUnreacted struct {
	ReviewID    string    `json:"review_id"`
	ReactorID   string    `json:"reactor_id"`
	OldReaction string    `json:"old_reaction"`
	RemovedAt   time.Time `json:"removed_at"`
}

// ReviewReported is published when a user submits an abuse report on a review.
// Consumed by: moderation-service (add to human review queue if report threshold exceeded),
// analytics (abuse rate tracking).
type ReviewReported struct {
	ReviewID   string    `json:"review_id"`
	MovieID    string    `json:"movie_id"`
	ReporterID string    `json:"reporter_id"`
	Reason     string    `json:"reason"` // "spam" | "hate_speech" | "unmarked_spoiler" | "other"
	ReportedAt time.Time `json:"reported_at"`
}
