package service

import (
	"context"
	"time"

	"github.com/Ab-dex/view-aura/internal/modules/quota/domain"
	"github.com/Ab-dex/view-aura/internal/modules/quota/repository"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

type QuotaService interface {
	// CheckUploadAllowed returns whether the user may upload a file of the
	// given size. Does NOT mutate any counters.
	CheckUploadAllowed(ctx context.Context, userID string, fileSizeBytes int64) (domain.CheckResult, error)

	// RecordUpload increments both the storage counter and the daily counter.
	// Call after a successful upload.Complete().
	RecordUpload(ctx context.Context, userID string, sizeBytes int64) error

	// RecordDelete decrements the storage counter.
	// Call when an asset is permanently removed.
	RecordDelete(ctx context.Context, userID string, sizeBytes int64) error

	// GetStatus returns the user's current usage snapshot.
	GetStatus(ctx context.Context, userID string) (*domain.QuotaStatus, error)

	// SetTier upgrades or downgrades a user's tier.
	// Called by the payment event consumer on subscription changes.
	SetTier(ctx context.Context, userID string, tier domain.Tier) error
}

type quotaService struct{ repo repository.QuotaRepository }

func NewQuotaService(repo repository.QuotaRepository) QuotaService {
	return &quotaService{repo: repo}
}

func (s *quotaService) CheckUploadAllowed(ctx context.Context, userID string, fileSizeBytes int64) (domain.CheckResult, error) {
	tier, err := s.repo.GetTier(ctx, userID)
	if err != nil {
		return domain.CheckResult{}, err
	}
	limits := domain.AllTiers[tier]

	// Check 1: file size cap.
	if fileSizeBytes > limits.MaxFileSizeBytes {
		return domain.CheckResult{
			Allowed:    false,
			DenialCode: "quota_file_too_large",
			DenialMsg:  "file exceeds the maximum size for your plan",
		}, nil
	}

	// Check 2: storage cap.
	used, err := s.repo.GetStorageUsed(ctx, userID)
	if err != nil {
		return domain.CheckResult{}, err
	}
	if used+fileSizeBytes > limits.MaxStorageBytes {
		return domain.CheckResult{
			Allowed:    false,
			DenialCode: "quota_storage_exceeded",
			DenialMsg:  "storage quota exceeded for your plan",
		}, nil
	}

	// Check 3: daily upload cap.
	daily, err := s.repo.GetDailyUploads(ctx, userID)
	if err != nil {
		return domain.CheckResult{}, err
	}
	if daily >= limits.MaxDailyUploads {
		return domain.CheckResult{
			Allowed:    false,
			DenialCode: "quota_daily_exceeded",
			DenialMsg:  "daily upload limit reached for your plan",
		}, nil
	}

	return domain.CheckResult{Allowed: true}, nil
}

func (s *quotaService) RecordUpload(ctx context.Context, userID string, sizeBytes int64) error {
	if err := s.repo.IncrementStorage(ctx, userID, sizeBytes); err != nil {
		return err
	}
	return s.repo.IncrementDailyUploads(ctx, userID)
}

func (s *quotaService) RecordDelete(ctx context.Context, userID string, sizeBytes int64) error {
	return s.repo.IncrementStorage(ctx, userID, -sizeBytes)
}

func (s *quotaService) GetStatus(ctx context.Context, userID string) (*domain.QuotaStatus, error) {
	tier, err := s.repo.GetTier(ctx, userID)
	if err != nil {
		return nil, err
	}
	limits := domain.AllTiers[tier]

	storage, err := s.repo.GetStorageUsed(ctx, userID)
	if err != nil {
		return nil, err
	}
	daily, err := s.repo.GetDailyUploads(ctx, userID)
	if err != nil {
		return nil, err
	}

	storagePct := 0.0
	if limits.MaxStorageBytes > 0 {
		storagePct = float64(storage) / float64(limits.MaxStorageBytes) * 100
	}
	dailyPct := 0.0
	if limits.MaxDailyUploads > 0 {
		dailyPct = float64(daily) / float64(limits.MaxDailyUploads) * 100
	}

	return &domain.QuotaStatus{
		UserID:           userID,
		Tier:             tier,
		Limits:           limits,
		StorageUsedBytes: storage,
		DailyUploadsUsed: daily,
		StorageUsedPct:   storagePct,
		DailyUsedPct:     dailyPct,
		AsOf:             time.Now(),
	}, nil
}

func (s *quotaService) SetTier(ctx context.Context, userID string, tier domain.Tier) error {
	if _, ok := domain.AllTiers[tier]; !ok {
		return apierror.Validation("unknown tier: "+string(tier), nil)
	}
	return s.repo.SetTier(ctx, userID, tier)
}
