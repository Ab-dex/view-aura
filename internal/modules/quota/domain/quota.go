package domain

import "time"

// Tier is the user's subscription plan, which determines resource limits.
type Tier string

const (
	TierFree   Tier = "free"
	TierPro    Tier = "pro"
	TierStudio Tier = "studio"
)

// TierLimits defines the resource ceiling for a given tier.
type TierLimits struct {
	Tier             Tier
	MaxStorageBytes  int64  // total storage cap
	MaxDailyUploads  int    // uploads allowed per calendar day (UTC)
	MaxFileSizeBytes int64  // per-file cap
	TranscodeSpeed   string // "standard" | "priority" | "express"
	RetentionDays    int    // days before inactive uploads are purged
}

// AllTiers is the authoritative limit table.
// Source of truth — payment service reads this on subscription events.
var AllTiers = map[Tier]TierLimits{
	TierFree: {
		Tier:             TierFree,
		MaxStorageBytes:  5 * 1024 * 1024 * 1024, // 5 GB
		MaxDailyUploads:  3,
		MaxFileSizeBytes: 500 * 1024 * 1024, // 500 MB
		TranscodeSpeed:   "standard",
		RetentionDays:    30,
	},
	TierPro: {
		Tier:             TierPro,
		MaxStorageBytes:  100 * 1024 * 1024 * 1024, // 100 GB
		MaxDailyUploads:  50,
		MaxFileSizeBytes: 10 * 1024 * 1024 * 1024, // 10 GB
		TranscodeSpeed:   "priority",
		RetentionDays:    365,
	},
	TierStudio: {
		Tier:             TierStudio,
		MaxStorageBytes:  5 * 1024 * 1024 * 1024 * 1024, // 5 TB
		MaxDailyUploads:  1000,
		MaxFileSizeBytes: 50 * 1024 * 1024 * 1024, // 50 GB
		TranscodeSpeed:   "express",
		RetentionDays:    1825, // 5 years
	},
}

// QuotaStatus is a snapshot of a user's current usage.
type QuotaStatus struct {
	UserID           string
	Tier             Tier
	Limits           TierLimits
	StorageUsedBytes int64
	DailyUploadsUsed int
	StorageUsedPct   float64 // 0.0–100.0
	DailyUsedPct     float64
	AsOf             time.Time
}

// CheckResult is returned by CheckUploadAllowed.
type CheckResult struct {
	Allowed    bool
	DenialCode string // "quota_storage_exceeded" | "quota_daily_exceeded" | "quota_file_too_large"
	DenialMsg  string
}
