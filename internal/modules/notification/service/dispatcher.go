package service

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/notification/domain"
)

// Dispatcher is the interface the service uses to send a notification over an
// external channel (push, email).  Keeping this as an interface means the
// service package has zero dependency on FCM, APNs, SendGrid, etc. — the
// concrete adapters live in an infrastructure package and are injected at
// wire time.
//
// The service only ever calls Dispatch; the implementation decides whether to
// fan out to FCM, APNs, both, or a no-op stub.
type Dispatcher interface {
	// Dispatch sends the notification over the given channel.
	// Implementations must be safe to call concurrently.
	// A non-nil error means delivery FAILED and should be logged;
	// the in-app record is already persisted before Dispatch is called.
	Dispatch(ctx context.Context, ch domain.Channel, n *domain.Notification, tokens []string) error
}

// NoopDispatcher is a null implementation used in tests and local dev.
type NoopDispatcher struct{}

func (NoopDispatcher) Dispatch(_ context.Context, _ domain.Channel, _ *domain.Notification, _ []string) error {
	return nil
}
