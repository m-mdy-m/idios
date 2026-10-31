<!-- Keep it short. Delete any section that does not apply. -->

# Pull Request

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

<!-- The main pieces, grouped by layer if it helps (domain / application / storage / cli / web). -->

-

## How it was tested

<!-- Paste real output. Do not tick a box for a check you did not run. -->

- [ ] `make check` (fmt-check, test, vet)
- [ ] `make test-race`
- [ ] `make smoke` (needed for workflow, CLI or storage changes)
- [ ] `make psx-check`
- [ ] Added or updated tests; they fail without this change
- [ ] Tried the main flow by hand (CLI / Web)

```text
make check
```

## Compatibility checklist

<!-- Only the lines this PR touches. Remove the rest. -->

- [ ] **Schema:** new numbered migration; shipped migrations are unchanged
- [ ] **Legacy importer:** still read-only
- [ ] **CLI / JSON protocol:** unchanged, or documented in `docs/reference/cli.md` and `docs/arch/http-api.md`
- [ ] **Layering:** `domain` stays pure; `cli` does not write SQL and the Web UI does not bypass Application (see `AGENTS.md`)
- [ ] **Docs and `CHANGELOG.md`** updated for anything users will notice

## Mobile / PWA and release matrix (when applicable)

- [ ] PWA manifest, service worker, install icons and offline shell checked
- [ ] Android ABI(s) and APK(s) checked
- [ ] iOS device/simulator architecture checked
- [ ] Signing / provisioning requirements documented
- [ ] E2E setup command:
- [ ] E2E test command:
- [ ] Runner OS/architecture and emulator/device used:
- [ ] API endpoint is HTTPS and protected; loopback-only server policy is unchanged

## Screenshots / terminal output

<!-- CLI output changes: before and after. Use a plain-text block, and also check with `--no-color`. -->

## Risk and rollback

- **Impact:** low / medium / high
- **What could break:**
- **How to undo:**

## Notes for reviewers

<!-- Where to start reading, what is safe to skim, trade-offs, known limitations, open questions. -->
