package service

import (
	"github.com/Ab-dex/view-aura/internal/platform/config"
	"github.com/google/wire"
)

// ProviderSet wires the notification service.
// The caller must also bind a concrete Dispatcher implementation
// (e.g. wire.Bind(new(Dispatcher), new(*NoopDispatcher))).
func ProvideEmailNotificationService(config *config.Config) NotificationSender {
	return NewEmailServiceClient(config.EmailClient.BaseURL, config.EmailClient.ApiKey)
}

var ProviderSet = wire.NewSet(NewNotificationService, ProvideEmailNotificationService)
