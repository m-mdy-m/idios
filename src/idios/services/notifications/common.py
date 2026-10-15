"""Shared helpers used by platform notification providers."""
from __future__ import annotations

import os
from typing import Optional


def clean(text: str, limit: int) -> str:
    """Normalize short notification text and cap it for native provider limits."""
    text = "".join(ch for ch in text if ch == "\n" or ch.isprintable())
    text = " ".join(text.split())
    return text if len(text) <= limit else text[: limit - 1] + "…"


def session_env(env: Optional[dict] = None) -> dict:
    """Make a user's desktop session bus discoverable to cron-launched jobs."""
    env = dict(os.environ if env is None else env)
    bus = f"/run/user/{os.getuid()}/bus" if hasattr(os, "getuid") else ""
    if "DBUS_SESSION_BUS_ADDRESS" not in env and bus and os.path.exists(bus):
        env["DBUS_SESSION_BUS_ADDRESS"] = f"unix:path={bus}"
    # Don't overwrite a real DISPLAY or Wayland-related environment.
    if "DISPLAY" not in env and "WAYLAND_DISPLAY" not in env:
        env.setdefault("DISPLAY", ":0")
    return env
