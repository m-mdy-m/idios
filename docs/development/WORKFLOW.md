# Workflow invariants

## Issue lifecycle

```text
open ──start (prerequisites clear)──> in_progress
                                         │
                                         └─ create PR with mandatory explanation
                                                   │
                                      accept ──────┘
                                         │
                                         v
                              accepted PR + Completion on main
                                         │
                                         v
                                    Issue: done
```

An Issue cannot be manually changed to `done`. Accepting a PR writes one Completion record and changes the Issue to `done` as a single Engine operation. Closing a PR without acceptance does not create a completion.

## Structural relationships vs prerequisites vs knowledge

- A parent Issue may have child Sub-issues. The parent and children must use the same Repository. An active child affects whether its parent can be completed.
- `depends_on` is a workflow edge. It can cross repositories; an unfinished prerequisite blocks starting the dependent Issue.
- Concept and other supported semantic relationships add context without blocking an Issue.
- A Note can be linked to another entity. If marked graph-visible, it appears as a node with `note_about` edge. It is not an Issue and does not itself impose workflow rules.
- Project membership groups Issues across repositories but does not change their ownership.

## Scheduler and notification behavior

Scheduled jobs are persisted in `scheduled_jobs`; each due execution records a `job_runs` row and advances `next_run_at`. When both the global notification switch and per-job `notify` are enabled, a local inbox row is created in `notifications`. The process is driven by the TUI's in-app timer and stops when the application exits; no OS scheduler or background daemon is installed.

## What acceptance does not mean

The current release is not a Git implementation. It does not create real commits, compute a diff, push branches, merge code or synchronize with GitHub. The `main` Completion history is logical domain data.
