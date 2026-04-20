"""
ViewAura Recommendation Service — entry point.
"""
from __future__ import annotations

import logging
from contextlib import asynccontextmanager

import structlog
import uvicorn
from fastapi import FastAPI
from prometheus_fastapi_instrumentator import Instrumentator

from app.api import router
from app.config import get_settings
from app import consumer, feature_store
from app.recommender import register_movies, MovieFeatures
from app.recommender.mood import load as load_mood

import numpy as np

structlog.configure(
    processors=[
        structlog.stdlib.add_log_level,
        structlog.stdlib.add_logger_name,
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.JSONRenderer(),
    ],
    wrapper_class=structlog.BoundLogger,
    context_class=dict,
    logger_factory=structlog.PrintLoggerFactory(),
)

log = structlog.get_logger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    cfg = get_settings()
    log.info("recommendation-service starting", env=cfg.env, port=cfg.port)

    # ── Feature store (Redis) ─────────────────────────────────────────────────
    await feature_store.init(cfg.feature_store.redis_url)

    # ── Mood model ────────────────────────────────────────────────────────────
    # Loads ~22MB sentence-transformer model and computes 40 cluster centroids.
    load_mood(cfg.mood.model_id, cfg.mood.device)

    # ── Seed movie catalogue ──────────────────────────────────────────────────
    # In production: call GET /internal/movies/catalogue from the Go API and
    # build MovieFeatures objects with real genre embeddings. For now we seed
    # with random unit vectors so the service starts without the Go API.
    _seed_catalogue(cfg.feature_store.vector_dim)

    # ── Kafka consumer ────────────────────────────────────────────────────────
    await consumer.start()

    log.info("recommendation-service ready")
    yield

    # ── Shutdown ──────────────────────────────────────────────────────────────
    await feature_store.close()
    log.info("recommendation-service shut down")


def _seed_catalogue(dim: int) -> None:
    """
    Seed a stub catalogue for local dev and tests.
    Each movie gets a random normalised embedding and a single mood cluster.
    Replace with a real API call in production.
    """
    rng = np.random.default_rng(42)
    seed_movies = [
        MovieFeatures(
            movie_id=f"movie-{i:04d}",
            title=f"Movie {i}",
            genres=["Drama", "Thriller"][i % 2:i % 2 + 1],
            embedding=_unit(rng.random(dim).astype(np.float32)),
            mood_clusters=[f"mood_{(i % 40):02d}"],
        )
        for i in range(200)
    ]
    register_movies(seed_movies)
    log.info("stub movie catalogue seeded", count=len(seed_movies))


def _unit(v: np.ndarray) -> np.ndarray:
    n = np.linalg.norm(v)
    return v / n if n > 0 else v


def create_app() -> FastAPI:
    cfg = get_settings()

    app = FastAPI(
        title="ViewAura Recommendation Service",
        description="Hybrid collaborative + content + mood recommendation engine",
        version="0.1.0",
        docs_url="/docs" if cfg.env != "production" else None,
        redoc_url=None,
        lifespan=lifespan,
    )

    app.include_router(router)

    Instrumentator(
        should_group_status_codes=True,
        excluded_handlers=["/health", "/metrics"],
    ).instrument(app).expose(app)

    return app


app = create_app()


if __name__ == "__main__":
    cfg = get_settings()
    logging.basicConfig(level=getattr(logging, cfg.log_level, logging.INFO))
    uvicorn.run(
        "app.main:app",
        host="0.0.0.0",
        port=cfg.port,
        workers=cfg.workers,
        log_level=cfg.log_level.lower(),
        reload=cfg.env == "local",
    )