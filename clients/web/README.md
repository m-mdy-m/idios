# Browser interface

This directory contains IDIOS's React + TypeScript UI. Vite runs it during development; the Go server provides the API; a release build embeds the compiled UI in the `idios` executable.

## Requirements

- Node.js 22 or newer
- pnpm 9.x
- The Go toolchain and C compiler required by the main project

## Run with hot reload

Open two terminals from the repository root.

Terminal 1:

```sh
go run ./cmd/idios serve --no-open
```

Terminal 2:

```sh
cd clients/web
pnpm install
pnpm dev
```

Open the URL shown by Vite, normally `http://127.0.0.1:5173`. Requests to `/api` are forwarded to `http://127.0.0.1:43170`.

## Build and lint

From the repository root:

```sh
make web-lint
make web-build
make build
```

`make build` produces the embedded production UI and Go binaries under `dist/`. Node.js is not required to run those binaries.

## Code boundaries

Pages and components live in `src/`. API helpers live in `src/lib/api.ts`; request/response types live in `src/types/idios.ts`. Use the Go API for reads and writes; do not access SQLite or copy workflow validation into the browser.
