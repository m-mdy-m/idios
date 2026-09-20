"""Minimal local-model client (Phase 1, first slice).

A thin, swappable interface over "some local LLM I can POST a prompt
to and get text back." The default implementation talks to Ollama
(https://ollama.com) because it's the lowest-friction way to run a
small quantized model on the target hardware — see docs/decisions.md
for why this was picked over llama.cpp-server/LM Studio/vLLM, and for
how to swap it out if you're already running something else.

Nothing here fabricates a response: a connection failure, timeout, or
unexpected response shape raises ModelUnavailableError, never a canned
string.
"""
from __future__ import annotations

import json
import urllib.error
import urllib.request
from typing import Protocol

from idios.domain.errors import ModelUnavailableError


class LLMClient(Protocol):
    def generate(self, system: str, prompt: str) -> str: ...


class OllamaClient:
    """Calls Ollama's /api/generate endpoint. Requires `ollama serve`
    running locally and the model already pulled
    (`ollama pull <model>`)."""

    def __init__(self, model: str, host: str = "http://localhost:11434", timeout: float = 60.0):
        self._model = model
        self._host = host.rstrip("/")
        self._timeout = timeout

    def generate(self, system: str, prompt: str) -> str:
        payload = json.dumps(
            {
                "model": self._model,
                "system": system,
                "prompt": prompt,
                "stream": False,
                "options": {"temperature": 0.2},
            }
        ).encode("utf-8")
        request = urllib.request.Request(
            f"{self._host}/api/generate",
            data=payload,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(request, timeout=self._timeout) as response:
                body = json.loads(response.read().decode("utf-8"))
        except (urllib.error.URLError, TimeoutError, ConnectionError) as exc:
            raise ModelUnavailableError(
                f"couldn't reach Ollama at {self._host} for model "
                f"'{self._model}': {exc}. Is `ollama serve` running and "
                f"is the model pulled (`ollama pull {self._model}`)?"
            ) from exc
        except json.JSONDecodeError as exc:
            raise ModelUnavailableError(f"Ollama returned an unreadable response: {exc}") from exc

        try:
            return body["response"]
        except KeyError as exc:
            raise ModelUnavailableError(f"Ollama response missing 'response' field: {body}") from exc
