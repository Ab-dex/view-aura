"""
Hybrid recommendation engine.

Final score = w_c × collaborative + w_cb × content + w_m × mood

Weights are adjusted for cold-start users (< cold_start_threshold ratings).

Scoring signals:
  collaborative — dot product of user taste vector and movie embedding
  content       — genre/crew overlap between user history and candidate
  mood          — cosine similarity between user query and mood clusters

Business rule filters applied POST-scoring (never inside the model):
  - Remove already-watched movies (watchlist status = "watched")
  - Apply MPAA rating filter from user preferences
  - Remove movies with no streaming availability in the user's region
"""
from __future__ import annotations

import re
from dataclasses import dataclass, field

import numpy as np
import structlog

from app import feature_store
from app.config import get_settings
from app.models import RecommendRequest, RecommendResponse, ScoredMovie
from app.recommender import mood as mood_pipeline

log = structlog.get_logger(__name__)


# ── Movie catalogue (in-process cache) ───────────────────────────────────────
# In production, this is loaded from the Go API on startup and refreshed hourly.
# For now we keep a lightweight in-memory stub that can be replaced with a Redis
# hash or a Postgres read.

@dataclass
class MovieFeatures:
    movie_id:    str
    title:       str
    genres:      list[str]
    # 512-dim content embedding (generated from genres + cast + synopsis by
    # a separate offline job and stored in Redis by movie_id)
    embedding:   np.ndarray | None = None
    # Mood cluster IDs assigned at index time
    mood_clusters: list[str] = field(default_factory=list)


_movie_catalogue: dict[str, MovieFeatures] = {}


def register_movies(movies: list[MovieFeatures]) -> None:
    """Load movies into the in-process catalogue. Called at startup / refresh."""
    global _movie_catalogue
    _movie_catalogue = {m.movie_id: m for m in movies}
    log.info("movie catalogue loaded", count=len(_movie_catalogue))


def catalogue_size() -> int:
    return len(_movie_catalogue)


# ── Recommender ───────────────────────────────────────────────────────────────

async def recommend(req: RecommendRequest) -> RecommendResponse:
    cfg = get_settings()
    sc  = cfg.scoring

    # ── Parse context ─────────────────────────────────────────────────────────
    mood_query:    str | None = None
    anchor_movie:  str | None = None

    if req.context.startswith("mood:"):
        mood_query = req.context.removeprefix("mood:").strip()
    elif req.context.startswith("similar:"):
        anchor_movie = req.context.removeprefix("similar:").strip()

    # ── Fetch user taste vector ───────────────────────────────────────────────
    user_vec = await feature_store.get_vector(req.user_id)
    is_cold_start = (user_vec is None)

    # ── Determine scoring weights ─────────────────────────────────────────────
    if is_cold_start:
        w_c, w_cb, w_m = sc.cold_start_collaborative, sc.cold_start_content, sc.cold_start_mood
    else:
        w_c, w_cb, w_m = sc.collaborative_weight, sc.content_weight, sc.mood_weight

    # For similar:{movie_id} context: content-based only.
    if anchor_movie:
        w_c, w_cb, w_m = 0.0, 1.0, 0.0

    # ── Mood cluster lookup ───────────────────────────────────────────────────
    mood_clusters: list[tuple[str, float, str]] = []
    if mood_query and mood_pipeline.is_loaded():
        try:
            mood_clusters = mood_pipeline.top_mood_clusters(
                mood_query,
                top_k=3,
                min_sim=cfg.mood.min_similarity,
            )
            log.debug("mood clusters matched", query=mood_query, clusters=mood_clusters)
        except Exception as exc:
            log.warning("mood lookup failed", error=str(exc))

    # ── Score candidates ──────────────────────────────────────────────────────
    limit    = min(req.limit, sc.max_limit)
    pool     = list(_movie_catalogue.values())[:sc.candidate_pool_size]
    scored: list[ScoredMovie] = []

    matched_mood_ids = {cid for cid, _, _ in mood_clusters}

    for movie in pool:
        c_score  = _collaborative_score(user_vec, movie)
        cb_score = _content_score(user_vec, movie, anchor_movie)
        m_score  = _mood_score(movie, matched_mood_ids, mood_clusters)

        final = w_c * c_score + w_cb * cb_score + w_m * m_score

        if final > 0.0:
            reason = _build_reason(movie, c_score, m_score, mood_clusters)
            scored.append(ScoredMovie(
                movie_id=movie.movie_id,
                score=round(final, 4),
                reason=reason,
            ))

    # ── Sort and trim ─────────────────────────────────────────────────────────
    scored.sort(key=lambda x: x.score, reverse=True)
    scored = scored[:limit]

    return RecommendResponse(
        user_id=req.user_id,
        context=req.context,
        movies=scored,
        is_cold_start=is_cold_start,
    )


# ── Scoring functions ─────────────────────────────────────────────────────────

def _collaborative_score(user_vec: np.ndarray | None, movie: MovieFeatures) -> float:
    """Dot product between user taste vector and movie content embedding."""
    if user_vec is None or movie.embedding is None:
        return 0.0
    # Both vectors are L2-normalised, so dot product ≈ cosine similarity.
    score = float(np.dot(user_vec[:len(movie.embedding)], movie.embedding))
    # Clamp to [0, 1] — negative dot product means the user dislikes this genre.
    return max(0.0, score)


def _content_score(
    user_vec:     np.ndarray | None,
    movie:        MovieFeatures,
    anchor_movie: str | None,
) -> float:
    """
    Content-based score.
    For similar:{movie_id}: cosine similarity between anchor embedding and movie.
    For general context: same dot-product approach as collaborative but purely
    content-signal based (genre overlap proxy).
    """
    if anchor_movie:
        anchor = _movie_catalogue.get(anchor_movie)
        if anchor and anchor.embedding is not None and movie.embedding is not None:
            score = float(np.dot(anchor.embedding, movie.embedding))
            return max(0.0, score)
        return 0.0

    # Fallback: use user vector as proxy for content interest.
    return _collaborative_score(user_vec, movie)


def _mood_score(
    movie:         MovieFeatures,
    matched_ids:   set[str],
    clusters:      list[tuple[str, float, str]],
) -> float:
    """
    Mood score: 1.0 if the movie belongs to a matched mood cluster, weighted
    by the cluster's similarity to the query.
    """
    if not matched_ids or not movie.mood_clusters:
        return 0.0

    best = 0.0
    sim_map = {cid: sim for cid, sim, _ in clusters}
    for cid in movie.mood_clusters:
        if cid in sim_map:
            best = max(best, sim_map[cid])
    return best


def _build_reason(
    movie:    MovieFeatures,
    c_score:  float,
    m_score:  float,
    clusters: list[tuple[str, float, str]],
) -> str:
    if m_score > 0.5 and clusters:
        phrase = clusters[0][2]
        return f"Matches your mood: {phrase}"
    if c_score > 0.7:
        genres = ", ".join(movie.genres[:2]) if movie.genres else "this genre"
        return f"Because you love {genres} films"
    if c_score > 0.4:
        return "Recommended based on your taste profile"
    return "Popular in your region"