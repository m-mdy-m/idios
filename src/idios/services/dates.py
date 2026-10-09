"""Small, predictable date phrases for planning: today, tomorrow, monday, 2026-10-12, +3d."""
from __future__ import annotations

import re
from datetime import date, timedelta

from idios.domain.errors import Invalid

_WEEKDAYS = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"]
_ISO = re.compile(r"^\d{4}-\d{2}-\d{2}$")
_PLUS = re.compile(r"^\+(\d{1,3})d?$")
_TIME = re.compile(r"^([01]?\d|2[0-3]):([0-5]\d)$")

WHEN_HINT = "plan tomorrow: ...   plan monday: ...   plan 2026-10-12: ...   plan +2d: ..."


def parse_when(word: str, today: date) -> date:
    """Turn a phrase into a date. ``monday`` means the *next* Monday after today."""
    w = word.strip().lower()
    if w in ("", "tomorrow", "tmrw", "tom"):
        return today + timedelta(days=1)
    if w == "today":
        return today
    if w == "yesterday":
        return today - timedelta(days=1)
    for i, name in enumerate(_WEEKDAYS):
        if w == name or (len(w) >= 3 and name.startswith(w)):
            ahead = (i - today.weekday()) % 7 or 7
            return today + timedelta(days=ahead)
    m = _PLUS.match(w)
    if m:
        return today + timedelta(days=int(m.group(1)))
    if _ISO.match(w):
        try:
            return date.fromisoformat(w)
        except ValueError:
            pass
    raise Invalid(f"I don't understand the day '{word.strip()}'.", hint=WHEN_HINT)


def label(day: date, today: date) -> str:
    """'Today · Thu 8 Oct', 'Tomorrow · Fri 9 Oct', 'Mon 12 Oct'."""
    stamp = f"{day:%a} {day.day} {day:%b}"
    delta = (day - today).days
    names = {0: "Today", 1: "Tomorrow", -1: "Yesterday"}
    return f"{names[delta]} · {stamp}" if delta in names else stamp


def parse_clock(text: str) -> str:
    """Validate ``HH:MM`` (24h) and return it zero-padded."""
    m = _TIME.match(text.strip())
    if not m:
        raise Invalid(f"'{text.strip()}' is not a time.", hint=":schedule 21:00   (24-hour HH:MM)")
    return f"{int(m.group(1)):02d}:{m.group(2)}"
