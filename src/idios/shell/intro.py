"""The first-run banner. Shown once (and again with ``:intro``)."""
from __future__ import annotations

from idios import __version__
from idios.shell.style import style

# Block letters, one list of rows per letter (figlet "standard" style).
_LETTERS = {
    "I": [" ___ ",
          "|_ _|",
          " | | ",
          " | | ",
          "|___|"],
    "D": [" ____  ",
          "|  _ \\ ",
          "| | | |",
          "| |_| |",
          "|____/ "],
    "O": ["  ___  ",
          " / _ \\ ",
          "| | | |",
          "| |_| |",
          " \\___/ "],
    "S": [" ____  ",
          "/ ___| ",
          "\\___ \\ ",
          " ___) |",
          "|____/ "],
}
_ROW_COLORS = ["bcyan", "bcyan", "bblue", "bmagenta", "bmagenta"]


def art(word: str = "IDIOS") -> list[str]:
    """The word as plain ASCII rows (no colour)."""
    return ["".join(_LETTERS[c][row] for c in word).rstrip() for row in range(5)]


def logo() -> str:
    """The ASCII logo and a one-line tagline: shown on every interactive start."""
    rows = [style.paint(row, color, "bold") for row, color in zip(art(), _ROW_COLORS)]
    tag = style.dim(f"learn · connect · recall   v{__version__}")
    return "\n".join(["", *("  " + r for r in rows), "  " + tag, ""])


def banner() -> str:
    rows = [style.paint(row, color, "bold") for row, color in zip(art(), _ROW_COLORS)]
    out = ["", *("  " + r for r in rows), ""]
    out.append("  " + style.bold("A personal cognitive and learning operating system."))
    out.append("  " + style.dim(f"v{__version__} · your notes live in ~/.idios on this machine"))
    out += [
        "",
        "  " + style.label("How it works"),
        "    " + "You read. IDIOS keeps the questions, answers, highlights and ideas,",
        "    " + "files them under what you are reading, and connects them.",
        "",
        "  " + style.label("Try it"),
        f"    {style.command('goal: Learn Assembly')}",
        f"    {style.command('book: Programming from the Ground Up')}",
        f"    {style.command('Why does the CPU need registers?')}",
        "",
        "  " + style.dim("Type ") + style.command(":help") + style.dim(" for examples, ")
        + style.command("clear") + style.dim(" to clear the screen, ")
        + style.command(":quit") + style.dim(" to leave."),
        "",
    ]
    return "\n".join(out)
