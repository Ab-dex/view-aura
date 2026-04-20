from __future__ import annotations
from fastapi import APIRouter, HTTPException, status
from app.analyzers import analyze, is_loaded
from app.models import HealthResponse, SceneAnalyzeRequest, SceneAnalyzeResponse

router = APIRouter()


@router.get("/health", response_model=HealthResponse)
async def health() -> HealthResponse:
    return HealthResponse(
        status="ok",
        models={
            "clip":    "ready" if is_loaded() else "not_loaded",
            "whisper": "ready" if is_loaded() else "not_loaded",
        },
    )


@router.post("/analyze", response_model=SceneAnalyzeResponse)
async def analyze_scene(req: SceneAnalyzeRequest) -> SceneAnalyzeResponse:
    if not is_loaded():
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
                            detail="Models not loaded yet")
    try:
        return await analyze(req.video_url, req.audio_url, req.asset_id)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc))