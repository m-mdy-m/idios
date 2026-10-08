# Changelog

All notable changes to IDIOS are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

## [0.3.0] — 2026-10-08 — Rewrite

IDIOS was rewritten from scratch to follow `SKILL.md` and `AGENTS.md`:
a local-first learning environment where `idios` opens an interactive shell,
instead of a decision engine with an LLM client and a JSON state file.

### Added
- Interactive shell (`idios`): deterministic input parser, active context
  (goal / source / location / concept / question), automatic internal sessions,
  "Welcome back" resume, ambiguity menu instead of silent guessing.
- First-run welcome banner with an ASCII logo (once, interactive terminals only;
  `:intro` replays it).
- `clear` / `cls` / `:clear` clears the screen and redraws the context.
- Colour: automatic on a terminal, off when piped, `NO_COLOR` / `IDIOS_COLOR`
  respected; kinds have stable colours, search matches are highlighted,
  errors are red with a hint.
- Help with worked examples: a one-screen `:help` plus topics
  (`:help learn | sources | context | concepts | search | manage | examples`).
- SQLite storage (`~/.idios/idios.db`, override with `IDIOS_HOME` or `--home`)
  with stable ULID-style ids and display numbers (`q3`, `c1`).
- Sources (book, article, paper, video, website) as references: `url:` and
  `path:` set where the current source lives; files are never copied.
- Goals, questions (open/answered), answers, notes, highlights, quotes,
  concepts, topics, categories, tags, people/authors.
- Typed relation graph, independent from tags and categories
  (`link A to B as contains`).
- BM25 search over everything (`:search`, `idios search`).
- Source, concept, question and goal views; `:status`; `:shelf`; `:show`;
  `:delete` always confirms first.
- Export to Markdown or JSON (`idios export`).
- `python -m idios`, and `idios run FILE` to replay a script of inputs as a
  transcript.
- `examples/`: six runnable sessions (five `.idios` plus a Python API script),
  an index in `examples/README.md`, and `examples/run.sh` (`--all`).
- Docs: `docs/README.md`, `docs/ARCHITECTURE.md`, an ADR log under `docs/adr/`
  (ADRs, SQLite via the standard library, the deterministic parser) with a
  template, plus `SKILL.md` and `AGENTS.md`.
- Project health: LICENSE (MIT), SECURITY, CODE_OF_CONDUCT, SUPPORT, ROADMAP,
  CODEOWNERS and a pull-request template; `psx.yml` defines the rules, and
  `tests/test_experience.py` verifies that every file those rules expect exists.
- Tooling: `Makefile` (`setup`, `run`, `test`, `lint`, `check`, `psx`,
  `examples`, `build`, `clean`), `scripts/setup.sh`, `scripts/test.sh`,
  `scripts/build.sh`, `scripts/clean.sh`, `.pre-commit-config.yaml`,
  `.editorconfig`, `.gitattributes` and `.env.example`.
- Test suite that runs offline, including architecture rules
  (`tests/test_architecture.py`): no AI or network imports, no third-party
  runtime dependencies, SQL only in `storage/`, no terminal I/O below the shell.

### Changed
- Layering is now `cli → shell → services → domain`, with SQLite behind
  repositories; `cli/main.py` is a thin entry point over `search`, `show`,
  `shelf`, `run` and `export`.
- Issue templates and `docs/CONTRIBUTING.md` describe the shell and the rules
  enforced by the tests, not the old decision engine.
- `.gitignore` covers `*.db`, `build/`, `data/` and archives instead of `data/`
  alone, so a local database is never committed by accident.

### Removed
- Decision engine, JEV, model registry, LLM client, JSON state storage and
  `configs/*.toml`. The core now has no runtime dependencies.
- The old `examples/01_status_and_basics.sh` and
  `examples/02_search_and_graph.py`, replaced by `.idios` sessions.
- `tests/unit/`, the test suite for the removed modules.

### Fixed
- `pytest -q` collects the rewritten suite again: `tests/unit/` no longer
  imports deleted modules, and `tests/__init__.py` no longer shadows the
  `conftest` helpers.

[Unreleased]: https://github.com/m-mdy-m/idios/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/m-mdy-m/idios/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/m-mdy-m/idios/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/m-mdy-m/idios/releases/tag/v0.1.0