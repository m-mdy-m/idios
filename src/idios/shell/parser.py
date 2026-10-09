"""Input parser: a handful of explicit rules, no scoring.

``parse(line)`` turns one line of user input into an ``Intent``. It knows
nothing about context or the database, so it is trivial to test. Anything
that cannot be classified safely comes back as ``statement`` or
``maybe_question`` and the shell asks the user what it should be saved as.

Kinds
-----
directive      ``:search register``       text=name, arg=rest
note/highlight/quote/answer/concept/topic/category/tag/author
               ``note: ...``              text=payload
goal/source    ``goal: X`` / ``source: X`` text=title (arg="bare" if no colon)
did/plan       ``did: X`` / ``plan tomorrow: X``  text=X, arg=when ("" = default)
task_mark      ``done tk1 tk2`` / ``skip tk3``     text=refs, arg=done|skip
reference      ``url: https://...`` / ``path: ~/Shelf/x.pdf`` for the current source
source_add     ``source add``
new_source     ``book: X``                text=title, arg=source type
location       ``chapter 2``              text=field, arg=value
link           ``link A to B [as type]``  text=A, arg=B, extra=type
question       ``Why ...?``
maybe_question ``Why registers matter``   starts like a question, no '?'
statement      anything else; extra="answer" if it reads like an answer
empty
"""
from __future__ import annotations

import re
from dataclasses import dataclass

from idios.domain.models import SourceType


@dataclass(frozen=True)
class Intent:
    kind: str
    text: str = ""
    arg: str = ""
    extra: str = ""


#: ``keyword: payload`` forms, mapped to intent kinds
_PREFIXES = {
    "note": "note",
    "highlight": "highlight",
    "quote": "quote",
    "answer": "answer",
    "concept": "concept",
    "topic": "topic",
    "category": "category",
    "tag": "tag",
    "author": "author",
    "goal": "goal",
    "source": "source",
    "url": "reference",
    "path": "reference",
}
_SOURCE_TYPES = {t.value for t in SourceType}
_LOCATION_WORDS = {"chapter": "chapter", "page": "page", "section": "section",
                   "time": "timestamp", "timestamp": "timestamp"}
_BARE_QUIT = {"help", "quit", "exit", "status", "clear", "cls"}

# did: ...   plan: ...   plan tomorrow: ...   did yesterday: ...
_TASK_RE = re.compile(r"^(did|plan)(?:\s+(\S+?))?\s*:\s*(.*)$", re.IGNORECASE | re.DOTALL)
# done tk1 tk2   skip tk3
_MARK_RE = re.compile(r"^(done|skip)\s+((?:[a-z]{1,2}\d+[\s,]*)+)$", re.IGNORECASE)
_PREFIX_RE = re.compile(r"^([A-Za-z]+)\s*:\s*(.*)$", re.DOTALL)
_ANSWER_REF_RE = re.compile(r"^answer\s+([A-Za-z]{1,2}\d+)\s*:\s*(.*)$", re.IGNORECASE | re.DOTALL)
_LINK_RE = re.compile(r"^link\s+(.+?)\s+to\s+(.+?)(?:\s+as\s+([A-Za-z_ \-]+))?$",
                      re.IGNORECASE)
# "chapter 2", "page 14", "section 2.3", "time 12:30" -- one short token with a digit
# (or a roman numeral), so ordinary sentences like "page numbers are boring" stay sentences.
_BARE_LOCATION_RE = re.compile(
    r"^(chapter|page|section|time|timestamp)\s+((?=\S*\d|[ivx]{1,6}$)\S{1,20})$", re.IGNORECASE)
_BARE_CONTEXT_RE = re.compile(r"^(goal|source)\s+(\S.*)$", re.IGNORECASE)
_THIS_RE = re.compile(r"^(highlight|quote)(\s+this)?$", re.IGNORECASE)

_QUESTION_WORDS = ("what", "why", "how", "when", "where", "who", "whom", "whose", "which",
                   "is", "are", "was", "were", "does", "do", "did", "can", "could",
                   "should", "would", "will", "shall", "may", "might")
_ANSWER_CUES = re.compile(
    r"^(because|since|it is|it's|its|they are|they're|this is|that is|"
    r"a|an|the|i think|i believe|i guess|probably|maybe|basically)\b",
    re.IGNORECASE,
)


def parse(line: str) -> Intent:
    text = line.strip()
    if not text:
        return Intent("empty")

    if text.startswith(":"):
        name, _, rest = text[1:].partition(" ")
        return Intent("directive", name.strip().lower(), rest.strip())
    if text.lower() in _BARE_QUIT:
        return Intent("directive", text.lower())

    m = _ANSWER_REF_RE.match(text)
    if m:
        return Intent("answer", m.group(2).strip(), m.group(1))

    m = _TASK_RE.match(text)
    if m:
        return Intent(m.group(1).lower(), m.group(3).strip(), (m.group(2) or "").strip())
    m = _MARK_RE.match(text)
    if m:
        return Intent("task_mark", m.group(2).strip(), m.group(1).lower())

    m = _PREFIX_RE.match(text)
    if m:
        word, payload = m.group(1).lower(), m.group(2).strip()
        if word in _PREFIXES:
            return Intent(_PREFIXES[word], payload)
        if word in _SOURCE_TYPES:
            return Intent("new_source", payload, word)
        if word in _LOCATION_WORDS:
            return Intent("location", _LOCATION_WORDS[word], payload)

    m = _THIS_RE.match(text)
    if m:
        return Intent(m.group(1).lower(), "", extra="this")

    if text.lower() in ("source add", "add source"):
        return Intent("source_add")

    m = _LINK_RE.match(text)
    if m:
        return Intent("link", m.group(1).strip(), m.group(2).strip(),
                      (m.group(3) or "related_to").strip())

    m = _BARE_LOCATION_RE.match(text)
    if m:
        return Intent("location", _LOCATION_WORDS[m.group(1).lower()], m.group(2).strip())

    m = _BARE_CONTEXT_RE.match(text)
    if m and len(text.split()) <= 12 and not text.endswith(("?", "؟")):
        return Intent(m.group(1).lower(), m.group(2).strip(), "bare")

    if text.endswith(("?", "؟", "？")):
        return Intent("question", text)

    first = text.split()[0].lower().strip(",")
    if first in _QUESTION_WORDS and len(text.split()) >= 3:
        return Intent("maybe_question", text)

    return Intent("statement", text, extra="answer" if _ANSWER_CUES.match(text) else "")
