# IDIOS Full-Screen Terminal UI

The IDIOS TUI is a **full-screen, mouse-aware terminal application**, not a line-oriented prompt and not a second CLI. It borrows the interaction model of a GitHub profile and repository page: persistent tabs, a workspace sidebar, dashboard cards, lists with selectable rows, detail pages, visible action buttons, in-place forms and a dedicated Graph Explorer.

The implementation lives in `internal/tui` and uses Bubble Tea v2 for the event loop and Lip Gloss v2 for presentation. These are UI-only dependencies. The TUI calls `internal/application` use cases; it does not issue SQL, shell out to `idios`, import the CLI, or implement its own workflow rules.

## Launch

```sh
make build
./dist/idios-tui
./dist/idios-tui --db ./work.sqlite3
./dist/idios-tui --no-color
```

The default database is `~/.idios/idios-engine.sqlite3`. Set `IDIOS_DB` or pass `--db` to use another path. `--no-color` and `NO_COLOR=1` disable the palette.

With colour disabled the screen contains **no escape sequences at all**: text stays greppable, copy-paste stays clean, and the width the UI measures is the width the terminal draws. Because nothing is conveyed by styling in that mode, the current tab or control is marked with a `▸` rune and status words such as `BLOCKED` are always printed.

### Requirements

- Go 1.25 or newer to build from source.
- A C compiler on `PATH` (`gcc` or `clang`) for `mattn/go-sqlite3`.
- A terminal with alternate-screen and mouse reporting support. Recent Windows Terminal, iTerm2, Kitty, WezTerm and modern Linux terminals work well. Mouse support may be limited when running through a remote shell or a multiplexer whose mouse mode is disabled.

The app uses a full-screen alternate buffer. Press `Q` or `Ctrl+C` to leave it and return to the previous terminal contents.

## Layout and interaction model

The layout has four persistent areas:

1. **Application bar** — IDIOS home, search, Issues, Pull Requests and New.
2. **Workspace tabs** — Overview, Repositories, Projects, Issues, Pull Requests, Graph, Concepts and Settings. On a Repository page this is followed by the Repository's own tabs.
3. **Workspace sidebar** — primary destinations and recent Repository shortcuts. It is hidden automatically when the terminal is narrow so the content area remains usable.
4. **Main content** — summary cards, selectable lists, detail panels, buttons, and forms. A status line displays the latest action or validation error.

Buttons can be clicked with the left mouse button. Lists support keyboard selection as well. A selected item is highlighted; `Enter` opens it. Search filters the active list without changing persistent data. Forms remain open when validation fails so the entered text is not lost.

### Global keyboard shortcuts

| Key | Action |
|---|---|
| `↑` / `↓`, `j` / `k` | Move through selectable rows |
| `Enter` / `Space` | Open the selected row or activate it |
| `Tab` / `Shift+Tab` | Cycle through global Workspace pages |
| `1`–`8` | Open Overview, Repositories, Projects, Issues, Pull Requests, Graph, Concepts or Settings |
| `/` | Start filtering the current list |
| `Enter` while searching | Keep the current filter and return to the page |
| `Esc` while searching | Clear the filter and leave search mode |
| `N` | Open the creation hub |
| `G` | Open the Workspace Graph Explorer |
| `R` | Refresh data from the Engine |
| `?` or `H` | Open the in-app usage guide |
| `Esc` | Return to the previous detail page or Workspace |
| `Q` / `Ctrl+C` | Quit the TUI |

The top tabs and sidebar can also be clicked; keyboard shortcuts are alternatives, not the only navigation method.

### Editing forms

Forms are displayed inside the main TUI; they do not start a separate prompt. Click any input area or its label to focus it.

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` | Move between inputs and the submit action |
| `Enter` | Move to the next input; on the final input it submits |
| `Ctrl+Enter` / `Ctrl+S` | Submit from any input |
| `←` / `→` | Move the caret within the active input |
| `Home` / `End` | Move to the beginning or end |
| `Backspace` / `Delete` | Delete before or at the caret |
| `Ctrl+U` | Clear the active input |
| Paste | Insert pasted text at the caret (newlines become spaces in single-line fields) |
| `Esc` | Cancel and return to the prior page |

The Engine remains responsible for required fields and business rules. For example, the TUI cannot mark an Issue `done` directly. The Issue must be completed through a PR acceptance.

## Workspace pages

### Overview

The Overview is the profile-style landing page. It shows repository/project/concept counts, Issue status totals, blocked work, open Pull Requests, recent completion totals, pinned Repository summaries and Issues that need attention.

### Repositories

Each Repository has its own page and local tabs:

- **Overview** — description, `main` branch, Issue and Pull Request counts.
- **Issues** — selectable work items, blockers, statuses, priorities, labels and milestones.
- **Pull Requests** — review/acceptance records, their status, and linked Issues.
- **Branches** — V1 displays the default `main` branch. Creating, switching or merging non-main branches is not enabled yet.
- **Graph** — the selected Repository's connected neighborhood, including cross-Repository edges.
- **Labels** — global labels visible to the Repository plus labels scoped to that Repository.
- **Milestones** — Repository milestones and due dates.
- **Settings** — simple local key/value settings for the selected Repository.
- **Activity** — recent events and Completion Records for that Repository.

### Issues and Projects

A workspace-wide Issue list lets you see work across Repositories. Opening an Issue shows its description, status, blocker list, assigned Labels/Milestones, and available actions. You can start an eligible Issue, add or remove a blocking prerequisite, link a Concept, assign a Label or Milestone, and open its graph neighborhood.

Projects collect Issues from multiple Repositories. Open a Project to see progress and add references such as `CPU#4` or `ARLIZ#3`. A Project does not own an Issue; its Repository remains the Issue's owner.

### Pull Requests

A Pull Request stores the explanation and outcome of a work item. Review it from the global Pull Requests page or the owning Repository. Accepting a PR atomically records a Completion on that Repository's `main` history and marks its Issue done. Closing a PR does not complete the Issue.

IDIOS V1 is a workflow/history model, **not Git**: no commit hash, source file diff, file snapshot, or real merge is created. `main` is the logical history of accepted work; additional Branch operations are future work.

## Graph Explorer

The Graph Explorer displays a typed graph returned by the Application Core. Every visible node can be clicked to open its detail page where supported. Use the Graph button on an Issue or Concept to focus the view on that node; use **Show whole workspace** to return to the complete graph.

The graph has three readable sections:

1. **Graph summary** — node and relationship counts, current focus and the blocking-edge explanation.
2. **Relationship map** — a compact adjacency view, grouped by source node. Each arrow retains the direction returned by the Engine.
3. **All nodes** — a deterministic, selectable list of nodes. Select a row and press `Enter`, or click its reference.

Example:

```text
╭─ GRAPH SUMMARY ───────────────────────────────────────────────╮
│ 3 nodes · 2 typed relationships                               │
│ depends_on blocks work; Concept links provide context.         │
╰───────────────────────────────────────────────────────────────╯
RELATIONSHIP MAP
ARLIZ#3 · Write chapter three
  ├─ ⛔ depends_on · BLOCKING  →  CPU#4 · Learn electricity basics
  └─ ◇ uses_concept           →  concept:Electricity

ALL NODES
[ISSUE] ARLIZ#3  ·  BLOCKED
[ISSUE] CPU#4    ·  OPEN
[CONCEPT] Electricity
```

This is an adjacency map, not a guessed tree: the same node can be referenced by several edges. Relationship names are always displayed; `depends_on` is explicitly marked as **BLOCKING**, while `uses_concept` and other semantic relationships do not block work by themselves. Node/edge ordering is deterministic. Long text is clipped to the available panel width and the relationship map is bounded so one very large graph cannot flood the whole screen.

## Small terminals and troubleshooting

- At less than **48 columns or 18 rows**, IDIOS shows a compact message rather than drawing beyond the terminal. Enlarge the terminal to restore the full layout.
- **A button does not respond to a click:** make sure the terminal or multiplexer has mouse reporting enabled; keyboard shortcuts and `Enter` remain available.
- **A start action is blocked:** open the Issue and inspect its explicit prerequisites. A Concept link is not a blocker.
- **A PR cannot be accepted:** complete or resolve the Issue's blockers and active Sub-issues first.
- **A label or milestone is rejected:** check the scope. Repository milestones apply only to Issues in that Repository; Project milestones require the Issue to be a member of that Project.
- **Different views show different data:** point them to the same `--db` path. CLI and TUI share the same Engine store.

## Architecture boundary

`cmd/idios-tui` is a composition root. It opens the SQLite Store and wires `application.Service` into `internal/tui`. Bubble Tea owns the event loop and Lip Gloss only styles terminal content. The TUI can shape navigation and render returned data, but must not introduce SQL calls or repeat domain validation. A future GUI can call the same Application through another transport without importing the TUI.
