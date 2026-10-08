"""Stable identifiers and timestamps.

IDs look like ``concept_01HZX...``: a type prefix plus a ULID-style value
(48-bit millisecond time + 80 random bits, Crockford base32). They sort by
creation time and never depend on titles.
"""
from __future__ import annotations

import os
import time
from datetime import datetime, timezone

_ALPHABET = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"


def _encode(value: int, length: int) -> str:
    chars = []
    for _ in range(length):
        chars.append(_ALPHABET[value & 31])
        value >>= 5
    return "".join(reversed(chars))


def new_id(prefix: str) -> str:
    millis = int(time.time() * 1000)
    rand = int.from_bytes(os.urandom(10), "big")
    return f"{prefix}_{_encode(millis, 10)}{_encode(rand, 16)}"


def now() -> str:
    """UTC timestamp, second precision, ISO 8601."""
    return datetime.now(timezone.utc).isoformat(timespec="seconds")
