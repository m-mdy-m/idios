# Changelog

All notable changes to idios are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Planned
- Embedding-based semantic search as opt-in retrieval mode
- `idios export` — dump knowledge base to markdown
- JEV service integration tests
- `configs/models.toml` hot-reload on SIGHUP

---

## [0.2.0] — 2026-09-20

### Added
- `idios learn` — interactive learning REPL powered by `rich`; add goals,
  record evidence, annotate concepts and review understanding levels.
- `idios search <query>` — BM25 keyword search over the full knowledge base
  from the terminal; `--top-k` controls result count.
- `RetrievalEngine` — hand-rolled BM25 (k1=1.5, b=0.75) across goals,
  questions, tasks, evidence and graph nodes. No embeddings, no network.
  Optional graph-expansion pass pulls in related concepts.
- `KnowledgeGraph` — typed concept nodes (CONCEPT, SKILL, DOMAIN, TOOL,
  PROJECT) with PREREQUISITE / ENABLES / RELATED / PART_OF / CONTRADICTS
  edges. Supports transitive closure, shortest-path queries, BFS neighbourhood
  and subgraph extraction.
- `LearningEngine` with full CRUD for Goal, Question, Evidence, Concept,
  Curiosity, Task; `DefinitionOfDone` tracks mastery criteria per goal.
- `ModelRegistry` — maps roles (router, fast_general, reasoning, coder,
  embedding, reranker, verifier) to configured backends from `models.toml`.
- `index_stats()` on `RetrievalEngine` reports live counts for all entity
  kinds and graph topology.
- `idios status` now prints graph and index statistics.
- Unit tests for `KnowledgeGraph`, `RetrievalEngine` and `LearningEngine`.

### Changed
- `cli/main.py` is now the sole composition root — the only file that
  imports concrete backends. All other modules depend on protocols only.
- `configs/default.toml` extended with `[hardware]` and `[budgets]` sections.
- Storage path moved to `data/state/state.json` (was `data/state.json`).

### Fixed
- `LocalDecisionEngine` now returns `PRACTICE` with correct `constraints`
  dict when `weak_concepts` is non-empty.
- `JsonFileStorageProvider` key iteration no longer returns internal
  metadata keys prefixed with `__`.

---

## [0.1.0] — 2026-09-19

### Added
- Initial project scaffold: `src/` layout with `setuptools` + `pyproject.toml`.
- `idios status` — smoke-test command that validates config → storage →
  model-registry → decision-engine wiring end to end.
- `idios decide` — run one `DecisionState` through the configured engine.
- `idios learn-demo` — zero-AI demonstration of the Learning Core
  (loops through a fixed scenario without any model call).
- `LocalDecisionEngine` — transparent rule-based decision baseline; four
  rules covering NEW, weak-concepts, project-context and insufficient-signal.
- `AppConfig` / `load_app_config` — TOML loader with `local.toml` overlay
  and `IDIOS_CONFIG_DIR` environment variable override.
- `JsonFileStorageProvider` — single-file JSON storage with atomic writes.
- `DecisionState`, `DecisionResult`, `TaskState`, `DecisionAction`,
  `AgentTask` — typed Pydantic contracts for the cognitive layer.
- `scripts/bootstrap.sh` — creates `.venv`, installs editable + dev deps.
- Unit tests for config loader, local decision engine and state models.

[Unreleased]: https://github.com/m-mdy-m/idios/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/m-mdy-m/idios/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/m-mdy-m/idios/releases/tag/v0.1.0
