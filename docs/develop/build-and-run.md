# Build and run

## Build requirements

- Go 1.25+ and a C compiler for SQLite/CGO.
- Node.js 22+ and pnpm 9.15.4 for the Web bundle.
- GNU Make.

## Build

```sh
make build
```

Build output:

```text
dist/
  idios       # Linux/macOS
  idios.exe   # Windows
```

The release executable contains the compiled Web files. Node.js and pnpm are not required to run a released binary.

## Run the current build

```sh
make run-built
```

On Linux/macOS this rebuilds and starts `dist/idios`. On Windows the Make target uses `scripts/restart-built.ps1` to stop a previous IDIOS server on the configured local port, rebuild, and start the new executable. It refuses to stop an unrelated process.

## Run without building an executable

Start the Go server directly:

```sh
go run ./cmd/idios serve --no-open
```

To work on the Web client with hot reload, use a development server in one terminal and the Go API server in another:

```sh
# Terminal 1
make web-install
cd clients/web
pnpm dev

# Terminal 2, from the repository root
go run ./cmd/idios serve --no-open
```

The Web development server uses the Vite configuration in `clients/web/vite.config.ts`; API requests are routed to the local Go server.

## Build variables

- `GO` chooses the Go executable.
- `PNPM` chooses the pnpm command.
- `BIN_DIR` changes the output directory (default `dist`).
- `VERSION` sets the version embedded in the executable.
- `CGO_ENABLED` controls CGO; SQLite builds require it enabled.

On Windows, `scripts/build.ps1 -Version 3.1.0` can be called directly.
