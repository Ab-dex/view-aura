package repository

import (
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
	"github.com/google/wire"
)

func ProvideNotificationRepository(pool *shareddb.Pool) NotificationRepository {
	return NewNotificationRepository(pool.Pool)
}

func ProvidePreferenceRepository(pool *shareddb.Pool) PreferenceRepository {
	return NewPreferenceRepository(pool.Pool)
}

func ProvidePushTokenRepository(pool *shareddb.Pool) PushTokenRepository {
	return NewPushTokenRepository(pool.Pool)
}

var ProviderSet = wire.NewSet(
	ProvideNotificationRepository,
	ProvidePreferenceRepository,
	ProvidePushTokenRepository,
)
