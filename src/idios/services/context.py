"""Active context and the internal session.

Context answers "what is the user working on right now?" so that every
highlight, note or question can be filed without repeating --source or
--chapter. Sessions are internal bookkeeping: one is opened automatically
when IDIOS starts and closed when it exits.
"""
from __future__ import annotations

from typing import Optional

from idios.domain.errors import Invalid
from idios.domain.ids import now
from idios.domain.models import Concept, Context, Goal, Question, Session, Source
from idios.storage.repos import Store

LOCATION_FIELDS = ("chapter", "page", "section", "timestamp")


class ContextService:
    def __init__(self, store: Store) -> None:
        self.store = store
        self.ctx: Context = store.context.load()
        self.session: Optional[Session] = None

    # -- loading ---------------------------------------------------------
    @property
    def goal(self) -> Optional[Goal]:
        return self.store.goals.get(self.ctx.goal_id) if self.ctx.goal_id else None

    @property
    def source(self) -> Optional[Source]:
        return self.store.sources.get(self.ctx.source_id) if self.ctx.source_id else None

    @property
    def concept(self) -> Optional[Concept]:
        return self.store.concepts.get(self.ctx.concept_id) if self.ctx.concept_id else None

    @property
    def question(self) -> Optional[Question]:
        return self.store.questions.get(self.ctx.question_id) if self.ctx.question_id else None

    # -- changing --------------------------------------------------------
    def set_goal(self, goal: Goal) -> None:
        self.ctx.goal_id = goal.id
        self._save()

    def set_source(self, source: Source) -> None:
        if self.ctx.source_id != source.id:
            self.ctx.clear_location()  # a location only means something inside its source
        self.ctx.source_id = source.id
        self._save()

    def set_location(self, field: str, value: str) -> None:
        if field not in LOCATION_FIELDS:
            raise Invalid(f"I don't know the location '{field}'.")
        value = value.strip()
        if not value:
            raise Invalid(f"Which {field}?", hint=f"{field} 2")
        if field == "chapter":  # a new chapter invalidates the old page/section
            self.ctx.page = self.ctx.section = None
        setattr(self.ctx, field, value)
        self._save()

    def set_concept(self, concept: Optional[Concept]) -> None:
        self.ctx.concept_id = concept.id if concept else None
        self._save()

    def set_question(self, question: Optional[Question]) -> None:
        self.ctx.question_id = question.id if question else None
        self._save()

    def clear_focus(self) -> None:
        self.ctx.concept_id = None
        self.ctx.question_id = None
        self._save()

    def reset(self) -> None:
        """Forget everything (used when the user declines to resume)."""
        self.ctx = Context(session_id=self.ctx.session_id)
        self._save()

    def forget(self, entity_type: str, entity_id: str) -> None:
        """Drop a deleted entity from the context."""
        attr = {"goal": "goal_id", "source": "source_id",
                "concept": "concept_id", "question": "question_id"}.get(entity_type)
        if attr and getattr(self.ctx, attr) == entity_id:
            setattr(self.ctx, attr, None)
            if attr == "source_id":
                self.ctx.clear_location()
            self._save()

    # -- sessions (internal) ----------------------------------------------
    def start_session(self) -> Session:
        self.end_session()
        session = Session(goal_id=self.ctx.goal_id, source_id=self.ctx.source_id)
        self.store.sessions.add(session)
        self.session = session
        self.ctx.session_id = session.id
        self._save()
        return session

    def end_session(self) -> None:
        if self.session is not None:
            self._snapshot()
            self.session.ended_at = now()
            self.store.sessions.update(self.session)
            self.session = None

    def record_activity(self) -> None:
        if self.session is not None:
            self.session.activity_count += 1
            self._snapshot()
            self.store.sessions.update(self.session)

    # -- internals -------------------------------------------------------
    def _snapshot(self) -> None:
        s = self.session
        if s is None:
            return
        s.goal_id, s.source_id = self.ctx.goal_id, self.ctx.source_id
        s.chapter, s.page = self.ctx.chapter, self.ctx.page
        s.section, s.timestamp = self.ctx.section, self.ctx.timestamp

    def _save(self) -> None:
        self.store.context.save(self.ctx)
