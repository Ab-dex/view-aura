from __future__ import annotations
import logging
from contextlib import asynccontextmanager
import structlog, uvicorn
from fastapi import FastAPI
from prometheus_fastapi_instrumentator import Instrumentator
from app.api import router
from app.config import get_settings

structlog.configure(processors=[structlog.stdlib.add_log_level,
    structlog.processors.TimeStamper(fmt="iso"),
    structlog.processors.JSONRenderer()],
    wrapper_class=structlog.BoundLogger, context_class=dict,
    logger_factory=structlog.PrintLoggerFactory())

log = structlog.get_logger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    cfg = get_settings()
    log.info("watermark-engine-service starting", env=cfg.env, port=cfg.port)
    # No model loading needed — DCT is computed on-the-fly (CPU only)
    log.info("watermark-engine-service ready")
    yield
    log.info("watermark-engine-service shutting down")


def create_app() -> FastAPI:
    cfg = get_settings()
    app = FastAPI(
        title="ViewAura Watermark Engine",
        description="DCT frequency-domain forensic watermark — embed per-viewer payload in video frames",
        version="0.1.0",
        docs_url="/docs" if cfg.env != "production" else None,
        lifespan=lifespan,
    )
    app.include_router(router)
    Instrumentator(excluded_handlers=["/health", "/metrics"]).instrument(app).expose(app)
    return app


app = create_app()

if __name__ == "__main__":
    cfg = get_settings()
    logging.basicConfig(level=getattr(logging, cfg.log_level, logging.INFO))
    uvicorn.run("app.main:app", host="0.0.0.0", port=cfg.port,
                workers=cfg.workers, reload=cfg.env == "local")