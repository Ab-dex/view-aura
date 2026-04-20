from __future__ import annotations
from fastapi import APIRouter, HTTPException, status
from app.analyzers import generate, is_loaded
from app.models import GenerateRequest, GenerateResponse, HealthResponse

router = APIRouter()


@router.get("/health", response_model=HealthResponse)
async def health() -> HealthResponse:
    from app.config import get_settings
    cfg = get_settings()
    return HealthResponse(
        status="ok",
        model=f"whisper-{cfg.models.whisper_model}" if is_loaded() else "not_loaded",
    )


@router.post("/generate", response_model=GenerateResponse)
async def generate_subtitles(req: GenerateRequest) -> GenerateResponse:
    if not is_loaded():
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
                            detail="Whisper model not loaded yet")
    try:
        return await generate(req.audio_url, req.asset_id, req.language, req.format)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc))