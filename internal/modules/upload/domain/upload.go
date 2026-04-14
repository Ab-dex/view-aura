package domain

import "time"

// ─── Enumerations ─────────────────────────────────────────────────────────────

// UploadStatus follows the state machine from the PRD:
//
//	initiated → uploading → validating → processing → completed
//	                                              ↘ failed / aborted
type UploadStatus string

const (
	StatusInitiated  UploadStatus = "initiated"  // presigned URL issued, client has not started yet
	StatusUploading  UploadStatus = "uploading"  // client is actively PUT-ing chunks to R2
	StatusValidating UploadStatus = "validating" // checksum + format check running
	StatusProcessing UploadStatus = "processing" // transcoding / AI analysis in progress
	StatusCompleted  UploadStatus = "completed"  // asset is live / ready
	StatusFailed     UploadStatus = "failed"     // validation or processing error
	StatusAborted    UploadStatus = "aborted"    // user cancelled or quota denied
)

// ─── Entities ─────────────────────────────────────────────────────────────────

// UploadSession is ephemeral state held in Redis while the upload is in flight.
// It is created on POST /uploads/initiate and removed on complete or abort.
type UploadSession struct {
	ID           string
	UserID       string
	FileName     string
	ContentType  string // MIME type, e.g. "video/mp4"
	SizeBytes    int64
	AssetKey     string // R2 object key: uploads/{userID}/{uploadID}/{filename}
	PresignedURL string // returned to client, never persisted past the session
	Status       UploadStatus
	ExpiresAt    time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// MediaAsset is the durable record written to Postgres when the upload
// is confirmed complete. It persists through the full pipeline lifecycle.
type MediaAsset struct {
	ID          string
	UploadID    string
	UserID      string
	MovieID     string // linked by editorial after the pipeline completes
	StorageKey  string // R2 object key
	ContentType string
	SizeBytes   int64
	// Status tracks asset lifecycle after the upload is complete.
	// Values: "uploading" | "processing" | "ready" | "failed" | "moderated"
	Status string
	// SourceInfo carries technical metadata extracted during validation:
	// codec, resolution, frame rate, audio channels, etc.
	SourceInfo map[string]any
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ─── Commands ─────────────────────────────────────────────────────────────────

type InitiateUploadCmd struct {
	UserID      string
	FileName    string
	ContentType string
	SizeBytes   int64
}

type CompleteUploadCmd struct {
	UploadID string
	UserID   string
}

type AbortUploadCmd struct {
	UploadID string
	UserID   string
	Reason   string // "user_cancelled" | "quota_exceeded" | "validation_failed"
}
