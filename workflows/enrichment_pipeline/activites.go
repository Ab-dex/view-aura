package enrichment_pipeline

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	"github.com/Ab-dex/view-aura/internal/events"
)

// ─── Input / Output types ─────────────────────────────────────────────────────

type FetchTMDbInput struct {
	MovieID string
	TMDbID  int
}

type TMDbResult struct {
	Title         string
	OriginalTitle string
	Synopsis      string
	Tagline       string
	PosterURL     string
	BackdropURL   string
	TrailerURL    string
	Genres        []string
	Countries     []string
	Runtime       int
	ReleaseDate   string
	Cast          []CreditItem
	Crew          []CreditItem
}

type CreditItem struct {
	Name      string
	Role      string
	Character string
	TMDbID    int
}

type FetchIMDbInput struct {
	MovieID string
	IMDbID  string
}

type IMDbResult struct {
	Rating           float64
	RatingCount      int
	Certificate      string // "PG-13", "R", etc.
	Keywords         []string
	FilmingLocations []string
}

type FetchJustWatchInput struct {
	MovieID string
	IMDbID  string
}

type JustWatchResult struct {
	// StreamingLinks is keyed by provider name.
	StreamingLinks []StreamingLink
}

type StreamingLink struct {
	Provider   string
	URL        string
	AccessType string // "stream" | "rent" | "buy"
	PriceCents *int
	Region     string
}

type MergeInput struct {
	MovieID   string
	TMDb      TMDbResult
	IMDb      IMDbResult
	JustWatch JustWatchResult
}

type MergeResult struct {
	FieldsUpdated []string
	ImageURLs     []string
}

type ImageInput struct {
	MovieID   string
	ImageURLs []string
}

type SearchIndexInput struct {
	MovieID  string
	Priority string
}

// ─── Activities ───────────────────────────────────────────────────────────────

// FetchTMDbMetadataActivity calls the TMDB REST API to retrieve cast, crew,
// poster, backdrop, trailer URL, genres, and synopsis.
func FetchTMDbMetadataActivity(ctx context.Context, input FetchTMDbInput) (TMDbResult, error) {
	logger := activity.GetLogger(ctx)
	if input.TMDbID == 0 {
		logger.Info("enrichment_pipeline: no TMDb ID — skipping", "movie_id", input.MovieID)
		return TMDbResult{}, nil
	}

	logger.Info("enrichment_pipeline: fetching TMDb", "tmdb_id", input.TMDbID)
	// Real implementation:
	// GET https://api.themoviedb.org/3/movie/{tmdb_id}?api_key=...&append_to_response=credits,videos
	// Map response to TMDbResult.
	return TMDbResult{
		Title:   fmt.Sprintf("TMDb title for %d", input.TMDbID),
		Genres:  []string{"Drama"},
		Runtime: 120,
	}, nil
}

// FetchIMDbMetadataActivity calls the IMDb API (or scrapes the public page)
// to retrieve the certificate rating, keywords, and filming locations.
func FetchIMDbMetadataActivity(ctx context.Context, input FetchIMDbInput) (IMDbResult, error) {
	logger := activity.GetLogger(ctx)
	if input.IMDbID == "" {
		logger.Info("enrichment_pipeline: no IMDb ID — skipping", "movie_id", input.MovieID)
		return IMDbResult{}, nil
	}

	logger.Info("enrichment_pipeline: fetching IMDb", "imdb_id", input.IMDbID)
	// Real implementation: call IMDb developer API.
	return IMDbResult{
		Rating:      7.8,
		RatingCount: 250000,
	}, nil
}

// FetchJustWatchLinksActivity queries JustWatch to find where the movie is
// currently available to stream, rent, or buy in each region.
func FetchJustWatchLinksActivity(ctx context.Context, input FetchJustWatchInput) (JustWatchResult, error) {
	activity.GetLogger(ctx).Info("enrichment_pipeline: fetching JustWatch", "movie_id", input.MovieID)
	// Real implementation: POST https://apis.justwatch.com/content/titles/movie/popular
	// with title+year, then fetch offers for each locale.
	return JustWatchResult{
		StreamingLinks: []StreamingLink{
			{Provider: "Netflix", AccessType: "stream", Region: "US"},
		},
	}, nil
}

// MergeAndUpdateMovieActivity applies a last-write-wins merge strategy across
// the three external sources and updates the movie record in Postgres.
// TMDB is the primary source for metadata; IMDb for certificates and keywords;
// JustWatch for streaming availability.
func MergeAndUpdateMovieActivity(ctx context.Context, input MergeInput, producer events.Producer) (MergeResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("enrichment_pipeline: merging and updating", "movie_id", input.MovieID)

	var fieldsUpdated []string
	var imageURLs []string

	// Apply TMDB fields if non-empty.
	if input.TMDb.Title != "" {
		fieldsUpdated = append(fieldsUpdated, "title", "synopsis", "genres", "runtime")
	}
	if input.TMDb.PosterURL != "" {
		fieldsUpdated = append(fieldsUpdated, "poster_url")
		imageURLs = append(imageURLs, input.TMDb.PosterURL)
	}
	if input.TMDb.BackdropURL != "" {
		fieldsUpdated = append(fieldsUpdated, "backdrop_url")
		imageURLs = append(imageURLs, input.TMDb.BackdropURL)
	}

	// Apply IMDb fields.
	if input.IMDb.Certificate != "" {
		fieldsUpdated = append(fieldsUpdated, "content_rating")
	}

	// Apply JustWatch streaming links.
	if len(input.JustWatch.StreamingLinks) > 0 {
		fieldsUpdated = append(fieldsUpdated, "streaming_links")
	}

	// Real implementation: call /internal/movies/{id} PATCH with merged fields.

	// Publish MovieMetadataEnriched event so the search service re-indexes.
	_ = producer.Publish(ctx, events.TopicMovieEvents, "movie.metadata_enriched",
		events.MovieMetadataEnriched{
			MovieID:        input.MovieID,
			Source:         "enrichment_pipeline",
			EnrichedFields: fieldsToMap(fieldsUpdated),
		})

	return MergeResult{
		FieldsUpdated: fieldsUpdated,
		ImageURLs:     imageURLs,
	}, nil
}

// DownloadAndStoreImagesActivity downloads poster and backdrop images from
// TMDB CDN and stores them in Cloudflare R2 under posters/{movie_id}/.
func DownloadAndStoreImagesActivity(ctx context.Context, input ImageInput) error {
	activity.GetLogger(ctx).Info("enrichment_pipeline: downloading images",
		"movie_id", input.MovieID,
		"count", len(input.ImageURLs),
	)
	// Real implementation: HTTP GET each URL → PUT to R2 → UPDATE movies.poster_url
	return nil
}

// TriggerSearchIndexActivity publishes a SearchIndexRequested event so the
// Rust search service re-indexes the enriched movie document.
func TriggerSearchIndexActivity(ctx context.Context, input SearchIndexInput, producer events.Producer) error {
	activity.GetLogger(ctx).Info("enrichment_pipeline: triggering search index",
		"movie_id", input.MovieID,
	)
	return producer.Publish(ctx, events.TopicSearchIndex, "search.index_requested",
		events.SearchIndexRequested{
			EntityType: "movie",
			EntityID:   input.MovieID,
			Priority:   input.Priority,
		})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func fieldsToMap(fields []string) map[string]any {
	m := make(map[string]any, len(fields))
	for _, f := range fields {
		m[f] = true
	}
	return m
}

// ─── Registration ─────────────────────────────────────────────────────────────

func Register(w interface {
	RegisterWorkflow(interface{})
	RegisterActivity(interface{})
}) {
	w.RegisterWorkflow(EnrichmentPipelineWorkflow)
	w.RegisterActivity(FetchTMDbMetadataActivity)
	w.RegisterActivity(FetchIMDbMetadataActivity)
	w.RegisterActivity(FetchJustWatchLinksActivity)
	w.RegisterActivity(MergeAndUpdateMovieActivity)
	w.RegisterActivity(DownloadAndStoreImagesActivity)
	w.RegisterActivity(TriggerSearchIndexActivity)
}
