package events

type Topic = string

// Topic constants — all Kafka topic names used by the monolith and the
// cmd/worker binary. Having them in one place prevents typos and makes
// topic auditing trivial.
const (
	// User topics
	TopicUserEvents = "user.events"

	// Movie topics
	TopicMovieEvents = "movie.events"

	// Rating / review topics
	TopicRatings = "ratings"
	TopicReviews = "reviews"

	// Search indexing
	TopicSearchIndex = "search_index"

	// Watchlist topics
	TopicWatchlistEvents = "watchlist.events"

	// Social topics
	TopicSocialEvents = "social.events"

	// Notification fan-out
	TopicNotifications = "notifications"

	// Upload pipeline topics
	TopicUploadCompleted = "uploads.completed"
	TopicUploadFailed    = "uploads.failed"
	TopicUploadProgress  = "uploads.progress"

	// Payment topics
	TopicPaymentEvents        = "payment.events"
	TopicSubscriptionCreated  = "subscription.created"
	TopicSubscriptionCanceled = "subscription.canceled"
	TopicPaymentFailed        = "payment.failed"

	// Moderation topics
	TopicModerationSubmitted = "moderation.submitted"
	TopicModerationDecided   = "moderation.decided"
	TopicModerationEscalated = "moderation.escalated"

	// Workflow orchestration
	TopicWorkflowEvents = "workflow_events"

	// Analytics (produced by the monolith, consumed by Flink → ClickHouse)
	TopicBoxOffice   = "box_office"
	TopicWatchEvents = "watch_events"
)
