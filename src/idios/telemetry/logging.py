"""Structured logging / observability (section 18 of the master design).

Every orchestration step should be traceable: request/task id, decision,
confidence, retrieved sources, model selected, latency, tool calls,
verification result, memory mutations, final outcome. `TelemetryEvent`
is the typed shape for one such step; `log_event` emits it as one JSON
line so it's easy to grep/parse later without a logging backend.
"""
from __future__ import annotations

import json
import logging
import sys
import time
from dataclasses import asdict, dataclass, field
from typing import Any


@dataclass
class TelemetryEvent:
    request_id: str
    task_id: str
    step: str
    timestamp: float = field(default_factory=time.time)
    decision: str | None = None
    decision_confidence: float | None = None
    retrieved_sources: list[str] = field(default_factory=list)
    model_selected: str | None = None
    model_latency_ms: float | None = None
    tool_calls: list[str] = field(default_factory=list)
    verification_result: str | None = None
    memory_mutations: list[str] = field(default_factory=list)
    outcome: str | None = None
    extra: dict[str, Any] = field(default_factory=dict)


def get_logger(name: str = "idios") -> logging.Logger:
    logger = logging.getLogger(name)
    if not logger.handlers:
        handler = logging.StreamHandler(sys.stdout)
        handler.setFormatter(logging.Formatter("%(message)s"))
        logger.addHandler(handler)
        logger.setLevel(logging.INFO)
    return logger


def log_event(event: TelemetryEvent, logger: logging.Logger | None = None) -> None:
    (logger or get_logger()).info(json.dumps(asdict(event), default=str))
