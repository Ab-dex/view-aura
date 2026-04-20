from __future__ import annotations
from pydantic import BaseModel, Field


class EmbedRequest(BaseModel):
    """
    Embed a forensic watermark into a video asset.

    The payload is constructed from user_id + asset_id + unix timestamp,
    encoded into 64 bits and embedded into the frequency domain of sampled
    video frames. The watermark is invisible to the human eye but survives
    re-encoding, screen capture, and colour grading.
    """
    asset_id:      str
    user_id:       str   # the viewer/licensee whose ID is embedded
    source_url:    str   # R2 presigned GET URL for the source video
    output_key:    str   # R2 object key where the watermarked video is written


class EmbedResponse(BaseModel):
    asset_id:        str
    user_id:         str
    output_key:      str
    frames_marked:   int
    payload_hex:     str   # the 64-bit payload as hex — stored for leak tracing
    processing_secs: float = 0.0


class DecodeRequest(BaseModel):
    """Attempt to recover the watermark payload from a leaked video frame."""
    asset_id:   str
    frame_url:  str   # URL of a single suspicious frame image


class DecodeResponse(BaseModel):
    asset_id:    str
    payload_hex: str | None   # None if no watermark detected
    user_id:     str | None   # decoded from payload, None if unrecognised
    confidence:  float = 0.0  # 0.0–1.0


class HealthResponse(BaseModel):
    status: str