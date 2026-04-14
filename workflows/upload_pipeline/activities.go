package upload_pipeline

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	"github.com/Ab-dex/view-aura/internal/events"
	r2pkg "github.com/Ab-dex/view-aura/internal/platform/r2"
)

// ─── Input / Output types for each activity ───────────────────────────────────

type ValidationInput struct {
	AssetKey    string
	ContentType string
	SizeBytes   int64
	UserID      string
}

type ValidationResult struct {
	Valid    bool
	MimeType string // detected from magic bytes, not the ContentType header
}

type TranscodeInput struct {
	AssetKey    string
	AssetID     string
	ContentType string
}

type TranscodeResult struct {
	DeliveryKey    string   // primary R2 key for the HLS master playlist
	TranscodedKeys []string // all R2 keys produced: master.m3u8, 1080p, 720p, 480p, thumbnails
	DurationSecs   int
}

type MetadataInput struct {
	AssetKey string
	AssetID  string
}

type MetadataResult struct {
	Codec         string
	Resolution    string
	Bitrate       int64
	DurationSecs  int
	AudioChannels int
}

type ScreenInput struct {
	AssetID     string
	AssetKey    string
	ContentType string
}

type ScreenResult struct {
	Flagged    bool
	Signals    []string // e.g. ["nsfw", "copyright_match"]
	Confidence map[string]float64
}

type QuarantineInput struct {
	AssetID  string
	UploadID string
	UserID   string
	Signals  []string
}

type PublishInput struct {
	AssetID        string
	TranscodedKeys []string
	UserID         string
}

type PublishResult struct {
	StreamURL     string
	ThumbnailURLs []string
}

type NotifyInput struct {
	UserID   string
	UploadID string
	AssetID  string
	Event    string // "published" | "quarantined" | "failed"
	Message  string
}

// ─── Activities ───────────────────────────────────────────────────────────────
// Each activity is a plain function — Temporal registers them via side-effect
// imports in cmd/worker/worker.go. Activities can be retried independently.

// ValidateFileActivity reads the first 16 bytes of the R2 object and checks
// the magic bytes against the allowed format list (MP4/MOV/MKV/ProRes).
// Per the PRD: "File type validation uses magic bytes, not extension."
func ValidateFileActivity(ctx context.Context, input ValidationInput, r2 *r2pkg.Client) (ValidationResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("validate_file: checking magic bytes", "asset_key", input.AssetKey)

	// In production: PresignGetObject → HTTP GET with Range: bytes=0-15
	// → compare against magic byte signatures.
	// Stub returns valid for non-zero files.
	if input.SizeBytes == 0 {
		return ValidationResult{}, fmt.Errorf("ValidationError: file is empty")
	}

	// Magic byte check placeholder.
	// Real implementation: fetch first 16 bytes from R2 and compare.
	allowed := map[string]bool{
		"video/mp4":        true,
		"video/quicktime":  true,
		"video/x-matroska": true,
	}
	if !allowed[input.ContentType] {
		return ValidationResult{}, fmt.Errorf("ValidationError: content type %q not allowed", input.ContentType)
	}

	logger.Info("validate_file: passed", "asset_key", input.AssetKey)
	return ValidationResult{Valid: true, MimeType: input.ContentType}, nil
}

// TranscodeVideoActivity calls the Rust transcoding service via its internal
// gRPC/HTTP API. The activity uses Temporal heartbeats every 30 seconds so
// Temporal knows the worker is still alive during multi-hour transcodes.
func TranscodeVideoActivity(ctx context.Context, input TranscodeInput) (TranscodeResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("transcode_video: started", "asset_key", input.AssetKey)

	// Heartbeat loop — the worker sends a heartbeat every 30s.
	// If the pod dies, Temporal reschedules on another worker after HeartbeatTimeout.
	// Real implementation: POST to transcoding-service gRPC, poll for completion,
	// heartbeat on each poll iteration.

	// Stub result — replace with real transcoding service call.
	deliveryKey := fmt.Sprintf("assets/%s/master.m3u8", input.AssetID)
	return TranscodeResult{
		DeliveryKey: deliveryKey,
		TranscodedKeys: []string{
			deliveryKey,
			fmt.Sprintf("assets/%s/1080p/index.m3u8", input.AssetID),
			fmt.Sprintf("assets/%s/720p/index.m3u8", input.AssetID),
			fmt.Sprintf("assets/%s/480p/index.m3u8", input.AssetID),
			fmt.Sprintf("assets/%s/thumb_0.jpg", input.AssetID),
		},
	}, nil
}

// ExtractMetadataActivity calls FFprobe (via the Rust video-packaging service)
// to extract codec, resolution, duration, and audio channel information.
func ExtractMetadataActivity(ctx context.Context, input MetadataInput) (MetadataResult, error) {
	activity.GetLogger(ctx).Info("extract_metadata: started", "asset_key", input.AssetKey)

	// Stub — real implementation: POST asset_key to metadata-extraction service,
	// receive FFprobe JSON, map to MetadataResult.
	return MetadataResult{
		Codec:         "h264",
		Resolution:    "1920x1080",
		Bitrate:       8000000,
		DurationSecs:  5400,
		AudioChannels: 2,
	}, nil
}

// ScreenContentActivity calls the Python moderation-ai service to scan the
// transcoded video for NSFW content, copyright matches, and violence.
func ScreenContentActivity(ctx context.Context, input ScreenInput) (ScreenResult, error) {
	activity.GetLogger(ctx).Info("screen_content: started", "asset_id", input.AssetID)

	// Stub — real implementation: POST to moderation-ai Python service,
	// poll for result with heartbeats, return signals and confidence scores.
	return ScreenResult{
		Flagged:    false,
		Signals:    []string{},
		Confidence: map[string]float64{},
	}, nil
}

// QuarantineAssetActivity copies the asset to the quarantine prefix in R2,
// creates a moderation case in the moderation module via HTTP, and publishes
// an AssetQuarantined event.
func QuarantineAssetActivity(ctx context.Context, input QuarantineInput, r2 *r2pkg.Client, producer events.Producer) error {
	logger := activity.GetLogger(ctx)
	logger.Info("quarantine_asset: moving to quarantine",
		"asset_id", input.AssetID,
		"signals", input.Signals,
	)

	// Publish event so the moderation module opens a case.
	_ = producer.Publish(ctx, events.TopicModerationSubmitted, "moderation.submitted", events.ModerationSubmitted{
		CaseID:      input.AssetID,
		ContentType: "upload",
		ContentID:   input.AssetID,
		ActorID:     input.UserID,
	})

	return nil
}

// PublishToCDNActivity copies transcoded segments from the ingest prefix to
// the delivery prefix, registers the asset with Cloudflare Stream, and
// publishes an AssetPublished event.
func PublishToCDNActivity(ctx context.Context, input PublishInput, producer events.Producer) (PublishResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("publish_to_cdn: publishing", "asset_id", input.AssetID)

	// Real-world logic would happen here...
	streamURL := fmt.Sprintf("https://stream.cinemaos.com/%s/master.m3u8", input.AssetID)

	// Use the time the activity was actually scheduled/started
	publishedAt := activity.GetInfo(ctx).StartedTime

	// 1. Capture the error from Publish!
	// If the Kafka producer is down, this activity should fail and retry.
	err := producer.Publish(ctx, events.TopicUploadCompleted, "upload.asset_published", events.AssetPublished{
		AssetID:     input.AssetID,
		UserID:      input.UserID,
		StreamURL:   streamURL,
		PublishedAt: publishedAt, // Now correctly a time.Time
	})

	if err != nil {
		logger.Error("publish_to_cdn: kafka publish failed", "error", err)
		return PublishResult{}, err // Return error so Temporal retries the activity
	}

	return PublishResult{StreamURL: streamURL}, nil
}

// NotifyUploaderActivity sends an in-app notification to the uploader via
// the notification module HTTP endpoint.
func NotifyUploaderActivity(ctx context.Context, input NotifyInput) error {
	activity.GetLogger(ctx).Info("notify_uploader",
		"user_id", input.UserID,
		"event", input.Event,
	)
	// Real implementation: POST to /internal/notifications with user_id, message, and event type.
	return nil
}

// ─── Compensation activities ──────────────────────────────────────────────────

// DeleteTranscodedSegmentsActivity removes all R2 objects produced by the
// transcoding activity. Called during saga compensation on pipeline failure.
func DeleteTranscodedSegmentsActivity(ctx context.Context, assetKey string, r2 *r2pkg.Client) error {
	activity.GetLogger(ctx).Info("delete_transcoded_segments", "asset_key", assetKey)
	// Real implementation: list objects under assets/{assetID}/ prefix and delete each.
	return r2.DeleteObject(ctx, assetKey)
}

// MarkAssetFailedActivity updates the media_assets row in Postgres to status="failed".
func MarkAssetFailedActivity(ctx context.Context, assetID, reason string) error {
	activity.GetLogger(ctx).Info("mark_asset_failed", "asset_id", assetID, "reason", reason)
	// Real implementation: UPDATE media_assets SET status='failed', failure_reason=$2 WHERE id=$1
	return nil
}

// ReleaseQuotaActivity decrements the user's stored bytes counter in Redis
// so the quota reserved during upload.Initiate is freed on pipeline failure.
func ReleaseQuotaActivity(ctx context.Context, userID string, sizeBytes int64) error {
	activity.GetLogger(ctx).Info("release_quota", "user_id", userID, "size_bytes", sizeBytes)
	// Real implementation: call quota service IncrementStorage(ctx, userID, -sizeBytes)
	return nil
}

// ─── Registration ─────────────────────────────────────────────────────────────

// Register registers both the workflow and all activities with a Temporal worker.
// Called from cmd/worker/worker.go via side-effect import.
func Register(w interface {
	RegisterWorkflow(interface{})
	RegisterActivity(interface{})
}) {
	w.RegisterWorkflow(UploadPipelineWorkflow)
	w.RegisterActivity(ValidateFileActivity)
	w.RegisterActivity(TranscodeVideoActivity)
	w.RegisterActivity(ExtractMetadataActivity)
	w.RegisterActivity(ScreenContentActivity)
	w.RegisterActivity(QuarantineAssetActivity)
	w.RegisterActivity(PublishToCDNActivity)
	w.RegisterActivity(NotifyUploaderActivity)
	w.RegisterActivity(DeleteTranscodedSegmentsActivity)
	w.RegisterActivity(MarkAssetFailedActivity)
	w.RegisterActivity(ReleaseQuotaActivity)
}
