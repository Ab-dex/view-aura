from __future__ import annotations
import os
from functools import lru_cache
import yaml
from pydantic_settings import BaseSettings, SettingsConfigDict


class ModelConfig(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="SUB_MODELS_")
    whisper_model: str = "medium"
    device:        str = "cpu"


class SubtitleConfig(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="SUB_SUBTITLES_")
    max_chars_per_line: int   = 42
    max_words_per_cue:  int   = 12
    min_duration_secs:  float = 0.5
    merge_gap_secs:     float = 0.1
    formats: list[str] = ["srt", "vtt"]


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="SUB_", env_nested_delimiter="__")
    env:       str = "local"
    port:      int = 9104
    workers:   int = 1
    log_level: str = "DEBUG"
    models:    ModelConfig    = ModelConfig()
    subtitles: SubtitleConfig = SubtitleConfig()


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    s = Settings()
    path = os.environ.get("SUB_CONFIG_PATH", "config.yaml")
    if os.path.exists(path):
        with open(path) as f:
            raw = yaml.safe_load(f)
        for k, v in _flatten(raw, "SUB_").items():
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