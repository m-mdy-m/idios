"""KnowledgeGraph — structural operations over concept nodes and typed edges.

Everything here is pure Python + storage. No model calls, no network,
no AI. The graph grows organically as the user records evidence and
adds relations through the CLI.

Key operations
──────────────
  add_node(name, ...)          idempotent; updates if exists
  add_relation(A, type, B)     idempotent on (from, type, to)
  prerequisites(name)          direct prerequisite nodes
  all_prerequisites(name)      transitive closure, topologically ordered
  learning_path(A, B)          shortest directed path via PREREQUISITE + ENABLES
  missing_prerequisites(goal, known)   what you still need before tackling `goal`
  related(name, depth)         BFS neighbourhood at each depth level
  subgraph(names)              induced nodes + edges for a concept set
  stats()                      node/edge counts by type

Storage keys
────────────
  graph:node:<name>            GraphNode JSON
  graph:edge:<id>              GraphEdge JSON
"""
from __future__ import annotations

from collections import defaultdict, deque
from datetime import datetime, timezone
from typing import Iterator

from idios.graph.models import EDGE_LABELS, EdgeType, GraphEdge, GraphNode, NodeType, PathStep
from idios.storage.base import StorageProvider


_NAVIGATION_EDGES = {EdgeType.PREREQUISITE, EdgeType.ENABLES}


def _utcnow() -> datetime:
    return datetime.now(timezone.utc)


class KnowledgeGraph:
    def __init__(self, storage: StorageProvider) -> None:
        self._storage = storage

    # ── nodes ─────────────────────────────────────────────────────────────

    def add_node(
        self,
        name: str,
        node_type: NodeType = NodeType.CONCEPT,
        description: str = "",
        goal_id: str | None = None,
        tags: list[str] | None = None,
    ) -> GraphNode:
        """Upsert a node. Safe to call many times — only updates fields that
        are non-empty so callers can enrich incrementally."""
        existing = self._load_node(name)
        if existing:
            changed = False
            if description and not existing.description:
                existing.description = description
                changed = True
            if goal_id and goal_id not in existing.goal_ids:
                existing.goal_ids.append(goal_id)
                changed = True
            if tags:
                new_tags = [t for t in tags if t not in existing.tags]
                if new_tags:
                    existing.tags.extend(new_tags)
                    changed = True
            if changed:
                existing.updated_at = _utcnow()
                self._save_node(existing)
            return existing

        node = GraphNode(
            name=name,
            node_type=node_type,
            description=description,
            goal_ids=[goal_id] if goal_id else [],
            tags=tags or [],
        )
        self._save_node(node)
        return node

    def node(self, name: str) -> GraphNode | None:
        return self._load_node(name)

    def all_nodes(self) -> list[GraphNode]:
        keys = self._storage.list_keys("graph:node:")
        nodes = []
        for k in keys:
            raw = self._storage.get(k)
            if raw:
                try:
                    nodes.append(GraphNode.model_validate(raw))
                except Exception:
                    pass
        nodes.sort(key=lambda n: n.name)
        return nodes

    def delete_node(self, name: str) -> bool:
        """Remove a node and all its edges. Returns True if existed."""
        if not self._load_node(name):
            return False
        self._storage.delete(f"graph:node:{name}")
        # clean up attached edges
        for edge in self._all_edges():
            if edge.from_node == name or edge.to_node == name:
                self._storage.delete(f"graph:edge:{edge.id}")
        return True

    # ── edges ─────────────────────────────────────────────────────────────

    def add_relation(
        self,
        from_name: str,
        edge_type: EdgeType,
        to_name: str,
        weight: float = 1.0,
        note: str = "",
    ) -> GraphEdge:
        """Upsert an edge. Ensures both endpoint nodes exist."""
        self.add_node(from_name)
        self.add_node(to_name)

        # idempotent check
        for existing in self.edges_from(from_name):
            if existing.to_node == to_name and existing.edge_type == edge_type:
                if note and not existing.note:
                    existing.note = note
                    self._save_edge(existing)
                return existing

        edge = GraphEdge(
            from_node=from_name,
            to_node=to_name,
            edge_type=edge_type,
            weight=weight,
            note=note,
        )
        self._save_edge(edge)
        return edge

    def edges_from(self, name: str) -> list[GraphEdge]:
        return [e for e in self._all_edges() if e.from_node == name]

    def edges_to(self, name: str) -> list[GraphEdge]:
        return [e for e in self._all_edges() if e.to_node == name]

    def edges_between(self, a: str, b: str) -> list[GraphEdge]:
        return [
            e for e in self._all_edges()
            if (e.from_node == a and e.to_node == b)
            or (e.from_node == b and e.to_node == a)
        ]

    def delete_relation(self, from_name: str, edge_type: EdgeType, to_name: str) -> bool:
        for edge in self.edges_from(from_name):
            if edge.to_node == to_name and edge.edge_type == edge_type:
                self._storage.delete(f"graph:edge:{edge.id}")
                return True
        return False

    # ── prerequisite queries ───────────────────────────────────────────────

    def prerequisites(self, name: str) -> list[GraphNode]:
        """Direct prerequisite nodes (nodes with a PREREQUISITE edge TO `name`)."""
        prereq_names = {
            e.from_node for e in self.edges_to(name)
            if e.edge_type == EdgeType.PREREQUISITE
        }
        return [n for n in (self._load_node(p) for p in prereq_names) if n]

    def all_prerequisites(self, name: str) -> list[str]:
        """Transitive prerequisites in topological order (leaves first).
        Uses iterative DFS to avoid recursion limits on deep graphs.
        """
        visited: set[str] = set()
        order: list[str] = []

        stack = [name]
        while stack:
            current = stack[-1]
            if current not in visited:
                visited.add(current)
                prereqs = [
                    e.from_node for e in self.edges_to(current)
                    if e.edge_type == EdgeType.PREREQUISITE
                    and e.from_node not in visited
                ]
                stack.extend(prereqs)
            else:
                stack.pop()
                if current != name:
                    order.append(current)

        return order  # leaves first, the goal last (excluded)

    def missing_prerequisites(self, goal_concept: str, known_concepts: set[str]) -> list[str]:
        """All prerequisite concepts not yet in `known_concepts`, leaves-first."""
        return [p for p in self.all_prerequisites(goal_concept) if p not in known_concepts]

    # ── neighbourhood ─────────────────────────────────────────────────────

    def related(
        self,
        name: str,
        depth: int = 2,
        edge_types: set[EdgeType] | None = None,
    ) -> dict[int, list[tuple[GraphNode, GraphEdge]]]:
        """BFS neighbourhood grouped by hop distance.

        Returns {depth: [(node, edge_that_connected_us), ...]}
        Edge can traverse in either direction; edge_types filters which
        kinds of edges count as connections (None = all).
        """
        result: dict[int, list[tuple[GraphNode, GraphEdge]]] = {}
        visited: set[str] = {name}
        frontier = [name]

        for d in range(1, depth + 1):
            next_frontier: list[str] = []
            level: list[tuple[GraphNode, GraphEdge]] = []

            for current in frontier:
                candidates: list[GraphEdge] = []
                for e in self.edges_from(current):
                    if edge_types is None or e.edge_type in edge_types:
                        candidates.append(e)
                for e in self.edges_to(current):
                    if edge_types is None or e.edge_type in edge_types:
                        candidates.append(e)

                for edge in candidates:
                    other = edge.to_node if edge.from_node == current else edge.from_node
                    if other not in visited:
                        node = self._load_node(other)
                        if node:
                            level.append((node, edge))
                            next_frontier.append(other)
                            visited.add(other)

            if level:
                result[d] = level
            frontier = next_frontier
            if not frontier:
                break

        return result

    # ── path finding ───────────────────────────────────────────────────────

    def learning_path(
        self,
        from_name: str,
        to_name: str,
        edge_types: set[EdgeType] | None = None,
    ) -> list[PathStep] | None:
        """BFS shortest directed path from `from_name` to `to_name`.

        Only follows PREREQUISITE and ENABLES edges by default (directed,
        forward-only). Returns None if no path exists.
        """
        if edge_types is None:
            edge_types = _NAVIGATION_EDGES

        visited: set[str] = {from_name}
        # queue of paths: each path is a list of PathStep
        queue: deque[list[PathStep]] = deque([[PathStep(node=from_name)]])

        while queue:
            path = queue.popleft()
            current = path[-1].node

            if current == to_name:
                return path

            for edge in self.edges_from(current):
                if edge.edge_type in edge_types and edge.to_node not in visited:
                    visited.add(edge.to_node)
                    new_step = PathStep(
                        node=edge.to_node,
                        edge_type=edge.edge_type,
                        edge_label=EDGE_LABELS[edge.edge_type],
                    )
                    queue.append(path + [new_step])

        return None

    def reachable_from(self, name: str, edge_types: set[EdgeType] | None = None) -> set[str]:
        """All nodes reachable (directed) from `name`."""
        if edge_types is None:
            edge_types = _NAVIGATION_EDGES
        visited: set[str] = set()
        queue: deque[str] = deque([name])
        while queue:
            current = queue.popleft()
            for edge in self.edges_from(current):
                if edge.edge_type in edge_types and edge.to_node not in visited:
                    visited.add(edge.to_node)
                    queue.append(edge.to_node)
        return visited

    # ── subgraph ───────────────────────────────────────────────────────────

    def subgraph(self, names: list[str]) -> tuple[list[GraphNode], list[GraphEdge]]:
        """Nodes and edges induced by the given concept names."""
        name_set = set(names)
        nodes = [n for n in self.all_nodes() if n.name in name_set]
        edges = [
            e for e in self._all_edges()
            if e.from_node in name_set and e.to_node in name_set
        ]
        return nodes, edges

    def nodes_for_goal(self, goal_id: str) -> list[GraphNode]:
        return [n for n in self.all_nodes() if goal_id in n.goal_ids]

    # ── stats ──────────────────────────────────────────────────────────────

    def stats(self) -> dict:
        nodes = self.all_nodes()
        all_edges = self._all_edges()
        by_node_type: dict[str, int] = defaultdict(int)
        for n in nodes:
            by_node_type[n.node_type.value] += 1
        by_edge_type: dict[str, int] = defaultdict(int)
        for e in all_edges:
            by_edge_type[e.edge_type.value] += 1
        return {
            "nodes": len(nodes),
            "edges": len(all_edges),
            "by_node_type": dict(by_node_type),
            "by_edge_type": dict(by_edge_type),
        }

    # ── internal helpers ────────────────────────────────────────────────────

    def _load_node(self, name: str) -> GraphNode | None:
        raw = self._storage.get(f"graph:node:{name}")
        if not raw:
            return None
        try:
            return GraphNode.model_validate(raw)
        except Exception:
            return None

    def _save_node(self, node: GraphNode) -> None:
        self._storage.set(f"graph:node:{node.name}", node.model_dump(mode="json"))

    def _save_edge(self, edge: GraphEdge) -> None:
        self._storage.set(f"graph:edge:{edge.id}", edge.model_dump(mode="json"))

    def _all_edges(self) -> list[GraphEdge]:
        keys = self._storage.list_keys("graph:edge:")
        edges = []
        for k in keys:
            raw = self._storage.get(k)
            if raw:
                try:
                    edges.append(GraphEdge.model_validate(raw))
                except Exception:
                    pass
        return edges
