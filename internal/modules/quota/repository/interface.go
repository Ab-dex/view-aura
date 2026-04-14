package repository

import (
	"context"

	"github.com/Ab-dex/view-aura/internal/modules/quota/domain"
)

// QuotaRepository manages per-user quota counters entirely in Redis.
// No Postgres — all data is ephemeral or re-derivable from the payment service.
type QuotaRepository interface {
	// GetTier returns the user's current tier. Defaults to TierFree when unset.
	GetTier(ctx context.Context, userID string) (domain.Tier, error)
	// SetTier stores the user's tier (called by the payment event consumer).
	SetTier(ctx context.Context, userID string, tier domain.Tier) error

	// GetStorageUsed returns the user's current storage usage in bytes.
	GetStorageUsed(ctx context.Context, userID string) (int64, error)
	// IncrementStorage adds delta bytes (positive = add, negative = free).
	IncrementStorage(ctx context.Context, userID string, deltaBytes int64) error

	// GetDailyUploads returns uploads performed today (UTC day).
	GetDailyUploads(ctx context.Context, userID string) (int, error)
	// IncrementDailyUploads increments the daily counter and sets TTL to
	// the end of the current UTC day if this is the first increment.
	IncrementDailyUploads(ctx context.Context, userID string) error
}
