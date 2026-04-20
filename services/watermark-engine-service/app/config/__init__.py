from __future__ import annotations
import os
from functools import lru_cache
import yaml
from pydantic_settings import BaseSettings, SettingsConfigDict


class WatermarkConfig(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="WM_WATERMARK_")
    strength:       float = 0.08
    frames_per_sec: float = 1.0
    payload_bits:   int   = 64
    jpeg_quality:   int   = 95
    luma_only:      bool  = True


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="WM_", env_nested_delimiter="__")
    env:       str = "local"
    port:      int = 9105
    workers:   int = 2
    log_level: str = "DEBUG"
    watermark: WatermarkConfig = WatermarkConfig()


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    s = Settings()
    path = os.environ.get("WM_CONFIG_PATH", "config.yaml")
    if os.path.exists(path):
        with open(path) as f:
            raw = yaml.safe_load(f)
        for k, v in _flatten(raw, "WM_").items():
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