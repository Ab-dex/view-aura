"""
Redis feature store for 512-dim user taste vectors.

Vectors are stored as MessagePack-encoded float32 arrays under:
  user:{user_id}:embedding  →  msgpack(numpy float32 array, shape [512])

MessagePack is used instead of JSON because:
  - A 512-float JSON array is ~4KB; msgpack is ~2KB (no quotes, binary floats)
  - Serialisation/deserialisation is ~3× faster than json.loads/dumps
  - Sub-millisecond reads on the recommendation hot path

The vector dimension is intentionally 512 so future upgrades to larger
sentence-transformer models remain backward compatible by simply
re-generating the vectors (no schema migration needed).
"""
from __future__ import annotations

import numpy as np
import msgpack
import redis.asyncio as aioredis
import structlog

log = structlog.get_logger(__name__)

_redis: aioredis.Redis | None = None


async def init(redis_url: str) -> None:
    global _redis
    _redis = aioredis.from_url(redis_url, decode_responses=False)
    await _redis.ping()
    log.info("feature store Redis connected", url=redis_url)


async def close() -> None:
    if _redis:
        await _redis.aclose()


def _vector_key(user_id: str) -> str:
    return f"user:{user_id}:embedding"


async def get_vector(user_id: str) -> np.ndarray | None:
    """
    Retrieve the taste vector for a user. Returns None when the user
    has no vector (new user / cold start).
    """
    if _redis is None:
        raise RuntimeError("Feature store not initialised")

    raw = await _redis.get(_vector_key(user_id))
    if raw is None:
        return None

    try:
        array = msgpack.unpackb(raw, raw=True)
        return np.frombuffer(array, dtype=np.float32).copy()
    except Exception as exc:
        log.warning("failed to deserialise vector", user_id=user_id, error=str(exc))
        return None


async def set_vector(user_id: str, vector: np.ndarray, ttl_secs: int) -> None:
    """Store a taste vector for a user, replacing any existing value."""
    if _redis is None:
        raise RuntimeError("Feature store not initialised")

    packed = msgpack.packb(vector.astype(np.float32).tobytes())
    await _redis.setex(_vector_key(user_id), ttl_secs, packed)


async def update_vector_online(
    user_id:     str,
    movie_vector: np.ndarray,
    rating:      float,
    ttl_secs:    int,
    dim:         int = 512,
    lr:          float = 0.01,
) -> np.ndarray:
    """
    Perform an online incremental update to the user taste vector.

    Update rule (gradient step towards the rated item):
      v_new = v_old + lr × (rating / 5.0 - 0.5) × movie_vector

    Positive ratings (> 2.5) push the vector towards the movie.
    Negative ratings (< 2.5) push it away.

    The learning rate (lr=0.01) is deliberately small to prevent a single
    five-star rating from dominating the vector. A full ALS re-train via
    the Flink batch job corrects any accumulated drift weekly.
    """
    current = await get_vector(user_id)
    if current is None:
        # Cold start: initialise with the movie vector scaled by the rating.
        current = np.zeros(dim, dtype=np.float32)

    # Normalise rating to [-0.5, +0.5] range.
    signal = (rating / 5.0) - 0.5

    # Resize movie_vector to match the taste vector dimension if needed.
    if len(movie_vector) != dim:
        if len(movie_vector) < dim:
            movie_vector = np.pad(movie_vector, (0, dim - len(movie_vector)))
        else:
            movie_vector = movie_vector[:dim]

    updated = current + lr * signal * movie_vector
    # L2-normalise to keep the vector on the unit hypersphere.
    norm = np.linalg.norm(updated)
    if norm > 0:
        updated = updated / norm

    await set_vector(user_id, updated, ttl_secs)
    return updated