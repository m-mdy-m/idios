"""Unit tests for RetrievalEngine."""
import pytest
from pathlib import Path

from idios.storage.base import JsonFileStorageProvider
from idios.graph.engine import KnowledgeGraph
from idios.graph.models import EdgeType
from idios.retrieval.engine import RetrievalEngine, _tokenize, _bm25_score
from idios.retrieval.models import ResultKind
from idios.learning.engine import LearningEngine
from idios.learning.models import EvidenceKind


@pytest.fixture
def setup(tmp_path):
    storage   = JsonFileStorageProvider(tmp_path / "test.json")
    graph     = KnowledgeGraph(storage)
    engine    = LearningEngine(storage)
    retrieval = RetrievalEngine(storage, graph=graph)
    return storage, graph, engine, retrieval


def test_tokenize_removes_stop_words():
    tokens = _tokenize("this is a test of the tokenizer")
    assert "this" not in tokens
    assert "is" not in tokens
    assert "test" in tokens
    assert "tokenizer" in tokens


def test_bm25_score_zero_for_no_match():
    assert _bm25_score(["foo"], ["bar", "baz"]) == 0.0


def test_bm25_score_positive_for_match():
    assert _bm25_score(["foo"], ["foo", "bar"]) > 0.0


def test_search_goal(setup):
    storage, graph, engine, retrieval = setup
    engine.create_goal("Learn asyncio", scope=["async", "event_loop"])
    results = retrieval.search("asyncio")
    kinds = {r.kind for r in results}
    assert ResultKind.GOAL in kinds


def test_search_evidence(setup):
    storage, graph, engine, retrieval = setup
    goal = engine.create_goal("Test goal")
    engine.record_evidence(EvidenceKind.EXPERIMENT, "ran coroutine experiment", concept="coroutine")
    results = retrieval.search("coroutine experiment")
    kinds = {r.kind for r in results}
    assert ResultKind.EVIDENCE in kinds


def test_search_graph_nodes(setup):
    storage, graph, engine, retrieval = setup
    graph.add_node("event_loop", description="core async mechanism")
    results = retrieval.search("event loop async")
    kinds = {r.kind for r in results}
    assert ResultKind.GRAPH_NODE in kinds


def test_search_no_results(setup):
    _, _, _, retrieval = setup
    results = retrieval.search("xyzzy_nothing_matches_this_token")
    assert results == []


def test_search_graph_expansion(setup):
    storage, graph, engine, retrieval = setup
    graph.add_node("asyncio",    description="async lib")
    graph.add_node("event_loop", description="event loop")
    graph.add_relation("event_loop", EdgeType.PREREQUISITE, "asyncio")
    # searching "asyncio" should also surface event_loop via graph expansion
    results = retrieval.search("asyncio", expand_graph=True, top_k=10)
    titles = {r.title for r in results}
    assert "asyncio" in titles
    assert "event_loop" in titles


def test_search_ranked_by_score(setup):
    _, _, engine, retrieval = setup
    engine.create_goal("asyncio deep dive", scope=["asyncio"])
    engine.create_goal("unrelated goal",    scope=["cooking"])
    results = retrieval.search("asyncio", top_k=5)
    assert results[0].score >= results[-1].score


def test_context_for(setup):
    storage, graph, engine, retrieval = setup
    goal = engine.create_goal("Test", scope=["event_loop"])
    engine.record_evidence(EvidenceKind.CODE_EXECUTED, "traced event loop", concept="event_loop")
    graph.add_node("event_loop")
    graph.add_node("asyncio")
    graph.add_relation("event_loop", EdgeType.PREREQUISITE, "asyncio")
    ctx = retrieval.context_for("event_loop")
    assert ctx["concept"] == "event_loop"
    assert len(ctx["evidence"]) >= 1
    assert any(r["name"] == "asyncio" for r in ctx["related"])
    assert any(g["title"] == "Test" for g in ctx["goals"])


def test_index_stats(setup):
    _, _, engine, retrieval = setup
    engine.create_goal("G1")
    engine.create_goal("G2")
    stats = retrieval.index_stats()
    assert stats["goals"] == 2
    assert stats["questions"] == 0


def test_no_ai_imports():
    """Retrieval module must never import AI/model libraries."""
    import ast
    from pathlib import Path
    banned = {"idios.decision", "idios.models", "anthropic", "openai", "transformers"}
    src = Path("src/idios/retrieval")
    for py in src.glob("*.py"):
        tree = ast.parse(py.read_text(encoding="utf-8"))
        for node in ast.walk(tree):
            if isinstance(node, (ast.Import, ast.ImportFrom)):
                names = ([a.name for a in node.names] if isinstance(node, ast.Import)
                         else ([node.module] if node.module else []))
                for name in names:
                    for b in banned:
                        assert not (name and name.startswith(b)), \
                            f"{py} illegally imports {name}"
