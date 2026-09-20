"""JEVDecisionEngine — adapter for the external JEV service
(section 7.3 of the master design).

This is a real network-client shape, not a placeholder that pretends
to work. If no endpoint/credentials are configured, `decide()` raises
JEVNotConfiguredError immediately rather than silently falling back —
the composition root (currently cli/main.py) decides what to do about
that, e.g. fall back to LocalDecisionEngine.
"""
from __future__ import annotations

import os
from dataclasses import dataclass

from idios.domain.errors import JEVNotConfiguredError
from idios.domain.state import DecisionResult, DecisionState


@dataclass
class JevConnectionSettings:
    """Plain settings for reaching JEV. Deliberately not the pydantic
    config model from idios.config.loader, so this module has zero
    import dependency on the config layer."""

    endpoint: str
    api_key_env: str
    timeout_seconds: float = 10.0

    @property
    def api_key(self) -> str | None:
        return os.environ.get(self.api_key_env) or None


class JEVDecisionEngine:
    def __init__(self, settings: JevConnectionSettings):
        self._settings = settings

    def decide(self, state: DecisionState) -> DecisionResult:
        if not self._settings.endpoint or not self._settings.api_key:
            raise JEVNotConfiguredError(
                "JEV backend selected but no endpoint/API key is "
                "configured; set [jev].endpoint in configs/local.toml "
                f"and the {self._settings.api_key_env} environment "
                "variable, or switch decision_engine.backend to 'local'."
            )
        raise NotImplementedError(
            "JEV HTTP client isn't implemented yet — this is an honest "
            "stub, not a fabricated integration. See docs/decisions.md."
        )
