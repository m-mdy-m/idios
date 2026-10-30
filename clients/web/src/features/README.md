# Web feature modules

Each feature owns its route-level pages, feature-only CSS, view-model hooks, and feature components.
Other features must import only from a feature `index.ts` public entry point; deep imports across
feature boundaries are not allowed. Cross-feature reusable UI belongs in `design-system/`; reusable
API/domain contracts belong in `lib/`, `types/`, or a narrowly scoped shared module.

The `reading-skills` module owns the registered reading/learning skill library. Consumers import its
public API instead of importing skill files from another feature.

The Shelf module is intentionally cohesive: its PDF reader, reading notes/highlights, and AI companion
share document/session contracts, but their logic should continue to be split into focused components
and hooks rather than expanded inside one page component.
