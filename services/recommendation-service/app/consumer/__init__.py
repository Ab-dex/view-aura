"""
Kafka consumer for online taste vector updates.

Consumed topics:
  ratings       — RatingUpserted: user_id, movie_id, overall score
  watch_events  — WatchCompleted: user_id, movie_id (implicit positive signal)

On each event, the user's taste vector is updated incrementally via the
online gradient step in feature_store.update_vector_online().

This runs in a separate asyncio task from the HTTP server so event processing
never blocks the request path.
"""
from __future__ import annotations

import asyncio
import json
import structlog

from aiokafka import AIOKafkaConsumer
from app.config import get_settings
from app import feature_store
from app.recommender import _movie_catalogue

log = structlog.get_logger(__name__)

_running = False


async def start() -> None:
    """Spawn the consumer loop as a background asyncio task."""
    cfg = get_settings()
    if not cfg.kafka.brokers:
        log.info("kafka brokers not configured — consumer disabled")
        return

    asyncio.create_task(_run())


async def is_running() -> bool:
    return _running


async def _run() -> None:
    global _running
    cfg = get_settings()

    consumer = AIOKafkaConsumer(
        *cfg.kafka.topics,
        bootstrap_servers=cfg.kafka.brokers,
        group_id=cfg.kafka.group_id,
        auto_offset_reset=cfg.kafka.auto_offset_reset,
        value_deserializer=lambda m: json.loads(m.decode("utf-8")),
        enable_auto_commit=True,
    )

    await consumer.start()
    _running = True
    log.info("kafka consumer started", topics=cfg.kafka.topics)

    try:
        async for msg in consumer:
            await _handle(msg.topic, msg.value)
    except Exception as exc:
        log.error("kafka consumer crashed", error=str(exc))
        _running = False
    finally:
        await consumer.stop()
        _running = False


async def _handle(topic: str, payload: dict) -> None:
    """Route a Kafka message to the appropriate handler."""
    try:
        event_type = payload.get("event_type", "")

        if topic == "ratings" and event_type == "rating.upserted":
            await _on_rating(payload.get("payload", {}))

        elif topic == "watch_events":
            event = payload.get("payload", {}).get("event", "")
            if event in {"complete", "watched"}:
                await _on_watch_complete(payload.get("payload", {}))

    except Exception as exc:
        log.warning("event handling error — skipping", topic=topic, error=str(exc))


async def _on_rating(p: dict) -> None:
    """Update the user's taste vector on a new or updated rating."""
    user_id  = p.get("user_id")
    movie_id = p.get("movie_id")
    rating   = float(p.get("overall", 0.0))

    if not user_id or not movie_id:
        return

    movie = _movie_catalogue.get(movie_id)
    if movie is None or movie.embedding is None:
        # Movie not in catalogue yet — skip. The batch re-train will catch it.
        log.debug("movie not in catalogue, skipping vector update", movie_id=movie_id)
        return

    cfg = get_settings()
    await feature_store.update_vector_online(
        user_id=user_id,
        movie_vector=movie.embedding,
        rating=rating,
        ttl_secs=cfg.feature_store.vector_ttl_secs,
        dim=cfg.feature_store.vector_dim,
    )
    log.info("taste vector updated", user_id=user_id, movie_id=movie_id, rating=rating)


async def _on_watch_complete(p: dict) -> None:
    """
    Treat a completed watch as an implicit 3.5-star rating.
    This seeds the vector for users who watch without rating.
    """
    user_id  = p.get("user_id")
    movie_id = p.get("movie_id")
    if not user_id or not movie_id:
        return

    movie = _movie_catalogue.get(movie_id)
    if movie is None or movie.embedding is None:
        return

    cfg = get_settings()
    # Implicit signal is weaker than an explicit rating — lower lr applied
    # inside update_vector_online via the 3.5/5.0 - 0.5 = 0.2 signal value.
    await feature_store.update_vector_online(
        user_id=user_id,
        movie_vector=movie.embedding,
        rating=3.5,
        ttl_secs=cfg.feature_store.vector_ttl_secs,
        dim=cfg.feature_store.vector_dim,
    )
    log.info("implicit watch signal applied", user_id=user_id, movie_id=movie_id)