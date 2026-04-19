use anyhow::Context;
use meilisearch_sdk::search::SearchQuery;
use serde::{Deserialize, Serialize};

use crate::indexer::{documents::MovieDocument, MeiliSearchClient};

// ─── Request ──────────────────────────────────────────────────────────────────

#[derive(Debug, Deserialize, Default)]
pub struct MovieSearchParams {
    /// Free-text query. Empty string returns all published movies.
    pub q:            Option<String>,
    /// Genre filter e.g. "Drama,Thriller"
    pub genres:       Option<Vec<String>>,
    /// Country filter
    pub countries:    Option<Vec<String>>,
    /// Minimum avg_rating e.g. 3.5
    pub min_rating:   Option<f64>,
    /// Release year range
    pub year_from:    Option<i32>,
    pub year_to:      Option<i32>,
    /// Content rating e.g. "PG-13"
    pub content_rating: Option<String>,
    /// Language code e.g. "en"
    pub lang:         Option<String>,
    /// Streaming provider e.g. "Netflix"
    pub provider:     Option<String>,
    /// Sort field: "popularity_score" | "avg_rating" | "release_date"
    pub sort_by:      Option<String>,
    pub sort_dir:     Option<String>,   // "asc" | "desc"
    pub limit:        Option<usize>,
    pub offset:       Option<usize>,
}

// ─── Response ─────────────────────────────────────────────────────────────────

#[derive(Debug, Serialize)]
pub struct MovieSearchResult {
    pub movies:              Vec<MovieDocument>,
    pub total:               usize,
    pub limit:               usize,
    pub offset:              usize,
    pub processing_time_ms:  u128,
}

// ─── Search ───────────────────────────────────────────────────────────────────

pub async fn search_movies(
    meili: &MeiliSearchClient,
    params: MovieSearchParams,
) -> anyhow::Result<MovieSearchResult> {
    let limit = params.limit.unwrap_or(20).min(100);
    let offset = params.offset.unwrap_or(0);
    let query = params.q.as_deref().unwrap_or("");

    // Build filter expression.
    let mut filters: Vec<String> = vec!["status = \"published\"".to_string()];

    if let Some(genres) = &params.genres {
        if !genres.is_empty() {
            let g = genres
                .iter()
                .map(|g| format!("genres = \"{g}\""))
                .collect::<Vec<_>>()
                .join(" OR ");
            filters.push(format!("({g})"));
        }
    }
    if let Some(countries) = &params.countries {
        if !countries.is_empty() {
            let c = countries
                .iter()
                .map(|c| format!("countries = \"{c}\""))
                .collect::<Vec<_>>()
                .join(" OR ");
            filters.push(format!("({c})"));
        }
    }
    if let Some(min) = params.min_rating {
        filters.push(format!("avg_rating >= {min}"));
    }
    if let Some(year_from) = params.year_from {
        filters.push(format!("release_year >= {year_from}"));
    }
    if let Some(year_to) = params.year_to {
        filters.push(format!("release_year <= {year_to}"));
    }
    if let Some(cr) = &params.content_rating {
        filters.push(format!("content_rating = \"{cr}\""));
    }
    if let Some(lang) = &params.lang {
        filters.push(format!("original_lang = \"{lang}\""));
    }
    if let Some(provider) = &params.provider {
        filters.push(format!("streaming_providers = \"{provider}\""));
    }

    let filter_str = filters.join(" AND ");

    // Build sort expression.
    let sort_field = params.sort_by.as_deref().unwrap_or("popularity_score");
    let sort_dir   = params.sort_dir.as_deref().unwrap_or("desc");
    let sort_expr  = format!("{sort_field}:{sort_dir}");

    let index = meili.inner.index(&meili.cfg.movie_index);
    let mut q = SearchQuery::new(&index);
    q.with_query(query)
     .with_filter(&filter_str)
     .with_sort(&[sort_expr.as_str()])
     .with_limit(limit)
     .with_offset(offset);

    let result = index
        .execute_query::<MovieDocument>(&q)
        .await
        .context("executing movie search query")?;

    Ok(MovieSearchResult {
        total:              result.estimated_total_hits.unwrap_or(0),
        limit,
        offset,
        processing_time_ms: result.processing_time_ms as u128,
        movies:             result.hits.into_iter().map(|h| h.result).collect(),
    })
}