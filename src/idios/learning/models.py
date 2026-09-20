"""Learning Core domain models (Learning Core Extraction & AI
Decoupling refactor — see docs/learning.md).

Hard rule: this module must never import idios.decision, idios.models,
or any AI/model/network library. Every class here must work with
AI = OFF, JEV = OFF, MODEL unavailable, NETWORK unavailable — see
tests/unit/test_learning_engine.py::test_learning_core_has_no_ai_imports,
which checks this mechanically, not just by convention.

A few things from the refactor spec's object list (section 3) are
deliberately folded together rather than given their own class, per
the "don't create unnecessary abstractions" rule (section 30):
- ProjectGoal -> Goal.project (a plain field, not a subclass)
- Scope -> Goal.scope (a list of tags)
- Understanding -> UnderstandingLevel (an enum on Concept, not a class)
- ParkingLotItem -> Curiosity.parked (a flag; the "parking lot" is just
  the set of Curiosity objects with parked=True)
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


class GoalStatus(str, enum.Enum):
    ACTIVE = "active"
    PAUSED = "paused"
    DONE = "done"


class DefinitionOfDone(BaseModel):
    """Distinguishes 'sufficient to continue' from 'complete mastery'
    (section 5) — met once every criterion here is marked met, no more."""

    criteria: list[str]
    met: list[str] = Field(default_factory=list)

    def mark_met(self, criterion: str) -> None:
        if criterion not in self.criteria:
            raise ValueError(f"'{criterion}' is not one of this goal's criteria")
        if criterion not in self.met:
            self.met.append(criterion)

    def remaining(self) -> list[str]:
        return [c for c in self.criteria if c not in self.met]

    def is_met(self) -> bool:
        return not self.remaining()


class Goal(BaseModel):
    id: str = Field(default_factory=_new_id)
    title: str
    project: str | None = None
    scope: list[str] = Field(default_factory=list)
    definition_of_done: DefinitionOfDone | None = None
    status: GoalStatus = GoalStatus.ACTIVE
    created_at: datetime = Field(default_factory=_utcnow)


class QuestionStatus(str, enum.Enum):
    OPEN = "open"
    ANSWERED = "answered"
    PARKED = "parked"


class Question(BaseModel):
    id: str = Field(default_factory=_new_id)
    goal_id: str
    text: str
    status: QuestionStatus = QuestionStatus.OPEN
    created_at: datetime = Field(default_factory=_utcnow)


class TaskStatus(str, enum.Enum):
    OPEN = "open"
    IN_PROGRESS = "in_progress"
    BLOCKED = "blocked"
    DONE = "done"


class Task(BaseModel):
    id: str = Field(default_factory=_new_id)
    goal_id: str
    question_id: str | None = None
    title: str
    status: TaskStatus = TaskStatus.OPEN
    blocked_reason: str | None = None
    created_at: datetime = Field(default_factory=_utcnow)


class EvidenceKind(str, enum.Enum):
    """Section 8 — do not equate 'read' with 'understand'; evidence
    must be one of these, not a vague confidence bump."""

    EXPERIMENT = "experiment"
    CODE_EXECUTED = "code_executed"
    TEST_PASSED = "test_passed"
    PROBLEM_SOLVED = "problem_solved"
    CONCEPT_EXPLAINED = "concept_explained"
    PREDICTION_VERIFIED = "prediction_verified"
    IMPLEMENTATION_COMPLETED = "implementation_completed"
    BUG_DIAGNOSED = "bug_diagnosed"
    TEACH_BACK = "teach_back"
    OTHER = "other"


class Evidence(BaseModel):
    id: str = Field(default_factory=_new_id)
    concept: str | None = None
    task_id: str | None = None
    kind: EvidenceKind
    description: str
    created_at: datetime = Field(default_factory=_utcnow)


class Observation(BaseModel):
    id: str = Field(default_factory=_new_id)
    task_id: str
    text: str
    created_at: datetime = Field(default_factory=_utcnow)


class Experiment(BaseModel):
    id: str = Field(default_factory=_new_id)
    task_id: str
    description: str
    outcome: str | None = None
    created_at: datetime = Field(default_factory=_utcnow)


class UnderstandingLevel(str, enum.Enum):
    EXPOSURE = "exposure"
    RECOGNITION = "recognition"
    RECALL = "recall"
    UNDERSTANDING = "understanding"
    APPLICATION = "application"
    TRANSFER = "transfer"
    EXPLANATION = "explanation"


class ConceptStatus(str, enum.Enum):
    UNKNOWN = "unknown"
    PROVISIONAL = "provisional"
    VALIDATED = "validated"


class Concept(BaseModel):
    """Internal heuristics, not scientific measurements (section 8) —
    levels_evidenced only means 'some evidence touched this level'."""

    name: str
    status: ConceptStatus = ConceptStatus.UNKNOWN
    levels_evidenced: list[UnderstandingLevel] = Field(default_factory=list)
    evidence_ids: list[str] = Field(default_factory=list)
    open_gaps: list[str] = Field(default_factory=list)


class CuriosityDisposition(str, enum.Enum):
    """Section 6 anti-drift classification."""

    RELEVANT_NOW = "relevant_now"
    USEFUL_LATER = "useful_later"
    UNRELATED = "unrelated"


class Curiosity(BaseModel):
    """'Curiosity is captured, not suppressed' (section 6) — never just
    discarded; anything not relevant_now is parked, not lost."""

    id: str = Field(default_factory=_new_id)
    question: str
    origin_task_id: str | None = None
    disposition: CuriosityDisposition | None = None
    parked: bool = False
    created_at: datetime = Field(default_factory=_utcnow)


class SessionStage(str, enum.Enum):
    """The learning-loop state machine (section 4). Distinct from
    idios.domain.state.TaskState, which is the Cognitive orchestrator's
    state machine — these track different things and must stay separate."""

    STARTED = "started"
    LEARN = "learn"
    EXPERIMENT = "experiment"
    OBSERVE = "observe"
    UNDERSTAND = "understand"
    VALIDATE = "validate"
    DOCUMENT = "document"
    REFLECT = "reflect"
    NEXT_QUESTION = "next_question"


class LearningSession(BaseModel):
    id: str = Field(default_factory=_new_id)
    goal_id: str
    task_id: str | None = None
    stage: SessionStage = SessionStage.STARTED
    started_at: datetime = Field(default_factory=_utcnow)
    ended_at: datetime | None = None


class Reflection(BaseModel):
    id: str = Field(default_factory=_new_id)
    session_id: str
    understood: list[str] = Field(default_factory=list)
    unclear: list[str] = Field(default_factory=list)
    created_at: datetime = Field(default_factory=_utcnow)


class DocumentStage(str, enum.Enum):
    OBSERVATION = "observation"
    RAW_NOTE = "raw_note"
    STRUCTURED_UNDERSTANDING = "structured_understanding"
    DRAFT = "draft"
    REVIEW = "review"
    FINAL_ARTICLE = "final_article"


class LearningDocument(BaseModel):
    id: str = Field(default_factory=_new_id)
    concept: str | None = None
    stage: DocumentStage = DocumentStage.RAW_NOTE
    content: str = ""
    updated_at: datetime = Field(default_factory=_utcnow)


class Milestone(BaseModel):
    id: str = Field(default_factory=_new_id)
    goal_id: str
    title: str
    achieved: bool = False


class Progress(BaseModel):
    goal_id: str
    tasks_done: int = 0
    tasks_total: int = 0
    concepts_validated: int = 0
    concepts_total: int = 0
