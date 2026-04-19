use anyhow::Context;
use serde::Deserialize;
use tracing::{debug, info};

use crate::indexer::{documents::MovieDocument, MeiliSearchClient};

/// Envelope matches the events.Envelope struct published by the Go monolith.
#[derive(Debug, Deserialize)]
pub struct Envelope {
    pub id:         String,
    pub event_type: String,
    pub payload:    serde_json::Value,
}

// ─── Inbound event payloads ───────────────────────────────────────────────────

/// SearchIndexRequested (topic: search_index)
#[derive(Debug, Deserialize)]
struct SearchIndexRequested {
    entity_type:  String,
    entity_id:    String,
    priority:     Option<String>,
}

/// SearchIndexDeleted (topic: search_index)
#[derive(Debug, Deserialize)]
struct SearchIndexDeleted {
    entity_type: String,
    entity_id:   String,
}

/// MovieCreated / MoviePublished (topic: movie.events)
/// These carry a full denormalised movie payload so the search service
/// can build the index document without calling the Go API.
#[derive(Debug, Deserialize)]
struct MovieEventPayload {
    id:               String,
    title:            String,
    original_title:   Option<String>,
    slug:             String,
    synopsis:         Option<String>,
    tagline:          Option<String>,
    release_date:     Option<String>,
    runtime_mins:     Option<i32>,
    content_rating:   Option<String>,
    status:           String,
    original_lang:    Option<String>,
    genres:           Option<Vec<String>>,
    countries:        Option<Vec<String>>,
    poster_url:       Option<String>,
    backdrop_url:     Option<String>,
    trailer_url:      Option<String>,
    imdb_id:          Option<String>,
    tmdb_id:          Option<i32>,
    avg_rating:       Option<f64>,
    rating_count:     Option<i64>,
    popularity_score: Option<f64>,
    cast_names:       Option<Vec<String>>,
    director_names:   Option<Vec<String>>,
}

/// MovieRatingAggregateUpdated (topic: movie.events)
#[derive(Debug, Deserialize)]
struct RatingAggregateUpdated {
    movie_id:        String,
    avg_overall:     f64,
    rating_count:    i64,
    popularity:      f64,
}

// ─── Dispatcher ───────────────────────────────────────────────────────────────

pub async fn handle_event(
    envelope: &Envelope,
    meili: &MeiliSearchClient,
) -> anyhow::Result<()> {
    debug!(event_type = %envelope.event_type, "handling event");

    match envelope.event_type.as_str() {
        // Full index upsert for movies
        "movie.created" | "movie.published" | "movie.updated" => {
            handle_movie_upsert(&envelope.payload, meili).await
        }
        // Lightweight rating-only update
        "movie.rating_aggregate_updated" => {
            handle_rating_update(&envelope.payload, meili).await
        }
        // Index or delete request from any producer
        "search.index_requested" => {
            handle_index_requested(&envelope.payload, meili).await
        }
        "search.index_deleted" => {
            handle_index_deleted(&envelope.payload, meili).await
        }
        // Archived movies are removed from the index
        "movie.archived" => {
            let id = envelope.payload
                .get("movie_id")
                .and_then(|v| v.as_str())
                .context("missing movie_id in movie.archived")?;
            meili.delete_movie(id).await
        }
        other => {
            debug!(event_type = %other, "ignoring unknown event type");
            Ok(())
        }
    }
}

async fn handle_movie_upsert(
    payload: &serde_json::Value,
    meili: &MeiliSearchClient,
) -> anyhow::Result<()> {
    let p: MovieEventPayload = serde_json::from_value(payload.clone())
        .context("deserialising MovieEventPayload")?;

    // Only index published movies — drafts and archived stay out.
    if p.status != "published" {
        debug!(movie_id = %p.id, status = %p.status, "skipping non-published movie");
        return Ok(());
    }

    let release_year = p.release_date.as_deref()
        .and_then(|d| d.split('-').next())
        .and_then(|y| y.parse::<i32>().ok());

    let doc = MovieDocument {
        id:               p.id.clone(),
        title:            p.title,
        original_title:   p.original_title,
        slug:             p.slug,
        synopsis:         p.synopsis,
        tagline:          p.tagline,
        release_date:     p.release_date,
        release_year,
        runtime_mins:     p.runtime_mins,
        content_rating:   p.content_rating,
        status:           p.status,
        original_lang:    p.original_lang.unwrap_or_else(|| "en".to_string()),
        genres:           p.genres.unwrap_or_default(),
        countries:        p.countries.unwrap_or_default(),
        poster_url:       p.poster_url,
        backdrop_url:     p.backdrop_url,
        trailer_url:      p.trailer_url,
        imdb_id:          p.imdb_id,
        tmdb_id:          p.tmdb_id,
        avg_rating:       p.avg_rating.unwrap_or(0.0),
        rating_count:     p.rating_count.unwrap_or(0),
        popularity_score: p.popularity_score.unwrap_or(0.0),
        streaming_providers: vec![],  // hydrated by StreamingAvailabilityChanged
        cast_names:       p.cast_names.unwrap_or_default(),
        director_names:   p.director_names.unwrap_or_default(),
    };

    meili.upsert_movie(&doc).await?;
    info!(movie_id = %p.id, "movie indexed");
    Ok(())
}

async fn handle_rating_update(
    payload: &serde_json::Value,
    meili: &MeiliSearchClient,
) -> anyhow::Result<()> {
    let p: RatingAggregateUpdated = serde_json::from_value(payload.clone())
        .context("deserialising RatingAggregateUpdated")?;

    // Partial update: fetch existing document, patch rating fields, re-upsert.
    // MeiliSearch doesn't support partial updates on individual fields, so we
    // use upsert with only the updated fields populated.
    let patch = serde_json::json!({
        "id":               p.movie_id,
        "avg_rating":       p.avg_overall,
        "rating_count":     p.rating_count,
        "popularity_score": p.popularity,
    });

    meili.inner
        .index(&meili.cfg.movie_index)
        .add_or_update(&[patch], Some("id"))
        .await
        .context("patch rating aggregate")?;

    info!(movie_id = %p.movie_id, avg = p.avg_overall, "rating aggregate updated in index");
    Ok(())
}

async fn handle_index_requested(
    payload: &serde_json::Value,
    meili: &MeiliSearchClient,
) -> anyhow::Result<()> {
    let p: SearchIndexRequested = serde_json::from_value(payload.clone())
        .context("deserialising SearchIndexRequested")?;

    info!(
        entity_type = %p.entity_type,
        entity_id = %p.entity_id,
        priority = ?p.priority,
        "search index requested — requires full document fetch from Go API"
    );
    // In production: call the Go API /internal/movies/{id} to get the full
    // document and upsert it. For now, the event alone is insufficient without
    // the full payload — the movie.created / movie.published paths are the
    // primary index population mechanism.
    Ok(())
}

async fn handle_index_deleted(
    payload: &serde_json::Value,
    meili: &MeiliSearchClient,
) -> anyhow::Result<()> {
    let p: SearchIndexDeleted = serde_json::from_value(payload.clone())
        .context("deserialising SearchIndexDeleted")?;

    match p.entity_type.as_str() {
        "movie" => meili.delete_movie(&p.entity_id).await,
        other => {
            debug!(entity_type = %other, "delete not implemented for entity type");
            Ok(())
        }
    }
}