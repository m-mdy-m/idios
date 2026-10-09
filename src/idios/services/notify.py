"""Desktop notifications using whatever the operating system already has.

Uses what the system already has: ``notify-send`` (Linux), ``osascript`` (macOS),
PowerShell balloon tip (Windows). ``send`` returns False when none is available,
and the caller falls back to plain text.
"""
from __future__ import annotations

import os
import shutil
import subprocess
import sys
from typing import Callable, Optional

Runner = Callable[..., object]


def _clean(text: str, limit: int) -> str:
    text = "".join(ch for ch in text if ch == "\n" or ch.isprintable())
    return text if len(text) <= limit else text[: limit - 1] + "…"


def is_wsl() -> bool:
    try:
        with open("/proc/version", encoding="utf-8") as fh:
            return "microsoft" in fh.read().lower()
    except OSError:
        return False


def _balloon(title: str, body: str, exe: str) -> list[str]:
    q = lambda s: s.replace("'", "''")  # noqa: E731
    script = ("Add-Type -AssemblyName System.Windows.Forms;"
              "$n = New-Object System.Windows.Forms.NotifyIcon;"
              "$n.Icon = [System.Drawing.SystemIcons]::Information;$n.Visible = $true;"
              f"$n.ShowBalloonTip(15000, '{q(title)}', '{q(body)}',"
              "[System.Windows.Forms.ToolTipIcon]::Info);Start-Sleep -Seconds 15;$n.Dispose()")
    return [exe, "-NoProfile", "-Command", script]


def session_env(env: Optional[dict] = None) -> dict:
    """cron runs without a desktop session; point notify-send at the user's bus."""
    env = dict(os.environ if env is None else env)
    bus = f"/run/user/{os.getuid()}/bus" if hasattr(os, "getuid") else ""
    if "DBUS_SESSION_BUS_ADDRESS" not in env and bus and os.path.exists(bus):
        env["DBUS_SESSION_BUS_ADDRESS"] = f"unix:path={bus}"
    env.setdefault("DISPLAY", ":0")
    return env


def command_for(title: str, body: str, platform: str = sys.platform,
                which: Callable[[str], Optional[str]] = shutil.which,
                wsl: Optional[bool] = None) -> Optional[list[str]]:
    title, body = _clean(title, 100), _clean(body, 400)
    if platform.startswith("linux") and (is_wsl() if wsl is None else wsl) \
            and which("powershell.exe"):
        return _balloon(title, body, "powershell.exe")  # WSL: show it on the Windows desktop
    if platform == "darwin" and which("osascript"):
        esc = lambda s: s.replace("\\", "\\\\").replace('"', '\\"')  # noqa: E731
        return ["osascript", "-e", f'display notification "{esc(body)}" with title "{esc(title)}"']
    if platform == "win32" and which("powershell"):
        q = lambda s: s.replace("'", "''")  # noqa: E731
        script = ("Add-Type -AssemblyName System.Windows.Forms;"
                  "$n = New-Object System.Windows.Forms.NotifyIcon;"
                  "$n.Icon = [System.Drawing.SystemIcons]::Information;$n.Visible = $true;"
                  f"$n.ShowBalloonTip(15000, '{q(title)}', '{q(body)}',"
                  "[System.Windows.Forms.ToolTipIcon]::Info);Start-Sleep -Seconds 15;$n.Dispose()")
        return ["powershell", "-NoProfile", "-Command", script]
    if platform.startswith("linux") and which("notify-send"):
        return ["notify-send", "--app-name=IDIOS", title, body]
    return None


def send(title: str, body: str, run: Runner = subprocess.run,
         platform: str = sys.platform,
         which: Callable[[str], Optional[str]] = shutil.which) -> bool:
    cmd = command_for(title, body, platform, which)
    if cmd is None:
        return False
    try:
        run(cmd, check=True, timeout=30, capture_output=True, env=session_env())
    except (OSError, subprocess.SubprocessError):
        return False
    return True
