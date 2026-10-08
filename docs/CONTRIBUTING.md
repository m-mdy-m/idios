# Contributing to IDIOS

Read `SKILL.md` (what IDIOS is) and `AGENTS.md` (how to build it) first.
Before adding anything, ask: *does the user need this to learn?*

```bash
./scripts/bootstrap.sh
source .venv/bin/activate
pytest -q          # offline, no API keys
```

## Rules enforced by `tests/test_architecture.py`

1. No AI, embedding or network imports anywhere in `src/idios`.
2. No runtime dependencies beyond the standard library.
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

`psx.yml` lists the files a healthy project has (README, license, tests, CI, security policy, ADRs…).
Run `make psx` (or `psx check`) before opening a PR; `psx fix --dry-run` previews anything missing.
Record decisions that are hard to reverse in `docs/adr/`.
Try your change against the runnable scripts with `make examples`.
