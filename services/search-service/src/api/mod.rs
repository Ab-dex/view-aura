use std::sync::Arc;

use axum::{
    extract::{Query, State},
    http::StatusCode,
    response::{IntoResponse, Json},
    routing::get,
    Router,
};
use redis::aio::ConnectionManager;
use serde::{Deserialize, Serialize};
use tokio::sync::Mutex;
use tower_http::{
    cors::{Any, CorsLayer},
    request_id::{MakeRequestUuid, PropagateRequestIdLayer, SetRequestIdLayer},
    timeout::TimeoutLayer,
    trace::TraceLayer,
};
use std::time::Duration;

use crate::{
    config::Settings,
    indexer::MeiliSearchClient,
    search::{search_movies, MovieSearchParams},
    suggest::suggest,
};

// ─── Application state ────────────────────────────────────────────────────────

#[derive(Clone)]
pub struct AppState {
    pub meili:    Arc<MeiliSearchClient>,
    pub redis:    Arc<Mutex<ConnectionManager>>,
    pub settings: Arc<Settings>,
}

// ─── Router ───────────────────────────────────────────────────────────────────

pub fn router(state: AppState) -> Router {
    let cors = CorsLayer::new()
        .allow_origin(Any)
        .allow_methods(Any)
        .allow_headers(Any);

    Router::new()
        .route("/health",            get(health_handler))
        .route("/api/v1/search",     get(search_handler))
        .route("/api/v1/suggest",    get(suggest_handler))
        .route("/api/v1/search/stats", get(stats_handler))
        .with_state(state)
        .layer(cors)
        .layer(TraceLayer::new_for_http())
        .layer(TimeoutLayer::new(Duration::from_secs(10)))
        .layer(PropagateRequestIdLayer::x_request_id())
        .layer(SetRequestIdLayer::x_request_id(MakeRequestUuid))
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

async fn health_handler() -> impl IntoResponse {
    Json(serde_json::json!({ "status": "ok", "service": "search-service" }))
}

async fn search_handler(
    State(state): State<AppState>,
    Query(params): Query<MovieSearchParams>,
) -> impl IntoResponse {
    match search_movies(&state.meili, params).await {
        Ok(result) => (StatusCode::OK, Json(serde_json::to_value(result).unwrap())).into_response(),
        Err(e) => error_response(StatusCode::INTERNAL_SERVER_ERROR, &e.to_string()),
    }
}

#[derive(Deserialize)]
struct SuggestQuery {
    q: String,
}

async fn suggest_handler(
    State(state): State<AppState>,
    Query(params): Query<SuggestQuery>,
) -> impl IntoResponse {
    let mut redis = state.redis.lock().await;
    let ttl = state.settings.redis.suggest_ttl_secs;

    match suggest(&params.q, &state.meili, &mut redis, ttl).await {
        Ok(result) => (StatusCode::OK, Json(serde_json::to_value(result).unwrap())).into_response(),
        Err(e) => error_response(StatusCode::INTERNAL_SERVER_ERROR, &e.to_string()),
    }
}

async fn stats_handler(State(state): State<AppState>) -> impl IntoResponse {
    match state
        .meili
        .inner
        .index(&state.meili.cfg.movie_index)
        .get_stats()
        .await
    {
        Ok(stats) => (StatusCode::OK, Json(serde_json::json!({
            "number_of_documents": stats.number_of_documents,
            "is_indexing":         stats.is_indexing,
        }))).into_response(),
        Err(e) => error_response(StatusCode::SERVICE_UNAVAILABLE, &e.to_string()),
    }
}

// ─── Error helper ─────────────────────────────────────────────────────────────

#[derive(Serialize)]
struct ErrorBody {
    error: ErrorDetail,
}

#[derive(Serialize)]
struct ErrorDetail {
    message: String,
}

fn error_response(status: StatusCode, message: &str) -> axum::response::Response {
    let body = Json(ErrorBody {
        error: ErrorDetail { message: message.to_string() },
    });
    (status, body).into_response()
}