"""Knowledge Graph domain models.

Hard rule: no AI/model/decision imports. The graph is a pure
structural index over the learning domain — it knows which concepts
relate to which and how, nothing more.

Node  → a thing you can learn (Concept, Topic, Skill, Resource).
Edge  → a typed, directed relation between two nodes.

Storage keys: graph:node:<name>  graph:edge:<id>
"""
from __future__ import annotations

import enum
import uuid
from datetime import datetime, timezone

from pydantic import BaseModel, Field


def _new_id() -> str:
    return uuid.uuid4().hex


def _utcnow() -> datetime:
    return datetime.now(timezone.utc)


class NodeType(str, enum.Enum):
    CONCEPT = "concept"     # an idea or principle
    TOPIC = "topic"         # a broader subject area
    SKILL = "skill"         # a learnable ability
    RESOURCE = "resource"   # a book, doc, link


class EdgeType(str, enum.Enum):
    PREREQUISITE = "prerequisite"   # A must be known before B  (A → B means "learn A first")
    PART_OF = "part_of"             # A is a component of B
    RELATED = "related"             # bidirectional similarity
    ENABLES = "enables"             # knowing A makes B much easier
    EXEMPLIFIES = "exemplifies"     # A is a concrete example of B
    CONTRADICTS = "contradicts"     # A and B appear to conflict; requires resolution
    GENERALIZES = "generalizes"     # A is the abstract version of B


# Human-readable arrow labels for display
EDGE_LABELS: dict[EdgeType, str] = {
    EdgeType.PREREQUISITE: "─prereq→",
    EdgeType.PART_OF: "─part_of→",
    EdgeType.RELATED: "─related─",
    EdgeType.ENABLES: "─enables→",
    EdgeType.EXEMPLIFIES: "─example→",
    EdgeType.CONTRADICTS: "─contra──",
    EdgeType.GENERALIZES: "─general→",
}


class GraphNode(BaseModel):
    id: str = Field(default_factory=_new_id)
    name: str                                           # primary key, unique
    node_type: NodeType = NodeType.CONCEPT
    description: str = ""
    goal_ids: list[str] = Field(default_factory=list)  # goals that mention this node
    tags: list[str] = Field(default_factory=list)
    created_at: datetime = Field(default_factory=_utcnow)
    updated_at: datetime = Field(default_factory=_utcnow)


class GraphEdge(BaseModel):
    id: str = Field(default_factory=_new_id)
    from_node: str          # node name
    to_node: str            # node name
    edge_type: EdgeType
    weight: float = 1.0     # 0.0 = tentative, 1.0 = confident
    note: str = ""
    created_at: datetime = Field(default_factory=_utcnow)


class PathStep(BaseModel):
    node: str
    edge_type: EdgeType | None = None  # how we got here (None for start)
    edge_label: str = ""
