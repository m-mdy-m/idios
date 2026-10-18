# Developer documentation

This section is for maintainers and contributors. It describes module boundaries, persistence invariants, build/test workflows and extension points. It is not required reading for someone who only wants to use IDIOS.

## Recommended order

1. [Architecture](ARCHITECTURE.md) — dependency direction and ownership by package.
2. [TUI architecture](TUI.md) — modular presentation files, settings and interaction model.
3. [Storage and migrations](STORAGE.md) — SQLite schema, transactions and migration rules.
4. [Workflow invariants](WORKFLOW.md) — issue status, blocking edges and completion semantics.
5. [Adding features](EXTENDING.md) — the normal path from Domain to storage/application/adapters.
6. [Testing and contribution workflow](CONTRIBUTING.md) — local gates and code review expectations.
7. [CLI JSON contract](JSON_PROTOCOL.md) and [legacy import](MIGRATION.md) when needed.

The user-facing entry point is [the user guide](../user/README.md). Keep end-user instructions out of protocol and architecture documents when possible.
