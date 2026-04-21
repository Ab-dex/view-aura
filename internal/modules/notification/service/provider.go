package service

import (
	"github.com/google/wire"

	"github.com/Ab-dex/view-aura/internal/platform/cache"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	"github.com/Ab-dex/view-aura/internal/platform/resilience"
)

// ProvideDispatcher builds the three-tier push dispatcher.
//
//	Tier 1  Real FCM/APNs adapter — NoopDispatcher until credentials are configured
//	Tier 2  Redis per-user push outbox (replayed on next app open)
//	Tier 3  FileOutbox JSONL (ops-replayable)
//
// rdb is nullable — Tier 2 is skipped when Redis is not configured.
// To wire a real FCM adapter replace NoopDispatcher{} with:
//
//	fcm.NewDispatcher(cfg.FCM.ServerKey)
func ProvideDispatcher(rdb *cache.Client, cfg *config.Config) Dispatcher {
	primary := Dispatcher(NoopDispatcher{})
	logDir := cfg.Resilience.LogDir
	pushKeyPrefix := cfg.Resilience.PushOutboxKeyPrefix

	if rdb != nil {
		return NewResilientDispatcher(primary, rdb.Client, logDir, pushKeyPrefix)
	}
	return NewResilientDispatcher(primary, nil, logDir, pushKeyPrefix)
}

// ProvideNotificationSender exposes the ResilientMailer (built in the resilience
// layer) as the NotificationSender interface consumed by the auth module.
//
// The auth module calls SendEmail for verification and password-reset emails.
// ResilientMailer already handles all three tiers (HTTP → Redis → file outbox)
// so no additional wrapping is needed here.
//
// Wire binds *resilience.ResilientMailer → NotificationSender at the app level.
// This provider is the declaration point so the notification package owns the
// interface without importing the resilience package transitively.
func ProvideNotificationSender(mailer *resilience.ResilientMailer) NotificationSender {
	return mailer
}

// ProviderSet wires the notification service.
// Both ProvideDispatcher and ProvideNotificationSender return interfaces directly
// so no wire.Bind is needed.
var ProviderSet = wire.NewSet(
	// ProvideDispatcher,
	ProvideNotificationSender,
	NewNotificationService,
)
