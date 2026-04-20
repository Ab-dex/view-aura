from __future__ import annotations

import os
from functools import lru_cache

import yaml
from pydantic import field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class ScoringConfig(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="REC_SCORING_")

    collaborative_weight:      float = 0.5
    content_weight:            float = 0.3
    mood_weight:               float = 0.2
    cold_start_threshold:      int   = 10
    cold_start_collaborative:  float = 0.1
    cold_start_content:        float = 0.2
    cold_start_mood:           float = 0.7
    candidate_pool_size:       int   = 500
    default_limit:             int   = 20
    max_limit:                 int   = 100


class FeatureStoreConfig(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="REC_FEATURE_STORE_")

    redis_url:         str = "redis://localhost:6379"
    vector_key_prefix: str = "user:"
    vector_suffix:     str = ":embedding"
    vector_dim:        int = 512
    vector_ttl_secs:   int = 86400


class MoodConfig(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="REC_MOOD_")

    model_id:       str   = "sentence-transformers/all-MiniLM-L6-v2"
    device:         str   = "cpu"
    n_clusters:     int   = 40
    min_similarity: float = 0.25


class KafkaConfig(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="REC_KAFKA_")

    brokers:           str       = ""
    group_id:          str       = "recommendation-service"
    auto_offset_reset: str       = "earliest"
    topics:            list[str] = ["ratings", "watch_events"]


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_prefix="REC_",
        env_nested_delimiter="__",
    )

    env:        str = "local"
    port:       int = 9101
    workers:    int = 1
    log_level:  str = "DEBUG"
    go_api_url: str = "http://cinemaos-api:8080"

    scoring:       ScoringConfig       = ScoringConfig()
    feature_store: FeatureStoreConfig  = FeatureStoreConfig()
    mood:          MoodConfig          = MoodConfig()
    kafka:         KafkaConfig         = KafkaConfig()


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    settings = Settings()

    cfg_path = os.environ.get("REC_CONFIG_PATH", "config.yaml")
    if os.path.exists(cfg_path):
        with open(cfg_path) as f:
            raw = yaml.safe_load(f)
        flat = _flatten(raw, "REC_")
        for k, v in flat.items():
            if os.environ.get(k) is None:
                os.environ[k] = str(v)
        settings = Settings()

    return settings


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