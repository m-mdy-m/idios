# IDIOS web implementation guide

This folder is the boundary for front-end presentation work. Follow the architecture in the root
`AGENTS.md` for Go and domain changes; do not modify Go, CLI, TUI, storage, or release packaging
from a web-only task unless the task explicitly crosses that boundary.

## Architecture

- React + TypeScript + Vite + Tailwind CSS v4.
- Pages live in `src/pages/`; common layout in `src/components/layout/`; reusable UI primitives in
  `src/components/ui/`; API calls in `src/lib/api.ts`; shared state in `src/store/`.
- Type the wire contract in `src/types/idios.ts` and keep UI-only types separate from Go domain rules.
- The browser is a presentation adapter. Every domain mutation must call the Go API, which uses the
  existing `internal/application.Service` and SQLite store.
- Do not add a Node.js server. The release UI is a static build embedded by the Go `clients/web` package.

## Commands

Run from the repository root: `make web-install`, `make web-build`, and `make web-lint`.

## Design

Use IDIOS's dark-first graphite/blue visual identity, clear hierarchy, restrained motion, responsive
layouts, visible focus, and helpful loading/empty/error states. Keep keyboard interaction and touch
controls usable. Respect `prefers-reduced-motion`. Do not introduce external images or CDNs into the
runtime UI; installed assets must work offline.
