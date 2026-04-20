"""
Hate speech text classifier.

Uses a fine-tuned BERT model to classify review body text or captions
as hate speech. Returns a probability score 0.0–1.0.
"""
from __future__ import annotations

import structlog
from transformers import pipeline

log = structlog.get_logger(__name__)

_pipeline = None


def load(model_id: str, device: str) -> None:
    """Load the hate speech classifier. Called once at startup."""
    global _pipeline
    log.info("loading hate speech classifier", model=model_id, device=device)
    _pipeline = pipeline(
        "text-classification",
        model=model_id,
        device=0 if device == "cuda" else -1,
        truncation=True,
        max_length=512,
    )
    log.info("hate speech classifier ready")


def score_text(text: str) -> float:
    """
    Return hate speech probability (0.0–1.0) for a text string.
    Returns 0.0 when the text is empty.
    """
    if _pipeline is None:
        raise RuntimeError("Hate speech classifier not loaded — call load() first")

    if not text or not text.strip():
        return 0.0

    # Truncate to avoid overly long inputs.
    truncated = text[:2000]
    results = _pipeline(truncated)

    # Model labels vary by checkpoint; handle common variants.
    label_map = {
        "hate": True,
        "hate speech": True,
        "offensive": True,
        "HATE": True,
        "NON_HATE": False,
        "normal": False,
        "non-hate": False,
    }

    for r in results:
        if label_map.get(r["label"], False):
            return float(r["score"])

    return 0.0