"""
Spoiler classifier using zero-shot NLI (BART-large-mnli).

A "spoiler" is review text that reveals major plot points without the
author tagging it as a spoiler. The classifier scores the likelihood
that the text "reveals plot details" vs "discusses themes without spoilers".

Zero-shot is used here rather than a fine-tuned model because:
1. A labelled spoiler dataset is not available at MVP.
2. BART-mnli generalises well for this binary distinction.
3. A fine-tuned checkpoint can be swapped in later via config.
"""
from __future__ import annotations

import structlog
from transformers import pipeline

log = structlog.get_logger(__name__)

_pipeline = None

# Candidate labels used for zero-shot classification.
# The hypothesis template is: "This text {label}."
_CANDIDATE_LABELS = [
    "reveals major plot twists or story endings",
    "discusses the film's themes without revealing the plot",
]


def load(model_id: str, device: str) -> None:
    """Load the zero-shot classifier. Called once at startup."""
    global _pipeline
    log.info("loading spoiler classifier", model=model_id, device=device)
    _pipeline = pipeline(
        "zero-shot-classification",
        model=model_id,
        device=0 if device == "cuda" else -1,
    )
    log.info("spoiler classifier ready")


def score_text(text: str) -> float:
    """
    Return spoiler probability (0.0–1.0) for a review body.
    Returns 0.0 when the text is too short to analyse (< 50 chars).
    """
    if _pipeline is None:
        raise RuntimeError("Spoiler classifier not loaded — call load() first")

    if not text or len(text.strip()) < 50:
        return 0.0

    # Truncate — NLI models are typically capped at 1024 tokens.
    truncated = text[:1500]
    result = _pipeline(truncated, _CANDIDATE_LABELS, multi_label=False)

    # result["labels"][0] is the highest-scoring label.
    spoiler_label = _CANDIDATE_LABELS[0]
    for label, score in zip(result["labels"], result["scores"]):
        if label == spoiler_label:
            return float(score)

    return 0.0