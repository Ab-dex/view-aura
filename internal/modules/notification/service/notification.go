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
	Deliver(ctx context.Context, cmd domain.DeliverCmd) error
	List(ctx context.Context, filter domain.NotificationFilter) ([]*domain.Notification, int, error)
	UnreadCount(ctx context.Context, userID string) (int, error)
	MarkRead(ctx context.Context, id domain.NotificationID, userID string) error
	MarkAllRead(ctx context.Context, userID string) error
	Delete(ctx context.Context, id domain.NotificationID, userID string) error
	GetPreferences(ctx context.Context, userID string) ([]*domain.NotificationPreference, error)
	SetPreference(ctx context.Context, userID string, t domain.NotificationType, ch domain.Channel, enabled bool) error
	RegisterPushToken(ctx context.Context, userID, token, platform string) error
	UnregisterPushToken(ctx context.Context, userID, token string) error
}

type notificationService struct {
	notifs     repository.NotificationRepository
	prefs      repository.PreferenceRepository
	tokens     repository.PushTokenRepository
	dispatcher Dispatcher
	// redis is nullable — rate limiting degrades to fail-open when nil.
	redis *sharedcache.Client
}

// NewNotificationService constructs the service.
// redis may be nil — rate limiting is skipped (fail open) without it.
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

func (s *notificationService) Deliver(ctx context.Context, cmd domain.DeliverCmd) error {
	log := logger.FromContext(ctx)

	// Rate limit — skipped when Redis is unavailable (fail open).
	if err := s.checkRateLimit(ctx, cmd.RecipientID); err != nil {
		log.Warn().Str("user_id", cmd.RecipientID).Msg("notification rate limit exceeded")
		return nil
	}

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
			// ResilientDispatcher never returns an error (always degrades gracefully).
			if err := s.dispatcher.Dispatch(ctx, domain.ChannelPush, envelope, rawTokens); err != nil {
				log.Error().Err(err).Str("user_id", cmd.RecipientID).Msg("push dispatch failed")
			}
		}
	}

	return nil
}

// checkRateLimit enforces 100 notifications/user/hour via Redis INCR.
//
// When Redis is nil (not configured) or unavailable, the function fails open —
// all notifications are delivered without rate limiting.  This is intentional:
// a degraded notification system is better than a broken one.
func (s *notificationService) checkRateLimit(ctx context.Context, userID string) error {
	if s.redis == nil {
		// Redis not configured — fail open.
		return nil
	}

	key := sharedcache.NotifRateKey(userID)
	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		// Redis call failed — fail open rather than blocking delivery.
		return nil
	}
	if count == 1 {
		// First notification in this window — set the expiry.
		// Non-fatal if Expire fails (key will just not expire this window).
		_ = s.redis.Expire(ctx, key, time.Hour)
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
