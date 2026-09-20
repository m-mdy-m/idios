"""LearningEngine — deterministic learning workflow orchestration.

Implements the "First End-to-End Slice" (refactor spec, section 27):
create goal -> question -> task -> DoD -> start session -> record
learning/experiment -> record evidence -> validate -> reflect ->
update progress -> next question. Every method here is plain Python —
no model call, no network call, no JEV, no exceptions to that rule.

Depends only on idios.storage (a neutral persistence interface also
used by the Cognitive layer) and idios.learning.models. Must never
import idios.decision, idios.models, or anything AI-shaped — see
docs/learning.md and the import-check test in
tests/unit/test_learning_engine.py.
"""
from __future__ import annotations

from idios.learning.models import (
    Concept,
    ConceptStatus,
    Curiosity,
    CuriosityDisposition,
    DefinitionOfDone,
    Evidence,
    EvidenceKind,
    Experiment,
    Goal,
    LearningSession,
    Observation,
    Progress,
    Question,
    Reflection,
    SessionStage,
    Task,
    UnderstandingLevel,
)
from idios.storage.base import StorageProvider


class LearningEngine:
    """One StorageProvider backs every learning object. Keys are
    namespaced (`learning:<kind>:<id>`) so this can share a JSON file —
    or later a database — with the rest of IDIOS without collisions."""

    def __init__(self, storage: StorageProvider):
        self._storage = storage

    # -- goals / questions / tasks ------------------------------------

    def create_goal(self, title: str, project: str | None = None, scope: list[str] | None = None) -> Goal:
        goal = Goal(title=title, project=project, scope=scope or [])
        self._save_goal(goal)
        return goal

    def set_definition_of_done(self, goal: Goal, criteria: list[str]) -> Goal:
        goal.definition_of_done = DefinitionOfDone(criteria=criteria)
        self._save_goal(goal)
        return goal

    def create_question(self, goal: Goal, text: str) -> Question:
        question = Question(goal_id=goal.id, text=text)
        self._storage.set(f"learning:question:{question.id}", question.model_dump(mode="json"))
        return question

    def create_next_question(self, goal: Goal, text: str) -> Question:
        return self.create_question(goal, text)

    def create_task(self, goal: Goal, title: str, question: Question | None = None) -> Task:
        task = Task(goal_id=goal.id, question_id=question.id if question else None, title=title)
        self._storage.set(f"learning:task:{task.id}", task.model_dump(mode="json"))
        return task

    # -- sessions --------------------------------------------------------

    def start_session(self, goal: Goal, task: Task | None = None) -> LearningSession:
        session = LearningSession(goal_id=goal.id, task_id=task.id if task else None)
        self._save_session(session)
        return session

    def advance(self, session: LearningSession, to_stage: SessionStage) -> LearningSession:
        """Deterministic check (section 7) — no model decides this.
        Sessions may skip stages forward (the spec calls the sequence
        'a useful conceptual flow', not a mandatory checklist) but may
        never move backward."""
        order = list(SessionStage)
        if order.index(to_stage) < order.index(session.stage):
            raise ValueError(
                f"cannot move session backward from {session.stage.value} to {to_stage.value}"
            )
        session.stage = to_stage
        self._save_session(session)
        return session

    # -- evidence / experiments / observations ----------------------------

    def record_experiment(self, task: Task, description: str, outcome: str | None = None) -> Experiment:
        experiment = Experiment(task_id=task.id, description=description, outcome=outcome)
        self._storage.set(f"learning:experiment:{experiment.id}", experiment.model_dump(mode="json"))
        return experiment

    def record_observation(self, task: Task, text: str) -> Observation:
        observation = Observation(task_id=task.id, text=text)
        self._storage.set(f"learning:observation:{observation.id}", observation.model_dump(mode="json"))
        return observation

    def record_evidence(
        self,
        kind: EvidenceKind,
        description: str,
        concept: str | None = None,
        task: Task | None = None,
    ) -> Evidence:
        evidence = Evidence(
            kind=kind, description=description, concept=concept, task_id=task.id if task else None
        )
        self._storage.set(f"learning:evidence:{evidence.id}", evidence.model_dump(mode="json"))
        if concept:
            self._attach_evidence_to_concept(concept, evidence)
        return evidence

    def has_required_evidence(self, concept_name: str) -> bool:
        return bool(self._load_concept(concept_name).evidence_ids)

    def validate_concept(self, concept_name: str, open_gaps: list[str] | None = None) -> Concept:
        """Deterministic gate (section 7): no model gets to assert
        understanding for you — validation requires evidence on file."""
        concept = self._load_concept(concept_name)
        if not concept.evidence_ids:
            raise ValueError(f"cannot validate '{concept_name}': no evidence recorded yet")
        concept.status = ConceptStatus.VALIDATED
        if UnderstandingLevel.UNDERSTANDING not in concept.levels_evidenced:
            concept.levels_evidenced.append(UnderstandingLevel.UNDERSTANDING)
        concept.open_gaps = open_gaps or []
        self._save_concept(concept)
        return concept

    # -- reflection / curiosity ---------------------------------------------

    def reflect(self, session: LearningSession, understood: list[str], unclear: list[str]) -> Reflection:
        reflection = Reflection(session_id=session.id, understood=understood, unclear=unclear)
        self._storage.set(f"learning:reflection:{reflection.id}", reflection.model_dump(mode="json"))
        return reflection

    def capture_curiosity(
        self,
        question: str,
        disposition: CuriosityDisposition,
        origin_task: Task | None = None,
    ) -> Curiosity:
        """'Curiosity is captured, not suppressed' (section 6) — this
        never returns None and forgets the question."""
        curiosity = Curiosity(
            question=question,
            origin_task_id=origin_task.id if origin_task else None,
            disposition=disposition,
            parked=disposition != CuriosityDisposition.RELEVANT_NOW,
        )
        self._storage.set(f"learning:curiosity:{curiosity.id}", curiosity.model_dump(mode="json"))
        return curiosity

    def parking_lot(self) -> list[Curiosity]:
        keys = self._storage.list_keys("learning:curiosity:")
        items = [Curiosity.model_validate(self._storage.get(k)) for k in keys]
        return [c for c in items if c.parked]

    # -- progress ------------------------------------------------------------

    def update_progress(
        self,
        goal: Goal,
        tasks_done: int,
        tasks_total: int,
        concepts_validated: int,
        concepts_total: int,
    ) -> Progress:
        progress = Progress(
            goal_id=goal.id,
            tasks_done=tasks_done,
            tasks_total=tasks_total,
            concepts_validated=concepts_validated,
            concepts_total=concepts_total,
        )
        self._storage.set(f"learning:progress:{goal.id}", progress.model_dump(mode="json"))
        return progress

    # -- internal persistence helpers ------------------------------------------

    def _save_goal(self, goal: Goal) -> None:
        self._storage.set(f"learning:goal:{goal.id}", goal.model_dump(mode="json"))

    def _save_session(self, session: LearningSession) -> None:
        self._storage.set(f"learning:session:{session.id}", session.model_dump(mode="json"))

    def _load_concept(self, name: str) -> Concept:
        raw = self._storage.get(f"learning:concept:{name}")
        return Concept.model_validate(raw) if raw else Concept(name=name)

    def _save_concept(self, concept: Concept) -> None:
        self._storage.set(f"learning:concept:{concept.name}", concept.model_dump(mode="json"))

    def _attach_evidence_to_concept(self, name: str, evidence: Evidence) -> None:
        concept = self._load_concept(name)
        concept.evidence_ids.append(evidence.id)
        if concept.status == ConceptStatus.UNKNOWN:
            concept.status = ConceptStatus.PROVISIONAL
        self._save_concept(concept)
