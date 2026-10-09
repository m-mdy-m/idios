# Contributing to IDIOS

Read `AGENTS.md` (how to build it) and `docs/COMMANDS.md` (what it does) first.
Before adding anything, ask: *does the user need this to learn?*

```bash
./scripts/setup.sh
source .venv/bin/activate
pytest -q
```

## Rules enforced by `tests/test_architecture.py`

1. No runtime dependencies beyond the standard library.
2. No network imports in `src/idios`.
3. SQL lives only in `storage/` (plus the export dump).
4. `domain/`, `services/` and `storage/` never print, read input, or import the shell/CLI.

## Layout

```
cli/        argparse entry point (small surface)
shell/      parser, interpreter loop, rendering, terminal IO
services/   application logic (context, learning, knowledge, sources, search, views, export)
domain/     typed models, ids, errors
storage/    SQLite schema and repositories
```

Keep terminal text in `shell/render.py`, rules in services, SQL in repositories.

## Project shape (psx)

`psx.yml` lists the files a healthy project has (README, license, tests, CI, security policy…).
Run `make psx` (or `psx check`) before opening a PR; `psx fix --dry-run` previews anything missing.
Try your change against the runnable scripts with `make examples`.
