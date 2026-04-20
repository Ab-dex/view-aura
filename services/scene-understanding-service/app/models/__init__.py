from __future__ import annotations
from pydantic import BaseModel, Field


class SceneAnalyzeRequest(BaseModel):
    asset_id:   str
    video_url:  str   # R2 presigned GET URL or local path for testing
    audio_url:  str | None = None   # separate audio track if available


class ChapterMarker(BaseModel):
    index:       int
    start_secs:  float
    end_secs:    float
    title:       str        # auto-generated: "Scene 1", "Act 2 Opening", etc.
    emotion:     str        # dominant emotion label for this chapter
    emotion_score: float    # confidence 0.0–1.0
    transcript_preview: str = ""   # first 100 chars of dialogue


class SentimentPoint(BaseModel):
    time_secs: float
    score:     float   # -1.0 (negative) → +1.0 (positive)
    emotion:   str


class SceneAnalyzeResponse(BaseModel):
    asset_id:         str
    duration_secs:    float
    chapters:         list[ChapterMarker]
    sentiment_timeline: list[SentimentPoint]
    # Timestamps of peak tension and catharsis moments
    peak_tension_secs: list[float]
    catharsis_secs:    list[float]
    # Summary emotion tags for the whole asset
    dominant_emotions: list[str]
    # Auto-generated transcript (empty if audio unavailable)
    transcript:       str = ""
    processing_secs:  float = 0.0


class HealthResponse(BaseModel):
    status:  str
    models:  dict[str, str]