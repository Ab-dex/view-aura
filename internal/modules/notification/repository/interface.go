package repository

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/notification/domain"
)

// NotificationRepository manages persisted in-app notifications.
type NotificationRepository interface {
	// Create inserts a new notification row.
	Create(ctx context.Context, n *domain.Notification) (*domain.Notification, error)

	// GetByID fetches a single notification.
	GetByID(ctx context.Context, id domain.NotificationID) (*domain.Notification, error)

	// List returns paginated notifications for a user.
	List(ctx context.Context, filter domain.NotificationFilter) ([]*domain.Notification, int, error)

	// MarkRead marks a single notification as read.
	MarkRead(ctx context.Context, id domain.NotificationID, userID string) error

	// MarkAllRead marks every unread notification for a user as read.
	MarkAllRead(ctx context.Context, userID string) error

	// Delete removes a notification (owner only).
	Delete(ctx context.Context, id domain.NotificationID, userID string) error

	// UnreadCount returns the number of unread notifications — used for the
	// badge counter in the app header.
	UnreadCount(ctx context.Context, userID string) (int, error)
}

// PreferenceRepository manages per-user notification preferences.
type PreferenceRepository interface {
	// Upsert inserts or updates the opt-in/out setting for (user, type, channel).
	Upsert(ctx context.Context, pref *domain.NotificationPreference) error

	// GetAll returns all stored preference rows for a user.
	// Callers should treat missing rows as "enabled = true" (opt-out model).
	GetAll(ctx context.Context, userID string) ([]*domain.NotificationPreference, error)

	// IsEnabled returns whether notifications of the given type+channel are
	// enabled for the user. Returns true when no preference row exists.
	IsEnabled(ctx context.Context, userID string, t domain.NotificationType, ch domain.Channel) (bool, error)
}

// PushTokenRepository manages device push tokens.
type PushTokenRepository interface {
	// Upsert registers a token, replacing any existing row for the same
	// (user_id, token) pair.
	Upsert(ctx context.Context, token *domain.PushToken) error

	// Delete removes a specific token (e.g. on logout from a device).
	Delete(ctx context.Context, userID, token string) error

	// ListByUser returns all active push tokens for a user.
	ListByUser(ctx context.Context, userID string) ([]*domain.PushToken, error)
}
