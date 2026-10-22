<!-- Keep it short. Delete any section that does not apply. -->

## Summary

<!-- What changed and why, in 1-3 sentences. Lead with the user-visible outcome. -->

Closes #

## Type

- [ ] Bug fix
- [ ] Feature
- [ ] Refactor (no intended behavior change)
- [ ] Docs / CI only
- [ ] **Breaking** (CLI, JSON protocol or database schema) — describe the migration path below

## Changes

<!-- The main pieces, grouped by layer if it helps (domain / application / storage / cli / tui). -->

-

## How it was tested

<!-- Paste real output. Do not tick a box for a check you did not run. -->

- [ ] `make check` (fmt-check, test, vet)
- [ ] `make test-race`
- [ ] `make smoke` (needed for workflow, CLI or storage changes)
- [ ] `make psx-check`
- [ ] Added or updated tests; they fail without this change
- [ ] Tried the main flow by hand (CLI / TUI)

```text
$ make check
```

## Compatibility checklist

<!-- Only the lines this PR touches. Remove the rest. -->

- [ ] **Schema:** new numbered migration; shipped migrations are unchanged
- [ ] **Legacy importer:** still read-only
- [ ] **CLI / JSON protocol:** unchanged, or documented in `docs/user/CLI_REFERENCE.md` and `docs/development/WEB_API.md`
- [ ] **Layering:** `domain` stays pure; `cli` and `tui` do not import each other or write SQL (see `AGENTS.md`)
- [ ] **Docs and `CHANGELOG.md`** updated for anything users will notice

## Screenshots / terminal output

<!-- TUI or CLI output changes: before and after. Use a plain-text block, and also check with `--no-color`. -->

## Risk and rollback

- **Impact:** low / medium / high
- **What could break:**
- **How to undo:**

## Notes for reviewers

<!-- Where to start reading, what is safe to skim, trade-offs, known limitations, open questions. -->
