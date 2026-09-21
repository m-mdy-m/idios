"""Example 02 — Programmatic use of KnowledgeGraph and RetrievalEngine.

Shows how to populate the graph and run searches without going through
the CLI. Useful as a starting point for scripting or testing new features.

Run from the repo root (with venv activated):
    python examples/02_search_and_graph.py
"""
from __future__ import annotations

from pathlib import Path

from idios.graph.engine import KnowledgeGraph
from idios.graph.models import EdgeType, NodeType
from idios.retrieval.engine import RetrievalEngine
from idios.storage.base import JsonFileStorageProvider

# ── storage ───────────────────────────────────────────────────────────────────
# Using a temp file so this example is self-contained and non-destructive.
import tempfile

storage_file = Path(tempfile.mktemp(suffix=".json"))
storage = JsonFileStorageProvider(storage_file)

# ── graph ─────────────────────────────────────────────────────────────────────
graph = KnowledgeGraph(storage)

# Add concept nodes
graph.add_node("linear-algebra",  node_type=NodeType.TOPIC,
               description="Vectors, matrices, eigenvalues")
graph.add_node("calculus",        node_type=NodeType.TOPIC,
               description="Derivatives, integrals, limits")
graph.add_node("backpropagation", node_type=NodeType.CONCEPT,
               description="Gradient computation via chain rule")
graph.add_node("neural-network",  node_type=NodeType.CONCEPT,
               description="Parametric function composed of layers")
graph.add_node("transformer",     node_type=NodeType.CONCEPT,
               description="Attention-based sequence model",
               tags=["architecture", "nlp"])

# Add typed edges
graph.add_relation("linear-algebra",  EdgeType.PREREQUISITE, "neural-network")
graph.add_relation("calculus",        EdgeType.PREREQUISITE, "backpropagation")
graph.add_relation("backpropagation", EdgeType.PREREQUISITE, "neural-network")
graph.add_relation("neural-network",  EdgeType.ENABLES,      "transformer")

print("=== Graph stats ===")
stats = graph.stats()
for k, v in stats.items():
    print(f"  {k}: {v}")

print("\n=== Prerequisites of 'transformer' (transitive) ===")
prereqs = graph.all_prerequisites("transformer")
for p in prereqs:
    print(f"  {p.name} ({p.node_type.value})")

print("\n=== Learning path: linear-algebra → transformer ===")
path = graph.learning_path("linear-algebra", "transformer")
if path:
    for step in path:
        if step.edge_type is None:
            print(f"  {step.node}")
        else:
            print(f"  --[{step.edge_type.value}]--> {step.node}")
else:
    print("  no direct path found")

print("\n=== Missing prerequisites (I know: calculus) ===")
missing = graph.missing_prerequisites(
    "transformer",
    {"calculus"},
)
for m in missing:
    print(f"  {m.name}")

# ── retrieval ─────────────────────────────────────────────────────────────────
retrieval = RetrievalEngine(storage, graph=graph)

print("\n=== Search: 'gradient chain rule' ===")
results = retrieval.search("gradient chain rule", top_k=5, expand_graph=True)
if results:
    for r in results:
        print(f"  [{r.kind.value:<10}] score={r.score:.2f}  {r.snippet[:60]}")
else:
    print("  no results (knowledge base may be empty)")

print("\n=== Index stats ===")
idx = retrieval.index_stats()
for k, v in idx.items():
    print(f"  {k}: {v}")

# cleanup
storage_file.unlink(missing_ok=True)
print("\nDone.")
