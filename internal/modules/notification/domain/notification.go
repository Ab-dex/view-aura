package domain

import "time"

// ─── IDs ──────────────────────────────────────────────────────────────────────

type NotificationID string
type PreferenceID string

func (id NotificationID) String() string { return string(id) }

// ─── Notification type ────────────────────────────────────────────────────────

// NotificationType is the machine-readable event category.
// The front-end selects a template based on this value.
type NotificationType string

const (
	// Social
	NotifNewFollower      NotificationType = "new_follower"      // someone followed you
	NotifFolloweeActivity NotificationType = "followee_activity" // someone you follow did something

	// Content
	NotifMovieNowStreaming NotificationType = "movie_now_streaming" // watchlisted movie is now on a platform
	NotifNewSequel         NotificationType = "new_sequel"          // sequel / director's cut announced
	NotifReleaseDateChange NotificationType = "release_date_change" // release date moved
	NotifPriceDrop         NotificationType = "price_drop"          // rent/buy price dropped
	NotifNowInCinemas      NotificationType = "now_in_cinemas"      // theatrical release today

	// Community
	NotifReviewReaction    NotificationType = "review_reaction"    // someone reacted to your review
	NotifThreadReply       NotificationType = "thread_reply"       // someone replied to your post
	NotifChallengeComplete NotificationType = "challenge_complete" // you finished a challenge
	NotifChallengeNudge    NotificationType = "challenge_nudge"    // deadline approaching

	// Awards
	NotifAwardsUpdate NotificationType = "awards_update" // Oscar / Cannes result
)

// Channel is the delivery mechanism for a notification.
type Channel string

const (
	ChannelInApp Channel = "in_app" // persisted in the notifications table
	ChannelPush  Channel = "push"   // mobile / browser push (external service)
	ChannelEmail Channel = "email"  // email digest (external service)
)

// ─── Core entity ──────────────────────────────────────────────────────────────

// Notification is a single in-app notification record.
// Push and email are fire-and-forget via an external delivery service;
// only in_app notifications are stored here.
type Notification struct {
	ID     NotificationID
	UserID string
	Type   NotificationType
	// Title and Body are pre-rendered, localisation-ready strings.
	Title string
	Body  string
	// Payload carries extra data the client needs to deep-link correctly
	// (movie_id, review_id, challenge_id, etc.). Stored as JSONB.
	Payload map[string]any
	// ReferenceID + ReferenceType let the client navigate to the source
	// without parsing the payload (e.g. "movie", "tt0068646").
	ReferenceType string
	ReferenceID   string
	IsRead        bool
	CreatedAt     time.Time
	ReadAt        *time.Time
}

// ─── Delivery preferences ─────────────────────────────────────────────────────

// NotificationPreference stores a user's opt-in/out settings per type+channel.
// Missing rows are treated as opted-in (opt-out model).
type NotificationPreference struct {
	UserID    string
	Type      NotificationType
	Channel   Channel
	Enabled   bool
	UpdatedAt time.Time
}

// PushToken is a device push token (FCM / APNs) registered by a client.
type PushToken struct {
	ID        string
	UserID    string
	Token     string
	Platform  string // "ios", "android", "web"
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ─── Delivery envelope ────────────────────────────────────────────────────────

// DeliverCmd is the input to the delivery layer: one logical event that may
// fan out to multiple channels depending on the recipient's preferences.
type DeliverCmd struct {
	RecipientID   string
	Type          NotificationType
	Title         string
	Body          string
	Payload       map[string]any
	ReferenceType string
	ReferenceID   string
}

// ─── Filters ─────────────────────────────────────────────────────────────────

type NotificationFilter struct {
	UserID string
	IsRead *bool
	Limit  int
	Offset int
}
