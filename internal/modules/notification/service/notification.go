package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Ab-dex/view-aura/internal/modules/notification/domain"
	"github.com/Ab-dex/view-aura/internal/modules/notification/repository"
	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

// NotificationService is the single interface for all notification operations.
type NotificationService interface {
	// Deliver creates an in-app record and fans out to push/email if the user
	// has opted in.  Called by other modules (rating, review, social, etc.)
	// after a state-changing operation.
	Deliver(ctx context.Context, cmd domain.DeliverCmd) error

	// List returns paginated notifications for the authenticated user.
	List(ctx context.Context, filter domain.NotificationFilter) ([]*domain.Notification, int, error)

	// UnreadCount returns the badge count for the current user.
	UnreadCount(ctx context.Context, userID string) (int, error)

	// MarkRead marks one notification as read.
	MarkRead(ctx context.Context, id domain.NotificationID, userID string) error

	// MarkAllRead marks every unread notification for the user as read.
	MarkAllRead(ctx context.Context, userID string) error

	// Delete removes a notification owned by the user.
	Delete(ctx context.Context, id domain.NotificationID, userID string) error

	// ─── Preferences ────────────────────────────────────────────────────────

	// GetPreferences returns all stored preferences; the client merges with
	// the full type×channel matrix to show every possible setting.
	GetPreferences(ctx context.Context, userID string) ([]*domain.NotificationPreference, error)

	// SetPreference enables or disables a single type+channel combination.
	SetPreference(ctx context.Context, userID string, t domain.NotificationType, ch domain.Channel, enabled bool) error

	// ─── Push tokens ────────────────────────────────────────────────────────

	// RegisterPushToken registers a device token (call on app launch / after
	// permission grant).
	RegisterPushToken(ctx context.Context, userID, token, platform string) error

	// UnregisterPushToken removes a token (call on logout from a device).
	UnregisterPushToken(ctx context.Context, userID, token string) error
}

// ─── Implementation ───────────────────────────────────────────────────────────

type notificationService struct {
	notifs     repository.NotificationRepository
	prefs      repository.PreferenceRepository
	tokens     repository.PushTokenRepository
	dispatcher Dispatcher
	redis      *sharedcache.Client
}

func NewNotificationService(
	notifs repository.NotificationRepository,
	prefs repository.PreferenceRepository,
	tokens repository.PushTokenRepository,
	dispatcher Dispatcher,
	redis *sharedcache.Client,
) NotificationService {
	return &notificationService{
		notifs:     notifs,
		prefs:      prefs,
		tokens:     tokens,
		dispatcher: dispatcher,
		redis:      redis,
	}
}

// ─── Deliver ──────────────────────────────────────────────────────────────────

// Deliver is the single entry point for all notification creation.
// It:
//  1. Always persists an in-app record (ChannelInApp).
//  2. Fans out to push if enabled + tokens exist.
//  3. Fan-out to email is handled by a separate async worker (not here)
//     to avoid blocking the request path.
func (s *notificationService) Deliver(ctx context.Context, cmd domain.DeliverCmd) error {
	log := logger.FromContext(ctx)

	// ── Rate-limit: max 100 notifications per user per hour via Redis ─────
	if err := s.checkRateLimit(ctx, cmd.RecipientID); err != nil {
		log.Warn().Str("user_id", cmd.RecipientID).Msg("notification rate limit exceeded")
		return nil // silently drop — rate limiting is not an error to propagate
	}

	// ── Always create the in-app record ───────────────────────────────────
	inAppEnabled, err := s.prefs.IsEnabled(ctx, cmd.RecipientID, cmd.Type, domain.ChannelInApp)
	if err != nil {
		return err
	}

	var notif *domain.Notification
	if inAppEnabled {
		notif = &domain.Notification{
			ID:            domain.NotificationID(uuid.New().String()),
			UserID:        cmd.RecipientID,
			Type:          cmd.Type,
			Title:         cmd.Title,
			Body:          cmd.Body,
			Payload:       cmd.Payload,
			ReferenceType: cmd.ReferenceType,
			ReferenceID:   cmd.ReferenceID,
		}
		if notif, err = s.notifs.Create(ctx, notif); err != nil {
			return err
		}
		log.Info().
			Str("notification_id", notif.ID.String()).
			Str("user_id", cmd.RecipientID).
			Str("type", string(cmd.Type)).
			Msg("in-app notification created")
	}

	// ── Fan out to push if enabled ────────────────────────────────────────
	pushEnabled, err := s.prefs.IsEnabled(ctx, cmd.RecipientID, cmd.Type, domain.ChannelPush)
	if err != nil {
		log.Error().Err(err).Msg("check push preference")
	} else if pushEnabled {
		deviceTokens, err := s.tokens.ListByUser(ctx, cmd.RecipientID)
		if err != nil {
			log.Error().Err(err).Msg("list push tokens")
		} else if len(deviceTokens) > 0 {
			rawTokens := make([]string, len(deviceTokens))
			for i, t := range deviceTokens {
				rawTokens[i] = t.Token
			}
			// Use notif if we have it; build a minimal one for push-only delivery.
			envelope := notif
			if envelope == nil {
				envelope = &domain.Notification{
					ID:      domain.NotificationID(uuid.New().String()),
					UserID:  cmd.RecipientID,
					Type:    cmd.Type,
					Title:   cmd.Title,
					Body:    cmd.Body,
					Payload: cmd.Payload,
				}
			}
			if err := s.dispatcher.Dispatch(ctx, domain.ChannelPush, envelope, rawTokens); err != nil {
				// Non-fatal: log and continue.
				log.Error().Err(err).Str("user_id", cmd.RecipientID).Msg("push dispatch failed")
			}
		}
	}

	return nil
}

// checkRateLimit uses Redis INCR + EXPIRE to enforce 100 notifications/user/hour.
// Returns a non-nil error only when the limit is exceeded.
func (s *notificationService) checkRateLimit(ctx context.Context, userID string) error {
	key := sharedcache.NotifRateKey(userID)
	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		// Redis unavailable — fail open (allow delivery).
		return nil
	}
	if count == 1 {
		// First notification in this window — set the expiry.
		s.redis.Expire(ctx, key, time.Hour)
	}
	if count > 100 {
		return apierror.New(429, apierror.CodeTooManyRequests, "notification rate limit exceeded")
	}
	return nil
}

// ─── Inbox operations ─────────────────────────────────────────────────────────

func (s *notificationService) List(ctx context.Context, filter domain.NotificationFilter) ([]*domain.Notification, int, error) {
	return s.notifs.List(ctx, filter)
}

func (s *notificationService) UnreadCount(ctx context.Context, userID string) (int, error) {
	return s.notifs.UnreadCount(ctx, userID)
}

func (s *notificationService) MarkRead(ctx context.Context, id domain.NotificationID, userID string) error {
	return s.notifs.MarkRead(ctx, id, userID)
}

func (s *notificationService) MarkAllRead(ctx context.Context, userID string) error {
	return s.notifs.MarkAllRead(ctx, userID)
}

func (s *notificationService) Delete(ctx context.Context, id domain.NotificationID, userID string) error {
	return s.notifs.Delete(ctx, id, userID)
}

// ─── Preferences ─────────────────────────────────────────────────────────────

func (s *notificationService) GetPreferences(ctx context.Context, userID string) ([]*domain.NotificationPreference, error) {
	return s.prefs.GetAll(ctx, userID)
}

func (s *notificationService) SetPreference(ctx context.Context, userID string, t domain.NotificationType, ch domain.Channel, enabled bool) error {
	return s.prefs.Upsert(ctx, &domain.NotificationPreference{
		UserID:  userID,
		Type:    t,
		Channel: ch,
		Enabled: enabled,
	})
}

// ─── Push tokens ──────────────────────────────────────────────────────────────

func (s *notificationService) RegisterPushToken(ctx context.Context, userID, token, platform string) error {
	if token == "" {
		return apierror.Validation("push token must not be empty", nil)
	}
	return s.tokens.Upsert(ctx, &domain.PushToken{
		ID:       uuid.New().String(),
		UserID:   userID,
		Token:    token,
		Platform: platform,
	})
}

func (s *notificationService) UnregisterPushToken(ctx context.Context, userID, token string) error {
	return s.tokens.Delete(ctx, userID, token)
}
