package events

import "time"

// UploadInitiated is published when a user requests a presigned URL and begins
// the upload flow. Published immediately on session creation.
// Consumed by: quota-service (reserve storage against the user's limit),
// analytics (funnel tracking — how many initiated vs completed).
type UploadInitiated struct {
	UploadID    string    `json:"upload_id"`
	UserID      string    `json:"user_id"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	InitiatedAt time.Time `json:"initiated_at"`
}

// UploadCompleted is published when the client calls the /complete endpoint,
// confirming the file has fully landed in R2. This is the primary trigger for
// the upload pipeline Temporal workflow.
// Consumed by: workflow-engine (triggers upload_pipeline saga),
// quota-service (confirm storage usage), analytics.
type UploadCompleted struct {
	UploadID    string    `json:"upload_id"`
	AssetID     string    `json:"asset_id"` // ID of the created MediaAsset row
	UserID      string    `json:"user_id"`
	AssetKey    string    `json:"asset_key"` // R2 object key: uploads/{uid}/{uploadID}/{filename}
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	CompletedAt time.Time `json:"completed_at"`
}

// UploadFailed is published when an upload is aborted by the user or when
// server-side validation (file type, checksum, quota) rejects the upload.
// Consumed by: quota-service (release the reserved storage),
// notification-service (inform the uploader), analytics.
type UploadFailed struct {
	UploadID string `json:"upload_id"`
	UserID   string `json:"user_id"`
	// Reason is a human-readable failure description logged in the UI.
	Reason string `json:"reason"`
	// FailureCode is a machine-readable code for programmatic handling.
	// Values: "user_aborted" | "quota_exceeded" | "invalid_format" |
	//         "checksum_mismatch" | "file_too_large" | "storage_error"
	FailureCode string    `json:"failure_code"`
	FailedAt    time.Time `json:"failed_at"`
}

// AssetPublished is published by the upload_pipeline Temporal workflow after
// transcoding, moderation, and DRM packaging are all complete and the asset
// is live on Cloudflare Stream.
// Consumed by: movie-service (link asset to movie record),
// notification-service (notify uploader "Your video is live"),
// social-service (activity feed — "Producer X published a new video").
type AssetPublished struct {
	AssetID    string `json:"asset_id"`
	UploadID   string `json:"upload_id"`
	UserID     string `json:"user_id"`
	MovieID    string `json:"movie_id,omitempty"` // linked after editorial review
	StorageKey string `json:"storage_key"`
	// StreamURL is the Cloudflare Stream HLS delivery URL.
	StreamURL string `json:"stream_url"`
	// ThumbnailURLs are the auto-generated poster frames.
	ThumbnailURLs   []string  `json:"thumbnail_urls,omitempty"`
	DurationSeconds int       `json:"duration_seconds,omitempty"`
	PublishedAt     time.Time `json:"published_at"`
}

// AssetQuarantined is published when moderation flags an asset as violating
// policy and moves it to isolated storage pending human review.
// Consumed by: notification-service (inform uploader of pending review),
// moderation-service (add to human review queue).
type AssetQuarantined struct {
	AssetID  string `json:"asset_id"`
	UploadID string `json:"upload_id"`
	UserID   string `json:"user_id"`
	// AISignals lists the moderation categories that triggered quarantine.
	AISignals     []string  `json:"ai_signals"` // e.g. ["nsfw", "copyright_match"]
	QuarantinedAt time.Time `json:"quarantined_at"`
}

// TranscodeProgressUpdated is published periodically by the transcoding
// activity so the uploader can see a live progress bar via the API.
// Consumed by: upload-service (write upload:{id}:progress to Redis).
type TranscodeProgressUpdated struct {
	UploadID         string    `json:"upload_id"`
	AssetID          string    `json:"asset_id"`
	ProgressPct      int       `json:"progress_pct"`      // 0–100
	CurrentRendition string    `json:"current_rendition"` // e.g. "1080p"
	EstimatedSeconds int       `json:"estimated_seconds"`
	UpdatedAt        time.Time `json:"updated_at"`
}
