package upload_pipeline

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ─── Input / Output ───────────────────────────────────────────────────────────

// UploadPipelineInput is the payload unmarshalled from the Kafka
// uploads.completed envelope. Must match events.UploadCompleted exactly.
type UploadPipelineInput struct {
	UploadID    string `json:"upload_id"`
	AssetID     string `json:"asset_id"`
	UserID      string `json:"user_id"`
	AssetKey    string `json:"asset_key"` // R2 object key: uploads/{uid}/{id}/{filename}
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

// UploadPipelineOutput carries the final state of the pipeline.
type UploadPipelineOutput struct {
	AssetID    string
	StorageKey string // delivery key in R2 (differs from ingest key after copy)
	StreamURL  string // Cloudflare Stream HLS URL (empty until CDN publish)
	Status     string // "published" | "quarantined" | "failed"
}

// ─── Workflow ─────────────────────────────────────────────────────────────────

// UploadPipelineWorkflow is the Temporal workflow that orchestrates the full
// media processing pipeline from a raw R2 upload to a live CDN asset.
//
// Stage sequence:
//
//	ValidateFile
//	  → TranscodeVideo           (parallel with ExtractMetadata)
//	  → ExtractMetadata          (parallel with TranscodeVideo)
//	  → ScreenContent            (AI moderation)
//	  → if approved  → PublishToCDN → NotifyUploader(success)
//	  → if flagged   → QuarantineAsset → NotifyUploader(pending review)
//	  → if any error → CompensateAndCleanup
//
// Temporal's built-in retry + durable execution guarantees that a pod restart
// mid-pipeline does not restart from step 1.
func UploadPipelineWorkflow(ctx workflow.Context, input UploadPipelineInput) (*UploadPipelineOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("upload_pipeline: started",
		"upload_id", input.UploadID,
		"asset_id", input.AssetID,
		"size_bytes", input.SizeBytes,
	)

	// Activity options — applied to all activities unless overridden.
	actOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:        3,
			InitialInterval:        10 * time.Second,
			BackoffCoefficient:     2.0,
			MaximumInterval:        5 * time.Minute,
			NonRetryableErrorTypes: []string{"ValidationError", "QuotaError"},
		},
	}
	ctx = workflow.WithActivityOptions(ctx, actOpts)

	// ── Stage 1: Validate ────────────────────────────────────────────────────
	var validationResult ValidationResult
	if err := workflow.ExecuteActivity(ctx, ValidateFileActivity, ValidationInput{
		AssetKey:    input.AssetKey,
		ContentType: input.ContentType,
		SizeBytes:   input.SizeBytes,
		UserID:      input.UserID,
	}).Get(ctx, &validationResult); err != nil {
		logger.Error("upload_pipeline: validation failed", "error", err)
		_ = runCompensation(ctx, input, "validation_failed: "+err.Error())
		return &UploadPipelineOutput{AssetID: input.AssetID, Status: "failed"}, nil
	}

	// ── Stage 2: Transcode + Metadata (parallel) ─────────────────────────────
	transcodeCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 4 * time.Hour, // large files can take hours
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    30 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    10 * time.Minute,
		},
	})

	var transcodeResult TranscodeResult
	transcodeF := workflow.ExecuteActivity(transcodeCtx, TranscodeVideoActivity, TranscodeInput{
		AssetKey:    input.AssetKey,
		AssetID:     input.AssetID,
		ContentType: input.ContentType,
	})

	var metaResult MetadataResult
	metaF := workflow.ExecuteActivity(ctx, ExtractMetadataActivity, MetadataInput{
		AssetKey: input.AssetKey,
		AssetID:  input.AssetID,
	})

	if err := transcodeF.Get(ctx, &transcodeResult); err != nil {
		logger.Error("upload_pipeline: transcode failed", "error", err)
		_ = runCompensation(ctx, input, "transcode_failed: "+err.Error())
		return &UploadPipelineOutput{AssetID: input.AssetID, Status: "failed"}, nil
	}
	if err := metaF.Get(ctx, &metaResult); err != nil {
		// Metadata failure is non-fatal — log and continue.
		logger.Warn("upload_pipeline: metadata extraction failed", "error", err)
	}

	// ── Stage 3: AI Moderation Screen ────────────────────────────────────────
	var screenResult ScreenResult
	if err := workflow.ExecuteActivity(ctx, ScreenContentActivity, ScreenInput{
		AssetID:     input.AssetID,
		AssetKey:    input.AssetKey,
		ContentType: input.ContentType,
	}).Get(ctx, &screenResult); err != nil {
		logger.Error("upload_pipeline: content screening failed", "error", err)
		_ = runCompensation(ctx, input, "screening_error: "+err.Error())
		return &UploadPipelineOutput{AssetID: input.AssetID, Status: "failed"}, nil
	}

	// ── Stage 4: Publish or Quarantine ───────────────────────────────────────
	if screenResult.Flagged {
		logger.Info("upload_pipeline: asset flagged — quarantining",
			"signals", screenResult.Signals,
		)
		if err := workflow.ExecuteActivity(ctx, QuarantineAssetActivity, QuarantineInput{
			AssetID:  input.AssetID,
			UploadID: input.UploadID,
			UserID:   input.UserID,
			Signals:  screenResult.Signals,
		}).Get(ctx, nil); err != nil {
			logger.Error("upload_pipeline: quarantine failed", "error", err)
		}
		_ = workflow.ExecuteActivity(ctx, NotifyUploaderActivity, NotifyInput{
			UserID:   input.UserID,
			UploadID: input.UploadID,
			AssetID:  input.AssetID,
			Event:    "quarantined",
			Message:  "Your upload is pending moderation review.",
		}).Get(ctx, nil)
		return &UploadPipelineOutput{
			AssetID: input.AssetID,
			Status:  "quarantined",
		}, nil
	}

	// Content approved — publish to CDN.
	var publishResult PublishResult
	if err := workflow.ExecuteActivity(ctx, PublishToCDNActivity, PublishInput{
		AssetID:        input.AssetID,
		TranscodedKeys: transcodeResult.TranscodedKeys,
		UserID:         input.UserID,
	}).Get(ctx, &publishResult); err != nil {
		logger.Error("upload_pipeline: CDN publish failed", "error", err)
		_ = runCompensation(ctx, input, "publish_failed: "+err.Error())
		return &UploadPipelineOutput{AssetID: input.AssetID, Status: "failed"}, nil
	}

	_ = workflow.ExecuteActivity(ctx, NotifyUploaderActivity, NotifyInput{
		UserID:   input.UserID,
		UploadID: input.UploadID,
		AssetID:  input.AssetID,
		Event:    "published",
		Message:  "Your upload is live.",
	}).Get(ctx, nil)

	logger.Info("upload_pipeline: completed",
		"asset_id", input.AssetID,
		"stream_url", publishResult.StreamURL,
	)

	return &UploadPipelineOutput{
		AssetID:    input.AssetID,
		StorageKey: transcodeResult.DeliveryKey,
		StreamURL:  publishResult.StreamURL,
		Status:     "published",
	}, nil
}

// runCompensation executes cleanup activities in reverse order.
// Temporal's durable execution ensures compensations survive pod restarts.
func runCompensation(ctx workflow.Context, input UploadPipelineInput, reason string) error {
	compensateCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 5,
		},
	})

	// 1. Delete any partial transcoded segments from R2.
	_ = workflow.ExecuteActivity(compensateCtx, DeleteTranscodedSegmentsActivity, input.AssetKey).Get(ctx, nil)

	// 2. Mark the asset as failed in Postgres.
	_ = workflow.ExecuteActivity(compensateCtx, MarkAssetFailedActivity, input.AssetID, reason).Get(ctx, nil)

	// 3. Release quota reservation for this upload.
	_ = workflow.ExecuteActivity(compensateCtx, ReleaseQuotaActivity, input.UserID, input.SizeBytes).Get(ctx, nil)

	// 4. Notify the uploader of the failure.
	_ = workflow.ExecuteActivity(compensateCtx, NotifyUploaderActivity, NotifyInput{
		UserID:   input.UserID,
		UploadID: input.UploadID,
		AssetID:  input.AssetID,
		Event:    "failed",
		Message:  "Your upload could not be processed. " + reason,
	}).Get(ctx, nil)

	return nil
}
