"""Terminal input/output, kept apart from the shell logic so tests can script it."""
from __future__ import annotations

import sys
from pathlib import Path
from typing import Iterable, Optional, Protocol


class IO(Protocol):
    def read(self, prompt: str = "> ") -> Optional[str]:
        """One line of input, or None at end of input."""

    def write(self, text: str = "") -> None: ...

    def clear(self) -> None:
        """Clear the screen (a no-op when output is not a terminal)."""


class ConsoleIO:
    def __init__(self, history_file: Optional[Path] = None) -> None:
        self.history_file = history_file
        self._readline = None
        if sys.stdin.isatty():
            try:  # line editing + history where the platform offers it
                import readline

                self._readline = readline
                if history_file and history_file.exists():
                    readline.read_history_file(str(history_file))
            except (ImportError, OSError):
                self._readline = None

    def read(self, prompt: str = "> ") -> Optional[str]:
        if self._readline:  # readline must not count colour escapes as width
            from idios.shell.style import style
            prompt = style.readline_safe(prompt)
        try:
            return input(prompt)
        except EOFError:
            return None

    def write(self, text: str = "") -> None:
        print(text)

    def clear(self) -> None:
        if not sys.stdout.isatty():
            return
        if sys.platform == "win32":
            import os
            os.system("cls")
        else:  # clear screen, home the cursor, and drop the scrollback
            print("\033[2J\033[3J\033[H", end="", flush=True)

    def close(self) -> None:
        if self._readline and self.history_file:
            try:
                self.history_file.parent.mkdir(parents=True, exist_ok=True)
                self._readline.set_history_length(500)
                self._readline.write_history_file(str(self.history_file))
            except OSError:
                pass


class ScriptedIO:
    """Feeds a fixed list of lines and records everything written. For tests."""

    def __init__(self, lines: Iterable[str]) -> None:
        self._lines = list(lines)
        self.output: list[str] = []

    def read(self, prompt: str = "> ") -> Optional[str]:
        if not self._lines:
            return None
        line = self._lines.pop(0)
        self.output.append(f"{prompt}{line}")
        return line

    def write(self, text: str = "") -> None:
        self.output.append(text)

    def clear(self) -> None:
        self.output.append("<clear>")

    @property
    def text(self) -> str:
        return "\n".join(self.output)


class ScriptIO:
    """Plays a script file (``idios run FILE``): echoes each line like a transcript.

    Blank lines are skipped. Lines starting with ``#`` are narration and are
    printed dimmed instead of being executed.
    """

    def __init__(self, lines: Iterable[str], prompt: str = "> ") -> None:
        self._lines = [line.rstrip("\n") for line in lines]

    def read(self, prompt: str = "> ") -> Optional[str]:
        from idios.shell.style import style
        while self._lines:
            line = self._lines.pop(0)
            if not line.strip():
                continue
            if line.lstrip().startswith("#"):
                print(style.dim(line.lstrip()))
                continue
            print(f"{prompt}{style.command(line)}")
            return line
        return None

    def write(self, text: str = "") -> None:
        print(text)

    def clear(self) -> None:  # a transcript keeps everything
        pass
