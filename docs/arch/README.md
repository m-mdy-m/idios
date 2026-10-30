# Architecture documentation

These pages explain how IDIOS is built. They are for developers who need to understand where a behavior belongs before changing it.

## Read in this order

1. [Architecture overview](overview.md) — the executable, CLI, server, Web client, and storage.
2. [Service boundaries](service-boundaries.md) — what each Go and Web module owns.
3. [Storage](storage.md) — SQLite, migrations, and data ownership.
4. [AI and Shelf](ai-and-shelf.md) — model providers, reading context, and annotations.
5. [HTTP API](http-api.md) — how the browser talks to the Go application.

The [product feature map](../product/features.md) describes what each user-facing area does. The implementation is authoritative when a page and the source disagree.
