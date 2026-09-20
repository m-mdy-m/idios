"""Layered TOML config loading (Phase 0).

Load order: configs/default.toml, then configs/local.toml (machine-
specific overrides — not meant to hold secrets; secrets come from
environment variables, see .env.example). local.toml overrides
default.toml, merged recursively table-by-table.
"""
from __future__ import annotations

from pathlib import Path

try:
    import tomllib  # Python 3.11+
except ModuleNotFoundError:  # pragma: no cover - only hit on Python < 3.11
    import tomli as tomllib  # type: ignore[no-redef]

from pydantic import BaseModel, Field


class AppMeta(BaseModel):
    name: str = "idios"
    environment: str = "default"


class HardwareConfig(BaseModel):
    gpu_vram_gb: float = 0
    cpu_threads: int = 1
    allow_gpu: bool = True


class DecisionEngineConfig(BaseModel):
    backend: str = "local"  # "local" or "jev"


class JevConfig(BaseModel):
    endpoint: str = ""
    api_key_env: str = "IDIOS_JEV_API_KEY"
    timeout_seconds: float = 10.0


class StorageConfig(BaseModel):
    backend: str = "json_file"
    path: str = "data/state/state.json"


class BudgetsConfig(BaseModel):
    max_model_calls_per_task: int = 6
    max_retrieval_calls_per_task: int = 4
    max_tool_calls_per_task: int = 8
    token_budget_per_task: int = 8000


class AppConfig(BaseModel):
    app: AppMeta = Field(default_factory=AppMeta)
    hardware: HardwareConfig = Field(default_factory=HardwareConfig)
    decision_engine: DecisionEngineConfig = Field(default_factory=DecisionEngineConfig)
    jev: JevConfig = Field(default_factory=JevConfig)
    storage: StorageConfig = Field(default_factory=StorageConfig)
    budgets: BudgetsConfig = Field(default_factory=BudgetsConfig)


def _load_toml(path: Path) -> dict:
    if not path.exists():
        return {}
    with path.open("rb") as fh:
        return tomllib.load(fh)


def _deep_merge(base: dict, override: dict) -> dict:
    merged = dict(base)
    for key, value in override.items():
        if isinstance(value, dict) and isinstance(merged.get(key), dict):
            merged[key] = _deep_merge(merged[key], value)
        else:
            merged[key] = value
    return merged


def load_app_config(config_dir: Path) -> AppConfig:
    merged = _deep_merge(
        _load_toml(config_dir / "default.toml"),
        _load_toml(config_dir / "local.toml"),
    )
    return AppConfig.model_validate(merged)


def load_models_config(config_dir: Path) -> dict:
    return _load_toml(config_dir / "models.toml")
