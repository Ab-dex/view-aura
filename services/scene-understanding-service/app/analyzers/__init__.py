"""
Scene understanding engine.

Pipeline:
  1. Extract frames every N seconds via OpenCV.
  2. Embed each frame with CLIP (ViT-B/32) to get a 512-dim visual embedding.
  3. Detect scene changes by measuring cosine distance between consecutive embeddings.
  4. Score each detected scene against the emotion label set using CLIP zero-shot.
  5. Transcribe the audio track with Whisper to get word-level timestamps.
  6. Map transcript sentences to scenes for chapter titles.
  7. Compute a per-frame sentiment timeline from emotion scores.
  8. Identify peak-tension and catharsis timestamps.
"""
from __future__ import annotations

import io
import time
import tempfile
from pathlib import Path
from typing import Sequence

import cv2
import httpx
import numpy as np
import open_clip
import torch
import whisper
import structlog

from app.config import get_settings
from app.models import (
    ChapterMarker, SceneAnalyzeResponse, SentimentPoint,
)

log = structlog.get_logger(__name__)

_clip_model     = None
_clip_preprocess = None
_clip_tokenizer  = None
_whisper_model   = None

# Emotion polarity map: positive emotions → +1, negative → -1
_EMOTION_POLARITY = {
    "tense": -0.6, "joyful": 0.9, "melancholy": -0.4, "romantic": 0.7,
    "thrilling": 0.3, "contemplative": 0.1, "humorous": 0.8, "horrifying": -0.9,
}


def load() -> None:
    global _clip_model, _clip_preprocess, _clip_tokenizer, _whisper_model
    cfg = get_settings()

    log.info("loading CLIP model", model=cfg.models.clip_model)
    _clip_model, _, _clip_preprocess = open_clip.create_model_and_transforms(
        cfg.models.clip_model,
        pretrained=cfg.models.clip_pretrain,
        device=cfg.models.device,
    )
    _clip_model.eval()
    _clip_tokenizer = open_clip.get_tokenizer(cfg.models.clip_model)

    log.info("loading Whisper model", model=cfg.models.whisper_model)
    _whisper_model = whisper.load_model(
        cfg.models.whisper_model,
        device=cfg.models.device,
    )
    log.info("scene understanding models ready")


def is_loaded() -> bool:
    return _clip_model is not None and _whisper_model is not None


async def analyze(video_url: str, audio_url: str | None, asset_id: str) -> SceneAnalyzeResponse:
    cfg = get_settings()
    t0  = time.monotonic()

    # ── Download video to temp file ───────────────────────────────────────────
    video_path = await _download(video_url)

    # ── Extract frames ────────────────────────────────────────────────────────
    frames, timestamps, duration = _extract_frames(
        video_path, cfg.analysis.frame_interval_secs
    )
    log.info("frames extracted", count=len(frames), duration=duration)

    # ── CLIP embeddings ───────────────────────────────────────────────────────
    embeddings = _embed_frames(frames)

    # ── Scene change detection ────────────────────────────────────────────────
    scene_boundaries = _detect_scene_changes(
        embeddings, timestamps, cfg.analysis.scene_change_threshold, cfg.analysis.min_chapter_secs
    )

    # ── Emotion scoring per scene ─────────────────────────────────────────────
    emotion_labels = cfg.analysis.emotion_labels
    text_features  = _encode_emotion_labels(emotion_labels)
    scene_emotions = _score_scenes(embeddings, timestamps, scene_boundaries, text_features, emotion_labels)

    # ── Transcript ────────────────────────────────────────────────────────────
    transcript, word_segments = _transcribe(audio_url or video_path)

    # ── Build chapters ────────────────────────────────────────────────────────
    chapters = _build_chapters(scene_boundaries, scene_emotions, word_segments, duration)

    # ── Sentiment timeline ────────────────────────────────────────────────────
    sentiment = _build_sentiment_timeline(embeddings, timestamps, text_features, emotion_labels)

    # ── Peak moments ──────────────────────────────────────────────────────────
    peak_tension = [s.time_secs for s in sentiment if s.score < -0.4][:5]
    catharsis    = [s.time_secs for s in sentiment if s.score > 0.6][:5]

    # ── Dominant emotions (top 3 across the whole film) ───────────────────────
    from collections import Counter
    emotion_counts = Counter(c.emotion for c in chapters)
    dominant = [e for e, _ in emotion_counts.most_common(3)]

    return SceneAnalyzeResponse(
        asset_id=asset_id,
        duration_secs=duration,
        chapters=chapters,
        sentiment_timeline=sentiment,
        peak_tension_secs=peak_tension,
        catharsis_secs=catharsis,
        dominant_emotions=dominant,
        transcript=transcript,
        processing_secs=round(time.monotonic() - t0, 2),
    )


# ─── Internal helpers ─────────────────────────────────────────────────────────

async def _download(url: str) -> str:
    """Download a URL to a temp file. Returns the file path."""
    suffix = Path(url.split("?")[0]).suffix or ".mp4"
    tmp = tempfile.NamedTemporaryFile(delete=False, suffix=suffix)
    async with httpx.AsyncClient(timeout=120.0) as client:
        async with client.stream("GET", url) as resp:
            resp.raise_for_status()
            async for chunk in resp.aiter_bytes(chunk_size=65536):
                tmp.write(chunk)
    tmp.close()
    return tmp.name


def _extract_frames(
    video_path: str,
    interval_secs: int,
) -> tuple[list[np.ndarray], list[float], float]:
    cap = cv2.VideoCapture(video_path)
    fps = cap.get(cv2.CAP_PROP_FPS) or 25.0
    total_frames = int(cap.get(cv2.CAP_PROP_FRAME_COUNT))
    duration = total_frames / fps

    step = int(fps * interval_secs)
    frames, timestamps = [], []
    idx = 0
    while True:
        cap.set(cv2.CAP_PROP_POS_FRAMES, idx)
        ok, frame = cap.read()
        if not ok:
            break
        frames.append(cv2.cvtColor(frame, cv2.COLOR_BGR2RGB))
        timestamps.append(idx / fps)
        idx += step

    cap.release()
    return frames, timestamps, duration


def _embed_frames(frames: list[np.ndarray]) -> np.ndarray:
    from PIL import Image
    cfg = get_settings()

    tensors = []
    for f in frames:
        img = Image.fromarray(f)
        tensors.append(_clip_preprocess(img))

    batch = torch.stack(tensors).to(cfg.models.device)
    with torch.no_grad():
        feats = _clip_model.encode_image(batch)
        feats = feats / feats.norm(dim=-1, keepdim=True)

    return feats.cpu().numpy()


def _encode_emotion_labels(labels: list[str]) -> np.ndarray:
    cfg    = get_settings()
    tokens = _clip_tokenizer([f"a scene that feels {lbl}" for lbl in labels])
    tokens = tokens.to(cfg.models.device)
    with torch.no_grad():
        feats = _clip_model.encode_text(tokens)
        feats = feats / feats.norm(dim=-1, keepdim=True)
    return feats.cpu().numpy()


def _detect_scene_changes(
    embeddings: np.ndarray,
    timestamps: list[float],
    threshold:  float,
    min_secs:   int,
) -> list[float]:
    """Return timestamps where a new scene begins."""
    boundaries = [0.0]
    last_boundary_t = 0.0
    for i in range(1, len(embeddings)):
        cos_dist = 1.0 - float(np.dot(embeddings[i-1], embeddings[i]))
        if cos_dist > threshold and (timestamps[i] - last_boundary_t) >= min_secs:
            boundaries.append(timestamps[i])
            last_boundary_t = timestamps[i]
    return boundaries


def _score_scenes(
    embeddings:       np.ndarray,
    timestamps:       list[float],
    boundaries:       list[float],
    text_features:    np.ndarray,
    emotion_labels:   list[str],
) -> list[tuple[float, float, str, float]]:
    """
    For each scene segment, average the frame embeddings and score against
    emotion labels. Returns list of (start, end, emotion_label, score).
    """
    results = []
    boundaries_ext = boundaries + [timestamps[-1] + 1.0]

    for i in range(len(boundaries)):
        start, end = boundaries_ext[i], boundaries_ext[i + 1]
        # Collect frame embeddings in this scene window
        scene_embs = [
            embeddings[j]
            for j, t in enumerate(timestamps)
            if start <= t < end
        ]
        if not scene_embs:
            continue

        avg_emb = np.mean(scene_embs, axis=0)
        avg_emb = avg_emb / (np.linalg.norm(avg_emb) + 1e-8)

        scores = avg_emb @ text_features.T
        best_idx = int(np.argmax(scores))
        results.append((start, end, emotion_labels[best_idx], float(scores[best_idx])))

    return results


def _transcribe(source_path: str) -> tuple[str, list[dict]]:
    if _whisper_model is None:
        return "", []
    try:
        result = _whisper_model.transcribe(source_path, word_timestamps=True)
        full_text = result.get("text", "")
        segments  = result.get("segments", [])
        return full_text, segments
    except Exception as exc:
        log.warning("transcription failed", error=str(exc))
        return "", []


def _build_chapters(
    boundaries:    list[float],
    scene_emotions: list[tuple[float, float, str, float]],
    word_segments: list[dict],
    duration:      float,
) -> list[ChapterMarker]:
    chapters = []
    boundaries_ext = boundaries + [duration]

    for i, (start, end, emotion, score) in enumerate(scene_emotions):
        # Find dialogue preview from Whisper segments
        preview = ""
        for seg in word_segments:
            seg_start = seg.get("start", 0)
            if start <= seg_start < end:
                preview = seg.get("text", "").strip()[:100]
                break

        chapters.append(ChapterMarker(
            index=i,
            start_secs=round(start, 1),
            end_secs=round(end, 1),
            title=f"Scene {i + 1}",
            emotion=emotion,
            emotion_score=round(score, 3),
            transcript_preview=preview,
        ))

    return chapters


def _build_sentiment_timeline(
    embeddings:     np.ndarray,
    timestamps:     list[float],
    text_features:  np.ndarray,
    emotion_labels: list[str],
) -> list[SentimentPoint]:
    points = []
    for i, (emb, t) in enumerate(zip(embeddings, timestamps)):
        scores = emb @ text_features.T
        best_idx = int(np.argmax(scores))
        emotion  = emotion_labels[best_idx]
        polarity = _EMOTION_POLARITY.get(emotion, 0.0)
        points.append(SentimentPoint(
            time_secs=round(t, 1),
            score=round(polarity, 3),
            emotion=emotion,
        ))
    return points