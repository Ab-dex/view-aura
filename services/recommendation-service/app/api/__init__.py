from __future__ import annotations

from fastapi import APIRouter, HTTPException, status

from app import consumer, feature_store
from app.models import (
    HealthResponse,
    RecommendRequest,
    RecommendResponse,
    VectorUpdateRequest,
)
from app.recommender import catalogue_size, recommend
from app.recommender.mood import is_loaded as mood_loaded

router = APIRouter()


@router.get("/health", response_model=HealthResponse, tags=["ops"])
async def health() -> HealthResponse:
    return HealthResponse(
        status="ok",
        models_loaded=mood_loaded(),
        kafka_running=await consumer.is_running(),
    )


@router.post(
    "/recommend",
    response_model=RecommendResponse,
    tags=["recommendations"],
    summary="Get personalised movie recommendations for a user",
    description="""
Returns a ranked list of movie IDs with scores and human-readable reasons.

**Context values:**
- `home_feed` — standard hybrid (collaborative + content + mood)
- `mood:<text>` — elevate mood signal; text drives the embedding query
- `similar:<movie_id>` — content-based only, anchored to a specific movie

Cold-start users (< 10 ratings) automatically receive a mood-heavy weighting
and will have `is_cold_start: true` in the response.
    """,
)
async def get_recommendations(req: RecommendRequest) -> RecommendResponse:
    if catalogue_size() == 0:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="Movie catalogue not yet loaded — try again shortly",
        )
    try:
        return await recommend(req)
    except Exception as exc:
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=str(exc),
        )


@router.post(
    "/vectors/update",
    status_code=status.HTTP_204_NO_CONTENT,
    tags=["vectors"],
    summary="Trigger an online taste vector update for a user",
    description="""
Called directly by the Go monolith after a rating or watch event when
the Kafka consumer is unavailable or for immediate consistency.
    """,
)
async def update_vector(req: VectorUpdateRequest) -> None:
    from app.config import get_settings
    from app.recommender import _movie_catalogue

    cfg   = get_settings()
    movie = _movie_catalogue.get(req.movie_id)
    if movie is None or movie.embedding is None:
        # Not an error — movie may not be in the catalogue yet.
        return

    await feature_store.update_vector_online(
        user_id=req.user_id,
        movie_vector=movie.embedding,
        rating=req.rating,
        ttl_secs=cfg.feature_store.vector_ttl_secs,
        dim=cfg.feature_store.vector_dim,
    )