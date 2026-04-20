from __future__ import annotations

import os
from functools import lru_cache

import yaml
from pydantic import field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class Thresholds(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="MODERATION_THRESHOLDS_")

    nsfw_flag:           float = 0.5
    nsfw_reject:         float = 0.90
    hate_speech_flag:    float = 0.6
    hate_speech_reject:  float = 0.85
    spoiler_flag:        float = 0.55
    escalate_threshold:  float = 0.70


class ModelConfig(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="MODERATION_MODELS_")

    nsfw_image:   str = "Falconsai/nsfw_image_detection"
    hate_speech:  str = "Hate-speech-CNERG/dehatebert-mono-english"
    spoiler_zsl:  str = "facebook/bart-large-mnli"
    device:       str = "cpu"


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_prefix="MODERATION_",
        env_nested_delimiter="__",
    )

    env:     str = "local"
    port:    int = 9102
    workers: int = 1
    log_level: str = "DEBUG"

    thresholds: Thresholds = Thresholds()
    models:     ModelConfig = ModelConfig()


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    """Load settings. Config file values are overridden by env vars."""
    settings = Settings()

    # Overlay config.yaml if present.
    cfg_path = os.environ.get("MODERATION_CONFIG_PATH", "config.yaml")
    if os.path.exists(cfg_path):
        with open(cfg_path) as f:
            raw = yaml.safe_load(f)

        # Flatten and apply only keys not already set by env.
        flat = _flatten(raw, "MODERATION_")
        for k, v in flat.items():
            if os.environ.get(k) is None:
                os.environ[k] = str(v)

        # Reload with the new env vars.
        settings = Settings()

    return settings


def _flatten(d: dict, prefix: str) -> dict[str, str]:
    """Recursively flatten a nested dict into UPPER_CASE env-var names."""
    out: dict[str, str] = {}
    for k, v in d.items():
        key = prefix + k.upper()
        if isinstance(v, dict):
            out.update(_flatten(v, key + "_"))
        else:
            out[key] = str(v)
    return out