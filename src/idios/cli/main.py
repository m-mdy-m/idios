"""IDIOS CLI entrypoint.

Composition root: the ONLY file that imports concrete backends.
All other modules depend on protocols/interfaces only.

Commands
────────
  idios status          Config + engine health check
  idios decide          Run one DecisionState (rule-based)
  idios learn           Interactive Learning REPL  ← new
  idios search <query>  Quick search from terminal  ← new
  idios learn-demo      Demo run (zero-AI, existing)
"""
from __future__ import annotations

import argparse
import os
from pathlib import Path

from idios.config.loader import AppConfig, load_app_config, load_models_config
from idios.decision.local import LocalDecisionEngine
from idios.domain.errors import JEVNotConfiguredError
from idios.domain.state import DecisionState, TaskState
from idios.models.registry import ModelRegistry
from idios.storage.base import JsonFileStorageProvider
from idios.telemetry.logging import get_logger


def _repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def _config_dir() -> Path:
    override = os.environ.get("IDIOS_CONFIG_DIR")
    return Path(override) if override else _repo_root() / "configs"


def _build_core(app_config: AppConfig):
    """Construct storage, graph, engine, retrieval — shared by all new commands."""
    from idios.graph.engine import KnowledgeGraph
    from idios.learning.engine import LearningEngine
    from idios.retrieval.engine import RetrievalEngine

    storage  = JsonFileStorageProvider(_repo_root() / app_config.storage.path)
    graph    = KnowledgeGraph(storage)
    engine   = LearningEngine(storage)
    retrieval = RetrievalEngine(storage, graph=graph)
    return storage, graph, engine, retrieval


def build_decision_engine(config: AppConfig):
    if config.decision_engine.backend == "jev":
        from idios.decision.jev import JevConnectionSettings, JEVDecisionEngine
        return JEVDecisionEngine(
            JevConnectionSettings(
                endpoint=config.jev.endpoint,
                api_key_env=config.jev.api_key_env,
                timeout_seconds=config.jev.timeout_seconds,
            )
        )
    return LocalDecisionEngine()


# ── status ─────────────────────────────────────────────────────────────────────

def cmd_status(args: argparse.Namespace) -> int:
    logger     = get_logger()
    config_dir = _config_dir()
    app_config = load_app_config(config_dir)
    models_cfg = load_models_config(config_dir)
    registry   = ModelRegistry.from_config(models_cfg)
    storage    = JsonFileStorageProvider(_repo_root() / app_config.storage.path)
    dec_engine = build_decision_engine(app_config)

    print(f"idios status — environment: {app_config.app.environment}")
    print(f"decision engine backend: {app_config.decision_engine.backend} ({type(dec_engine).__name__})")
    print(f"model roles configured: {[r.value for r in registry.roles()]}")
    print(f"storage: {app_config.storage.backend} → {app_config.storage.path}")

    # learning stats
    from idios.graph.engine import KnowledgeGraph
    from idios.retrieval.engine import RetrievalEngine
    graph    = KnowledgeGraph(storage)
    retrieval = RetrievalEngine(storage, graph=graph)
    idx      = retrieval.index_stats()
    print(f"goals: {idx['goals']}  questions: {idx['questions']}  "
          f"evidence: {idx['evidence']}  concepts: {idx['concepts']}")
    print(f"graph: {idx['graph_nodes']} nodes  {idx['graph_edges']} edges")

    storage.set("last_status_check", {"ok": True})

    try:
        result = dec_engine.decide(
            DecisionState(goal="smoke test", current_task_state=TaskState.NEW)
        )
        print(f"sample decision: action={result.action.value} confidence={result.confidence}")
    except JEVNotConfiguredError as exc:
        print(f"decision engine not usable yet: {exc}")
        logger.info("jev_not_configured")
        return 1
    return 0


# ── decide ─────────────────────────────────────────────────────────────────────

def cmd_decide(args: argparse.Namespace) -> int:
    app_config = load_app_config(_config_dir())
    engine     = build_decision_engine(app_config)

    state = DecisionState(
        goal=args.goal,
        current_project=args.project,
        current_task_state=TaskState(args.task_state),
        weak_concepts=args.weak_concept or [],
    )

    print(f"goal: {state.goal}")
    if state.current_project:
        print(f"project: {state.current_project}")
    if state.weak_concepts:
        print(f"weak concepts: {', '.join(state.weak_concepts)}")
    print(f"task state: {state.current_task_state.value}\n")

    try:
        result = engine.decide(state)
    except JEVNotConfiguredError as exc:
        print(f"decision engine not usable yet: {exc}")
        return 1

    print(f"-> action: {result.action.value}  (confidence: {result.confidence})")
    if result.reason_codes:
        print(f"   reason: {', '.join(result.reason_codes)}")
    if result.fallback_action:
        print(f"   fallback: {result.fallback_action.value}")
    return 0


# ── learn (interactive REPL) ───────────────────────────────────────────────────

def cmd_learn(args: argparse.Namespace) -> int:
    from idios.cli.interactive import LearningREPL

    app_config              = load_app_config(_config_dir())
    storage, graph, engine, retrieval = _build_core(app_config)

    repl = LearningREPL(engine, graph, retrieval, storage)
    repl.run()
    return 0


# ── search (one-shot from terminal) ───────────────────────────────────────────

def cmd_search(args: argparse.Namespace) -> int:
    app_config              = load_app_config(_config_dir())
    storage, graph, _, retrieval = _build_core(app_config)

    query   = " ".join(args.query)
    results = retrieval.search(query, top_k=args.top_k)

    if not results:
        print(f"No results for: {query}")
        return 0

    from idios.retrieval.models import KIND_ICONS
    print(f"\nSearch: {query!r}  ({len(results)} hits)\n")
    for r in results:
        icon = KIND_ICONS.get(r.kind, "·")
        print(f"  {icon}  [{r.kind.value:<10}]  {r.snippet[:75]}  ({r.score:.2f})")

    return 0


# ── learn-demo ─────────────────────────────────────────────────────────────────

def cmd_learn_demo(args: argparse.Namespace) -> int:
    from idios.learning.demo import run_demo
    storage_path = _repo_root() / "data" / "state" / "learning-demo.json"
    run_demo(storage_path)
    return 0


# ── parser ─────────────────────────────────────────────────────────────────────

def main(argv: list[str] | None = None) -> int:
    parser     = argparse.ArgumentParser(prog="idios")
    subparsers = parser.add_subparsers(dest="command", required=True)

    # status
    sp = subparsers.add_parser("status", help="Show config + engine health.")
    sp.set_defaults(func=cmd_status)

    # decide
    dp = subparsers.add_parser("decide", help="Run one DecisionState.")
    dp.add_argument("--goal",         required=True)
    dp.add_argument("--project",      default=None)
    dp.add_argument("--task-state",   default="NEW", choices=[s.value for s in TaskState])
    dp.add_argument("--weak-concept", action="append", default=None)
    dp.set_defaults(func=cmd_decide)

    # learn  (interactive REPL)
    lp = subparsers.add_parser("learn", help="Interactive learning REPL.")
    lp.set_defaults(func=cmd_learn)

    # search
    srp = subparsers.add_parser("search", help="Search the knowledge base.")
    srp.add_argument("query", nargs="+", help="Search terms.")
    srp.add_argument("--top-k", type=int, default=10)
    srp.set_defaults(func=cmd_search)

    # learn-demo
    ldp = subparsers.add_parser("learn-demo", help="Demo run (zero AI).")
    ldp.set_defaults(func=cmd_learn_demo)

    args = parser.parse_args(argv)
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
