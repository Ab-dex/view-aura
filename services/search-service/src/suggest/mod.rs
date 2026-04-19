use anyhow::Context;
use redis::aio::ConnectionManager;
use redis::AsyncCommands;
use serde::Serialize;

use crate::indexer::MeiliSearchClient;

const MAX_SUGGESTIONS: usize = 8;

#[derive(Debug, Serialize)]
pub struct SuggestResult {
    pub query:       String,
    pub suggestions: Vec<Suggestion>,
    pub from_cache:  bool,
}

#[derive(Debug, Serialize)]
pub struct Suggestion {
    pub id:         String,
    pub title:      String,
    pub year:       Option<i32>,
    pub poster_url: Option<String>,
    pub entity_type: String,   // "movie" | "person"
}

/// Return autocomplete suggestions for a given prefix.
///
/// Cache strategy:
///   1. Check Redis ZSet `search:suggest:{prefix}` — hit returns immediately.
///   2. On miss: query MeiliSearch, store top-N in Redis with TTL.
///
/// The Redis ZSet is populated either by this function (on-demand) or by the
/// `SearchSuggestCacheWarmRequested` Kafka event handler (pre-warm).
pub async fn suggest(
    prefix: &str,
    meili: &MeiliSearchClient,
    redis: &mut ConnectionManager,
    ttl_secs: u64,
) -> anyhow::Result<SuggestResult> {
    if prefix.len() < 2 {
        return Ok(SuggestResult {
            query:       prefix.to_string(),
            suggestions: vec![],
            from_cache:  false,
        });
    }

    let cache_key = format!("search:suggest:{}", prefix.to_lowercase());

    // ── Cache read ────────────────────────────────────────────────────────────
    let cached: Vec<String> = redis
        .zrevrange(&cache_key, 0, (MAX_SUGGESTIONS - 1) as isize)
        .await
        .unwrap_or_default();

    if !cached.is_empty() {
        let suggestions = cached
            .into_iter()
            .filter_map(|s| serde_json::from_str::<Suggestion>(&s).ok())
            .collect();

        return Ok(SuggestResult {
            query:       prefix.to_string(),
            suggestions,
            from_cache:  true,
        });
    }

    // ── Cache miss: query MeiliSearch ────────────────────────────────────────
    let index = meili.inner.index(&meili.cfg.movie_index);
    let result = index
        .search()
        .with_query(prefix)
        .with_filter("status = \"published\"")
        .with_limit(MAX_SUGGESTIONS)
        .with_attributes_to_retrieve(["id", "title", "release_year", "poster_url"])
        .execute::<serde_json::Value>()
        .await
        .context("suggest MeiliSearch query")?;

    let mut suggestions: Vec<Suggestion> = result
        .hits
        .iter()
        .filter_map(|hit| {
            let v = &hit.result;
            Some(Suggestion {
                id:          v.get("id")?.as_str()?.to_string(),
                title:       v.get("title")?.as_str()?.to_string(),
                year:        v.get("release_year").and_then(|y| y.as_i64()).map(|y| y as i32),
                poster_url:  v.get("poster_url").and_then(|u| u.as_str()).map(|s| s.to_string()),
                entity_type: "movie".to_string(),
            })
        })
        .collect();

    suggestions.truncate(MAX_SUGGESTIONS);

    // ── Populate cache ────────────────────────────────────────────────────────
    if !suggestions.is_empty() {
        let _: redis::RedisResult<()> = async {
            for (i, s) in suggestions.iter().enumerate() {
                let score = (MAX_SUGGESTIONS - i) as f64;
                let _: () = redis
                    .zadd(&cache_key, serde_json::to_string(s).unwrap_or_default(), score)
                    .await?;
            }
            let _: () = redis.expire(&cache_key, ttl_secs as i64).await?;
            Ok(())
        }
        .await;
    }

    Ok(SuggestResult {
        query:       prefix.to_string(),
        suggestions,
        from_cache:  false,
    })
}