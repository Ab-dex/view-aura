"""
NSFW image classifier.

Uses a fine-tuned ViT model from HuggingFace to classify images as
"normal" or "nsfw". Frame URLs (R2 presigned URLs or S3 paths) are
fetched and scored independently; the maximum score across frames is
used as the asset-level signal.
"""
from __future__ import annotations

import io
import structlog
from typing import Sequence

import httpx
from PIL import Image
from transformers import pipeline

log = structlog.get_logger(__name__)

_pipeline = None


def load(model_id: str, device: str) -> None:
    """Load the NSFW classifier. Called once at startup."""
    global _pipeline
    log.info("loading nsfw classifier", model=model_id, device=device)
    _pipeline = pipeline(
        "image-classification",
        model=model_id,
        device=0 if device == "cuda" else -1,
    )
    log.info("nsfw classifier ready")


def score_image_bytes(image_bytes: bytes) -> float:
    """Return NSFW probability (0.0–1.0) for raw image bytes."""
    if _pipeline is None:
        raise RuntimeError("NSFW classifier not loaded — call load() first")

    image = Image.open(io.BytesIO(image_bytes)).convert("RGB")
    results = _pipeline(image)

    # The model returns labels like "nsfw" and "normal".
    for r in results:
        if r["label"].lower() in {"nsfw", "explicit", "unsafe"}:
            return float(r["score"])
    return 0.0


async def score_frame_urls(
    frame_urls: Sequence[str],
    timeout: float = 10.0,
) -> float:
    """
    Fetch frames from presigned URLs and return the maximum NSFW score.
    Non-fatal: individual frame fetch failures are logged and skipped.
    """
    if not frame_urls:
        return 0.0

    max_score = 0.0
    async with httpx.AsyncClient(timeout=timeout) as client:
        for url in frame_urls:
            try:
                resp = await client.get(url)
                resp.raise_for_status()
                score = score_image_bytes(resp.content)
                log.debug("frame scored", url=url[:60], score=round(score, 3))
                max_score = max(max_score, score)
            except Exception as exc:
                log.warning("frame fetch failed", url=url[:60], error=str(exc))

    return max_score