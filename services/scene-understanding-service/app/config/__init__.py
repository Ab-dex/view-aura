from __future__ import annotations
import os
from functools import lru_cache
import yaml
from pydantic_settings import BaseSettings, SettingsConfigDict


class ModelConfig(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="SCENE_MODELS_")
    clip_model:    str = "ViT-B-32"
    clip_pretrain: str = "openai"
    whisper_model: str = "base"
    device:        str = "cpu"


class AnalysisConfig(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="SCENE_ANALYSIS_")
    frame_interval_secs:     int   = 5
    scene_change_threshold:  float = 0.35
    min_chapter_secs:        int   = 60
    emotion_labels: list[str] = [
        "tense", "joyful", "melancholy", "romantic",
        "thrilling", "contemplative", "humorous", "horrifying",
    ]


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="SCENE_", env_nested_delimiter="__")
    env:     str = "local"
    port:    int = 9103
    workers: int = 1
    log_level: str = "DEBUG"
    models:   ModelConfig   = ModelConfig()
    analysis: AnalysisConfig = AnalysisConfig()


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    s = Settings()
    cfg_path = os.environ.get("SCENE_CONFIG_PATH", "config.yaml")
    if os.path.exists(cfg_path):
        with open(cfg_path) as f:
            raw = yaml.safe_load(f)
        for k, v in _flatten(raw, "SCENE_").items():
            if os.environ.get(k) is None:
                os.environ[k] = str(v)
        s = Settings()
    return s


def _flatten(d: dict, prefix: str) -> dict[str, str]:
    out: dict[str, str] = {}
    for k, v in d.items():
        key = prefix + k.upper()
        if isinstance(v, dict):
            out.update(_flatten(v, key + "_"))
        elif isinstance(v, list):
            out[key] = ",".join(str(i) for i in v)
        else:
            out[key] = str(v)
    return out