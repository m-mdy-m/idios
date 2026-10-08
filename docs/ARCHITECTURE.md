# Architecture

IDIOS is a small personal tool with a deep internal model. Layers only call downward.

```text
cli          argparse entry point; `idios` with no arguments opens the shell
 └─ shell    parser · interpreter loop · render · style (colour) · help · io
     └─ services   context · learning · knowledge · sources · search · views · export
         └─ domain       typed dataclasses, ids, user-facing errors
         └─ storage      SQLite schema + repositories (the only SQL)
```

## Request flow

1. `shell/parser.py` turns a line into an `Intent` (no database, no context).
2. `shell/shell.py` picks a handler and calls a service.
3. Services apply the active context (source, location, goal, concept) and use repositories.
4. `shell/render.py` formats the result; `shell/style.py` adds colour only on a terminal.

## Rules (enforced by `tests/test_architecture.py`)

- No AI, embedding or network imports; no runtime dependencies.
- SQL only in `storage/` (and the export dump).
- `domain/`, `services/`, `storage/` never print or read input.

## Data

Entities have stable ULID-style ids and a never-reused display number (`q3`, `c1`).
Mentions (`refs`), tags and typed relations live in separate tables on purpose: a tag is not
a relation and a category is not a relation.

Decisions are recorded in [docs/adr](adr/README.md).
