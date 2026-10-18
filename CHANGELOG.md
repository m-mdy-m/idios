# Changelog

All notable changes to IDIOS are documented here. This file follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the Go Engine release line
uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **TUI no-color rendering:** with `NO_COLOR`/`--no-color` the screen no longer emits escape
  sequences. Lip Gloss rendered the previous bold+underline "active control" style as a
  per-rune sequence, which split words such as `Overview` into fragments and made measured
  line widths larger than the terminal. The active tab/control is now marked with a `▸` rune,
  so nothing depends on styling to be understood.
- **TUI compact view:** the too-small-terminal message can no longer render wider than the
  terminal window.
- **Concurrent first start:** two IDIOS processes opening the same fresh database at the same
  moment could fail with `database is locked`, because SQLite reports `SQLITE_BUSY` at once
  while another connection switches the journal mode. Opening now retries briefly while the
  database is contended.
- **Storage test portability:** the reserved-character database test keeps `?` (which needs URI
  escaping) out of filenames on Windows, where it is not a legal filename character.

## [1.0.0] - 2026-10-17 — First stable Go Engine release

IDIOS 1.0.0 establishes the new Go Engine architecture and workflow model. This is a
re-platforming from the former Python application, not an in-place Python upgrade and not
a promise of complete feature parity. The legacy Python database remains separate and can
be read through the explicit importer described in `docs/MIGRATION.md`.

### Added

- **Modular Go Engine:** distinct Domain, Application/use cases, Store ports, SQLite adapter,
  CLI adapter, rendering helpers, configuration and independent TUI executable. The Core
  does not depend on terminal UI code or CLI flag parsing.
- **Workspace and Repository model:** local-first workspace data, independent repositories,
  a default `main` branch for each repository, repository settings and repository-scoped
  activity history.
- **Issues and Sub-issues:** Issues replace the old Task model in the Go Engine; issue
  descriptions, status, priority, due date, parent/child structure and blocker information
  are represented in the domain. Parent cycles and dependency cycles are rejected.
- **Cross-repository prerequisites:** typed `depends_on` relations connect Issues across
  repositories. Starting an Issue is blocked while its required prerequisites remain
  incomplete; ordinary semantic links do not automatically become blocking dependencies.
- **Required PR workflow:** each Issue must be completed through a Pull Request. The PR
  records the explanation/review context. Accepting it atomically creates a
  `CompletionRecord` on the repository's logical `main` history and marks the Issue `done`.
- **Logical Branch and completion history:** `main`, Branch metadata, PR target references,
  completion sequence and history relationships. The current release intentionally activates
  only `main`; additional branch creation, checkout and merging are future work.
- **Projects across repositories:** project membership can group Issues from multiple
  repositories without making the Project itself a Repository.
- **Knowledge model and graph:** Concepts, typed semantic relations, Issue-to-Concept links,
  and a graph representation that distinguishes semantic links from operational blockers.
- **Labels and milestones:** global and repository-scoped Labels, plus Repository- and
  Project-scoped Milestones that can be assigned to Issues.
- **Activity records:** structured activity events for relevant domain changes.
- **SQLite persistence:** `database/sql` with `github.com/mattn/go-sqlite3`, a focused
  connection setup, transaction/query helpers, scalar/time conversion, typed SQLite error
  mapping, and embedded, versioned SQL migrations.
- **Safer storage startup:** SQLite connection settings for foreign-key enforcement and
  busy timeout, WAL journaling for writable databases, read-only opening for legacy import, migration
  sequence validation, rejection of a database schema newer than the Engine, and a version
  re-check inside the migration write transaction to serialize concurrent initialization.
- **Command-line interface:** separate `idios` executable, flags for the database path and
  machine-readable JSON output, repository/issue/project/concept/label/milestone/PR/graph
  commands, stable error envelopes and a versioned JSON output contract for future GUI clients.
- **Terminal interface:** separate `idios-tui` full-screen application built with Bubble Tea v2 and Lip Gloss v2. It provides persistent GitHub-inspired Workspace/Repository tabs, a sidebar, dashboard cards, clickable mouse-aware controls, selectable lists, in-place editable forms with caret and paste support, visible operation/validation feedback, responsive compact layouts and a typed Graph Explorer with clickable nodes and blocking-vs-semantic relationship labels. It is event-driven and is not a line-oriented `cli --interactive` prompt. The UI dependencies are isolated to `internal/tui`.
- **Legacy import command:** read-only import of Concepts and Tasks from the former Python
  database into the new schema. Imported completed Tasks receive explicitly labelled
  migration PRs and Completion Records so the new workflow invariant is preserved.
- **Developer and project infrastructure:** Go-focused `Makefile`, PSX v3 project policy,
  GitHub Actions CI and release workflows, CodeQL, secret scanning, Dependabot updates for
  Go modules, pre-commit configuration, contribution guidance, security policy, issue/PR
  templates, build scripts and end-to-end smoke workflow.
- **Documentation:** architecture and extension guides, workflow invariants, SQLite storage
  and migration guidance, CLI reference, TUI/Graph usage, JSON protocol and contributor
  instructions.
- **Tests:** domain/application/CLI/storage/TUI tests, including workflow invariants,
  dependency handling, JSON contract behavior, SQLite migration behavior and graph rendering.

### Changed

- **Language and runtime:** the Engine source tree is now Go (`go.mod`) rather than the former
  Python application. Go 1.25+ and a C compiler are required to build with `go-sqlite3` and
  CGO enabled.
- **Storage separation:** the Go Engine uses its own SQLite schema and default database
  (`~/.idios/idios-engine.sqlite3`). It must not open the prior Python database as if it had
  the same schema.
- **Task semantics:** new work is represented as an Issue. Daily planning, reminders,
  schedules and other Python-era Task behaviors are not implicitly reproduced by the Go
  Issue workflow.
- **Completion semantics:** an Issue is not marked `done` directly; PR acceptance and
  Completion Record creation are the required path.
- **UI entry points:** the primary interfaces for this release are the Go CLI and a separate
  Go TUI. The TUI uses the Application Core directly rather than spawning the CLI as a
  subprocess. A Python GUI is not part of this release; the JSON CLI contract is a future
  integration boundary, not a long-running daemon/API server.
- **Quality automation:** CI, CodeQL, release packaging and dependency update policy now target
  the Go Engine and its build requirements. Release bundles are produced for Linux amd64,
  macOS arm64 and Windows amd64 by the release workflow.
- **Storage organization:** the former custom database driver/pool wrapper was removed in
  favor of standard `database/sql`; connection management, query helpers, value conversion,
  SQLite error translation and migrations have focused responsibilities.

### Migration and compatibility notes

- The previous Python application and its database are **not overwritten or migrated in
  place**. Make a backup and use `legacy-import --from PATH` to read the old database.
- The importer currently imports Concepts, Tasks as Issues in a `Legacy Tasks` Repository,
  supported semantic relations and selected old metadata as migration context. It does not
  yet import Goals, Sources, Questions, Notes, schedules or time-tracking records as first-class
  Go entities; these remain untouched in the original database.
- Old `done` Tasks receive an explicitly labelled synthetic migration PR and Completion
  Record. This is migration scaffolding—not evidence that a real old PR or Git commit existed.
- The `0.4.x` and earlier entries below are preserved as **historical releases of the legacy
  Python application**. They are not Go Engine releases and do not imply feature parity.

### Not included in 1.0.0

- Real Git commits, repository file snapshots, source diffs or Git merge algorithms.
- Creating/checking out multiple active Branches or merging Branch histories; only the logical
  `main` workflow is active in this release.
- Remote hosting integration, authentication, multi-user collaboration or synchronization.
- Advanced graph analytics, dependency impact analysis beyond current readiness/blocker rules,
  or a graphical desktop client.
- Full feature parity with the former Python learning notebook, desktop schedule UI and
  notification/reminder subsystem.

---

## Legacy Python application history

The following entries document releases of the former Python implementation, retained as
historical context. They are separate from the Go Engine's `1.x` version line.

## [0.4.3] — Legacy Python application

### Added
- Windows desktop notifications using WinRT Toast and per-user `idios:` URI activation, so clicking a notification can open the schedule details after the reminder process exits.
- Modular desktop-notification providers for Linux and macOS, with command-line fallbacks where supported. Click activation depends on the selected desktop backend and environment.
- Read-only Tkinter schedule dashboard with overdue, today, completed, skipped and upcoming tasks, plus linked goal/source context.
- Reusable dark UI components: IDIOS wordmark, color and typography tokens, cards, status pills, buttons and a scrollable layout.
- Tests for URI activation validation, safe Toast XML escaping, notification-provider behavior and schedule data grouping/rendering.

### Changed
- Replaced the MIT license with the Mozilla Public License 2.0 (MPL-2.0). Changes to MPL-covered files must remain available under MPL when distributed; larger works may combine IDIOS with differently licensed code. See `LICENSE` for the complete terms.
- Refreshed the schedule window with a dark dashboard, colored status metrics, grouped task cards and improved spacing; supported Windows builds also receive a dark title-bar hint.
- Documented notification installation and platform caveats while keeping third-party notification dependencies optional.
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

