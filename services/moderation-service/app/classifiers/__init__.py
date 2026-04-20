"""
Screening engine — orchestrates all classifiers for a single content item.

Decision logic (mirrors Go moderation/service/moderation.go thresholds):
  - Any signal exceeding its reject threshold  → "reject"
  - Any signal exceeding escalate_threshold    → "escalate_to_human"
  - All signals below flag thresholds          → "approve"

The Go module applies its own final auto-decision thresholds on top of
the recommendation returned here (auto-approve < 0.3, auto-reject > 0.95).
"""
from __future__ import annotations

import structlog

from app.classifiers import nsfw as nsfw_clf
from app.classifiers import hate_speech as hs_clf
from app.classifiers import spoiler as spoiler_clf
from app.config import get_settings
from app.models import ScreenRequest, ScreenResponse

log = structlog.get_logger(__name__)


def load_all() -> None:
    """Load all classifiers at startup. Called from the FastAPI lifespan."""
    cfg = get_settings()
    device = cfg.models.device

    nsfw_clf.load(cfg.models.nsfw_image, device)
    hs_clf.load(cfg.models.hate_speech, device)
    spoiler_clf.load(cfg.models.spoiler_zsl, device)

    log.info("all classifiers loaded")


async def screen(req: ScreenRequest) -> ScreenResponse:
    """
    Run all applicable classifiers and return a consolidated decision.

    What runs per content type:
      review   → hate_speech + spoiler (+ nsfw if image_url provided)
      upload   → nsfw (frame_urls) + hate_speech (if transcript/caption)
      post     → hate_speech + spoiler + nsfw (if image_url)
      profile  → nsfw (image_url) + hate_speech (bio text)
      comment  → hate_speech
    """
    cfg = get_settings()
    t = cfg.thresholds

    signals:    list[str]        = []
    confidence: dict[str, float] = {}

    # ── NSFW image analysis ───────────────────────────────────────────────────
    if req.content_type in {"upload", "post", "profile"}:
        nsfw_score = 0.0

        if req.frame_urls:
            nsfw_score = await nsfw_clf.score_frame_urls(req.frame_urls)
        elif req.image_url:
            try:
                import httpx
                async with httpx.AsyncClient(timeout=10.0) as client:
                    resp = await client.get(req.image_url)
                    resp.raise_for_status()
                    nsfw_score = nsfw_clf.score_image_bytes(resp.content)
            except Exception as exc:
                log.warning("nsfw image fetch failed", error=str(exc))

        if nsfw_score >= t.nsfw_flag:
            signals.append("nsfw")
            confidence["nsfw"] = round(nsfw_score, 4)

    # ── Hate speech text analysis ─────────────────────────────────────────────
    if req.content_type in {"review", "post", "profile", "comment", "upload"}:
        if req.text:
            hs_score = hs_clf.score_text(req.text)
            if hs_score >= t.hate_speech_flag:
                signals.append("hate_speech")
                confidence["hate_speech"] = round(hs_score, 4)

    # ── Spoiler detection ─────────────────────────────────────────────────────
    if req.content_type in {"review", "post"}:
        if req.text:
            spoiler_score = spoiler_clf.score_text(req.text)
            if spoiler_score >= t.spoiler_flag:
                signals.append("spoiler")
                confidence["spoiler"] = round(spoiler_score, 4)

    # ── Determine recommendation ──────────────────────────────────────────────
    recommendation = _decide(signals, confidence, t)

    log.info(
        "content screened",
        content_type=req.content_type,
        content_id=req.content_id,
        signals=signals,
        recommendation=recommendation,
    )

    return ScreenResponse(
        signals=signals,
        confidence=confidence,
        recommendation=recommendation,
    )


def _decide(
    signals:    list[str],
    confidence: dict[str, float],
    t,
) -> str:
    if not signals:
        return "approve"

    for signal, score in confidence.items():
        # Check hard-reject thresholds.
        reject_threshold = {
            "nsfw":        t.nsfw_reject,
            "hate_speech": t.hate_speech_reject,
        }.get(signal, 1.0)

        if score >= reject_threshold:
            log.info("auto-reject threshold exceeded", signal=signal, score=score)
            return "reject"

    # Check escalation threshold.
    max_score = max(confidence.values(), default=0.0)
    if max_score >= t.escalate_threshold:
        return "escalate_to_human"

    # Signals present but below escalation threshold — still flag for review.
    return "escalate_to_human"