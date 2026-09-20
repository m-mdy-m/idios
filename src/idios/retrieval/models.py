"""Retrieval domain models. No AI imports."""
from __future__ import annotations

import enum

from pydantic import BaseModel


class ResultKind(str, enum.Enum):
    CONCEPT = "concept"
    EVIDENCE = "evidence"
    GOAL = "goal"
    QUESTION = "question"
    TASK = "task"
    GRAPH_NODE = "graph_node"


KIND_ICONS: dict[ResultKind, str] = {
    ResultKind.CONCEPT: "🧩",
    ResultKind.EVIDENCE: "📌",
    ResultKind.GOAL: "🎯",
    ResultKind.QUESTION: "❓",
    ResultKind.TASK: "📋",
    ResultKind.GRAPH_NODE: "🔵",
}


class SearchResult(BaseModel):
    kind: ResultKind
    id: str
    title: str
    snippet: str
    score: float
    source_key: str
    metadata: dict = {}
