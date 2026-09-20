"""Model registry (sections 7.9-7.10 of the master design).

Holds capability profiles per role and resolves which one to use for a
given role. No model is actually loaded here — this only tracks *which*
model/backend is configured for each role, so routing logic and the
rest of the codebase never hard-code a vendor or model name. Actual
loading/inference wiring belongs to Phase 1+.
"""
from __future__ import annotations

import enum
from dataclasses import dataclass, field


class ModelRole(str, enum.Enum):
    ROUTER = "router"
    FAST_GENERAL = "fast_general"
    REASONING = "reasoning"
    CODER = "coder"
    EMBEDDING = "embedding"
    RERANKER = "reranker"
    VERIFIER = "verifier"


@dataclass
class ModelProfile:
    role: ModelRole
    backend: str
    model_id: str | None = None
    max_context_tokens: int | None = None
    enabled: bool = True
    extra: dict = field(default_factory=dict)


class ModelRegistry:
    def __init__(self, profiles: dict[ModelRole, ModelProfile]):
        self._profiles = profiles

    @classmethod
    def from_config(cls, models_config: dict) -> "ModelRegistry":
        profiles: dict[ModelRole, ModelProfile] = {}
        known = {"role", "backend", "model_id", "max_context_tokens", "enabled"}
        for _key, raw in models_config.get("models", {}).items():
            role = ModelRole(raw["role"])
            profiles[role] = ModelProfile(
                role=role,
                backend=raw["backend"],
                model_id=raw.get("model_id"),
                max_context_tokens=raw.get("max_context_tokens"),
                enabled=raw.get("enabled", True),
                extra={k: v for k, v in raw.items() if k not in known},
            )
        return cls(profiles)

    def resolve(self, role: ModelRole) -> ModelProfile:
        try:
            profile = self._profiles[role]
        except KeyError as exc:
            raise KeyError(f"no model profile configured for role={role.value}") from exc
        if not profile.enabled:
            raise KeyError(f"model profile for role={role.value} is disabled")
        return profile

    def roles(self) -> list[ModelRole]:
        return list(self._profiles.keys())
