package service

import "github.com/google/wire"

// ProviderSet wires the notification service.
// The caller must also bind a concrete Dispatcher implementation
// (e.g. wire.Bind(new(Dispatcher), new(*NoopDispatcher))).
var ProviderSet = wire.NewSet(NewNotificationService)
