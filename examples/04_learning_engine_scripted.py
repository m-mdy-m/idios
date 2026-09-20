"""Example 04 — Scripted use of LearningEngine (no REPL, no AI).

Demonstrates adding goals, questions, evidence and concepts through
the Python API rather than the interactive REPL. Useful for bulk
import scripts, migration from another tool, or testing.

Run from the repo root (with venv activated):
    python examples/04_learning_engine_scripted.py
"""
from __future__ import annotations

import tempfile
from pathlib import Path

from idios.graph.engine import KnowledgeGraph
from idios.graph.models import EdgeType, NodeType
from idios.learning.engine import LearningEngine
from idios.learning.models import (
    DefinitionOfDone,
    GoalStatus,
    QuestionStatus,
    UnderstandingLevel,
)
from idios.retrieval.engine import RetrievalEngine
from idios.storage.base import JsonFileStorageProvider

# ── setup ─────────────────────────────────────────────────────────────────────
storage_file = Path(tempfile.mktemp(suffix=".json"))
storage  = JsonFileStorageProvider(storage_file)
graph    = KnowledgeGraph(storage)
engine   = LearningEngine(storage)
retrieval = RetrievalEngine(storage, graph=graph)

# ── add a goal ────────────────────────────────────────────────────────────────
goal = engine.add_goal(
    title="Understand transformer attention",
    project="mini-transformer",
    scope=["nlp", "deep-learning"],
    definition_of_done=DefinitionOfDone(criteria=[
        "explain self-attention from scratch",
        "implement scaled dot-product attention",
        "explain why positional encoding is needed",
    ]),
)
print(f"Goal: {goal.title!r}  id={goal.id[:8]}…")

# ── add questions under the goal ──────────────────────────────────────────────
q1 = engine.add_question("What is the role of Q, K, V matrices?", goal_id=goal.id)
q2 = engine.add_question("Why does attention scale by sqrt(d_k)?",  goal_id=goal.id)
q3 = engine.add_question("How does multi-head attention differ from single-head?", goal_id=goal.id)

print(f"Questions: {len([q1, q2, q3])} added")

# ── record evidence ───────────────────────────────────────────────────────────
ev1 = engine.add_evidence(
    content="Q, K, V are linear projections of the input sequence. "
            "Q is the query (what we search for), K is the key (what we "
            "compare against), V is the value (what we retrieve).",
    question_id=q1.id,
    goal_id=goal.id,
    source="Vaswani et al. 2017",
)
ev2 = engine.add_evidence(
    content="Scaling by sqrt(d_k) prevents dot-products from growing too "
            "large in high dimensions, which would push softmax into "
            "regions with very small gradients.",
    question_id=q2.id,
    goal_id=goal.id,
    source="Attention Is All You Need, §3.2.1",
)
# Mark q1 and q2 as answered
engine.answer_question(q1.id)
engine.answer_question(q2.id)

print(f"Evidence: {len([ev1, ev2])} pieces recorded")

# ── track concepts ────────────────────────────────────────────────────────────
c_attn = engine.add_concept(
    name="self-attention",
    description="Mechanism that relates positions within a single sequence.",
    understanding=UnderstandingLevel.PARTIAL,
    goal_id=goal.id,
)
c_pos = engine.add_concept(
    name="positional-encoding",
    description="Injecting sequence order information into token embeddings.",
    understanding=UnderstandingLevel.NONE,
    goal_id=goal.id,
)
engine.update_understanding(c_attn.name, UnderstandingLevel.SOLID)

print(f"Concepts: {c_attn.name!r} → {UnderstandingLevel.SOLID.value}, "
      f"{c_pos.name!r} → {c_pos.understanding.value}")

# ── build the concept graph ───────────────────────────────────────────────────
graph.add_node("self-attention",       node_type=NodeType.CONCEPT,
               description=c_attn.description, tags=["nlp"])
graph.add_node("positional-encoding",  node_type=NodeType.CONCEPT,
               description=c_pos.description,  tags=["nlp"])
graph.add_node("multi-head-attention", node_type=NodeType.CONCEPT,
               description="Parallel self-attention with h heads", tags=["nlp"])

graph.add_relation("self-attention",      EdgeType.PREREQUISITE, "multi-head-attention")
graph.add_relation("positional-encoding", EdgeType.ENABLES,      "transformer")

# ── check DoD progress ───────────────────────────────────────────────────────
loaded_goal = engine.get_goal(goal.id)
if loaded_goal and loaded_goal.definition_of_done:
    dod = loaded_goal.definition_of_done
    print(f"\nDefinition of Done — {len(dod.remaining())} / {len(dod.criteria)} remaining:")
    for c in dod.criteria:
        mark = "✓" if c in dod.met else "·"
        print(f"  {mark} {c}")

# ── search ────────────────────────────────────────────────────────────────────
print("\n=== Search: 'softmax gradient' ===")
results = retrieval.search("softmax gradient", top_k=5)
for r in results:
    print(f"  [{r.kind.value:<10}] {r.snippet[:70]}  ({r.score:.2f})")

print("\n=== Index stats ===")
for k, v in retrieval.index_stats().items():
    print(f"  {k}: {v}")

# cleanup
storage_file.unlink(missing_ok=True)
print("\nDone.")
