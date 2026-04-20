from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field


class RecommendRequest(BaseModel):
    """
    POST /recommend request body.
    Matches the gRPC RecRequest fields described in the architecture doc.
    """
    user_id:         str
    limit:           int  = Field(default=20, ge=1, le=100)
    # context controls which signals are emphasised.
    # home_feed        → standard hybrid (collaborative + content + mood)
    # mood:{text}      → elevated mood weight; text drives the mood embedding
    # similar:{movie}  → content-based only, anchored to a specific movie
    context:         str  = "home_feed"
    exclude_watched: bool = True


class ScoredMovie(BaseModel):
    """A single recommendation result."""
    movie_id: str
    score:    float = Field(ge=0.0, le=1.0)
    reason:   str   = ""   # human-readable explanation e.g. "Because you loved Parasite"


class RecommendResponse(BaseModel):
    user_id:      str
    context:      str
    movies:       list[ScoredMovie]
    is_cold_start: bool = False


class VectorUpdateRequest(BaseModel):
    """
    POST /vectors/update — called internally or by the Kafka consumer
    to refresh a user's taste vector after a rating or watch event.
    """
    user_id:  str
    movie_id: str
    # Implicit signals from watch events use a synthetic rating.
    rating:   float = Field(ge=0.0, le=5.0)
    event:    Literal["rating", "watch_complete", "watch_partial"] = "rating"


class HealthResponse(BaseModel):
    status:        str
    models_loaded: bool
    kafka_running: bool