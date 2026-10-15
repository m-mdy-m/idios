"""Stable deep links shared by native notification providers and the CLI."""
from __future__ import annotations

from pathlib import Path
from typing import Optional
from urllib.parse import parse_qsl, urlencode, urlsplit

PROTOCOL = "idios"


def activation_uri(home: Optional[Path] = None) -> str:
    """Build the only supported notification target, carrying the data folder."""
    params = {"home": str(home.expanduser().resolve())} if home is not None else {}
    query = urlencode(params)
    return f"{PROTOCOL}://schedule/open" + (f"?{query}" if query else "")


def home_from_activation_uri(uri: str) -> Optional[Path]:
    """Validate a notification target and recover its optional data folder."""
    parsed = urlsplit(uri)
    if (parsed.scheme.lower() != PROTOCOL or parsed.netloc.lower() != "schedule"
            or parsed.path != "/open" or parsed.fragment):
        raise ValueError("unsupported URI")
    query = parse_qsl(parsed.query, keep_blank_values=True)
    if any(key != "home" for key, _value in query):
        raise ValueError("unsupported query parameter")
    values = [value for key, value in query if key == "home"]
    if len(values) > 1:
        raise ValueError("multiple data folders were supplied")
    if values and not values[0].strip():
        raise ValueError("empty data folder")
    return Path(values[0]).expanduser() if values else None
