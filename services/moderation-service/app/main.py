"""
ViewAura Moderation Service

FastAPI application entry point. Models are loaded during the lifespan
startup event so the /health endpoint returns "not_loaded" while models
are downloading and "ready" once inference is available.
"""
from __future__ import annotations

import logging
from contextlib import asynccontextmanager

import structlog
import uvicorn
from fastapi import FastAPI
from prometheus_fastapi_instrumentator import Instrumentator

from app.api import router
from app.classifiers import load_all
from app.config import get_settings

# ── Logging ───────────────────────────────────────────────────────────────────
structlog.configure(
    processors=[
        structlog.stdlib.add_log_level,
        structlog.stdlib.add_logger_name,
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.StackInfoRenderer(),
        structlog.processors.format_exc_info,
        structlog.processors.JSONRenderer(),
    ],
    wrapper_class=structlog.BoundLogger,
    context_class=dict,
    logger_factory=structlog.PrintLoggerFactory(),
)

log = structlog.get_logger(__name__)


# ── Lifespan ──────────────────────────────────────────────────────────────────

@asynccontextmanager
async def lifespan(app: FastAPI):
    """Load all ML models on startup. Release resources on shutdown."""
    cfg = get_settings()
    log.info("moderation-service starting", env=cfg.env, port=cfg.port)

    # Model loading is CPU/GPU intensive — happens once, blocks here.
    # The Kubernetes readiness probe will fail until this completes.
    load_all()

    log.info("moderation-service ready")
    yield

    log.info("moderation-service shutting down")


# ── Application ───────────────────────────────────────────────────────────────

def create_app() -> FastAPI:
    cfg = get_settings()

    app = FastAPI(
        title="ViewAura Moderation Service",
        description="AI content screening — NSFW, hate speech, spoiler detection",
        version="0.1.0",
        docs_url="/docs" if cfg.env != "production" else None,
        redoc_url=None,
        lifespan=lifespan,
    )

    app.include_router(router)

    # Expose /metrics for Prometheus scraping.
    Instrumentator(
        should_group_status_codes=True,
        should_ignore_untemplated=True,
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