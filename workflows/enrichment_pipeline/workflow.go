package enrichment_pipeline

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ─── Input / Output ───────────────────────────────────────────────────────────

// EnrichmentPipelineInput is triggered when a new movie is created in the
// catalog (events.MovieCreated) or when an admin requests a manual re-enrichment.
type EnrichmentPipelineInput struct {
	MovieID string `json:"movie_id"`
	IMDbID  string `json:"imdb_id,omitempty"`
	TMDbID  int    `json:"tmdb_id,omitempty"`
	// Priority controls queue ordering in the worker.
	// "high" for new releases, "normal" for background enrichment.
	Priority string `json:"priority"`
}

type EnrichmentPipelineOutput struct {
	MovieID        string
	FieldsEnriched []string // list of fields that were updated
	SearchIndexed  bool
}

// ─── Workflow ─────────────────────────────────────────────────────────────────

// EnrichmentPipelineWorkflow fetches metadata from 5 external APIs (TMDB,
// IMDb, JustWatch, Rotten Tomatoes, MusicBrainz), merges the results,
// updates the movie record, and triggers a search re-index.
//
// Flow:
//
//	FetchTMDbMetadata         (parallel)
//	FetchIMDbMetadata         (parallel)
//	FetchJustWatchLinks       (parallel)
//	  → WaitForAllFetches
//	  → MergeAndUpdateMovie
//	  → DownloadAndStorePosters
//	  → TriggerSearchIndex
//	  → NotifyEnrichmentComplete
func EnrichmentPipelineWorkflow(ctx workflow.Context, input EnrichmentPipelineInput) (*EnrichmentPipelineOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("enrichment_pipeline: started", "movie_id", input.MovieID)

	actOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    5,
			InitialInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    2 * time.Minute,
			// External API rate limits and transient 5xx are retryable.
			NonRetryableErrorTypes: []string{"NotFoundError"},
		},
	}
	ctx = workflow.WithActivityOptions(ctx, actOpts)

	// ── Stage 1: Parallel external API fetches ────────────────────────────────
	var tmdbResult TMDbResult
	var imdbResult IMDbResult
	var justwatchResult JustWatchResult

	tmdbF := workflow.ExecuteActivity(ctx, FetchTMDbMetadataActivity, FetchTMDbInput{
		MovieID: input.MovieID,
		TMDbID:  input.TMDbID,
	})
	imdbF := workflow.ExecuteActivity(ctx, FetchIMDbMetadataActivity, FetchIMDbInput{
		MovieID: input.MovieID,
		IMDbID:  input.IMDbID,
	})
	justwatchF := workflow.ExecuteActivity(ctx, FetchJustWatchLinksActivity, FetchJustWatchInput{
		MovieID: input.MovieID,
		IMDbID:  input.IMDbID,
	})

	// Collect results — non-fatal if individual sources fail.
	if err := tmdbF.Get(ctx, &tmdbResult); err != nil {
		logger.Warn("enrichment_pipeline: TMDB fetch failed", "error", err)
	}
	if err := imdbF.Get(ctx, &imdbResult); err != nil {
		logger.Warn("enrichment_pipeline: IMDb fetch failed", "error", err)
	}
	if err := justwatchF.Get(ctx, &justwatchResult); err != nil {
		logger.Warn("enrichment_pipeline: JustWatch fetch failed", "error", err)
	}

	// ── Stage 2: Merge and update the movie record ────────────────────────────
	var mergeResult MergeResult
	if err := workflow.ExecuteActivity(ctx, MergeAndUpdateMovieActivity, MergeInput{
		MovieID:   input.MovieID,
		TMDb:      tmdbResult,
		IMDb:      imdbResult,
		JustWatch: justwatchResult,
	}).Get(ctx, &mergeResult); err != nil {
		logger.Error("enrichment_pipeline: merge failed", "error", err)
		return nil, err
	}

	// ── Stage 3: Download and store poster / backdrop / trailer ──────────────
	if len(mergeResult.ImageURLs) > 0 {
		if err := workflow.ExecuteActivity(ctx, DownloadAndStoreImagesActivity, ImageInput{
			MovieID:   input.MovieID,
			ImageURLs: mergeResult.ImageURLs,
		}).Get(ctx, nil); err != nil {
			// Image download failure is non-fatal.
			logger.Warn("enrichment_pipeline: image download failed", "error", err)
		}
	}

	// ── Stage 4: Trigger search re-index ─────────────────────────────────────
	indexed := false
	if err := workflow.ExecuteActivity(ctx, TriggerSearchIndexActivity, SearchIndexInput{
		MovieID:  input.MovieID,
		Priority: input.Priority,
	}).Get(ctx, nil); err != nil {
		logger.Warn("enrichment_pipeline: search index trigger failed", "error", err)
	} else {
		indexed = true
	}

	logger.Info("enrichment_pipeline: completed",
		"movie_id", input.MovieID,
		"fields_enriched", mergeResult.FieldsUpdated,
	)

	return &EnrichmentPipelineOutput{
		MovieID:        input.MovieID,
		FieldsEnriched: mergeResult.FieldsUpdated,
		SearchIndexed:  indexed,
	}, nil
}
