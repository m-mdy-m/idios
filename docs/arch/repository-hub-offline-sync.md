# Offline synchronization outbox

This document specifies the P2 synchronization layer that follows the Local Git working-copy foundation. It is a design RFC; it does not claim that remote operations can run without connectivity.

## Core rule

Local Git operations complete locally. Remote publication is a separate, queued operation.

A disconnected provider must not make local status, branch creation, staging, committing, local tags or diff unavailable. Push, fork, remote release publication and provider PR/MR creation require the target provider to be reachable and authorized.

## Outbox record

A queued operation should persist:
- stable operation ID and idempotency key;
- operation kind (`push_branch`, `publish_tag`, `create_release`, `create_change_request`, `update_change_request`);
- local repository and working-copy IDs;
- provider connection and remote repository IDs;
- source branch/commit and target branch where relevant;
- immutable payload snapshot plus payload/schema version;
- dependency operation IDs;
- created/updated/attempted timestamps;
- retry count, state and last structured error;
- resulting remote object URL/ID after success.

Do not store provider tokens in outbox payloads. Resolve credentials at execution time from the secure provider connection store.

## State machine

Allowed states:
- `queued` — waiting for connectivity or user-triggered sync;
- `running` — currently executing;
- `succeeded` — remote operation completed and result recorded;
- `blocked` — requires user input, permission or conflict resolution;
- `failed` — non-retryable or exhausted error;
- `cancelled` — user explicitly discarded the operation.

State changes must be transactional and survive application restart. A retry after crash must be safe and idempotent.

## Scheduling and execution

- Provide explicit Sync now and optional connectivity-aware resume.
- Resume only operations whose dependencies succeeded.
- Use bounded retry with backoff for transient network/rate-limit failures.
- Do not retry validation, permission or conflict failures blindly.
- Before every push, fetch/compare the remote target and detect non-fast-forward updates.
- Never force-push or overwrite remote history without explicit user choice and a separate confirmation.
- A successful remote action records the provider URL/ID and marks the outbox item succeeded.

## Conflict workflow

A conflict moves an operation to `blocked`, not `failed` or `succeeded`.

Show:
- local and remote commit IDs;
- ahead/behind counts when available;
- conflicting paths;
- supported resolution strategies (merge, rebase, manual resolution, cancel);
- an explicit preview of the resulting history before applying the resolution.

Do not silently choose a winner. Do not discard local uncommitted changes while fetching or resolving.

## Sync center UX

The Sync Center should group items into Queued, Running, Blocked, Failed and Completed. Each row should show the target repository/provider, operation, branch/commit, last attempt, next action and a useful error.

Actions:
- Retry eligible operations;
- Resolve conflict;
- Open local branch/worktree;
- Open resulting provider object after success;
- Cancel only when the operation has not become externally committed.

## Data integrity and security

- Idempotency keys must prevent duplicate remote PR/MR creation after a timeout where the provider supports reconciliation.
- For providers without idempotency support, reconcile by searching for a known operation marker before retrying.
- Keep audit history for state transitions.
- Do not expose tokens in logs or UI.
- Provider disconnect must leave the local working copy and outbox intact; queued operations become blocked until reauthorization.
- Database migrations must be backward compatible and tested for restart recovery.

## Dependencies

- #68 — Local Git working copies and offline branch/commit workflows.
- #72 — provider Explore/Fork and capability behavior.
- #73 — this offline sync queue and conflict recovery implementation.
