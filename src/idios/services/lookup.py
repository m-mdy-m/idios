"""Resolve what the user typed to an entity: ``q12``, a full id, or a name."""
from __future__ import annotations

import re
from typing import Optional

from idios.domain.models import ENTITY_TYPES, Entity
from idios.storage.repos import Store

_REF = re.compile(r"^([a-z]{1,2})(\d+)$", re.IGNORECASE)
_BY_LETTERS = {v[2]: k for k, v in ENTITY_TYPES.items()}
#: entity types that can be found by a name or title
_NAMED = ("concept", "source", "goal", "topic", "category", "tag", "person")


def lookup(store: Store, text: str) -> Optional[Entity]:
    text = " ".join(text.split())
    if not text:
        return None
    match = _REF.match(text)
    if match and match.group(1).lower() in _BY_LETTERS:
        etype = _BY_LETTERS[match.group(1).lower()]
        return store.by_type[etype].get_by_seq(int(match.group(2)))
    if "_" in text:
        prefix = text.split("_", 1)[0]
        for etype, meta in ENTITY_TYPES.items():
            if meta[1] == prefix:
                return store.by_type[etype].get(text)
    for etype in _NAMED:
        repo = store.by_type[etype]
        hit = repo.by_name(text)  # type: ignore[attr-defined]
        if hit:
            return hit
    for etype in _NAMED:  # fall back to a unique fragment
        hits = store.by_type[etype].like(text)  # type: ignore[attr-defined]
        if len(hits) == 1:
            return hits[0]
    return None
