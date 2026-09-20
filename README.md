# idios

> A personal cognitive and learning operating system

```
$ idios status
idios status — environment: default
decision engine backend: local (LocalDecisionEngine)
model roles configured: ['router', 'fast_general', 'reasoning', 'coder', 'embedding', 'reranker', 'verifier']
storage: json_file → data/state/state.json
goals: 3  questions: 12  evidence: 8  concepts: 5
graph: 7 nodes  4 edges
sample decision: action=retrieve confidence=0.9
```

---

## Table of contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Quick start](#quick-start)
- [CLI reference](#cli-reference)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

---

## Features

- **Knowledge graph** — typed concept nodes with prerequisite/enables edges;
  transitive closure, learning-path finding, BFS neighbourhood queries.
- **BM25 retrieval** — hand-rolled keyword search over goals, questions,
  evidence, tasks and graph nodes. No embeddings required.
- **Interactive learning REPL** — `idios learn` gives a full terminal UI
  (powered by `rich`) for adding goals, recording evidence and reviewing concepts.
- **Pydantic throughout** — every domain object is typed and validated.
  No silent dicts.

---

## Requirements

| Requirement | Version |
|---|---|
| Python | ≥ 3.10 |

---

## Installation

See [docs/INSTALLATION.md](docs/INSTALLATION.md) for full details.

**Quick path (development):**

```bash
git clone https://github.com/m-mdy-m/idios.git
cd idios
./scripts/bootstrap.sh
source .venv/bin/activate
```
---

## Quick start

```bash
# 1. Check everything wired correctly
idios status

# 2. Run a decision against the local engine
idios decide --goal "understand transformer attention" \
             --task-state NEW

# 3. Open the interactive learning REPL
idios learn

# 4. Quick knowledge-base search from the terminal
idios search "BM25 retrieval"

# 5. Zero-AI demo of the Learning Core
idios learn-demo
```

See [examples/](examples/) for complete worked examples.

---

## CLI reference

```
idios <command> [options]
```

| Command | Description |
|---|---|
| `status` | Config + engine health check; prints index stats and a smoke-test decision |
| `decide` | Run one `DecisionState` through the configured decision engine |
| `learn` | Interactive learning REPL (add goals, evidence, concepts) |
| `search <query>` | BM25 search across the full knowledge base |
| `learn-demo` | End-to-end demo of the Learning Core with no AI |

### `idios decide` options

```
--goal TEXT          What you are trying to accomplish (required)
--project TEXT       Current project context
--task-state STATE   NEW | CLASSIFY | RETRIEVE | DECIDE | PREPARE |
                     ACT | VERIFY | REFLECT | COMMIT_MEMORY | COMPLETE
                     (default: NEW)
--weak-concept TEXT  Concept you are weak on; repeatable
```

### `idios search` options

```
query ...     One or more search terms (positional)
--top-k INT   Maximum results to return (default: 10)
```

## Configuration

| File | Purpose |
|---|---|
| `configs/default.toml` | Application defaults (storage, decision engine, hardware) |
| `configs/local.toml` | Local overrides — never committed to version control |

## Contributing

See [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md).

---

## License

MIT — see `LICENSE`.
