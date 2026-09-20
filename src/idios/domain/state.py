"""Structured state models for IDIOS's Cognitive/decision layer.

These are the typed contracts referenced throughout docs/architecture.md.
Keep this module free of any model/vendor-specific imports (see AGENTS.md).

Learning-domain concepts (Concept, Curiosity, etc.) live in
idios.learning.models, not here — this module is Cognitive-layer only
(decisions, agents), and per the Learning Core Extraction refactor
(docs/learning.md) the two must not be conflated.
"""
from __future__ import annotations

import enum
import uuid
from typing import Any

from pydantic import BaseModel, Field


def _new_id() -> str:
    return uuid.uuid4().hex


class TaskState(str, enum.Enum):
    NEW = "NEW"
    CLASSIFY = "CLASSIFY"
    RETRIEVE = "RETRIEVE"
    DECIDE = "DECIDE"
    PREPARE = "PREPARE"
    ACT = "ACT"
    VERIFY = "VERIFY"
    REFLECT = "REFLECT"
    COMMIT_MEMORY = "COMMIT_MEMORY"
    COMPLETE = "COMPLETE"


class DecisionAction(str, enum.Enum):
    ANSWER = "answer"
    TEACH = "teach"
    QUESTION_USER = "question_user"
    RETRIEVE = "retrieve"
    PRACTICE = "practice"
    PROJECT_TASK = "project_task"
    RESEARCH = "research"
    REVIEW = "review"
    DEFER_CURIOSITY = "defer_curiosity"
    ESCALATE = "escalate"


class DecisionState(BaseModel):
    """Everything a DecisionEngine needs in order to decide the next
    action (section 7.3 of the master design)."""

    task_id: str = Field(default_factory=_new_id)
    goal: str
    current_project: str | None = None
    current_task_state: TaskState = TaskState.NEW
    weak_concepts: list[str] = Field(default_factory=list)
    recent_evidence: list[str] = Field(default_factory=list)
    context: dict[str, Any] = Field(default_factory=dict)


class DecisionResult(BaseModel):
    action: DecisionAction
    confidence: float = Field(ge=0.0, le=1.0)
    reason_codes: list[str] = Field(default_factory=list)
    constraints: dict[str, Any] = Field(default_factory=dict)
    fallback_action: DecisionAction | None = None


class AgentTask(BaseModel):
    """The structured contract passed to any role-based agent
    (sections 7.11 and 24). Agents communicate through this, never
    through free-form conversational transcripts."""

    task: dict[str, Any]
    state: dict[str, Any]
    evidence: list[str] = Field(default_factory=list)
    allowed_tools: list[str] = Field(default_factory=list)
    constraints: dict[str, Any] = Field(default_factory=dict)
    expected_output_schema: dict[str, Any] | None = None
