"""Storage abstraction (Phase 0).

Everything above this layer (decision engine, orchestrator, learning
state) should depend on `StorageProvider`, never on a concrete backend,
so swapping JSON-file storage for SQLite/Postgres later is a one-file
change plus a config flip (see docs/decisions.md).
"""
from __future__ import annotations

import json
import threading
from pathlib import Path
from typing import Any, Protocol


class StorageProvider(Protocol):
    def get(self, key: str) -> Any | None: ...
    def set(self, key: str, value: Any) -> None: ...
    def delete(self, key: str) -> None: ...
    def list_keys(self, prefix: str = "") -> list[str]: ...


class JsonFileStorageProvider:
    """Minimal durable key-value store backed by a single JSON file.

    Adequate for Phase 0/1 state volumes. Swap for SQLite once
    concurrent writers or query needs (the knowledge graph, concept-
    state lookups) outgrow a single JSON blob.
    """

    def __init__(self, path: Path):
        self._path = path
        self._lock = threading.Lock()
        self._path.parent.mkdir(parents=True, exist_ok=True)
        if not self._path.exists():
            self._path.write_text("{}", encoding="utf-8")

    def _read_all(self) -> dict[str, Any]:
        try:
            return json.loads(self._path.read_text(encoding="utf-8") or "{}")
        except json.JSONDecodeError:
            return {}

    def _write_all(self, data: dict[str, Any]) -> None:
        self._path.write_text(json.dumps(data, indent=2, default=str), encoding="utf-8")

    def get(self, key: str) -> Any | None:
        with self._lock:
            return self._read_all().get(key)

    def set(self, key: str, value: Any) -> None:
        with self._lock:
            data = self._read_all()
            data[key] = value
            self._write_all(data)

    def delete(self, key: str) -> None:
        with self._lock:
            data = self._read_all()
            data.pop(key, None)
            self._write_all(data)

    def list_keys(self, prefix: str = "") -> list[str]:
        with self._lock:
            return [k for k in self._read_all() if k.startswith(prefix)]
