package repository

import (
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
	"github.com/google/wire"
)

func provideNotificationRepository(pool *shareddb.Pool) NotificationRepository {
	return NewNotificationRepository(pool.Pool)
}

func providePreferenceRepository(pool *shareddb.Pool) PreferenceRepository {
	return NewPreferenceRepository(pool.Pool)
}

func providePushTokenRepository(pool *shareddb.Pool) PushTokenRepository {
	return NewPushTokenRepository(pool.Pool)
}

var ProviderSet = wire.NewSet(
	provideNotificationRepository,
	providePreferenceRepository,
	providePushTokenRepository,
)
