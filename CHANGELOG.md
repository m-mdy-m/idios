# Changelog

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.4.2]

### Removed
- `SKILL.md`, `ROADMAP.md`, `SUPPORT.md` and `docs/adr/` (psx rules `support`, `roadmap`, `adr` turned off).

### Changed
- Docs, banner, CLI help and GitHub templates now describe what IDIOS does instead of what it lacks;
  stale issue/PR template wording removed.
- The project describes itself as "a terminal notebook for learning" in `pyproject.toml`, the
  README, `idios --help` and the module docstrings.
- CI tests Python 3.10, the declared minimum, and a second job runs `psx check --fail-on error`
  on every push.

### Added
- `docs/COMMANDS.md`: every command with examples.

### Fixed
- Reminders from cron: `notify-send` now gets the user's session bus and `DISPLAY`; under WSL the
  notification is shown on the Windows desktop via `powershell.exe`.

## [0.4.1]

### Changed
- The ASCII logo now appears on every interactive start (not only the first); the full welcome
  still shows once. `IDIOS_NO_LOGO=1` hides the logo. Scripts and pipes never show it.

## [0.4.0]

### Added
- Evening loop: `did: …` records what you did, `plan[ when]: …` plans a day (default tomorrow),
  `done tk1 tk2` / `skip tk3` update tasks, `:plan` shows today and tomorrow.
- `:review` walks through the day's open items (done / not yet → tomorrow / skip) and saves a note.
  Offered automatically on launch after the review time; never in scripts or pipes.
- Scheduled reminder: `:schedule HH:MM|install|remove|test` and `idios schedule`, backed by the
  OS scheduler (crontab / Task Scheduler) installed only after confirmation. `idios remind`
  sends a desktop notification (notify-send / osascript / PowerShell) or prints.
- Task entity (`tk`), schema v2 with automatic migration, search/export/delete support.
- `idios plan`, `idios review`, `idios remind` CLI commands; `:help plan`; example `07_evening_loop.idios`.

## [0.3.1]

### Added
- First-run welcome banner with an ASCII logo (once, interactive terminals only; `:intro` replays it).
- `clear` / `cls` / `:clear` clears the screen and redraws the context. The old `:clear`
  (forget current concept and question) is now `:forget`.
- Colour: automatic on a terminal, off when piped, `NO_COLOR` / `IDIOS_COLOR` respected;
  kinds have stable colours, search matches are highlighted, errors are red with a hint.
- Help with worked examples: a one-screen `:help` plus topics
  (`:help learn | sources | context | concepts | search | manage | examples`).
- `examples/`: five runnable `.idios` sessions and a Python API example;
  `idios run FILE` plays a script as a transcript, `--home` selects a data folder.
- `url:` and `path:` set where the current source lives.
- psx integration: `psx.yml`, and the project files its rules expect (LICENSE, SECURITY,
  CODE_OF_CONDUCT, SUPPORT, ROADMAP, ADRs, ARCHITECTURE, Makefile, CODEOWNERS, CodeQL and
  secret-scan workflows, pre-commit, editorconfig). `psx check` runs in CI.

## [0.3.0] — Rewrite

IDIOS was rewritten from scratch to follow `SKILL.md` and `AGENTS.md`:
a local-first learning environment where `idios` opens an interactive shell,
instead of a decision engine with an LLM client and a JSON state file.

### Added
- Interactive shell (`idios`): deterministic input parser, active context
  (goal / source / location / concept / question), automatic internal sessions,
  "Welcome back" resume, ambiguity menu instead of silent guessing.
- SQLite storage (`~/.idios/idios.db`, override with `IDIOS_HOME`) with stable
  ULID-style ids and display numbers (`q3`, `c1`).
- Sources (book, article, paper, video, website) as references: paths and URLs
  are stored, files are never copied.
- Goals, questions (open/answered), answers, notes, highlights, quotes,
  concepts, topics, categories, tags, people/authors.
- Typed relation graph, independent from tags and categories.
- BM25 search over everything (`:search`, `idios search`).
- Source, concept, question and goal views; `:status`; `:shelf`.
- Export to Markdown or JSON (`idios export`).
- Test suite that runs offline, including architecture rules.

### Removed
- Decision engine, JEV, model registry, LLM client, JSON state storage,
  `rich` and `pydantic` dependencies. The core now has no runtime dependencies.
