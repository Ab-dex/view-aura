from __future__ import annotations
from fastapi import APIRouter, HTTPException, status
from app.analyzers import embed_video, decode_from_url
from app.models import (
    DecodeRequest, DecodeResponse, EmbedRequest, EmbedResponse, HealthResponse,
)

router = APIRouter()


@router.get("/health", response_model=HealthResponse)
async def health() -> HealthResponse:
    return HealthResponse(status="ok")


@router.post("/embed", response_model=EmbedResponse, status_code=status.HTTP_200_OK,
             summary="Embed an invisible forensic watermark into a video")
async def embed(req: EmbedRequest) -> EmbedResponse:
    try:
        return await embed_video(req.source_url, req.asset_id, req.user_id, req.output_key)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc))


@router.post("/decode", response_model=DecodeResponse, status_code=status.HTTP_200_OK,
             summary="Attempt to recover a watermark payload from a video frame")
async def decode(req: DecodeRequest) -> DecodeResponse:
    try:
        return await decode_from_url(req.frame_url, req.asset_id)
    except Exception as exc:
        raise HTTPException(status_code=500, detail=str(exc))