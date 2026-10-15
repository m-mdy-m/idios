# Architecture

IDIOS is a small personal tool with a deep internal model. Layers only call downward.

```text
cli
 ├─ shell       parser · interpreter loop · render · help · io
 ├─ services    context · learning · knowledge · sources · search · views · export
 │   └─ notifications  activation URI · shared helpers · WinRT Toast · desktop providers
 ├─ ui          optional Tkinter desktop surfaces + reusable dark theme
 ├─ domain      typed dataclasses, ids, user-facing errors
 └─ storage     SQLite schema and repositories — the only SQL
```

## Request flow

1. `shell/parser.py` turns a line into an `Intent` (no database, no context).
2. `shell/shell.py` picks a handler and calls a service.
3. Services apply the active context (source, location, goal, concept) and use repositories.
4. `shell/render.py` formats the result; `shell/style.py` adds colour only on a terminal.

## Rules (enforced by `tests/test_architecture.py`)

- The base install has no third-party runtime dependencies; Linux/macOS can use the optional `.[notifications]` provider, while Windows Toast uses built-in WinRT/PowerShell.
- SQL only in `storage/` (and the export dump).
- `domain/`, `services/`, `storage/` never print or read input.
- `services/notifications/windows.py` sends modern `ToastGeneric` notifications through WinRT, creates the required per-user Start Menu shortcut/AppUserModelID, and registers `idios:` for cold-click activation.
- `services/notifications/activation.py` owns the deep-link contract, `windows.py` owns WinRT Toast setup, and `desktop.py` owns Linux/BSD/macOS providers. The clicked schedule is read-only and uses the same SQLite data as the CLI.

## Data

Entities have stable ULID-style ids and a never-reused display number (`q3`, `c1`).
Mentions (`refs`), tags and typed relations live in separate tables on purpose: a tag is not
a relation and a category is not a relation.

