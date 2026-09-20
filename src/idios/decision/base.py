"""Decision engine interface (section 7.3 of the master design).

The orchestrator (and, for now, the CLI) must depend only on this
Protocol, never on a concrete engine, so JEV can be swapped in later
without touching orchestration code.
"""
from __future__ import annotations

from typing import Protocol, runtime_checkable

from idios.domain.state import DecisionResult, DecisionState


@runtime_checkable
class DecisionEngine(Protocol):
    def decide(self, state: DecisionState) -> DecisionResult: ...
