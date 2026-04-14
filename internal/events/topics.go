package events

type Topic = string

// Topic constants — all Kafka topic names used by the monolith and workers.
// Having them in one place prevents typos and makes topic auditing trivial.
//
// Topic naming convention:
//
//	{entity}.{verb}           — single action event
//	{entity}.{noun}           — aggregate / state topics
//	{entity}.{concern}.{verb} — scoped sub-domain events
const (
	// ── User / auth ───────────────────────────────────────────────────────────
	TopicUserEvents = "user.events" // registered, profile_updated, deactivated

	// ── Movie / catalog ───────────────────────────────────────────────────────
	TopicMovieEvents = "movie.events" // created, updated, archived, enriched

	// ── Ratings and reviews ───────────────────────────────────────────────────
	TopicRatings = "ratings"
	TopicReviews = "reviews"

	// ── Search ────────────────────────────────────────────────────────────────
	TopicSearchIndex = "search_index" // index_requested, index_completed

	// ── Watchlist ─────────────────────────────────────────────────────────────
	TopicWatchlistEvents = "watchlist.events" // item_added, item_removed, status_changed

	// ── Social ────────────────────────────────────────────────────────────────
	TopicSocialEvents = "social.events" // followed, unfollowed, activity_recorded

	// ── Notifications ─────────────────────────────────────────────────────────
	TopicNotifications = "notifications"

	// ── Upload pipeline ───────────────────────────────────────────────────────
	TopicUploadInitiated = "uploads.initiated"
	TopicUploadCompleted = "uploads.completed"
	TopicUploadFailed    = "uploads.failed"
	TopicUploadProgress  = "uploads.progress"

	// ── Payment / billing ─────────────────────────────────────────────────────
	TopicPaymentEvents        = "payment.events"
	TopicSubscriptionCreated  = "subscription.created"
	TopicSubscriptionCanceled = "subscription.canceled"
	TopicPaymentFailed        = "payment.failed"

	// ── Moderation ────────────────────────────────────────────────────────────
	TopicModerationSubmitted     = "moderation.submitted"
	TopicModerationDecided       = "moderation.decided"
	TopicModerationEscalated     = "moderation.escalated"
	TopicModerationCreated       = "moderation.created"
	TopicModerationAppealDecided = "moderation.appeal_decided"

	// ── Workflow orchestration ────────────────────────────────────────────────
	TopicWorkflowEvents = "workflow_events"

	// ── Legacy analytics (kept for backward compat) ───────────────────────────
	TopicBoxOffice   = "box_office"
	TopicWatchEvents = "watch_events"

	// =========================================================================
	// BEHAVIORAL EVENTS — consumed by recommendation-service and Flink jobs
	// =========================================================================

	// ── Playback lifecycle ────────────────────────────────────────────────────

	// TopicPlaybackSessionStarted is emitted when a user presses play.
	// Payload: PlaybackSessionStartedEvent.
	// Consumers: recommendation-service (positive signal), viewing_history, analytics-pipeline.
	TopicPlaybackSessionStarted = "user.playback.session_started"

	// TopicPlaybackHeartbeat is emitted every 10 seconds while playing.
	// Payload: PlaybackHeartbeatEvent.
	// Consumers: viewing_history (resume position), analytics-pipeline (QoE aggregation).
	// High-volume — dedicated consumer group with higher partition count.
	TopicPlaybackHeartbeat = "user.playback.heartbeat"

	// TopicPlaybackAbandoned is emitted when the user stops before 90% completion.
	// Payload: PlaybackAbandonedEvent.
	// Consumers: recommendation-service (negative signal), analytics-pipeline.
	TopicPlaybackAbandoned = "user.playback.abandoned"

	// TopicPlaybackCompleted is emitted when the user reaches >= 90% of runtime.
	// Payload: PlaybackCompletedEvent.
	// Consumers: recommendation-service (strong positive), viewing_history, notification module.
	TopicPlaybackCompleted = "user.playback.completed"

	// TopicPlaybackQoE carries client-side quality-of-experience telemetry.
	// Payload: PlaybackQoEEvent.
	// Consumers: analytics-pipeline (per-ISP/region buffering metrics, CDN health alerts).
	TopicPlaybackQoE = "user.playback.qoe"

	// ── Search behavioural events ─────────────────────────────────────────────

	// TopicSearchQuery is emitted when a user executes a search.
	// Payload: SearchQueryEvent.
	// Consumers: analytics-pipeline, recommendation-service (intent signal).
	TopicSearchQuery = "user.search.query"

	// TopicSearchClick is emitted when a user clicks a result.
	// Payload: SearchClickEvent.
	// Consumers: recommendation-service (CTR signal), analytics-pipeline (search ranking).
	TopicSearchClick = "user.search.click"

	// TopicSearchAbandoned is emitted when a search produced zero clicks.
	// Payload: SearchAbandonedEvent.
	// Consumers: analytics-pipeline (catalogue gap detection, search quality alerting).
	TopicSearchAbandoned = "user.search.abandoned"

	// ── Content performance ───────────────────────────────────────────────────

	// TopicTrendingScoreUpdated is emitted by the Flink analytics pipeline when
	// a movie's trending score changes significantly.
	// Payload: TrendingScoreUpdatedEvent.
	// Consumers: recommendation-service (boost popular content), search-service (ranking).
	TopicTrendingScoreUpdated = "content.trending_score_updated"

	// ── Experiment / feature flags ────────────────────────────────────────────

	// TopicExperimentExposure is emitted when a user is assigned to an experiment treatment.
	// Payload: ExperimentExposureEvent.
	// Consumers: analytics-pipeline (intent-to-treat experiment analysis).
	TopicExperimentExposure = "experiment.exposure"

	// ── Entitlement ───────────────────────────────────────────────────────────

	// TopicEntitlementGranted is emitted when a user gains access to content.
	// Payload: EntitlementGrantedEvent.
	// Consumers: playback module (access cache warm-up), notification module.
	TopicEntitlementGranted = "entitlement.granted"

	// TopicEntitlementRevoked is emitted when access is removed.
	// Payload: EntitlementRevokedEvent.
	// Consumers: playback module (invalidate cached access grants).
	TopicEntitlementRevoked = "entitlement.revoked"
)
