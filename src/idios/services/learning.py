"""Learning objects: goals, questions, answers, notes, highlights, quotes.

Everything recorded here is filed under the active context (source,
location, goal, concept) so the user never has to repeat it.
"""
from __future__ import annotations

from typing import Optional

from idios.domain.errors import Invalid
from idios.domain.models import (
    Answer, Entity, Goal, Highlight, Note, Question, QuestionStatus, Quote,
)
from idios.services.context import ContextService
from idios.storage.repos import Store


def _text(text: str, what: str, hint: str) -> str:
    text = text.strip()
    if not text:
        raise Invalid(f"Nothing to save as a {what}.", hint=hint)
    return text


class LearningService:
    def __init__(self, store: Store, context: ContextService) -> None:
        self.store = store
        self.context = context

    # -- goals -----------------------------------------------------------
    def goal(self, title: str) -> tuple[Goal, bool]:
        title = " ".join(title.split())
        if not title:
            raise Invalid("A goal needs a title.", hint="goal: Learn Assembly")
        existing = self.store.goals.by_name(title)
        if existing:
            return existing, False
        return self.store.goals.add(Goal(title=title)), True

    def find_goal(self, text: str) -> Optional[Goal]:
        text = " ".join(text.split())
        exact = self.store.goals.by_name(text)
        if exact:
            return exact
        hits = self.store.goals.like(text)
        return hits[0] if len(hits) == 1 else None

    # -- questions & answers ---------------------------------------------
    def ask(self, text: str) -> Question:
        text = _text(text, "question", "Why does the CPU need registers?")
        ctx = self.context.ctx
        question = Question(text=text, goal_id=ctx.goal_id, source_id=ctx.source_id,
                            chapter=ctx.chapter, page=ctx.page,
                            section=ctx.section, timestamp=ctx.timestamp)
        self.store.questions.add(question)
        self._attach(question, concept=True)
        self.context.set_question(question)
        self.context.record_activity()
        return question

    def answer(self, question: Question, text: str) -> Answer:
        text = _text(text, "answer", "answer: <your answer>")
        answer = self.store.answers.add(Answer(question_id=question.id, text=text))
        question.status = QuestionStatus.ANSWERED
        self.store.questions.update(question)
        self.context.record_activity()
        return answer

    def answers_to(self, question: Question) -> list[Answer]:
        return self.store.answers.find("question_id = ?", (question.id,))

    def open_questions(self) -> list[Question]:
        return self.store.questions.find("status = 'open'")

    # -- notes, highlights, quotes ---------------------------------------
    def note(self, text: str) -> Note:
        text = _text(text, "note", "note: Registers are CPU-local storage")
        note = Note(text=text, **self._location())
        self.store.notes.add(note)
        self._attach(note, concept=True, question=True)
        self.context.record_activity()
        return note

    def highlight(self, text: str) -> Highlight:
        text = _text(text, "highlight", "highlight: <text from the source>")
        source = self._require_source("highlight")
        item = Highlight(source_id=source.id, text=text, **self._location())
        self.store.highlights.add(item)
        self._attach(item, concept=True)
        self.context.record_activity()
        return item

    def quote(self, text: str) -> Quote:
        text = _text(text, "quote", "quote: <memorable passage>")
        source = self._require_source("quote")
        item = Quote(source_id=source.id, text=text, **self._location())
        self.store.quotes.add(item)
        self._attach(item, concept=True)
        self.context.record_activity()
        return item

    # -- deleting (callers confirm first) --------------------------------
    def impact(self, entity: Entity) -> str:
        """What will disappear along with this entity (shown before confirming)."""
        links = self.store.links
        parts = []
        if entity.entity_type == "source":
            for label, repo in (("highlight", self.store.highlights),
                                ("quote", self.store.quotes)):
                n = repo.count("source_id = ?", (entity.id,))
                if n:
                    parts.append(f"{n} {label}{'s' if n != 1 else ''}")
        elif entity.entity_type == "question":
            n = self.store.answers.count("question_id = ?", (entity.id,))
            if n:
                parts.append(f"{n} answer{'s' if n != 1 else ''}")
        n = len(links.relations_of(entity.entity_type, entity.id))
        if n:
            parts.append(f"{n} relation{'s' if n != 1 else ''}")
        return ", ".join(parts)

    def delete(self, entity: Entity) -> None:
        etype = entity.entity_type
        if etype == "question":  # answers cascade in SQL, but their links need purging
            for ans in self.answers_to(entity):  # type: ignore[arg-type]
                self.store.links.purge("answer", ans.id)
        if etype == "source":
            for repo, kind in ((self.store.highlights, "highlight"),
                               (self.store.quotes, "quote")):
                for item in repo.find("source_id = ?", (entity.id,)):
                    self.store.links.purge(kind, item.id)
        self.store.links.purge(etype, entity.id)
        self.store.by_type[etype].delete(entity.id)
        self.context.forget(etype, entity.id)

    # -- internals -------------------------------------------------------
    def _require_source(self, what: str):
        source = self.context.source
        if source is None:
            raise Invalid(f"A {what} belongs to a source, and no source is selected.",
                          hint="source: <title>")
        return source

    def _location(self) -> dict[str, Optional[str]]:
        c = self.context.ctx
        return {"chapter": c.chapter, "page": c.page,
                "section": c.section, "timestamp": c.timestamp}

    def _attach(self, owner: Entity, concept: bool = False, question: bool = False) -> None:
        """Record what an item is about, from the active context."""
        links, ctx = self.store.links, self.context.ctx
        t = owner.entity_type
        if ctx.source_id and t != "question":  # questions carry source_id directly
            links.add_ref(t, owner.id, "source", ctx.source_id)
        if ctx.goal_id and t != "question":
            links.add_ref(t, owner.id, "goal", ctx.goal_id)
        if concept and ctx.concept_id:
            links.add_ref(t, owner.id, "concept", ctx.concept_id)
        if question and ctx.question_id:
            links.add_ref(t, owner.id, "question", ctx.question_id)
