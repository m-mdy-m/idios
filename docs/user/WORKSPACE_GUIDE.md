# Workspace guide

## The main pieces

### Overview and profile

Overview is the workspace home. It shows your display name and username, bio, workspace README, active Issues, important/pinned items, a rotating Quote, and recent Activity grouped by time window. Choose **Edit profile** to change your workspace identity and README text. This is a local IDIOS profile; it is not a public hosted page.

Pin repositories or Issues that need attention. A pinned unfinished Issue remains visible in Important items with its current status. Use **Unpin** to remove it from that list; the Issue itself is not deleted.

### Repository

A Repository is an independent work area. It owns its Issues, branches, PR records, labels, milestones, settings and activity. Its Overview reads like a project home: it shows the purpose/README scaffold, Issue and PR totals, recent Issues, recent PRs, and shortcuts into the repository graph and activity. New repositories start with the configurable `REPOSITORY_TEMPLATE` scaffold; replace the prompts with the actual purpose, scope, onboarding steps and architecture decisions.

The current release uses the logical `main` branch for completion history; branch creation, switching and actual Git merges are not implemented. The repository README content is local IDIOS metadata, not a file committed into a Git working tree.

### Issue and Sub-issue

An Issue is one unit of work with a status, priority, description and optional due date. When created in the TUI, its Description begins with a selected `ISSUE_TEMPLATE` body: `task`, `bug`, `feature`, or `research` by default. Use that scaffold to capture context and verifiable acceptance criteria, then remove or replace the prompts that do not apply. Templates can be extended in `~/.idios/config.json`; see the [template guide](ISSUE_AND_PR_TEMPLATES.md).

A Sub-issue is a smaller child Issue attached to its parent. In this version, parent and child must belong to the same Repository.

The normal workflow is:

`open → in_progress → accepted Pull Request → done`

A user cannot start an Issue with unfinished blocking prerequisites. A completed Issue is produced by accepting its Pull Request, not by directly changing the status to done.

### Blocking prerequisites (`depends_on`)

Use this when work **must wait** for another Issue. It can cross repository boundaries. The graph renders this as a distinct blocking edge, and the source Issue stays blocked until the prerequisite is complete or cancelled under the engine's rules.

A prerequisite is different from a Sub-issue: a child describes decomposition, while `depends_on` describes execution order. Neither is the same thing as a knowledge link.

### Pull Request and Completion

A Pull Request records an explanation of the work for an Issue. Its TUI form starts with the configurable `PULL_REQUEST_TEMPLATE`, including summary, related Issue, implementation details, verification, risks and reviewer guidance. Replace checklist prompts with factual results before submitting. Accepting it creates a Completion record on the logical `main` branch and marks the Issue done. This is an internal workflow record only: IDIOS does not create a Git commit, compare file diffs, merge branches or contact a remote code host.

### Project

A Project groups Issues around one outcome and can span multiple Repositories. It does not take ownership of those Issues; each Issue still belongs to its original Repository.

### Concept and knowledge graph

A Concept is reusable knowledge, such as “virtual memory” or “event-loop ownership”. Semantic links connect Concepts to Issues and to each other. They add context but do **not** block work.

The Graph view shows typed relationships among Repositories, branches, Issues, Projects, Concepts, PRs, completion records, labels, milestones and graph-visible Notes. It reads top to bottom:

1. **Summary and legend** — how many nodes and links are on screen, and what each marker means.
2. **What is blocked right now** — every Issue that is waiting, with the unfinished prerequisite under it (`ARLIZ#3` ⛔ waits for `CPU#4 [OPEN]`). Prerequisites that are already done are not listed here.
3. **Nodes and their links** — each node followed by its links, worded as short sentences and sorted by importance: ⛔ blocking, → workflow (sub-issues, pull requests, completions), ◇ context (concepts, notes, labels, milestones, projects). Each link is printed once, under the more central node, so an Issue lists its Concepts rather than the Concept listing every Issue.
4. **Other nodes** — nodes that are only linked from the ones above, or not linked yet.

Only `depends_on` ever blocks work. Every other relation is context.

Controls above the list:

- **Show whole workspace** — leave a focused view (shown only when focused).
- **Distance: N → next** — when focused on a node, cycle how many links away from it are drawn (1, 2 or 3).
- **Ownership links: hidden → show** — ownership plumbing (a repository owns its Issues and branches, a PR targets a branch) is hidden by default because it connects almost everything. Branches, labels and milestones that have no other links are hidden with it. Turn it on to see the full structure.

Click or press Enter on any reference to open it. “View graph” on an Issue or Concept focuses the graph on that node and shows its links in both directions.

### Notes

Notes are quick capture for anything that does not yet deserve an Issue. Each note can have multiple reusable labels, entered as a comma-separated list in the **Labels** field (for example `idea, research, read-later`). Notes with the same label share a filter chip; use `/` to search titles, content and labels. Edit a note to replace its labels. Labels are stored separately so a label can be reused across many notes.

To connect a note to work, select it and choose **Link / graph settings**. Select an entity type and its reference (for example `issue`, `IDIOS#2`; `repository`, `IDIOS`; or `concept`, `Knowledge graph`). Choose `yes` for “Show in graph?” to add a Note node and its link to the graph. Choose `no` to retain the relationship without adding the Note to graph visualization.

Notes are stored locally. Deleting a Note does not delete the Issue, Repository or Concept it was linked to.

### Scheduled jobs and reminders

Open **Scheduled jobs** and add or edit a reminder. Schedules are interpreted in the computer's local time:

- `interval` repeats after a duration, such as `30m`, `1h`, `2d` or `1w` (a bare number is minutes).
- `once` runs at one date and time, written `YYYY-MM-DD HH:MM`, for example `2026-10-25 14:30`.
- `daily` repeats at `HH:MM`, for example `09:00`.
- `weekly` repeats on a weekday and time, for example `Mon 09:00`.
- `monthly` repeats on a day and time, for example `15 09:00`. If the day is beyond a month's length, it runs on that month's final day.

You can edit a job's schedule, pause/resume recurring jobs, run it immediately, or delete it. A one-time job is automatically disabled after it runs and can be scheduled again by editing its date/time. The scheduled job stores a run history; a notification is put in the local inbox only when both the job and the global notification switch allow it. Missed recurring intervals are skipped rather than replayed in a burst.

**Important:** reminders are processed while IDIOS is open. Overdue reminders are processed as soon as the TUI starts again, but there is no always-running background service, so a reminder cannot appear at its scheduled time while IDIOS is closed. The check interval in notification settings controls how promptly due jobs are noticed while the app is open.

### Quotes

Save Quotes you want to revisit. If quotes exist, IDIOS advances the workspace quote rotation on each TUI launch. Manage them in the **Quotes** screen; the next quote shown in Intro and Overview comes from the same saved collection.

### Activity

Overview and Repository Activity show recent event records, such as workspace changes, Issue actions, note changes and accepted PRs. The timeline is a compact audit view, not a time tracker.

## Demo data and existing work

Use `idios-tui-demo` to explore the built-in sample workspace. It stores data in `~/.idios/demo/idios-demo.sqlite3` by default (override with `IDIOS_DEMO_DB`) and automatically seeds it idempotently. The normal `idios-tui` uses `~/.idios/idios-engine.sqlite3` and never seeds sample data on startup. The demo binary ignores `IDIOS_DB` when selecting its default path; however, an explicit `--db PATH` overrides either default, so do not point both apps at the same file unless that is intentional. The CLI command `idios seed demo` remains an explicit operation on the CLI-selected database and can mix sample records into it. Database files and existing records are never silently deleted.
