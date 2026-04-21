package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/client"

	"github.com/Ab-dex/view-aura/internal/modules/upload/domain"
	"github.com/Ab-dex/view-aura/internal/modules/upload/repository"
	"github.com/Ab-dex/view-aura/internal/platform/resilience"
)

const (
	temporalDeferredTTL    = 48 * time.Hour
	maxTemporalDeferredLen = 5_000
)

// DeferredWorkflowInput is what we persist when Temporal is unavailable.
// It mirrors workflows/upload_pipeline.UploadPipelineInput exactly so the
// OutboxWorker can start the workflow without any translation.
type DeferredWorkflowInput struct {
	WorkflowID  string `json:"workflow_id"`
	WorkflowFn  string `json:"workflow_fn"` // "upload_pipeline"
	UploadID    string `json:"upload_id"`
	AssetID     string `json:"asset_id"`
	UserID      string `json:"user_id"`
	AssetKey    string `json:"asset_key"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	QueuedAt    string `json:"queued_at"`
}

// ResilientTemporalClient wraps the raw Temporal client with three-tier
// degradation so a Temporal outage does not fail the upload complete flow:
//
//	Tier 1  Start workflow via Temporal SDK
//	Tier 2  LPUSH to Redis deferred queue (OutboxWorker starts workflow on recovery)
//	Tier 3  FileOutbox JSONL + mark asset status = "processing_deferred"
//
// A nil temporalClient skips Tier 1 (useful when Temporal is not configured).
// A nil rdb skips Tier 2.
type ResilientTemporalClient struct {
	temporal    client.Client // may be nil
	breaker     *resilience.InProcessBreaker
	rdb         goredis.UniversalClient
	assets      repository.MediaAssetRepository
	outbox      *resilience.FileOutbox
	deferredKey string
	taskQueue   string
}

// NewResilientTemporalClient builds the wrapper.
//
//   - temporalClient: pass nil when Temporal is not configured/reachable at startup
//   - rdb:            pass nil when Redis is not configured
//   - assets:         MediaAssetRepository to mark deferred assets
//   - logDir:         Tier 3 log directory
//   - deferredKey:    Redis list key (from config)
//   - taskQueue:      Temporal task queue name (from config)
func NewResilientTemporalClient(
	temporalClient client.Client,
	rdb goredis.UniversalClient,
	assets repository.MediaAssetRepository,
	logDir, deferredKey, taskQueue string,
) *ResilientTemporalClient {
	if deferredKey == "" {
		deferredKey = "outbox:temporal:deferred"
	}
	if taskQueue == "" {
		taskQueue = "viewaura-main"
	}
	return &ResilientTemporalClient{
		temporal:    temporalClient,
		breaker:     resilience.NewInProcessBreaker("temporal", 3, 60*time.Second),
		rdb:         rdb,
		assets:      assets,
		outbox:      resilience.NewFileOutbox(logDir, "temporal"),
		deferredKey: deferredKey,
		taskQueue:   taskQueue,
	}
}

// StartUploadPipeline starts the upload_pipeline Temporal workflow for the
// given completed upload.  On degradation the asset row is updated to
// "processing_deferred" so the client sees a meaningful status.
func (r *ResilientTemporalClient) StartUploadPipeline(
	ctx context.Context,
	input DeferredWorkflowInput,
) error {
	// ── Tier 1: Temporal SDK ──────────────────────────────────────────────────
	if r.temporal != nil && r.breaker.Allow() {
		err := r.startWorkflow(ctx, input)
		if err == nil {
			r.breaker.RecordSuccess()
			return nil
		}
		r.breaker.RecordFailure()
		log.Warn().
			Err(err).
			Str("upload_id", input.UploadID).
			Str("asset_id", input.AssetID).
			Msg("resilient_temporal: workflow start failed — queuing in redis")
	} else if r.breaker.IsOpen() {
		log.Warn().
			Str("upload_id", input.UploadID).
			Msg("resilient_temporal: circuit open — queuing in redis")
	}

	// ── Tier 2: Redis deferred queue ──────────────────────────────────────────
	if r.rdb != nil {
		if err := r.pushToRedis(ctx, input); err == nil {
			// Mark the asset so the user sees "processing_deferred" not "uploading".
			r.markDeferred(ctx, input.AssetID)
			log.Info().
				Str("asset_id", input.AssetID).
				Msg("resilient_temporal: workflow deferred to redis queue")
			return nil
		}
		log.Warn().
			Str("asset_id", input.AssetID).
			Msg("resilient_temporal: redis queue failed — writing to file outbox")
	}

	// ── Tier 3: File outbox ───────────────────────────────────────────────────
	r.outbox.Write("temporal:deferred", input.UserID, input)
	r.markDeferred(ctx, input.AssetID)
	log.Error().
		Str("asset_id", input.AssetID).
		Msg("resilient_temporal: workflow written to file outbox (temporal + redis both unavailable)")

	// Return nil — the upload completed successfully from the user's perspective.
	// The pipeline will start when the systems recover.
	return nil
}

func (r *ResilientTemporalClient) startWorkflow(ctx context.Context, input DeferredWorkflowInput) error {
	opts := client.StartWorkflowOptions{
		ID:        input.WorkflowID,
		TaskQueue: r.taskQueue,
	}
	// The workflow function is referenced by name so this package does not
	// import the workflows package (avoids circular dependency).
	_, err := r.temporal.ExecuteWorkflow(ctx, opts, input.WorkflowFn, input)
	if err != nil {
		return fmt.Errorf("temporal: start %s: %w", input.WorkflowFn, err)
	}
	return nil
}

func (r *ResilientTemporalClient) pushToRedis(ctx context.Context, input DeferredWorkflowInput) error {
	input.QueuedAt = time.Now().UTC().Format(time.RFC3339)
	raw, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("resilient_temporal: marshal: %w", err)
	}

	tctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	pipe := r.rdb.Pipeline()
	pipe.LPush(tctx, r.deferredKey, string(raw))
	pipe.LTrim(tctx, r.deferredKey, 0, maxTemporalDeferredLen-1)
	pipe.Expire(tctx, r.deferredKey, temporalDeferredTTL)

	if _, err := pipe.Exec(tctx); err != nil {
		return fmt.Errorf("resilient_temporal: redis pipeline: %w", err)
	}
	return nil
}

// markDeferred updates the asset status to "processing_deferred" so the
// user's UI shows "Your upload is being processed" rather than stale state.
// Non-fatal — logged but never propagated.
func (r *ResilientTemporalClient) markDeferred(ctx context.Context, assetID string) {
	if err := r.assets.UpdateStatus(ctx, assetID, "processing_deferred"); err != nil {
		log.Warn().Err(err).Str("asset_id", assetID).
			Msg("resilient_temporal: could not mark asset as processing_deferred")
	}
}

// DrainDeferredQueue is called by the OutboxWorker to replay deferred workflows
// when Temporal recovers.  Returns the number of successfully started workflows.
func (r *ResilientTemporalClient) DrainDeferredQueue(ctx context.Context) int {
	if r.rdb == nil || r.temporal == nil {
		return 0
	}

	var started int
	for {
		val, err := r.rdb.RPop(ctx, r.deferredKey).Result()
		if err == goredis.Nil {
			break
		}
		if err != nil {
			log.Warn().Err(err).Msg("resilient_temporal: drain rpop failed")
			break
		}

		var input DeferredWorkflowInput
		if err := json.Unmarshal([]byte(val), &input); err != nil {
			log.Warn().Err(err).Msg("resilient_temporal: unmarshal deferred input")
			continue
		}

		if err := r.startWorkflow(ctx, input); err != nil {
			// Temporal still down — put it back and stop for this tick.
			_ = r.rdb.RPush(ctx, r.deferredKey, val)
			log.Debug().Msg("resilient_temporal: still unavailable — re-queued")
			break
		}

		// Restore normal status now that the workflow has started.
		if err := r.assets.UpdateStatus(ctx, input.AssetID, string(domain.StatusProcessing)); err != nil {
			log.Warn().Err(err).Str("asset_id", input.AssetID).
				Msg("resilient_temporal: could not update asset status after deferred start")
		}
		started++
	}
	return started
}
