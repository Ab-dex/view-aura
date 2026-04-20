from __future__ import annotations

from typing import Annotated, Literal

from pydantic import BaseModel, Field, HttpUrl


class ScreenRequest(BaseModel):
    """
    Request body for POST /screen.
    Matches the payload sent by the Go httpAIClient.Screen() call.
    """
    content_type: Literal["review", "upload", "post", "profile", "comment"]
    content_id:   str = Field(..., min_length=1)

    # Optional enrichment — callers may include pre-fetched content
    # to avoid the service needing to call back to the Go API.
    text:        str | None = None             # review body or caption text
    frame_urls:  list[str] | None = Field(     # presigned URLs for video frames
        default=None,
        description="R2 presigned URLs for sampled video frames (for upload screening)",
    )
    image_url:   str | None = None             # single image (profile photo, post image)


class ScreenResponse(BaseModel):
    """
    Response body for POST /screen.
    Must match AIScreeningResult in the Go moderation/service/ai_client.go.
    """
    signals:        list[str]           # e.g. ["nsfw", "hate_speech"]
    confidence:     dict[str, float]    # signal → score
    recommendation: Literal["approve", "reject", "escalate_to_human"]


class HealthResponse(BaseModel):
    status:  str
    models:  dict[str, str]  # model_name → "ready" | "not_loaded"