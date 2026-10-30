# Architecture overview

IDIOS is a local-first application with one Go executable, a CLI, a local HTTP server, and an embedded React Web client.

## Main parts

```text
idios executable
├── CLI adapter
├── local HTTP server
│   └── Web client (embedded static assets)
└── Go application
    ├── domain rules and models
    ├── application services
    ├── persistence ports
    ├── SQLite storage
    ├── AI runtime and provider adapters
    └── diagnostics, scheduler, updater, notifications
```

The CLI and Web client are adapters. They call the same application services and use the same SQLite database path. The browser does not write SQL and does not own business rules.

## Runtime model

The default server address is `127.0.0.1:43170`. The server binds to loopback by default. It can run in the foreground (`idios serve`) or as a per-user background service (`idios server start`).

The Web build is compiled into static assets and embedded into the Go executable for release builds. Node.js and pnpm are needed to build the Web client, not to run the released executable.

## Deployment shape

IDIOS is a **modular monolith**, not a set of independently deployed microservices. Domain services have clear interfaces and narrow persistence ports, but run inside the same process and share one application lifecycle. This keeps a personal local installation lightweight. A domain should become a separate process only when it needs an independent lifecycle, security boundary, scaling policy, or failure isolation.

## Source of truth

- Domain behavior: `internal/domain/` and `internal/application/`.
- Persistence and migrations: `internal/storage/sqlite/`.
- CLI adapter: `clients/cli/`.
- HTTP adapter: `clients/server/`.
- Web routes and UI: `clients/web/src/`.
- Public protocol/validation: `internal/protocol/` and `internal/validation/`.

If this page conflicts with implementation, the implementation is authoritative. Update this page when the architecture changes.
