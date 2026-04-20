"""
Mood detection pipeline.

Converts free-text mood queries ("3am existential dread", "feel-good rainy day")
into a ranked list of genre/mood cluster IDs that drive the content-based filter.

Pipeline:
  1. Embed the query using a sentence-transformer (all-MiniLM-L6-v2, 384-dim).
  2. Compute cosine similarity against 40 pre-indexed mood cluster centroids.
  3. Return the top-K matching cluster IDs with their similarity scores.

The cluster centroids are initialised at startup from a hand-curated seed list
of 40 mood phrases covering the main emotional registers of film. In production
these centroids would be updated weekly from the search_queries ClickHouse table
using the top mood-style queries.
"""
from __future__ import annotations

import numpy as np
import structlog
from sentence_transformers import SentenceTransformer
from sklearn.metrics.pairwise import cosine_similarity

log = structlog.get_logger(__name__)

# ── Seed mood phrases ─────────────────────────────────────────────────────────
# 40 cluster seeds covering genre, tone, and temporal/contextual moods.
MOOD_SEEDS: list[str] = [
    "feel-good uplifting comedy",
    "heartwarming family film",
    "dark psychological thriller",
    "terrifying horror movie",
    "action-packed adventure",
    "romantic love story",
    "epic fantasy world-building",
    "gritty crime drama",
    "thought-provoking science fiction",
    "existential philosophical drama",
    "laugh-out-loud slapstick",
    "slow-burn atmospheric mystery",
    "coming-of-age teenage story",
    "inspirational true story",
    "intense war film",
    "quiet contemplative art house",
    "surreal dreamlike narrative",
    "edge-of-seat suspense",
    "bittersweet melancholy",
    "explosive summer blockbuster",
    "witty sharp dialogue",
    "beautiful cinematography nature",
    "nostalgic retro classic",
    "bizarre cult midnight movie",
    "emotional tearjerker drama",
    "light anime animated",
    "documentary real world",
    "foreign language film",
    "musical singing dancing",
    "superhero comic book",
    "cozy comfortable background film",
    "3am existential dread",
    "rainy day comfort watch",
    "girls night out fun",
    "boys action fun",
    "mind-bending plot twist",
    "short film under 90 minutes",
    "epic long multi-hour saga",
    "oscar award winning prestige",
    "hidden gem underrated indie",
]

_model: SentenceTransformer | None = None
_centroids: np.ndarray | None = None          # shape: [n_clusters, 384]
_cluster_ids: list[str] = []


def load(model_id: str, device: str) -> None:
    """
    Load the sentence-transformer and compute cluster centroids.
    Called once at startup. Blocks on model download (~22MB) on first run.
    """
    global _model, _centroids, _cluster_ids

    log.info("loading mood embedding model", model=model_id, device=device)
    _model = SentenceTransformer(model_id, device=device)

    log.info("computing mood cluster centroids", n_clusters=len(MOOD_SEEDS))
    embeddings = _model.encode(MOOD_SEEDS, normalize_embeddings=True)
    _centroids    = np.array(embeddings, dtype=np.float32)
    _cluster_ids  = [f"mood_{i:02d}" for i in range(len(MOOD_SEEDS))]

    log.info("mood pipeline ready", n_clusters=len(MOOD_SEEDS))


def is_loaded() -> bool:
    return _model is not None


def encode_query(text: str) -> np.ndarray:
    """Embed a free-text mood query. Returns a normalised 384-dim vector."""
    if _model is None:
        raise RuntimeError("Mood model not loaded — call load() first")
    return _model.encode([text], normalize_embeddings=True)[0]


def top_mood_clusters(
    query_text: str,
    top_k:        int   = 3,
    min_sim:      float = 0.25,
) -> list[tuple[str, float, str]]:
    """
    Return the top-K mood clusters matching the query.

    Returns a list of (cluster_id, similarity_score, mood_phrase) tuples,
    ordered by similarity descending.
    """
    if _centroids is None:
        raise RuntimeError("Mood centroids not computed — call load() first")

    query_vec = encode_query(query_text).reshape(1, -1)
    sims      = cosine_similarity(query_vec, _centroids)[0]

    top_indices = np.argsort(sims)[::-1][:top_k]
    results = []
    for idx in top_indices:
        score = float(sims[idx])
        if score >= min_sim:
            results.append((_cluster_ids[idx], score, MOOD_SEEDS[idx]))

    return results