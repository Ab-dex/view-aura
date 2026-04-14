package main

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/Ab-dex/view-aura/internal/events"
	"github.com/Ab-dex/view-aura/internal/platform/cache"
	"github.com/Ab-dex/view-aura/internal/platform/config"
	"github.com/Ab-dex/view-aura/internal/platform/db"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
	r2pkg "github.com/Ab-dex/view-aura/internal/platform/r2"
	stripepkg "github.com/Ab-dex/view-aura/internal/platform/stripe"
	// Workflow + activity definitions — imported for side-effect registration.
	// _ "github.com/Ab-dex/view-aura/workflows/enrichment_pipeline"
	// _ "github.com/Ab-dex/view-aura/workflows/moderation_saga"
	// _ "github.com/Ab-dex/view-aura/workflows/upload_pipeline"
)

// Worker is the root struct for the Temporal worker process.
type Worker struct {
	cfg      *config.Config
	db       *db.Pool
	redis    *cache.Client
	r2       *r2pkg.Client
	stripe   *stripepkg.Client
	temporal client.Client
	producer events.Producer
	consumer events.Consumer
}

// NewWorker is the Wire-injectable constructor.
func NewWorker(
	cfg *config.Config,
	pool *db.Pool,
	redis *cache.Client,
	r2 *r2pkg.Client,
	stripe *stripepkg.Client,
	temporal client.Client,
	producer events.Producer,
	consumer events.Consumer,
) *Worker {
	return &Worker{
		cfg:      cfg,
		db:       pool,
		redis:    redis,
		r2:       r2,
		stripe:   stripe,
		temporal: temporal,
		producer: producer,
		consumer: consumer,
	}
}

// Run starts the Temporal worker and the Kafka consumer loop.
// Blocks until ctx is cancelled (e.g. SIGTERM received in main.go).
func (w *Worker) Run(ctx context.Context) error {
	// 1. Initialize Temporal Worker
	tw := worker.New(w.temporal, w.cfg.Temporal.TaskQueue, worker.Options{
		MaxConcurrentActivityExecutionSize:     10,
		MaxConcurrentWorkflowTaskExecutionSize: 10,
	})

	// Start Temporal (this is non-blocking)
	if err := tw.Start(); err != nil {
		return fmt.Errorf("temporal worker start: %w", err)
	}
	// Ensure Temporal stops when this function exits
	defer tw.Stop()

	// 2. Configure Kafka Consumer
	if w.consumer != nil {
		topics := []string{
			string(events.TopicUploadCompleted),
			string(events.TopicModerationSubmitted),
		}

		// Subscribe is a separate call (non-blocking)
		if err := w.consumer.Subscribe(topics); err != nil {
			return fmt.Errorf("kafka subscribe: %w", err)
		}

		// Run is blocking. It will hold the function here until ctx is cancelled.
		// When Run returns, the 'defer tw.Stop()' above will trigger.
		if err := w.consumer.Run(ctx, w.dispatchEvent); err != nil {
			return fmt.Errorf("kafka consumer run: %w", err)
		}

		return nil
	}

	// 3. Fallback for local dev (No Kafka)
	logger.FromContext(context.Background()).Info().Msg("no kafka consumer configured, blocking on context only")
	<-ctx.Done()
	return nil
}

// dispatchEvent routes an incoming Kafka Envelope to the correct handler.
func (w *Worker) dispatchEvent(ctx context.Context, env events.Envelope) error {
	switch env.Topic {
	case events.TopicUploadCompleted:
		return w.handleUploadCompleted(ctx, env)
	case events.TopicModerationSubmitted:
		return w.handleModerationSubmitted(ctx, env)
	default:
		// Unknown topic — acknowledge and skip.
		return nil
	}
}

// handleUploadCompleted triggers the upload_pipeline Temporal workflow.
func (w *Worker) handleUploadCompleted(ctx context.Context, env events.Envelope) error {
	_, err := w.temporal.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "upload-pipeline-" + env.ID,
			TaskQueue: w.cfg.Temporal.TaskQueue,
		},
		"UploadPipelineWorkflow", // registered by workflows/upload_pipeline import
		env.Payload,
	)
	return err
}

// handleModerationSubmitted triggers the moderation_saga Temporal workflow.
func (w *Worker) handleModerationSubmitted(ctx context.Context, env events.Envelope) error {
	_, err := w.temporal.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "moderation-saga-" + env.ID,
			TaskQueue: w.cfg.Temporal.TaskQueue,
		},
		"ModerationSagaWorkflow", // registered by workflows/moderation_saga import
		env.Payload,
	)
	return err
}
