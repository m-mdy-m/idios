"""Desktop notification providers for Linux, BSD and macOS."""
from __future__ import annotations

import os
import shutil
import subprocess
import sys
from pathlib import Path
from typing import Callable, Optional

from .activation import activation_uri
from .common import clean, session_env

Runner = Callable[..., object]


def _get_asyncio():
    """Lazy import to avoid Windows _overlapped issues at module load time."""
    import asyncio
    return asyncio


def _run_desktop_notifier(
    title: str,
    body: str,
    on_click: Optional[Callable[[], None]],
    timeout: int,
) -> bool:
    try:
        from desktop_notifier import DesktopNotifier
    except ImportError:
        return False

    asyncio = _get_asyncio()

    if sys.platform == "darwin":
        # macOS notifications need CoreFoundation's run loop integrated with
        # asyncio for activation callbacks to be delivered reliably.
        try:
            from rubicon.objc.eventloop import EventLoopPolicy
            asyncio.set_event_loop_policy(EventLoopPolicy())
        except ImportError:
            if on_click is not None:
                return False

    async def dispatch() -> bool:
        notifier = DesktopNotifier(app_name="IDIOS")
        if on_click is None:
            await notifier.send(title=title, message=body)
            return True

        loop = asyncio.get_running_loop()
        clicked = asyncio.Event()
        dismissed = asyncio.Event()

        def mark(event: asyncio.Event) -> None:
            loop.call_soon_threadsafe(event.set)

        await notifier.send(
            title=title,
            message=body,
            on_clicked=lambda: mark(clicked),
            on_dismissed=lambda: mark(dismissed),
            timeout=timeout if sys.platform.startswith("linux") else None,
        )
        click_task = asyncio.create_task(clicked.wait())
        dismiss_task = asyncio.create_task(dismissed.wait())
        try:
            await asyncio.wait(
                {click_task, dismiss_task}, timeout=timeout,
                return_when=asyncio.FIRST_COMPLETED,
            )
        finally:
            click_task.cancel()
            dismiss_task.cancel()
        if clicked.is_set():
            on_click()
        return True

    try:
        return asyncio.run(dispatch())
    except Exception:
        # Native notification services vary across desktop environments. The
        # caller can use a CLI provider or print the reminder instead.
        return False


def _send_terminal_notifier(
    title: str,
    body: str,
    home: Optional[Path],
    which: Callable[[str], Optional[str]],
) -> bool:
    terminal_notifier = which("terminal-notifier")
    if not terminal_notifier:
        return False
    command = [sys.executable, "-m", "idios", "--_open-schedule", activation_uri(home)]
    try:
        subprocess.run(
            [terminal_notifier, "-title", clean(title, 100), "-message", clean(body, 400),
             "-execute", subprocess.list2cmdline(command) if os.name == "nt" else _shell_join(command)],
            check=True, timeout=15, capture_output=True,
        )
        return True
    except (OSError, subprocess.SubprocessError):
        return False


def _shell_join(parts: list[str]) -> str:
    # shlex is only needed on macOS/Unix; import it lazily.
    import shlex
    return shlex.join(parts)


def _send_notify_send(
    title: str,
    body: str,
    run: Runner,
    which: Callable[[str], Optional[str]],
    on_click: Optional[Callable[[], None]],
    timeout: int,
) -> bool:
    exe = which("notify-send")
    if not exe:
        return False
    cmd = [exe, "--app-name=IDIOS"]
    if on_click is not None:
        # Freedesktop Notifications' reserved "default" action fires for a
        # body-click. --wait keeps this short-lived job alive for the callback.
        cmd.extend(["--wait", "--expire-time=300000", "--action=default=Open IDIOS"])
    cmd.extend([clean(title, 100), clean(body, 400)])
    try:
        proc = run(cmd, check=True, timeout=max(timeout + 10, 30), capture_output=True,
                   text=True, env=session_env())
    except (OSError, subprocess.SubprocessError):
        return False
    if on_click is not None and getattr(proc, "stdout", "").strip() == "default":
        on_click()
    return True


def _send_osascript(
    title: str,
    body: str,
    run: Runner,
    which: Callable[[str], Optional[str]],
    on_click: Optional[Callable[[], None]],
) -> bool:
    if on_click is not None:
        # osascript's basic display-notification command has no click callback.
        # terminal-notifier can launch a cold process when the user clicks.
        return False
    exe = which("osascript")
    if not exe:
        return False

    def quote(value: str) -> str:
        return value.replace("\\", "\\\\").replace('"', '\\"')

    script = f'display notification "{quote(clean(body, 400))}" with title "{quote(clean(title, 100))}"'
    try:
        run([exe, "-e", script], check=True, timeout=15, capture_output=True)
    except (OSError, subprocess.SubprocessError):
        return False
    return True


def send_desktop(
    title: str,
    body: str,
    *,
    run: Runner = subprocess.run,
    platform: str = sys.platform,
    which: Callable[[str], Optional[str]] = shutil.which,
    on_click: Optional[Callable[[], None]] = None,
    home: Optional[Path] = None,
    interaction_timeout: int = 300,
) -> bool:
    """Choose the best available provider for a Unix-like desktop."""
    title, body = clean(title, 100), clean(body, 400)
    if platform == "darwin" and on_click is not None:
        if _send_terminal_notifier(title, body, home, which):
            return True

    if _run_desktop_notifier(title, body, on_click, interaction_timeout):
        return True

    if platform.startswith("linux") or "bsd" in platform:
        return _send_notify_send(title, body, run, which, on_click, interaction_timeout)
    if platform == "darwin":
        return _send_osascript(title, body, run, which, on_click)
    return False
