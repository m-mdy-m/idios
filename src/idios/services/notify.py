"""Compatibility facade for native desktop notifications.

Modern interactive notifications live in ``services.notifications``. The
public ``send`` name is kept stable for the shell and CLI; Windows Toasts no
longer use the deprecated ``NotifyIcon.ShowBalloonTip`` API.
"""
from __future__ import annotations

import os
import shutil
import sys
from typing import Callable, Optional

from idios.services.notifications import clean as _clean
from idios.services.notifications import send, session_env


def is_wsl() -> bool:
    try:
        with open("/proc/version", encoding="utf-8") as fh:
            return "microsoft" in fh.read().lower()
    except OSError:
        return False


def command_for(
    title: str,
    body: str,
    platform: str = sys.platform,
    which: Callable[[str], Optional[str]] = shutil.which,
    wsl: Optional[bool] = None,
) -> Optional[list[str]]:
    """Return a simple non-interactive fallback command, if one exists.

    Interactive notifications need an OS activation target and are dispatched
    by ``send`` rather than this command-preview helper.
    """
    title, body = _clean(title, 100), _clean(body, 400)
    if platform == "win32":
        # Windows dispatches Toast directly through the modular send() provider;
        # this helper only previews simple CLI fallback commands.
        return None
    if platform == "darwin" and which("osascript"):
        def quote(value: str) -> str:
            return value.replace("\\", "\\\\").replace('"', '\\"')
        return ["osascript", "-e", f'display notification "{quote(body)}" with title "{quote(title)}"']
    if platform.startswith("linux") or "bsd" in platform:
        exe = which("notify-send")
        if exe:
            return [exe, "--app-name=IDIOS", title, body]
    return None
