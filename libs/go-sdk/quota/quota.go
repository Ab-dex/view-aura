// Package quota provides storage and upload quota status operations.
package quota

import (
	"context"
	"net/http"
)

type doer interface {
	Do(ctx context.Context, method, path string, body, out any) error
}

// Service wraps all quota endpoints.
type Service struct{ doer doer }

func New(d doer) *Service { return &Service{doer: d} }

// ─── Types ────────────────────────────────────────────────────────────────────

// StorageInfo holds current storage usage vs limits.
type StorageInfo struct {
	UsedBytes int64   `json:"used_bytes"`
	MaxBytes  int64   `json:"max_bytes"`
	UsedPct   float64 `json:"used_pct"`
}

// DailyUploadInfo holds today's upload count vs the daily limit.
type DailyUploadInfo struct {
	Used    int     `json:"used"`
	Max     int     `json:"max"`
	UsedPct float64 `json:"used_pct"`
}

// Limits contains the plan-level resource ceilings.
type Limits struct {
	MaxFileSizeBytes int64  `json:"max_file_size_bytes"`
	TranscodeSpeed   string `json:"transcode_speed"` // "standard" | "priority" | "express"
	RetentionDays    int    `json:"retention_days"`
}

// QuotaStatus is the full usage snapshot returned by GetStatus.
type QuotaStatus struct {
	Tier         string          `json:"tier"` // "free" | "pro" | "studio"
	Storage      StorageInfo     `json:"storage"`
	DailyUploads DailyUploadInfo `json:"daily_uploads"`
	Limits       Limits          `json:"limits"`
	AsOf         string          `json:"as_of"`
}

// TierInfo is the condensed response from GetTier.
type TierInfo struct {
	Tier   string `json:"tier"`
	Limits Limits `json:"limits"`
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// GetStatus returns the caller's full quota usage snapshot.
//
//	status, err := client.Quota.GetStatus(ctx)
//	fmt.Printf("storage: %.1f%% used\n", status.Storage.UsedPct)
func (s *Service) GetStatus(ctx context.Context) (*QuotaStatus, error) {
	var out QuotaStatus
	return &out, s.doer.Do(ctx, http.MethodGet, "/api/v1/me/quota", nil, &out)
}

// GetTier returns just the caller's current tier and its limits.
func (s *Service) GetTier(ctx context.Context) (*TierInfo, error) {
	var out TierInfo
	return &out, s.doer.Do(ctx, http.MethodGet, "/api/v1/me/quota/tier", nil, &out)
}
