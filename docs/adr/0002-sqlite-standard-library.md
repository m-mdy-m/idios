# 2. SQLite via the standard library, no runtime dependencies

Date: 2026-10-08

## Status

Accepted

## Context

IDIOS is a personal, local-first tool (`SKILL.md` §27, `AGENTS.md` §13). The data is
relational (questions have answers, concepts have relations) and small.

## Decision

Store everything in one SQLite file (`~/.idios/idios.db`) using the `sqlite3` module.
No ORM, no `rich`, no `pydantic`. Schema version lives in `PRAGMA user_version`.
Search is BM25 computed in Python from the rows, so the index can never be stale.

## Consequences

- Works offline with `pip install` and nothing else.
- SQL is confined to `storage/`, enforced by `tests/test_architecture.py`.
- Very large libraries (hundreds of thousands of records) would need a persistent index.
  That is not a problem this project has.
