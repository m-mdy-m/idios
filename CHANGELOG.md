# Changelog

All notable changes to IDIOS are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] — 2026-10-22

Version 2.0.0 adds the local Web workspace over the existing Go engine, adds dated Plans with daily
reminders and a simulated repository history (issues, virtual files, immutable tags), and restructures
the client adapters from `internal/` into `clients/` (`cli`, `tui`, `server`, `web`) around a shared
protocol and feature catalog.

### Changed
- Reworked Overview to follow a calmer, atmospheric dark layout: a full quote panel beside Notes, with Latest changes spanning the lower workspace.
- Moved quote controls to a compact lower toolbar and simplified the quote label to “Quotes”.
- Added subtle local SVG atmosphere assets for Quotes, Notes, and Latest changes; no external image requests or runtime network dependencies.
- Refined the Notes card with a soft visual backdrop, tag chips, and compact carousel controls.
- Reworked Recent activity into a date-grouped tree timeline with tighter rows, grouped event counts, and clearer hover affordances.
- Removed the redundant desktop Overview heading so content begins below the app breadcrumb; desktop layout fills the viewport without page scrolling, while narrow screens preserve natural scrolling.
- Rebalanced the Dashboard overview into a quote panel beside one compact Notes card, with the recent-activity tree across the full lower row.
- Removed Quick Capture from Overview, simplified the quote heading to “Quotes”, refined the quote panel into a quieter dark surface, and strengthened the activity connectors, row hierarchy, typography, and hover states.
- The desktop Overview now fills the available viewport without page scrolling at normal desktop heights; smaller screens and short windows retain natural scrolling instead of clipping content.

### Fixed
- Daily Plans now presents a forward-looking seven-day strip anchored on the selected date, so the default Today view starts with today at the far left rather than the previous Sunday.
- Replaced the simple notification beep with a three-stage, escalating original IDIOS plan alarm: on reminder arrival, again 5 seconds later, and a final action-style sequence 15 seconds after the second alarm.
- Added layered synthetic klaxon, impact, crack and ricochet-style effects plus an urgent system-generated voice line for later stages; no recorded firearm/explosion audio or copied film-character voice is used.
- Increased the default alert volume to 100% on the new volume preference key, retained an adjustable per-browser volume slider, and added a preview of the final alarm.
- Opening the plan/date or checking off a task stops pending repeat stages so the alert does not continue nagging after acknowledgement.

### Added

- **Dated Plans and task checklists:** added reusable plans with a local calendar date, description, and ordered checklist of completable tasks, with optional links to a Repository or Issue and a dedicated Plans page.
- **Plan input safety:** the application layer rejects plans with no non-empty checklist tasks, even when a client bypasses browser-side validation; plans may remain intentionally unlinked to an Issue or Repository.
- **Daily plan reminders:** added idempotent morning, midday, and evening check-ins configured in local 24-hour time. Reminders are persisted in the notification inbox and can trigger native desktop notifications; clicking a date reminder opens that day's plans.
- **IDIOS notification alert and browser permission controls:** added a three-stage escalating alarm for plan reminders, persistent per-browser volume control, sound toggle and final-stage preview, and an explicit browser-notification permission action. Audio playback respects browser autoplay permission and can be muted per browser.
- **Plan progress in Activity Atlas:** dates with planned tasks use a coral-to-teal color blend based on completed task ratio; manually marked rough days continue to override all activity and plan colors.
- **Simulated repository workflow:** PRs can include multiple in-progress Issues from one Repository. Merging a PR atomically completes all included Issues and records exactly one immutable simulated Commit/Completion with a short identifier; no Git binary, real branch, or host filesystem merge is invoked.
- **Issue classification:** added Task, Bug, Feature, and Research types, with repository-level delivered-work counts by type.
- **Virtual repository File Tree:** every Repository receives a virtual `README.md`; users can create, edit, inspect, and delete safely validated virtual files stored in SQLite.
- **Repository history tree:** repository history expands each simulated PR/Commit into its PR description and linked Issues, including issue type, saved description, priority, and completion timestamps.
- **Immutable release tags:** tags such as `0.0.0` and `0.1.0` keep independent file snapshots, changes from the previous tag, simulated commits included in the release interval, and a direct download link to the previous tag. ZIP downloads include IDIOS tag/history manifests and are explicitly not Git bundles.
- **Merge-snapshot safety:** once a new-format simulated Commit exists, tags use its immutable file snapshot and do not accidentally publish later unmerged virtual file edits; empty snapshots are distinguishable from legacy completions that predate snapshots.
- **Activity Atlas redesign:** replaced the wall of open monthly calendars with a compact, equal-height month selector and one expanded calendar at a time. The selected calendar always uses a fixed six-week, seven-day grid; month selection and day selection drive the existing activity timeline.
- **Repository README preview:** render each simulated repository README in Overview with safe Markdown formatting for headings, lists, links, quotes, and code blocks; provide a direct Edit README action.
- **Repository Insights disclosure:** group delivery metrics, completed-issue type counts, and latest merges behind one collapsed-by-default Insights control. Selecting a recent commit opens the full History tab with that commit expanded and its linked issues loaded.
- **Consistent repository tab spacing:** add a shared content frame and responsive padding across Issues, Pull requests, Branches, History, Files, Tags, Labels, Milestones, Settings, Activity, and Graph.

- **Personal rough-day markers:** added per-day red flags that override the green activity intensity, can be toggled from the selected-day action or by right-clicking a date, and persist in browser local storage. Activity days otherwise use five green intensity levels.
- **Activity month insights:** added a monthly snapshot, weekly event pulse, equal-height month selectors, and a fixed 6 × 7 calendar canvas with future dates left unselectable.
- **Activity timeline refinement:** added a count/type summary and structured date-grouped event rows with clear type badges and timestamps; full details are now collapsed by default and open on demand or when a date/filter is selected.
- **Overview redesign:** removed the redundant Active issues panel and generic metric-card wall, replaced the decorative Workspace Pulse with a compact context bar, displayed recent changes as visual event tiles, and condensed Notes into a small quick-capture card.

- **Local Web workspace:** added a responsive React + TypeScript interface for Dashboard, Issues, Repositories, Projects, Knowledge Graph, Notes, Activity, Notifications, and Settings.
- **Go HTTP server:** exposed a typed loopback-only API over the existing Application service and served the production Web UI from the `idios` binary.
- **Background scheduler:** added a user-level Go service option to process scheduled jobs while the server is running.
- **Autostart controls:** added `idios server install|uninstall|start|stop|status` and `idios web` commands, with per-user startup integration for Linux, macOS, and Windows.
- **Shared configuration:** added XDG config-path support, legacy config-path fallback, server/Web settings, and lexical `config.d/*.json` fragments.
- **Shared feature catalog:** added `internal/features` metadata for Web routes, TUI top/side/cycle navigation, feature descriptions, and route-specific entry points; clients read the same catalog from `/api/features`.
- **Client parity:** surfaced profile and pins, TUI keyboard-shortcut editing, notification/settings deep links, repository subview deep links, scheduled queue and history timestamps, and batch-create actions in Web.
- **Single-operation batch path:** TUI batch creation now executes each item through the same Go protocol command as single-item and Web actions.
- **Interactive knowledge graph:** extracted a graph canvas with manual node positioning, persisted local layout, cursor-centered zoom, drag-to-pan, focus controls, keyboard selection, and a relationship inspector.
- **Graph presentation modes:** introduced distinct Organic and Flowchart layouts, type-specific node shapes/sizes/colors, orthogonal flowchart connectors, and branch-hover subgraphs for relevant labels and milestones (hidden by default).
- **Graph motion design:** added smooth position morphing between layouts, animated relationship emphasis for a selected node, gentle node hover/selection feedback, and fade-in transitions for graph entities/edges. Animations respect `prefers-reduced-motion`, while pointer-driven dragging remains immediate.
- **Durable graph layout:** added SQLite-backed GraphLayout persistence for active mode, per-mode node positions, and per-mode pan/zoom, plus a reset endpoint that resets only the active layout; added storage, validation and HTTP API tests.
- **Reusable UI flows:** unified issue creation across Dashboard, Issues, and Repository views; added a common confirmation dialog built on the existing modal shell.
- **Validation pipeline:** added shared protocol validation for unknown fields, required values, types, choices, dates, colors, UTF-8, null bytes, and input-size limits, with application-layer checks before store access.
- **Internal diagnostics:** added log categories/filters, safe log clearing, structured request error context, request and failure counters, latency/memory/goroutine measurements, and database health detail.
- **Architecture references:** documented module boundaries, entity ownership and relations, issue/PR lifecycle, graph semantics, template precedence, and settings scope.
- **Gland workflow guidance:** documented how Projects, primary/supporting Repositories, Issues, Branches, IDIOS Pull Requests, Completions and linked Notes work together, including the current boundary between workflow tracking and real Git operations.

- **Web theme studio:** added a custom web-theme studio with live palette preview, editable dark/light base, color tokens, saved custom palettes, and a persisted per-browser selection.

### Changed

- **Minimal dashboard redesign:** replaced the oversized quote spotlight with a restrained dark editorial panel, removed the Current Focus block, turned Notes into a compact animated carousel, and grouped latest activity into a dense day-based tree timeline. Responsive behavior and reduced-motion preferences are preserved.
- **Build pipeline:** added React/TypeScript/Vite asset compilation and embedded Web UI bundles for release builds. Node.js/pnpm are build-time requirements only.
- **Documentation:** documented the local server, Web API, configuration layout and autostart behavior.
- **Application architecture:** strengthened validation at protocol and application boundaries so adapters share behavior and state transitions remain an application concern.
- **Log hygiene:** successful reads of the logs/monitoring endpoints no longer add repetitive log entries; error details are bounded and request bodies are not logged.
- **Graph semantics:** clarified the difference between blocking `depends_on` dependencies and descriptive/organizational relationships.

### Fixed

- Custom web themes validate their keys, base appearance, and all hex color values before the shared config is persisted, so existing built-in palettes and validated user-defined palettes are accepted rather than rejecting everything except `system`, `dark`, and `light`.
- Prevented unknown protocol/JSON fields and malformed mutation inputs from reaching handlers or storage.
- Made clearing logs truncate the shared log and remove the legacy log file, while preserving private file permissions.
- Ensured empty log history is represented as an empty list in the API, not as a fake log entry.
- Guarded command submission synchronously against rapid repeat submits and cleared stale “repository not found” route errors after navigating back to the repository index following deletion.

## [1.1.0] - 2026-10-20

### Added

- **Bulk operations:** Introduced batch creation for Issues, Quotes, and Scheduled Jobs, supporting up to 100 items per batch.
- **Batch validation and preview:** Added pre-execution validation, duplicate detection, shared settings, and a review step before creating items.
- **Batch execution results:** Added per-item status reporting, partial-failure handling, and retry support for eligible failed items without repeating successful operations.
- **Command palette:** Added a searchable command palette accessible with `Ctrl+K`.
- **Removal Center:** Introduced a unified interface for removing supported entities, including Repositories, Issues, Pull Requests, Projects, Concepts, Notes, Quotes, Scheduled Jobs, Labels, Milestones, and Notifications.
- **Removal safeguards:** Added confirmation flows, exact-name confirmation for Repository removal, and protections for entities with relevant dependencies or completion history.
- **Quote library:** Added 11 sourced quotations from notable figures in software engineering, computer science, mathematics, and statistics.
- **Quote attribution:** Added source metadata and reference links for seeded quotations.
- **Random quote reminders:** Introduced configurable, randomized Quote of the Moment notifications, with a default interval of 60–240 minutes.
- **Desktop notifications:** Added native notification integrations for Windows, macOS, and Linux, with an internal notification inbox as a fallback for retaining reminders.
- **Graceful shutdown:** Added keyboard-driven exit flows, shutdown feedback, and resource cleanup while retaining `Ctrl+C` as an emergency exit path.
- **UI tests:** Added tests covering batch operations, retry behavior, command palette behavior, shutdown flows, and related UI functionality.
- **Database tests:** Added coverage for relevant persistence and migration behavior.

### Changed

- **Navigation:** Removed the redundant Repository list from the sidebar and streamlined primary navigation.
- **Visual design:** Introduced the Midnight theme with improved contrast, clearer status indicators, and reduced visual clutter.
- **Keyboard interaction:** Improved keyboard navigation, batch-result scrolling, and access to common actions.
- **Notification settings:** Added controls for enabling quote reminders and configuring their delivery intervals.
- **Persistence:** Extended database migrations and settings to support quote attribution and reminder functionality while preserving existing user data.
- **Documentation:** Updated relevant TUI development guidance and feature documentation.

### Fixed

- Prevented batch retries from recreating items that had already succeeded.
- Improved handling of partial batch failures so one failed item does not invalidate unrelated successful operations.
- Added safeguards against destructive removal when relevant dependencies or completion history exist.
- Improved shutdown handling and terminal restoration during normal application exit.
- Improved feedback when desktop notification delivery fails.

### Notes

- Random quote reminders currently operate while IDIOS is running; they do not install a persistent background service or operating-system scheduled task.
- Desktop notification delivery depends on platform capabilities and available system utilities.
- Full project compilation and test-suite verification must be completed with the Go version required by the project.

## [1.0.4]
### Fixed
- show view upgrade
## [1.0.3] — 2026-10-20
### Fixed
- fix build in powershell
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
- **Copy:** all in-app text was rewritten to say what a screen is for and what to do next. Marketing phrases and repeated slogans were removed. The intro screen now shows a "Get started" checklist that follows the real state of the database.

### Fixed

- **Clean end-user downloads:** exclude `idios-tui-demo` and extended guides from portable archives and native installers. User-facing packages contain only the `idios` CLI, clean `idios-tui`, `README.md`, and `LICENSE`; the demo executable and extended guides remain available in the source repository for development and CI smoke tests.
- Release checks validate the exact four-file portable bundle and verify that native package payloads do not contain the demo executable.

## [1.0.0] — 2026-10-19

IDIOS 1.0.0 re-platformed IDIOS as a Go workspace engine. It introduces a
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
  service directly instead of spawning the CLI as a subprocess. TUI code lives in the top-level `clients/tui/`
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
a personal learning environment where `idios` opens an interactive shell,
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


