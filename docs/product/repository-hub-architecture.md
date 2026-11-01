# Repository Hub architecture contract

This document records the P0 boundaries for the Repository Hub redesign. It is an implementation contract, not a claim that every item below is already implemented.

## Product model

IDIOS must distinguish three entities:

1. **Local repository** — an IDIOS-owned workspace, or a real local Git working copy once LocalGitEngine is available.
2. **Linked remote repository** — a local IDIOS reference to an upstream repository on GitHub, GitLab or SourceForge.
3. **Provider connection** — credentials and account identity used to access one provider host/account.

A linked remote repository is not itself a local clone. A provider connection can own multiple linked remote repositories.

## Navigation contract

The Repository Hub landing page is an index, not a permanent split-pane workspace. It groups repositories by provider connection and makes provider/account identity visible.

Proposed route shape:

- `/repositories` — local repositories and entry to the Remote Repository Hub.
- `/repositories/remote/:repositoryId` — canonical remote repository page.
- `/repositories/remote/:repositoryId/:view` — durable view selection (`overview`, `code`, `issues`, `changes`, `commits`, `releases`, `tags`, `settings`).
- `/repositories/remote/:repositoryId/issues/:number` — issue detail route.
- `/repositories/remote/:repositoryId/changes/:number` — change request detail route.

The final route names must be reconciled with the current React Router tree before implementation. Route state must survive refresh, browser back/forward and direct navigation.

## Provider lifecycle invariants

- Linking imports a reference and provider metadata into IDIOS; it does not clone the repository.
- Unlinking removes the local Hub association and its provider-scoped cached metadata. It must not delete the upstream repository or a separate local working copy.
- Disconnecting an account removes/invalidates that connection's credentials and linked associations according to an explicit confirmation. It must not delete unrelated local Git data.
- Removing a repository from a collection is not unlinking it.
- Pinning, aliases, private notes and collection membership are IDIOS-local preferences.
- Provider operations are capability-gated. The UI must not show a mutation as available if the adapter cannot perform it.

## Local Git boundary

The current simulated IDIOS repository workflow model must remain distinct from a real Git working copy. Do not silently map simulated completion records to Git commits.

Define a provider-independent `LocalGitEngine` contract for:

- initialize/open/clone working copy;
- status, branches, create/switch branch;
- stage, commit, log and diff;
- create/list tags.

Local operations must not call provider APIs, read provider credentials or require network access. Push, fork, release publication and provider PR/MR creation remain online adapter operations.

Desktop and mobile must bind the contract to platform-supported implementations. Do not assume a system `git` executable is installed on every supported target. Engine capabilities and unavailable operations must be explicit.

Operations against the same managed working copy use a process-wide read/write lock: Git index and ref mutations are exclusive, while status/history/diff reads use a shared lock. This prevents overlapping requests handled by one IDIOS server process from racing on the Git index.

The current `CLIEngine` is a desktop backend and requires a system Git executable. On devices without a process-capable Git installation (including the current mobile targets), Local Git reports `available=false` with a reason and disables local mutations; it does not pretend that simulated IDIOS branches are real Git. A native or pure-Go mobile backend is a separate adapter and must pass the same contract tests before mobile support is advertised.

## UI interaction contract

- Collection, repository and provider management actions live in contextual toolbars or menus.
- Create forms open only after an explicit action and use the shared IDIOS Modal or a focused route.
- Modal forms must provide labels, validation, loading/disabled states, keyboard dismissal, focus visibility and useful failure messages.
- Missing remote metadata is shown as unavailable/unknown. It is never replaced with guessed values or raw connector implementation text.

## Delivery sequence

1. Establish the route and provider lifecycle contract.
2. Ship the provider-first index and dedicated repository page.
3. Move create workflows to explicit dialogs and standardize Overview actions.
4. Introduce LocalGitEngine behind an application-level interface.
5. Add platform-specific working-copy adapters and offline tests.

## Related issues

- #66 — provider-first navigation and dedicated repository pages.
- #67 — dialog-driven Issues and Change Request workflows.
- #68 — real Local Git working copies and offline branch/commit workflows.
