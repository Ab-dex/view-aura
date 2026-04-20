from __future__ import annotations
from typing import Literal
from pydantic import BaseModel, Field


class AssessRequest(BaseModel):
    asset_id:     str
    video_url:    str       # R2 presigned URL of the transcoded video
    # Optional: reference video for VMAF comparison (original mezzanine)
    # When absent, VMAF is run in no-reference mode (NR-VMAF estimate)
    reference_url: str | None = None


class BlockingArtifact(BaseModel):
    time_secs:   float
    score:       float   # 0.0 (clean) → 100.0 (severe blocking)


class AudioIssue(BaseModel):
    type:       Literal["clipping", "silence", "av_sync_drift"]
    time_secs:  float
    severity:   Literal["warning", "error"]
    detail:     str


class QualityReport(BaseModel):
    asset_id:          str
    # VMAF
    vmaf_score:        float | None    # None when reference unavailable and NR failed
    vmaf_min:          float | None
    vmaf_harmonic_mean: float | None
    # Blocking / compression artifacts
    blocking_score:    float           # mean blocking score across sampled frames
    blocking_samples:  list[BlockingArtifact]
    # Audio
    audio_issues:      list[AudioIssue]
    peak_loudness_db:  float | None
    # A/V sync
    av_sync_offset_secs: float | None
    # Overall verdict
    passed:            bool
    flags:             list[str]       # human-readable reasons for failure
    processing_secs:   float = 0.0


class HealthResponse(BaseModel):
    status:     str
    ffmpeg_available: bool