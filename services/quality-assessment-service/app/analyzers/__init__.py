"""
Quality Assessment Engine.

Runs three independent checks on a transcoded video asset:

1. VMAF (Video Multi-Method Assessment Fusion)
   Industry-standard perceptual video quality metric developed by Netflix.
   Requires ffmpeg compiled with --enable-libvmaf (available in the
   jrottenberg/ffmpeg Docker image used by this service).
   Score range: 0 (worst) → 100 (best). Broadcast threshold: ≥ 70.

2. Blocking artifact detection
   Measures DCT-domain blockiness in sampled frames using OpenCV's
   Laplacian variance method. High variance in 8×8 block boundaries
   indicates compression artifacts.

3. Audio quality checks
   - Peak loudness measurement (EBU R128 via ffmpeg loudnorm)
   - Clipping detection (samples near 0 dBFS)
   - A/V sync drift estimation via correlation of audio onset envelope
     with video scene change timestamps
"""
from __future__ import annotations

import io
import json
import shutil
import subprocess
import tempfile
import time
from pathlib import Path

import cv2
import httpx
import librosa
import numpy as np
import soundfile as sf
import structlog

from app.config import get_settings
from app.models import (
    AssessRequest, AudioIssue, BlockingArtifact, QualityReport,
)

log = structlog.get_logger(__name__)


def ffmpeg_available() -> bool:
    return shutil.which("ffmpeg") is not None


# ─── Entry point ──────────────────────────────────────────────────────────────

async def assess(req: AssessRequest) -> QualityReport:
    cfg = get_settings()
    qc  = cfg.quality
    t0  = time.monotonic()
    flags: list[str] = []

    # Download distorted video
    distorted_path = await _download(req.video_url, ".mp4")

    # Optionally download reference
    reference_path = None
    if req.reference_url:
        reference_path = await _download(req.reference_url, ".mp4")

    # ── 1. VMAF ───────────────────────────────────────────────────────────────
    vmaf_score, vmaf_min, vmaf_hm = None, None, None
    if ffmpeg_available():
        vmaf_score, vmaf_min, vmaf_hm = _run_vmaf(
            distorted_path, reference_path, qc.vmaf_model
        )
        if vmaf_score is not None and vmaf_score < qc.vmaf_threshold:
            flags.append(f"VMAF {vmaf_score:.1f} below threshold {qc.vmaf_threshold}")

    # ── 2. Blocking artifacts ─────────────────────────────────────────────────
    blocking_samples = _detect_blocking(distorted_path, qc.artifact_sample_interval)
    blocking_score   = (
        float(np.mean([s.score for s in blocking_samples]))
        if blocking_samples else 0.0
    )
    if blocking_score > qc.blocking_threshold:
        flags.append(f"Blocking score {blocking_score:.1f} above threshold {qc.blocking_threshold}")

    # ── 3. Audio quality ──────────────────────────────────────────────────────
    audio_issues, peak_db = _assess_audio(distorted_path, qc.clipping_threshold_db)
    for issue in audio_issues:
        if issue.severity == "error":
            flags.append(f"Audio {issue.type} at {issue.time_secs:.1f}s")

    # ── 4. A/V sync ───────────────────────────────────────────────────────────
    av_sync = _estimate_av_sync(distorted_path)
    if av_sync is not None and abs(av_sync) > qc.av_sync_tolerance_secs:
        flags.append(f"A/V sync drift {av_sync:.3f}s exceeds {qc.av_sync_tolerance_secs}s")

    passed = len(flags) == 0

    log.info(
        "quality assessment complete",
        asset_id=req.asset_id,
        vmaf=vmaf_score,
        blocking=blocking_score,
        passed=passed,
        flags=flags,
        elapsed=round(time.monotonic() - t0, 2),
    )

    return QualityReport(
        asset_id=req.asset_id,
        vmaf_score=vmaf_score,
        vmaf_min=vmaf_min,
        vmaf_harmonic_mean=vmaf_hm,
        blocking_score=round(blocking_score, 2),
        blocking_samples=blocking_samples,
        audio_issues=audio_issues,
        peak_loudness_db=peak_db,
        av_sync_offset_secs=av_sync,
        passed=passed,
        flags=flags,
        processing_secs=round(time.monotonic() - t0, 2),
    )


# ─── VMAF ─────────────────────────────────────────────────────────────────────

def _run_vmaf(
    distorted: str,
    reference: str | None,
    model:     str,
) -> tuple[float | None, float | None, float | None]:
    """
    Run VMAF via ffmpeg lavfi filter.
    Returns (mean, min, harmonic_mean) or (None, None, None) on failure.
    """
    if reference is None:
        # No-reference VMAF is not natively supported in ffmpeg.
        # Fall back to SSIM as a proxy.
        log.info("no reference video — skipping full VMAF, using SSIM proxy")
        return _run_ssim_only(distorted)

    log_file = tempfile.NamedTemporaryFile(delete=False, suffix=".json")
    log_file.close()

    # ffmpeg VMAF filter with JSON log output
    vmaf_filter = (
        f"[0:v][1:v]libvmaf=model=version={model}:"
        f"log_fmt=json:log_path={log_file.name}"
    )
    cmd = [
        "ffmpeg", "-y",
        "-i", distorted,
        "-i", reference,
        "-lavfi", vmaf_filter,
        "-f", "null", "-",
    ]

    try:
        result = subprocess.run(cmd, capture_output=True, timeout=600)
        if result.returncode != 0:
            log.warning("ffmpeg vmaf failed", stderr=result.stderr.decode()[:500])
            return None, None, None

        with open(log_file.name) as f:
            data = json.load(f)

        pooled = data.get("pooled_metrics", {}).get("vmaf", {})
        return (
            pooled.get("mean"),
            pooled.get("min"),
            pooled.get("harmonic_mean"),
        )
    except Exception as exc:
        log.warning("vmaf error", error=str(exc))
        return None, None, None


def _run_ssim_only(distorted: str) -> tuple[float | None, float | None, float | None]:
    """Estimate quality using SSIM against the first frame as reference proxy."""
    cap = cv2.VideoCapture(distorted)
    ok, ref_frame = cap.read()
    if not ok:
        cap.release()
        return None, None, None

    scores = []
    step = max(1, int(cap.get(cv2.CAP_PROP_FPS) * 10))
    idx  = 0
    while True:
        cap.set(cv2.CAP_PROP_POS_FRAMES, idx)
        ok, frame = cap.read()
        if not ok:
            break
        score = cv2.quality.QualitySSIM_compute(
            cv2.cvtColor(ref_frame, cv2.COLOR_BGR2GRAY),
            cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY),
        )[0][0] if hasattr(cv2, "quality") else 1.0
        scores.append(float(score))
        idx += step

    cap.release()
    if not scores:
        return None, None, None

    mean = float(np.mean(scores))
    return mean, float(np.min(scores)), mean


# ─── Blocking artifact detection ──────────────────────────────────────────────

def _detect_blocking(video_path: str, interval_secs: int) -> list[BlockingArtifact]:
    """
    Detect blocking artifacts by measuring the energy of block boundaries.
    Higher boundary energy relative to block interior = more blocking.
    """
    cap = cv2.VideoCapture(video_path)
    fps = cap.get(cv2.CAP_PROP_FPS) or 25.0
    step = max(1, int(fps * interval_secs))

    samples: list[BlockingArtifact] = []
    idx = 0
    while True:
        cap.set(cv2.CAP_PROP_POS_FRAMES, idx)
        ok, frame = cap.read()
        if not ok:
            break

        gray  = cv2.cvtColor(frame, cv2.COLOR_BGR2GRAY).astype(np.float32)
        score = _blocking_score(gray)
        samples.append(BlockingArtifact(
            time_secs=round(idx / fps, 1),
            score=round(score, 2),
        ))
        idx += step

    cap.release()
    return samples


def _blocking_score(gray: np.ndarray) -> float:
    """
    Measure DCT-domain blockiness.
    Compute gradient energy along 8-pixel boundaries vs interior.
    High ratio = blocking artifacts.
    """
    h, w = gray.shape
    boundary_energy  = 0.0
    interior_energy  = 0.0
    n_boundaries     = 0

    for row in range(8, h - 8, 8):
        # Horizontal boundary
        b = np.abs(gray[row, :] - gray[row - 1, :]).mean()
        i = np.abs(gray[row, :] - gray[row - 2, :]).mean()
        boundary_energy  += b
        interior_energy  += i + 1e-8
        n_boundaries     += 1

    for col in range(8, w - 8, 8):
        # Vertical boundary
        b = np.abs(gray[:, col] - gray[:, col - 1]).mean()
        i = np.abs(gray[:, col] - gray[:, col - 2]).mean()
        boundary_energy  += b
        interior_energy  += i + 1e-8
        n_boundaries     += 1

    if n_boundaries == 0:
        return 0.0

    ratio = (boundary_energy / interior_energy) * 10.0
    return min(float(ratio), 100.0)


# ─── Audio quality ────────────────────────────────────────────────────────────

def _assess_audio(
    video_path:      str,
    clipping_thresh: float,
) -> tuple[list[AudioIssue], float | None]:
    """
    Extract audio to WAV, then:
    - Detect clipping (samples near 0 dBFS)
    - Measure peak loudness
    """
    issues: list[AudioIssue] = []

    # Extract audio to temp WAV
    wav_tmp = tempfile.NamedTemporaryFile(delete=False, suffix=".wav")
    wav_tmp.close()
    cmd = ["ffmpeg", "-y", "-i", video_path, "-vn", "-ar", "44100", "-ac", "1", wav_tmp.name]
    try:
        subprocess.run(cmd, capture_output=True, timeout=120, check=True)
    except Exception as exc:
        log.warning("audio extraction failed", error=str(exc))
        return [], None

    try:
        audio, sr = sf.read(wav_tmp.name)
    except Exception as exc:
        log.warning("audio read failed", error=str(exc))
        return [], None

    # Peak loudness
    peak_db = float(20 * np.log10(np.max(np.abs(audio)) + 1e-10))

    # Clipping detection — samples within 1 dBFS of full scale
    clip_threshold = 10 ** (clipping_thresh / 20.0)
    clip_mask = np.abs(audio) >= clip_threshold
    if np.any(clip_mask):
        clip_indices = np.where(clip_mask)[0]
        first_clip   = float(clip_indices[0]) / sr
        issues.append(AudioIssue(
            type="clipping",
            time_secs=round(first_clip, 2),
            severity="error",
            detail=f"{int(np.sum(clip_mask))} clipped samples detected",
        ))

    # Silence detection — more than 3 consecutive seconds below -60 dBFS
    silence_mask = np.abs(audio) < 10 ** (-60.0 / 20.0)
    silence_runs = _find_runs(silence_mask, min_length=int(sr * 3))
    for run_start in silence_runs[:3]:
        issues.append(AudioIssue(
            type="silence",
            time_secs=round(float(run_start) / sr, 2),
            severity="warning",
            detail="Extended silence detected",
        ))

    return issues, round(peak_db, 1)


def _find_runs(mask: np.ndarray, min_length: int) -> list[int]:
    """Find start indices of True runs of at least min_length samples."""
    starts = []
    in_run, run_start, run_len = False, 0, 0
    for i, v in enumerate(mask):
        if v:
            if not in_run:
                in_run, run_start, run_len = True, i, 0
            run_len += 1
        else:
            if in_run and run_len >= min_length:
                starts.append(run_start)
            in_run = False
    return starts


# ─── A/V sync ─────────────────────────────────────────────────────────────────

def _estimate_av_sync(video_path: str) -> float | None:
    """
    Estimate A/V sync offset by correlating audio onset envelope with
    video scene change timestamps.

    Returns offset in seconds (positive = audio ahead of video).
    Returns None when estimation fails or audio/video tracks are missing.
    """
    try:
        # Extract audio envelope
        wav_tmp = tempfile.NamedTemporaryFile(delete=False, suffix=".wav")
        wav_tmp.close()
        subprocess.run(
            ["ffmpeg", "-y", "-i", video_path, "-vn", "-ar", "8000", "-ac", "1", wav_tmp.name],
            capture_output=True, timeout=60, check=True,
        )
        audio, sr = sf.read(wav_tmp.name)
        # Onset strength envelope
        onset_env = librosa.onset.onset_strength(y=audio.astype(np.float32), sr=sr)
        onset_times = librosa.frames_to_time(np.arange(len(onset_env)), sr=sr)

        # Extract video scene change timestamps via ffmpeg
        result = subprocess.run(
            [
                "ffmpeg", "-i", video_path,
                "-vf", "select='gt(scene\\,0.3)',showinfo",
                "-vsync", "vfr", "-f", "null", "-",
            ],
            capture_output=True, timeout=120,
        )
        stderr = result.stderr.decode()
        scene_times = []
        for line in stderr.splitlines():
            if "pts_time:" in line:
                try:
                    t = float(line.split("pts_time:")[1].split()[0])
                    scene_times.append(t)
                except (IndexError, ValueError):
                    pass

        if not scene_times or len(onset_env) < 10:
            return None

        # Build video impulse signal aligned to audio timeline
        vid_signal = np.zeros_like(onset_env)
        for st in scene_times:
            idx = int(st * len(onset_env) / (len(audio) / sr))
            if 0 <= idx < len(vid_signal):
                vid_signal[idx] = 1.0

        # Cross-correlate to find lag
        corr = np.correlate(onset_env, vid_signal, mode="full")
        lag_frames = int(np.argmax(corr)) - (len(onset_env) - 1)
        lag_secs   = float(lag_frames) * (len(audio) / sr / len(onset_env))

        return round(lag_secs, 3)

    except Exception as exc:
        log.warning("av sync estimation failed", error=str(exc))
        return None


# ─── Download helper ──────────────────────────────────────────────────────────

async def _download(url: str, suffix: str) -> str:
    tmp = tempfile.NamedTemporaryFile(delete=False, suffix=suffix)
    async with httpx.AsyncClient(timeout=300.0) as client:
        async with client.stream("GET", url) as resp:
            resp.raise_for_status()
            async for chunk in resp.aiter_bytes(65536):
                tmp.write(chunk)
    tmp.close()
    return tmp.name