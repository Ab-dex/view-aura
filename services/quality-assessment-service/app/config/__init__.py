from __future__ import annotations
import os
from functools import lru_cache
import yaml
from pydantic_settings import BaseSettings, SettingsConfigDict


class QualityConfig(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="QA_QUALITY_")
    vmaf_model:               str   = "hd"
    artifact_sample_interval: int   = 30
    clipping_threshold_db:    float = -1.0
    av_sync_tolerance_secs:   float = 0.083
    vmaf_threshold:           float = 70.0
    blocking_threshold:       float = 15.0


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="QA_", env_nested_delimiter="__")
    env:       str = "local"
    port:      int = 9106
    workers:   int = 1
    log_level: str = "DEBUG"
    quality:   QualityConfig = QualityConfig()


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    s = Settings()
    path = os.environ.get("QA_CONFIG_PATH", "config.yaml")
    if os.path.exists(path):
        with open(path) as f:
            raw = yaml.safe_load(f)
        for k, v in _flatten(raw, "QA_").items():
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
        else:
            out[key] = str(v)
    return out