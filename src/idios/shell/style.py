"""Terminal colours (ANSI), switched off automatically when they would be noise.

Colour is on only when stdout is a terminal that supports it. It is off when:
  * output is piped or redirected (so scripts and tests see plain text),
  * ``NO_COLOR`` is set (https://no-color.org),
  * ``TERM=dumb``.
``IDIOS_COLOR=always`` or ``IDIOS_COLOR=never`` overrides the detection.

Everything is a plain string function: ``style.ok("saved")``. With colour
off they return the text untouched, so layout never depends on it. Pad text
*before* styling it, because escape codes have no visible width.
"""
from __future__ import annotations

import os
import re
import sys

_RESET = "\033[0m"
_CODES = {
    "bold": "1", "dim": "2", "italic": "3", "underline": "4",
    "red": "31", "green": "32", "yellow": "33", "blue": "34",
    "magenta": "35", "cyan": "36", "white": "37",
    "bred": "91", "bgreen": "92", "byellow": "93", "bblue": "94",
    "bmagenta": "95", "bcyan": "96",
}
_ANSI = re.compile(r"\033\[[0-9;]*m")

#: colour per kind of thing, so a Question always looks like a Question
KIND_COLORS = {
    "source": "bblue", "goal": "bgreen", "question": "byellow", "answer": "green",
    "note": "blue", "highlight": "yellow", "quote": "cyan", "concept": "bmagenta",
    "topic": "magenta", "category": "magenta", "tag": "cyan", "person": "cyan",
    "session": "dim",
}


def detect() -> bool:
    mode = os.environ.get("IDIOS_COLOR", "auto").lower()
    if mode == "always":
        return True
    if mode == "never" or "NO_COLOR" in os.environ:
        return False
    if os.environ.get("TERM") == "dumb":
        return False
    try:
        tty = sys.stdout.isatty()
    except (AttributeError, ValueError):
        return False
    if tty and sys.platform == "win32":
        os.system("")  # enables ANSI escape processing on modern Windows consoles
    return tty


class Style:
    def __init__(self, enabled: bool | None = None) -> None:
        self.enabled = detect() if enabled is None else enabled

    # -- primitives ------------------------------------------------------
    def paint(self, text: str, *names: str) -> str:
        if not self.enabled or not text or not names:
            return text
        codes = ";".join(_CODES[n] for n in names)
        return f"\033[{codes}m{text}{_RESET}"

    def strip(self, text: str) -> str:
        return _ANSI.sub("", text)

    def readline_safe(self, text: str) -> str:
        """Wrap escapes so readline measures the prompt width correctly."""
        return _ANSI.sub(lambda m: f"\001{m.group(0)}\002", text)

    # -- semantic helpers ------------------------------------------------
    def bold(self, t: str) -> str: return self.paint(t, "bold")
    def dim(self, t: str) -> str: return self.paint(t, "dim")
    def ok(self, t: str) -> str: return self.paint(t, "bgreen", "bold")
    def error(self, t: str) -> str: return self.paint(t, "bred")
    def warn(self, t: str) -> str: return self.paint(t, "byellow")
    def accent(self, t: str) -> str: return self.paint(t, "bcyan", "bold")
    def command(self, t: str) -> str: return self.paint(t, "bcyan")
    def ref(self, t: str) -> str: return self.paint(t, "dim")
    def label(self, t: str) -> str: return self.paint(t, "bold", "blue")
    def match(self, t: str) -> str: return self.paint(t, "bold", "byellow")

    def heading(self, t: str) -> str:
        return self.paint(t, "bold", "bmagenta")

    def kind(self, entity_type: str, text: str, bold: bool = False) -> str:
        color = KIND_COLORS.get(entity_type, "white")
        return self.paint(text, color, "bold") if bold else self.paint(text, color)

    def number(self, n: int) -> str:
        return self.paint(str(n), "dim") if n == 0 else self.paint(str(n), "bold", "bgreen")


#: shared instance; the shell and renderers read ``style.enabled`` at call time
style = Style()


def set_enabled(enabled: bool) -> None:
    style.enabled = enabled
