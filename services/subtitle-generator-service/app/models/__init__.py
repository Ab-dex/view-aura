from __future__ import annotations
from typing import Literal
from pydantic import BaseModel, Field


class GenerateRequest(BaseModel):
    asset_id:  str
    audio_url: str          # R2 presigned URL for the audio/video file
    language:  str = "en"   # ISO-639-1 language hint; "auto" for detection
    format:    Literal["srt", "vtt"] = "vtt"


class SubtitleCue(BaseModel):
    index:      int
    start_secs: float
    end_secs:   float
    text:       str


class GenerateResponse(BaseModel):
    asset_id:        str
    language:        str        # detected or provided language
    format:          str
    subtitle_text:   str        # full SRT or VTT document as a string
    cues:            list[SubtitleCue]
    duration_secs:   float
    word_count:      int
    processing_secs: float = 0.0


class HealthResponse(BaseModel):
    status: str
    model:  str