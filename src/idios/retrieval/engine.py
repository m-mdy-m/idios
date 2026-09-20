"""RetrievalEngine — BM25 keyword search over the full IDIOS knowledge base.

Searches across: goals, questions, tasks, evidence, concepts (learning
state) and knowledge graph nodes. Optionally expands results by
traversing the knowledge graph to pull in related concepts.

No AI, no embeddings, no network. Uses a hand-rolled BM25 variant that
is accurate enough for a personal knowledge base of this scale.

Public API
──────────
  engine.search(query, top_k, expand_graph) → list[SearchResult]
  engine.context_for(concept_name)           → dict with everything known
  engine.index_stats()                       → document counts by kind
"""
from __future__ import annotations

import re
from collections import Counter
from typing import TYPE_CHECKING

from idios.retrieval.models import ResultKind, SearchResult
from idios.storage.base import StorageProvider

if TYPE_CHECKING:
    from idios.graph.engine import KnowledgeGraph


# ── tokenizer ────────────────────────────────────────────────────────────────

_STOP_WORDS = {
    "a", "an", "the", "is", "it", "in", "of", "to", "and", "or", "for",
    "on", "at", "by", "with", "from", "as", "be", "was", "are", "this",
    "that", "has", "have", "had", "do", "did", "not", "but", "if", "so",
}

def _tokenize(text: str) -> list[str]:
    tokens = re.findall(r"\b[a-z_][a-z0-9_]*\b", text.lower())
    return [t for t in tokens if t not in _STOP_WORDS and len(t) > 1]


# ── BM25 ─────────────────────────────────────────────────────────────────────
# Simplified BM25 (k1=1.5, b=0.75). IDF is approximated as 1.0 across all
# documents because we don't maintain a global DF table at this scale —
# the ranking still produces useful ordering for a personal KB.

_K1 = 1.5
_B = 0.75
_AVG_DOC_LEN = 12.0  # rough estimate for concept/evidence texts


def _bm25_score(query_tokens: list[str], doc_tokens: list[str]) -> float:
    if not doc_tokens or not query_tokens:
        return 0.0
    freq = Counter(doc_tokens)
    dl = len(doc_tokens)
    score = 0.0
    for qt in set(query_tokens):
        tf = freq.get(qt, 0)
        if tf == 0:
            continue
        tf_norm = tf * (_K1 + 1) / (tf + _K1 * (1 - _B + _B * dl / _AVG_DOC_LEN))
        score += tf_norm           # IDF ≈ 1 per term
    return score


# ── engine ────────────────────────────────────────────────────────────────────

class RetrievalEngine:
    """Searches all learning state and the knowledge graph."""

    def __init__(
        self,
        storage: StorageProvider,
        graph: "KnowledgeGraph | None" = None,
    ) -> None:
        self._storage = storage
        self._graph = graph

    # ── public ────────────────────────────────────────────────────────────

    def search(
        self,
        query: str,
        top_k: int = 10,
        expand_graph: bool = True,
        kinds: set[ResultKind] | None = None,
    ) -> list[SearchResult]:
        """Full-text search. Returns at most `top_k` ranked results.

        expand_graph: if True, also pull in graph-related concepts for
        the top concept hits, with a score penalty so direct hits still
        rank higher.
        """
        tokens = _tokenize(query)
        if not tokens:
            return []

        candidates: list[SearchResult] = []

        if kinds is None or ResultKind.GOAL in kinds:
            candidates.extend(self._search_goals(tokens))
        if kinds is None or ResultKind.QUESTION in kinds:
            candidates.extend(self._search_questions(tokens))
        if kinds is None or ResultKind.TASK in kinds:
            candidates.extend(self._search_tasks(tokens))
        if kinds is None or ResultKind.EVIDENCE in kinds:
            candidates.extend(self._search_evidence(tokens))
        if kinds is None or ResultKind.CONCEPT in kinds:
            candidates.extend(self._search_concepts(tokens))
        if kinds is None or ResultKind.GRAPH_NODE in kinds:
            candidates.extend(self._search_graph_nodes(tokens))

        # graph expansion — related nodes always surface as a fraction of the
        # hit that pulled them in, so direct matches still outrank expansions.
        if expand_graph and self._graph:
            concept_hits = sorted(
                (r for r in candidates if r.kind in (ResultKind.CONCEPT, ResultKind.GRAPH_NODE)),
                key=lambda r: r.score, reverse=True,
            )[:3]
            existing_keys: set[str] = {r.source_key for r in candidates}
            for hit in concept_hits:
                related = self._graph.related(hit.title, depth=1)
                for _, pairs in related.items():
                    for node, edge in pairs:
                        key = f"graph:node:{node.name}"
                        if key in existing_keys:
                            continue
                        existing_keys.add(key)
                        raw = self._storage.get(key)
                        if raw:
                            candidates.append(SearchResult(
                                kind=ResultKind.GRAPH_NODE,
                                id=raw.get("id", ""),
                                title=node.name,
                                snippet=(
                                    f"{node.name}  ({raw.get('node_type', 'concept')})"
                                    + (f"  — {raw.get('description', '')[:50]}"
                                       if raw.get("description") else "")
                                    + f"  [via {edge.edge_type.value} from {hit.title}]"
                                ),
                                score=hit.score * 0.35,
                                source_key=key,
                                metadata={"expanded_from": hit.title, "via": edge.edge_type.value},
                            ))

        # deduplicate: keep highest score per source key
        deduped: dict[str, SearchResult] = {}
        for r in candidates:
            if r.source_key not in deduped or r.score > deduped[r.source_key].score:
                deduped[r.source_key] = r

        ranked = sorted(deduped.values(), key=lambda r: r.score, reverse=True)
        return ranked[:top_k]

    def context_for(self, concept_name: str) -> dict:
        """Everything IDIOS knows about a concept: evidence, related nodes, goals."""
        result: dict = {
            "concept": concept_name,
            "learning_state": None,
            "evidence": [],
            "related": [],
            "goals": [],
            "graph_node": None,
        }

        # learning state
        raw = self._storage.get(f"learning:concept:{concept_name}")
        if raw:
            result["learning_state"] = raw

        # graph node
        if self._graph:
            node = self._graph.node(concept_name)
            if node:
                result["graph_node"] = node.model_dump()
            related = self._graph.related(concept_name, depth=2)
            for depth, pairs in related.items():
                for node_r, edge in pairs:
                    result["related"].append({
                        "name": node_r.name,
                        "relation": edge.edge_type.value,
                        "depth": depth,
                    })

        # evidence referencing this concept
        for k in self._storage.list_keys("learning:evidence:"):
            raw_e = self._storage.get(k)
            if raw_e and raw_e.get("concept") == concept_name:
                result["evidence"].append(raw_e)

        # goals with this concept in scope
        for k in self._storage.list_keys("learning:goal:"):
            raw_g = self._storage.get(k)
            if raw_g:
                scope = raw_g.get("scope", [])
                title = raw_g.get("title", "")
                if concept_name in scope or concept_name.lower() in title.lower():
                    result["goals"].append(raw_g)

        return result

    def index_stats(self) -> dict[str, int]:
        return {
            "goals": len(self._storage.list_keys("learning:goal:")),
            "questions": len(self._storage.list_keys("learning:question:")),
            "tasks": len(self._storage.list_keys("learning:task:")),
            "evidence": len(self._storage.list_keys("learning:evidence:")),
            "concepts": len(self._storage.list_keys("learning:concept:")),
            "graph_nodes": len(self._storage.list_keys("graph:node:")),
            "graph_edges": len(self._storage.list_keys("graph:edge:")),
        }

    # ── per-kind searchers ─────────────────────────────────────────────────

    def _search_goals(self, tokens: list[str]) -> list[SearchResult]:
        results = []
        for k in self._storage.list_keys("learning:goal:"):
            raw = self._storage.get(k)
            if not raw:
                continue
            text = f"{raw.get('title', '')} {' '.join(raw.get('scope', []))}"
            score = _bm25_score(tokens, _tokenize(text))
            if score > 0:
                results.append(SearchResult(
                    kind=ResultKind.GOAL,
                    id=raw.get("id", ""),
                    title=raw.get("title", ""),
                    snippet=f"{raw.get('title', '')}  [{raw.get('status', '')}]",
                    score=score,
                    source_key=k,
                    metadata={"status": raw.get("status", ""), "project": raw.get("project", "")},
                ))
        return results

    def _search_questions(self, tokens: list[str]) -> list[SearchResult]:
        results = []
        for k in self._storage.list_keys("learning:question:"):
            raw = self._storage.get(k)
            if not raw:
                continue
            score = _bm25_score(tokens, _tokenize(raw.get("text", "")))
            if score > 0:
                text = raw.get("text", "")
                results.append(SearchResult(
                    kind=ResultKind.QUESTION,
                    id=raw.get("id", ""),
                    title=text[:70],
                    snippet=f"{text}  [{raw.get('status', '')}]",
                    score=score,
                    source_key=k,
                    metadata={"status": raw.get("status", ""), "goal_id": raw.get("goal_id", "")},
                ))
        return results

    def _search_tasks(self, tokens: list[str]) -> list[SearchResult]:
        results = []
        for k in self._storage.list_keys("learning:task:"):
            raw = self._storage.get(k)
            if not raw:
                continue
            score = _bm25_score(tokens, _tokenize(raw.get("title", "")))
            if score > 0:
                results.append(SearchResult(
                    kind=ResultKind.TASK,
                    id=raw.get("id", ""),
                    title=raw.get("title", ""),
                    snippet=f"{raw.get('title', '')}  [{raw.get('status', '')}]",
                    score=score,
                    source_key=k,
                    metadata={"status": raw.get("status", "")},
                ))
        return results

    def _search_evidence(self, tokens: list[str]) -> list[SearchResult]:
        results = []
        for k in self._storage.list_keys("learning:evidence:"):
            raw = self._storage.get(k)
            if not raw:
                continue
            text = f"{raw.get('description', '')} {raw.get('concept', '')}"
            score = _bm25_score(tokens, _tokenize(text))
            if score > 0:
                desc = raw.get("description", "")
                results.append(SearchResult(
                    kind=ResultKind.EVIDENCE,
                    id=raw.get("id", ""),
                    title=desc[:60],
                    snippet=f"[{raw.get('kind', '')}] {desc}  (concept: {raw.get('concept', '—')})",
                    score=score,
                    source_key=k,
                    metadata={"kind": raw.get("kind", ""), "concept": raw.get("concept", "")},
                ))
        return results

    def _search_concepts(self, tokens: list[str]) -> list[SearchResult]:
        """Searches learning:concept:* entries (validated/provisional/unknown)."""
        results = []
        for k in self._storage.list_keys("learning:concept:"):
            raw = self._storage.get(k)
            if not raw:
                continue
            name = raw.get("name", "")
            text = f"{name} {' '.join(raw.get('open_gaps', []))}"
            score = _bm25_score(tokens, _tokenize(text))
            if score > 0:
                status = raw.get("status", "unknown")
                results.append(SearchResult(
                    kind=ResultKind.CONCEPT,
                    id=name,
                    title=name,
                    snippet=f"{name}  [{status}]",
                    score=score,
                    source_key=k,
                    metadata={"status": status},
                ))
        return results

    def _search_graph_nodes(
        self,
        tokens: list[str],
        name_filter: str | None = None,
        boost: float = 1.0,
    ) -> list[SearchResult]:
        """Searches graph:node:* entries."""
        results = []
        if name_filter:
            keys = [f"graph:node:{name_filter}"]
        else:
            keys = self._storage.list_keys("graph:node:")

        for k in keys:
            raw = self._storage.get(k)
            if not raw:
                continue
            name = raw.get("name", "")
            text = f"{name} {raw.get('description', '')} {' '.join(raw.get('tags', []))}"
            score = _bm25_score(tokens, _tokenize(text)) * boost
            if score > 0:
                results.append(SearchResult(
                    kind=ResultKind.GRAPH_NODE,
                    id=raw.get("id", ""),
                    title=name,
                    snippet=f"{name}  ({raw.get('node_type', 'concept')})"
                            + (f"  — {raw.get('description', '')[:50]}" if raw.get("description") else ""),
                    score=score,
                    source_key=k,
                    metadata={"node_type": raw.get("node_type", ""), "description": raw.get("description", "")},
                ))
        return results
