# Contributing to idios

Idios is a personal project. Contributions are welcome, but please read
this document before opening a PR — the project has a set of hard rules
that are enforced mechanically by the test suite, not just by convention.

---

## Before you start

1. **Open an issue first** for anything non-trivial. Discuss the
   approach before writing code. This avoids wasted effort on PRs that
   conflict with the project's direction.
2. **Read the architecture.** The design rationale lives in
   `docs/architecture.md` and open decisions in `docs/decisions.md`.
   PRs that violate the layering rules will be rejected.
3. **Check the existing tests.** `pytest -q` must pass before and after
   your change.

---

## Development setup

```bash
git clone https://github.com/m-mdy-m/idios.git
cd idios
./scripts/bootstrap.sh
source .venv/bin/activate
pytest -q            # should pass immediately
```

If `pytest -q` fails on a fresh clone, that is a bug — please open an issue.

---

## Hard rules (enforced by tests)

These are not style preferences. Breaking them will fail CI.

### 1. Composition root isolation

`cli/main.py` is the **only** file that may import concrete backends.
Every other module must depend on protocols/interfaces only.

```
# allowed anywhere
from idios.decision.base import DecisionEngine   # protocol

# only in cli/main.py
from idios.decision.local import LocalDecisionEngine   # concrete
from idios.decision.jev   import JEVDecisionEngine     # concrete
```

### 2. Learning Core isolation

`learning/`, `graph/`, and `retrieval/` must never import from:
- `idios.decision`
- `idios.models`
- Any AI / ML / network library

This is checked mechanically by
`tests/unit/test_learning_engine.py::test_learning_core_has_no_ai_imports`.

### 3. Local-first by default

Any code path reachable without a flag or config change must work with:
- No network access
- No model call
- No API key

If your feature requires a model or network, gate it behind
`configs/local.toml` or an explicit CLI flag.

### 4. Tests without AI

Unit tests in `tests/unit/` must pass with **no model, no network,
no API key**. Integration tests (if added) belong in `tests/integration/`
and must be skipped by default (`pytest -q` runs unit tests only).

---

## Workflow

### Branching

```
main          stable; tagged releases only
develop       integration branch; target PRs here
feat/<name>   new feature
fix/<name>    bug fix
chore/<name>  tooling, CI, docs
```

### Commits

Keep commits atomic. Prefer:

```
feat(graph): add transitive closure for ENABLES edges
fix(retrieval): stop BM25 returning zero-score results
chore(ci): pin actions to SHA digests
```

Format: `<type>(<scope>): <imperative sentence>`.
Types: `feat`, `fix`, `refactor`, `test`, `chore`, `docs`.

### Pull requests

Use the PR template. Every PR must:

- Target `develop`, not `main`
- Pass `pytest -q` without modification
- Have a `CHANGELOG.md` entry under `[Unreleased]`
- Not break the architecture rules above

### Review

PRs are reviewed by the maintainer. Expect feedback on architecture
and layering rules in addition to logic. Reviews aim to be specific
and actionable; respond in kind.

---

## Adding a new CLI command

1. Write the handler function `cmd_<name>(args)` in `cli/main.py`.
2. Register a subparser in `main()`.
3. Add the command to the CLI reference table in `README.md`.
4. Add at least one unit test that calls the handler directly (no
   subprocess, no network).

## Adding a new storage backend

1. Implement the `StorageProvider` protocol from `idios.storage.base`.
2. Register it in `cli/main.py::build_storage` (or equivalent).
3. Add a config key in `default.toml` and document it in
   `docs/INSTALLATION.md`.

## Adding a new decision backend

1. Implement `decide(state: DecisionState) -> DecisionResult`.
   The protocol is in `idios.decision.base`.
2. Register it in `cli/main.py::build_decision_engine`.
3. Document the config key in `README.md` and `docs/INSTALLATION.md`.

---

## Style

- **Python 3.10+** — use `match`, `X | Y` union types, `tomllib`.
- **Pydantic v2** for all domain objects.
- **`rich`** for all terminal output — no raw `print` for structured data.
- No third-party formatters are enforced (yet), but be consistent with
  the surrounding code.
- Docstrings on public classes and functions; module docstrings on every
  file (see existing modules for style).

---

## Questions

Open a [discussion](https://github.com/m-mdy-m/idios/discussions)
rather than an issue for questions.
