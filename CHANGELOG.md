# Changelog

All notable changes to IDIOS are documented here. This file follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the Go Engine uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.2] — 2026-10-20

- **Launch command:** running `idios` without arguments opens the normal interactive workspace, while explicit CLI commands such as `idios version` remain unchanged. `make install` and `make install-user` install the named executables on Unix-like systems and Windows; release builds include user-level Linux/macOS and Windows installers that put `idios` on PATH.
- **Cross-platform updates:** the TUI checks the latest stable GitHub Release in the background at startup. A lower-right Upgrade control opens the release changelog; the updater verifies `SHA256SUMS.txt`, stages both executables, replaces the old build after the current process exits, and relaunches the updated application. A responsive `IDIOS UPGRADE` animation shows download, verification and hand-off activity.
- **Windows PATH:** the native setup installer and the local Make installer add their per-user install directory to PATH and remove only the path entry they added when uninstalled. The Windows Make targets now use PowerShell rather than Unix-only `install` and `rm` commands, and stage/rollback both binaries during replacement.

### Added

- **Edit pull requests:** an open pull request can now be edited (title and description) in the TUI (`Edit pull request`) and in the CLI (`idios pr edit`), so it can follow the work until it is accepted. Accepted and closed pull requests stay read-only.

### Changed

- **Graph page:** rewritten to be readable. It now opens with a summary and a short legend, then **What is blocked right now** (each waiting Issue with its unfinished prerequisite), then every node with its links worded as sentences (“has pull request”, “uses concept”, “waits for”) and ordered blocking → workflow → context. Each link is printed once, under the more central node, instead of repeating relationship groups plus a separate all-nodes list. Ownership links (repository owns issue, PR targets branch) and unlinked branches/labels/milestones are hidden by default behind **Ownership links: hidden → show**; a focused graph has a **Distance** control and shows links in both directions. Long lists say how many nodes were left out instead of silently stopping at 40.
- **Templates:** the bundled Issue (`task`, `bug`, `feature`, `research`) and Pull Request templates in `internal/config/defaults.json` are shorter and use fill-in lines, a checkable “Done when” list, a verification block for real command output, and a risk/undo section. The GitHub issue forms and pull request template were rewritten the same way (duplicate-search and latest-version checks, “where does it happen”, version field, project checks and compatibility checklist). `.github/ISSUE_TEMPLATE/config.yml` now points to `SECURITY.md` at the repository root.
- **TUI redesign:** navigation items are now tabs (filled for the current page, quiet otherwise), the top bar no longer repeats the tabs, the repository header is one line instead of a banner, and panel titles use the accent color. The footer shows only the keys that work on the current screen (forms, filter mode, intro, lists).
- **Themes:** `midnight` (now cyan instead of purple), `paper` and `forest` were retuned for contrast, and `ocean`, `ember` and a high-contrast `contrast` theme were added. Packaged themes are now always loaded from the application, so configs written by older releases no longer freeze the old purple colors. and the new optional `on_accent` token controls text on accent backgrounds (fixes unreadable button text on light themes).
- **Copy:** all in-app text was rewritten to say what a screen is for and what to do next. Marketing phrases and repeated "local-first" taglines were removed. The intro screen now shows a "Get started" checklist that follows the real state of the database.

### Fixed

- **Clean end-user downloads:** exclude `idios-tui-demo` and extended guides from portable archives and native installers. User-facing packages contain only the `idios` CLI, clean `idios-tui`, `README.md`, and `LICENSE`; the demo executable and extended guides remain available in the source repository for development and CI smoke tests.
- Release checks validate the exact four-file portable bundle and verify that native package payloads do not contain the demo executable.

## [1.0.0] — 2026-10-19

IDIOS 1.0.0 is a re-platforming of IDIOS into a local-first Go workspace engine. It introduces a
repository and Issue workflow with structured dependencies, Pull Request-based completion,
knowledge relationships, a separate CLI and TUI, local SQLite persistence, and a refreshed
keyboard-first terminal experience. This release is **not** a claim of full feature parity with the
former Python application, and it does not perform real Git operations or remote synchronization.

### Added

- **Modular Go Engine:** a layered architecture with domain models, application use cases, storage
  ports, a SQLite adapter, configuration, CLI, rendering helpers, and a separate TUI presentation
  package. The domain and application core do not depend on terminal UI code or CLI flag parsing.
- **Workspace and repositories:** persistent local workspace data; repository metadata and settings;
  a default logical `main` branch for each repository; repository-level labels, milestones, and
  activity history.
- **Issues and sub-issues:** Issues with descriptions, status, priority, due dates, parent/child
  relationships, and blocker information. Parent-child relationships are validated, and invalid
  cycles are rejected.
- **Cross-repository dependencies:** typed `depends_on` relationships between Issues, including
  prerequisites in other repositories. An Issue cannot be started while required blockers remain
  incomplete; semantic graph relationships do not silently become workflow blockers.
- **Pull Request completion workflow:** Issues are completed through Pull Requests rather than by
  directly switching an Issue to `done`. Accepting a Pull Request creates a `CompletionRecord` on
  logical `main` history and marks the associated Issue complete.
- **Logical branch and completion history:** default branch metadata, Pull Request target references,
  ordered completion records, and relationships between Pull Requests, completions, Issues, and
  branches. Only the logical `main` workflow is active in this release.
- **Projects spanning repositories:** Projects can organize Issues from multiple repositories
  without turning the Project itself into a Repository.
- **Knowledge graph:** Concepts and typed semantic relations, Issue-to-Concept associations, and a
  graph projection that distinguishes structural relationships, workflow dependencies, and semantic
  connections.
- **Labels and milestones:** global and repository-scoped labels; Repository- and Project-scoped
  milestones; and Issue assignments for labels and milestones.
- **Workspace profile and important items:** a profile-style Overview with editable profile details
  and README content, important Repository and Issue pins, active work, and recent activity.
- **Notes and Quotes:** persistent free-form Notes that can be associated with supported workspace
  entities and explicitly included in the graph, plus saved Quotes that rotate in the workspace
  introduction/Overview.
- **Scheduled jobs and local notifications:** reminder jobs with intervals and enable/disable state,
  execution history, and a local notification inbox. Jobs are evaluated while the TUI is running;
  they are not an operating-system background service.
- **Customization:** selectable theme presets, semantic color-token overrides, layout density,
  configurable keyboard shortcuts, a startup-intro preference, Debug logging preferences, and
  editable `REPOSITORY_TEMPLATE`, `ISSUE_TEMPLATE`, and `PULL_REQUEST_TEMPLATE` sections. Template
  bodies are merged with packaged defaults and copied into new records.
- **Structured creation forms:** configurable task, bug, feature, and research Issue templates with
  title prefixes and multiline acceptance criteria; a configurable Pull Request review template;
  and a Repository README/overview scaffold that substitutes the Repository name. The TUI supports
  multiline typing, caret navigation, and multiline paste for template-backed fields.
- **Intro, help, and diagnostics:** an ASCII IDIOS wordmark, first-screen introduction, in-app help,
  keyboard-focused navigation, visible operation feedback, and a Logs screen for recent application
  events and errors.
- **Keyboard and mouse interaction:** a persistent full-screen, event-driven TUI with navigation,
  selectable content, focusable controls, clickable hit targets, editable single-line and multiline
  forms with caret/paste support, responsive narrow-terminal rendering, and a typed Graph Explorer.
- **Headless database checks:** add `--check` to both TUI entry points so automation can initialize
  and verify the selected database without requiring an interactive terminal.
- **Separate command-line interface:** an `idios` executable with database-path selection, JSON output
  for supported commands, repository/Issue/Project/Concept/Label/Milestone/Pull Request/graph
  workflows, and stable error/output structures intended for future integrations.
- **Demo workspace seed:** `idios-tui-demo` automatically provisions an isolated, persistent sample
  database covering Repository, parent/sub-Issue, blocking dependency, Project, Concept, graph, Label,
  Milestone, Pull Request, completion, Note, Quote, pin, Activity, Job and notification relationships.
  The seed is atomic and idempotent. The CLI `idios seed demo` remains available as an explicit action
  against the database selected for the CLI; the standard TUI never seeds a workspace on startup.
- **SQLite persistence and migrations:** storage built on `database/sql` and `go-sqlite3`, versioned
  embedded SQL migrations, transaction/query helpers, time and scalar conversion, and typed SQLite
  error handling.
- **Legacy database importer:** a read-only import path for supported data from the former Python
  database. Imported completed Tasks receive explicitly labelled migration Pull Requests and
  Completion Records so the new completion invariant is preserved without pretending that historical
  Git activity occurred.
- **Project quality infrastructure:** Go-focused Makefile targets, GitHub Actions CI and release
  workflows, CodeQL, secret scanning, Dependabot configuration, pre-commit settings, build scripts,
  contribution guidance, security policy, issue templates, and pull-request templates.
- **Automated tests:** domain/application/CLI/storage/TUI tests covering workflow invariants, blockers,
  JSON output, SQLite migrations, graph behavior, and terminal rendering.

### Changed

- **Runtime and language:** the Go Engine replaces the earlier Python implementation on the `1.x`
  development line. Building the Go Engine requires Go 1.25 or newer and a C toolchain when building
  the SQLite driver with CGO enabled.
- **Storage separation:** the Go Engine uses its own SQLite schema and defaults to
  `~/.idios/idios-engine.sqlite3`. It does not open or overwrite the legacy Python database as though
  both databases shared a schema.
- **Task model:** new work is tracked as an Issue. Python-era Goals, Sources, Questions, time tracking,
  daily planning, and other Task behaviors are not implicitly reproduced as first-class Go features.
- **Completion semantics:** an Issue is not considered complete merely because its status is edited;
  Pull Request acceptance and Completion Record creation are the supported completion path.
- **Interface boundaries:** the CLI and TUI are separate entry points. The TUI calls the Application
  service directly instead of spawning the CLI as a subprocess. TUI code lives in the top-level `tui/`
  package, separated from `internal/` application internals and split across navigation, rendering,
  actions, scheduling, logging, and page modules.
- **Configuration and UI copy:** user preferences are stored separately from packaged UI copy. The
  user's configuration is intended for customization such as theme, colors, density, shortcuts, and
  Debug mode; the English UI content resource is maintained with the application source.
- **Intro branding:** replace the placeholder five-row lettering with a purpose-built seven-row
  ASCII IDIOS wordmark, centered with a quiet divider and product tagline. The high-impact and
  compact variants adapt to terminal width and use printable ASCII characters only.
- **Documentation structure:** user guides are separated from developer and architecture references,
  with user-facing quick start, workflow, settings, shortcut, and CLI material prioritized.
- **Database access:** the previous custom database wrapper is replaced by standard `database/sql`
  with focused connection management, query helpers, migration logic, value conversion, and SQLite
  error translation.
- **Automation:** CI, static analysis, security checks, dependency update policy, and release packaging
  are configured around the Go Engine and its CGO/SQLite build requirements.

### Fixed

- **TUI no-color rendering:** active controls remain recognizable when color is disabled without
  relying on terminal style sequences that can split words or distort display-width calculations.
- **Compact terminal rendering:** the too-small-terminal message is constrained to the terminal width.
- **Shared TUI helpers:** restore the cursor-item and mouse-action message types, Issue/Pull Request
  references, Unicode-safe form editing, case-insensitive filtering, deterministic graph sorting,
  display-width truncation, line fitting, status/progress formatting, and panel rendering required
  by the modular TUI package.
- **TUI helper imports:** include the Bubble Tea v2 type import used by the UI helper adapter.
- **Concurrent first start:** retry briefly when another IDIOS process is initializing the same SQLite
  database and SQLite reports transient lock contention during journal-mode setup.
- **Storage test portability:** avoid using `?` in the reserved-character database filename on
  Windows, where it is not a valid filename character.
- **Semantic graph edge projection:** preserve the actual stored relation type (for example,
  `supports`) when projecting Concept relations into graph edges.
- **Multiline editor viewport test:** validate the caret marker separately from the underlying text
  so an inline cursor does not make a correctly rendered line appear to be missing.

### Documentation

- Reorganized user documentation separately from developer documentation.
- Added user-facing guides for getting started, workspace workflows, customization, shortcuts, and
  command usage.
- Retained developer references for architecture, contribution, migrations, TUI implementation, the
  JSON output contract, and extension points.
- Clarified how to try demo data, how Issue dependencies and Pull Request completion work, what local
  notifications do, and which capabilities are outside the initial release scope.
- Preserved the former Python application's changelog entries as historical context, separate from the
  Go Engine's version line.

### Build, test, and distribution

- Provide Makefile targets for formatting checks, tests, vetting, and building `idios`, `idios-tui`,
  and `idios-tui-demo` executables.
- Configure native CI and release automation for Linux amd64/arm64, macOS amd64/arm64, and Windows
  amd64/arm64. Each target builds all three executables with CGO and runs a SQLite-backed smoke workflow
  on its native runner.
- Publish native installation formats alongside portable archives: Linux `.deb` and `.rpm`, macOS
  `.pkg` installers and `.dmg` containers, and per-user Windows `-setup.exe` installers; retain
  Linux/macOS `.tar.gz` and Windows `.zip` bundles. End-user packages contain the CLI, clean TUI,
  README, and license; the demo TUI and extended guides remain developer/source-repository resources.
- Verify package contents, installable payloads, and SHA-256 checksums before/after publication. Native
  runners download and test the published release assets and exercise the installed binaries against the
  SQLite-backed smoke workflow.
- The release quality gate runs formatting checks, `go test ./...`, `go vet ./...`, and native build
  and runtime verification under Go 1.25+ with CGO and an architecture-matched C compiler.
- **Signing caveat:** the initial packaging workflow does not sign/notarize macOS packages or Authenticode-sign
  the Windows installer; operating-system security warnings may be shown until release signing is configured.

### Migration and compatibility notes

- **No in-place Python database conversion:** back up the old database before importing. The importer
  reads from the legacy path and writes to the new Go Engine database; it must not overwrite the
  source database.
- **Import coverage is intentionally limited:** supported Concepts, Tasks as Issues in a `Legacy
  Tasks` Repository, supported semantic relations, and selected metadata are imported. Goals, Sources,
  Questions, Notes, schedules, and time-tracking records are not yet imported as first-class Go entities
  and remain untouched in the legacy database.
- **Synthetic historical completion records:** completed legacy Tasks receive explicitly labelled
  migration Pull Requests and Completion Records. These are compatibility records, not proof of actual
  historical Pull Requests, Git commits, or merges.
- **Database path:** the default Go database is `~/.idios/idios-engine.sqlite3`. Use `--db PATH` or
  `IDIOS_DB` to select a different path; consider a separate path to test the demo seed.
- **Build prerequisites:** Go 1.25+ and a supported C toolchain are required for the declared build
  configuration because SQLite is compiled with CGO enabled.

### Not included in the initial 1.0.0 scope

- Real Git commits, repository working-tree snapshots, file diffs, Git checkout, or merge algorithms.
- Creation and checkout of multiple active Branches or Branch-history merging; the initial workflow
  uses logical `main` metadata and completion history only.
- GitHub/GitLab hosting integration, remote authentication, cross-device synchronization, or multi-user
  collaboration.
- A graphical desktop application, hosted service, long-running daemon, or network API server.
- Advanced graph analytics and dependency-impact analysis beyond the current dependency and readiness
  rules.
- Complete parity with the former Python notebook, including all legacy entity types, time tracking,
  schedule installation, and operating-system push notification behavior.

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
- `docs/user/CLI_REFERENCE.md`: every command with examples.

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

