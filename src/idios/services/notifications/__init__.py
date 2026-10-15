"""Modular, cross-platform desktop notifications for IDIOS.

This module is the stable public facade. OS-specific implementations live in
``windows.py`` and ``desktop.py``; optional desktop providers are imported only
when used so the base package remains dependency-free.
"""
from __future__ import annotations

import shutil
import subprocess
import sys
from pathlib import Path
from typing import Callable, Optional

from .common import clean, session_env
from .desktop import send_desktop

Runner = Callable[..., object]


def _send_windows_toast(title: str, body: str, home: Optional[Path]) -> bool:
    try:
        from .windows import send_toast
        return send_toast(clean(title, 100), clean(body, 400), home)
    except Exception:
        # Platform-specific WinRT/PowerShell failures must not crash a reminder job.
        return False


def send(
    title: str,
    body: str,
    run: Runner = subprocess.run,
    platform: str = sys.platform,
    which: Callable[[str], Optional[str]] = shutil.which,
    *,
    on_click: Optional[Callable[[], None]] = None,
    home: Optional[Path] = None,
    interaction_timeout: int = 300,
) -> bool:
    """Send a native notification and open IDIOS when activation is supported.

    Windows uses URI activation on modern WinRT Toasts, so click-to-open works
    after the original scheduled command exits. Linux/macOS callbacks are
    handled by their desktop provider while it remains active (bounded timeout).
    """
    title, body = clean(title, 100), clean(body, 400)
    if platform == "win32":
        # Never silently fall back to the deprecated NotifyIcon balloon API.
        return _send_windows_toast(title, body, home)
    return send_desktop(
        title, body, run=run, platform=platform, which=which, on_click=on_click,
        home=home, interaction_timeout=interaction_timeout,
    )


__all__ = ["clean", "send", "session_env"]
