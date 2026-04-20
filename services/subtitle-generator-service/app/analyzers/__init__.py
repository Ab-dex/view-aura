"""
Subtitle generation engine.

1. Downloads audio/video via presigned URL.
2. Transcribes with Whisper (word-level timestamps).
3. Groups words into timed cues respecting reading-speed standards.
4. Renders to SRT or WebVTT format.
"""
from __future__ import annotations

import tempfile
import time
from pathlib import Path

import httpx
import whisper
import structlog

from app.config import get_settings
from app.models import GenerateResponse, SubtitleCue

log = structlog.get_logger(__name__)

_model = None


def load() -> None:
    global _model
    cfg = get_settings()
    log.info("loading Whisper model", model=cfg.models.whisper_model)
    _model = whisper.load_model(cfg.models.whisper_model, device=cfg.models.device)
    log.info("Whisper model ready")


def is_loaded() -> bool:
    return _model is not None


async def generate(
    audio_url: str,
    asset_id:  str,
    language:  str,
    fmt:       str,
) -> GenerateResponse:
    cfg = get_settings()
    t0  = time.monotonic()

    # Download to temp file
    suffix = Path(audio_url.split("?")[0]).suffix or ".mp4"
    tmp = tempfile.NamedTemporaryFile(delete=False, suffix=suffix)
    async with httpx.AsyncClient(timeout=300.0) as client:
        async with client.stream("GET", audio_url) as resp:
            resp.raise_for_status()
            async for chunk in resp.aiter_bytes(65536):
                tmp.write(chunk)
    tmp.close()

    # Transcribe
    opts = {"word_timestamps": True}
    if language != "auto":
        opts["language"] = language

    result  = _model.transcribe(tmp.name, **opts)
    lang    = result.get("language", language)
    segs    = result.get("segments", [])

    # Build cues from word-level timestamps
    cues = _build_cues(segs, cfg.subtitles.max_words_per_cue,
                       cfg.subtitles.max_chars_per_line,
                       cfg.subtitles.min_duration_secs,
                       cfg.subtitles.merge_gap_secs)

    duration = segs[-1]["end"] if segs else 0.0
    word_count = sum(len(c.text.split()) for c in cues)

    subtitle_text = _render_vtt(cues) if fmt == "vtt" else _render_srt(cues)

    return GenerateResponse(
        asset_id=asset_id,
        language=lang,
        format=fmt,
        subtitle_text=subtitle_text,
        cues=cues,
        duration_secs=round(duration, 2),
        word_count=word_count,
        processing_secs=round(time.monotonic() - t0, 2),
    )


# ─── Cue builder ──────────────────────────────────────────────────────────────

def _build_cues(
    segments:        list[dict],
    max_words:       int,
    max_chars:       int,
    min_duration:    float,
    merge_gap:       float,
) -> list[SubtitleCue]:
    """
    Group Whisper word timestamps into subtitle cues.
    Splits on max_words, max_chars, and natural sentence boundaries (. ? !).
    """
    raw_cues: list[tuple[float, float, str]] = []
    buf_words, buf_start, buf_end = [], 0.0, 0.0

    for seg in segments:
        words = seg.get("words", [])
        if not words:
            # Segment without word timestamps — treat whole segment as one cue
            text = seg.get("text", "").strip()
            if text:
                raw_cues.append((seg["start"], seg["end"], text))
            continue

        for w in words:
            word  = w.get("word", "").strip()
            start = w.get("start", 0.0)
            end   = w.get("end", 0.0)

            if not buf_words:
                buf_start = start

            buf_words.append(word)
            buf_end = end

            line = " ".join(buf_words)
            ends_sentence = line.rstrip().endswith((".", "?", "!"))
            too_many_words = len(buf_words) >= max_words
            too_long = len(line) >= max_chars

            if ends_sentence or too_many_words or too_long:
                raw_cues.append((buf_start, buf_end, line))
                buf_words, buf_start, buf_end = [], 0.0, 0.0

    if buf_words:
        raw_cues.append((buf_start, buf_end, " ".join(buf_words)))

    # Enforce minimum duration
    cues = []
    for i, (s, e, t) in enumerate(raw_cues):
        if (e - s) < min_duration:
            e = s + min_duration
        cues.append(SubtitleCue(index=i + 1, start_secs=round(s, 3),
                                end_secs=round(e, 3), text=t.strip()))

    return cues


# ─── Renderers ────────────────────────────────────────────────────────────────

def _ts_vtt(secs: float) -> str:
    h, rem = divmod(int(secs), 3600)
    m, s   = divmod(rem, 60)
    ms     = int((secs - int(secs)) * 1000)
    return f"{h:02d}:{m:02d}:{s:02d}.{ms:03d}"


def _ts_srt(secs: float) -> str:
    return _ts_vtt(secs).replace(".", ",")


def _render_vtt(cues: list[SubtitleCue]) -> str:
    lines = ["WEBVTT", ""]
    for c in cues:
        lines.append(f"{_ts_vtt(c.start_secs)} --> {_ts_vtt(c.end_secs)}")
        lines.append(c.text)
        lines.append("")
    return "\n".join(lines)


def _render_srt(cues: list[SubtitleCue]) -> str:
    lines = []
    for c in cues:
        lines.append(str(c.index))
        lines.append(f"{_ts_srt(c.start_secs)} --> {_ts_srt(c.end_secs)}")
        lines.append(c.text)
        lines.append("")
    return "\n".join(lines)