from __future__ import annotations
from fastapi import APIRouter, HTTPException, status
from app.analyzers import assess, ffmpeg_available
from app.models import AssessRequest, QualityReport, HealthResponse

router = APIRouter()


@router.get("/health", response_model=HealthResponse)
async def health() -> HealthResponse:
    return HealthResponse(status="ok", ffmpeg_available=ffmpeg_available())


@router.post(
    "/assess",
    response_model=QualityReport,
    summary="Run full quality assessment: VMAF + blocking + audio + A/V sync",
)
async def run_assessment(req: AssessRequest) -> QualityReport:
    try:
        return await assess(req)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc))