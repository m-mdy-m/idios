"""Unit tests for KnowledgeGraph — no AI, no network, in-memory storage."""
import pytest
import tempfile
from pathlib import Path

from idios.storage.base import JsonFileStorageProvider
from idios.graph.engine import KnowledgeGraph
from idios.graph.models import EdgeType, NodeType


@pytest.fixture
def graph(tmp_path):
    storage = JsonFileStorageProvider(tmp_path / "test.json")
    return KnowledgeGraph(storage)


def test_add_node_idempotent(graph):
    n1 = graph.add_node("asyncio", description="async lib")
    n2 = graph.add_node("asyncio", description="")   # should not overwrite
    assert n1.id == n2.id
    assert n2.description == "async lib"


def test_add_node_enriches_incrementally(graph):
    graph.add_node("asyncio")
    n = graph.add_node("asyncio", description="Python async lib", goal_id="goal1")
    assert n.description == "Python async lib"
    assert "goal1" in n.goal_ids


def test_add_relation_idempotent(graph):
    e1 = graph.add_relation("event_loop", EdgeType.PREREQUISITE, "asyncio")
    e2 = graph.add_relation("event_loop", EdgeType.PREREQUISITE, "asyncio")
    assert e1.id == e2.id
    assert len(graph._all_edges()) == 1


def test_prerequisites(graph):
    graph.add_relation("event_loop", EdgeType.PREREQUISITE, "asyncio")
    graph.add_relation("coroutine",  EdgeType.PREREQUISITE, "asyncio")
    prereqs = {p.name for p in graph.prerequisites("asyncio")}
    assert prereqs == {"event_loop", "coroutine"}


def test_all_prerequisites_transitive(graph):
    graph.add_relation("TCP",        EdgeType.PREREQUISITE, "HTTP")
    graph.add_relation("HTTP",       EdgeType.PREREQUISITE, "asyncio")
    all_p = graph.all_prerequisites("asyncio")
    assert "TCP" in all_p
    assert "HTTP" in all_p
    assert all_p.index("TCP") < all_p.index("HTTP")  # leaves first


def test_learning_path_found(graph):
    graph.add_relation("A", EdgeType.PREREQUISITE, "B")
    graph.add_relation("B", EdgeType.ENABLES,      "C")
    path = graph.learning_path("A", "C")
    assert path is not None
    assert [s.node for s in path] == ["A", "B", "C"]


def test_learning_path_not_found(graph):
    graph.add_node("isolated")
    graph.add_node("target")
    assert graph.learning_path("isolated", "target") is None


def test_learning_path_only_follows_navigation_edges(graph):
    # RELATED edge should NOT be traversed
    graph.add_relation("A", EdgeType.RELATED, "B")
    assert graph.learning_path("A", "B") is None


def test_missing_prerequisites(graph):
    graph.add_relation("event_loop", EdgeType.PREREQUISITE, "asyncio")
    graph.add_relation("coroutine",  EdgeType.PREREQUISITE, "asyncio")
    missing = graph.missing_prerequisites("asyncio", known_concepts={"event_loop"})
    assert "coroutine" in missing
    assert "event_loop" not in missing


def test_related_bfs(graph):
    graph.add_relation("A", EdgeType.PART_OF, "B")
    graph.add_relation("B", EdgeType.RELATED,  "C")
    related = graph.related("A", depth=2)
    depth1_names = {n.name for n, _ in related.get(1, [])}
    depth2_names = {n.name for n, _ in related.get(2, [])}
    assert "B" in depth1_names
    assert "C" in depth2_names


def test_delete_node_removes_edges(graph):
    graph.add_relation("A", EdgeType.PREREQUISITE, "B")
    graph.delete_node("A")
    assert graph.node("A") is None
    assert graph._all_edges() == []


def test_delete_relation(graph):
    graph.add_relation("A", EdgeType.RELATED, "B")
    assert graph.delete_relation("A", EdgeType.RELATED, "B") is True
    assert graph._all_edges() == []


def test_stats(graph):
    graph.add_node("X", node_type=NodeType.CONCEPT)
    graph.add_node("Y", node_type=NodeType.TOPIC)
    graph.add_relation("X", EdgeType.PART_OF, "Y")
    s = graph.stats()
    assert s["nodes"] == 2
    assert s["edges"] == 1
    assert s["by_node_type"]["concept"] == 1
    assert s["by_node_type"]["topic"]   == 1
