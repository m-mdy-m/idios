# Web client development

The Web client is React + TypeScript + Vite. It is built into static assets and embedded into the release executable.

## Main directories

```text
clients/web/src/
├── app/             startup, route definitions, and providers
├── design-system/   shared tokens and reusable visual primitives
├── features/        feature-owned UI, state, API adapters, and styles
├── lib/             shared browser infrastructure
├── styles/          reset, typography, and global style domains
└── types/           shared API and wire types
```

## Start the Web development server

```sh
cd clients/web
pnpm install --no-frozen-lockfile
pnpm dev
```

Start the Go API separately from the repository root:

```sh
go run ./cmd/idios serve --no-open
```

## Build and lint

```sh
pnpm run build
pnpm run lint
pnpm run check:architecture
pnpm run check:study-actions
pnpm run check:reading-skills
```

`build` runs feature/service boundary checks, reading-skill checks, action checks, copies PDF.js runtime assets, runs TypeScript project builds, and builds Vite assets.

## Component and CSS rules

- Keep page components small; move a coherent reusable behavior into a named component or hook.
- Put feature-specific components and styles inside their owning feature.
- Prefer CSS Modules or feature-scoped style files. Do not add broad global selectors for one page.
- Use Design System primitives for shared surface, spacing, typography, and controls.
- Cross-feature imports must use public `index.ts` exports.
- The Design System must not import from feature modules.
- Avoid empty wrappers around every HTML element. A component should own a reusable visual/behavior contract or a clear responsibility.

## Accessibility and motion

Keep keyboard focus visible, label icon-only controls, use semantic controls, and respect `prefers-reduced-motion`. Test narrow layouts as well as desktop layouts.
