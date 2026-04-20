from __future__ import annotations

from fastapi import APIRouter, HTTPException, status

from app.classifiers import screen
from app.models import HealthResponse, ScreenRequest, ScreenResponse

router = APIRouter()


@router.get("/health", response_model=HealthResponse, tags=["ops"])
async def health() -> HealthResponse:
    """
    Health check endpoint.
    Returns "ready" for each classifier that has been loaded.
    Called by Kubernetes readiness + liveness probes.
    """
    from app.classifiers import nsfw as nsfw_clf
    from app.classifiers import hate_speech as hs_clf
    from app.classifiers import spoiler as spoiler_clf

    return HealthResponse(
        status="ok",
        models={
            "nsfw":        "ready" if nsfw_clf._pipeline is not None else "not_loaded",
            "hate_speech": "ready" if hs_clf._pipeline is not None else "not_loaded",
            "spoiler":     "ready" if spoiler_clf._pipeline is not None else "not_loaded",
        },
    )


@router.post(
    "/screen",
    response_model=ScreenResponse,
    status_code=status.HTTP_200_OK,
    tags=["screening"],
    summary="Screen a piece of content for policy violations",
    description="""
Called by the Go moderation module (httpAIClient) and by the Temporal
upload_pipeline ScreenContentActivity.

Returns:
- **signals**: list of detected violation categories
- **confidence**: per-signal score (0.0–1.0)
- **recommendation**: "approve" | "reject" | "escalate_to_human"

The Go module applies its own auto-decision thresholds on the returned
recommendation (approve < 0.3, reject > 0.95).
    """,
)
async def screen_content(req: ScreenRequest) -> ScreenResponse:
    try:
        return await screen(req)
    except RuntimeError as exc:
        # Classifiers not loaded — service is starting up.
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail=f"Classifier not ready: {exc}",
        )
    except Exception as exc:
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Screening error: {exc}",
        )