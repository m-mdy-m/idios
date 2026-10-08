"""Read-only summaries for the status, source, concept, question and goal views.

These return plain data; turning it into text is the renderer's job.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

from idios.domain.models import Concept, Goal, Question, Source
from idios.services.context import ContextService
from idios.services.knowledge import KnowledgeService
from idios.services.sources import SourceService, path_exists
from idios.storage.repos import Store


@dataclass
class StatusView:
    goal: Optional[str]
    source: Optional[str]
    location: str
    concept: Optional[str]
    open_questions: int
    concepts: int
    highlights: int
    quotes: int
    notes: int


@dataclass
class SourceView:
    ref: str
    title: str
    authors: list[str]
    type: str
    path: Optional[str]
    path_missing: bool
    url: Optional[str]
    category: Optional[str]
    tags: list[str]
    questions: int
    highlights: int
    quotes: int
    concepts: int
    notes: int


@dataclass
class ConceptView:
    ref: str
    name: str
    sources: list[str]
    questions: list[tuple[str, str, str]]  # (ref, text, status)
    related: list[str]
    topics: list[str]
    highlights: int
    quotes: int
    notes: int
    description: Optional[str] = None


@dataclass
class QuestionView:
    ref: str
    text: str
    status: str
    source: Optional[str]
    location: str
    answers: list[str] = field(default_factory=list)


@dataclass
class GoalView:
    ref: str
    title: str
    sources: list[str]
    concepts: list[str]
    open_questions: int
    answered_questions: int


class ViewService:
    def __init__(self, store: Store, context: ContextService,
                 knowledge: KnowledgeService, sources: SourceService) -> None:
        self.store = store
        self.context = context
        self.knowledge = knowledge
        self.sources = sources

    def status(self) -> StatusView:
        s = self.store
        goal, source, concept = self.context.goal, self.context.source, self.context.concept
        return StatusView(
            goal=goal.title if goal else None,
            source=source.title if source else None,
            location=self.context.ctx.location_label(),
            concept=concept.name if concept else None,
            open_questions=s.questions.count("status = 'open'"),
            concepts=s.concepts.count(),
            highlights=s.highlights.count(),
            quotes=s.quotes.count(),
            notes=s.notes.count(),
        )

    # -- source ----------------------------------------------------------
    def source_view(self, source: Source) -> SourceView:
        s = self.store
        return SourceView(
            ref=source.ref,
            title=source.title,
            authors=[p.name for p in self.sources.authors(source)],
            type=source.type.value.capitalize(),
            path=source.path,
            path_missing=bool(source.path) and not path_exists(source.path or ""),
            url=source.url,
            category=self.sources.category_name(source),
            tags=self.sources.tags(source),
            questions=s.questions.count("source_id = ?", (source.id,)),
            highlights=s.highlights.count("source_id = ?", (source.id,)),
            quotes=s.quotes.count("source_id = ?", (source.id,)),
            concepts=len(self._concepts_of_source(source)),
            notes=len(s.links.owners_of("source", source.id, "note")),
        )

    def _concepts_of_source(self, source: Source) -> set[str]:
        links, s = self.store.links, self.store
        found = {o_id for _, o_id in links.owners_of("source", source.id, "concept")}
        owners: list[tuple[str, str]] = [("question", q.id) for q in
                                         s.questions.find("source_id = ?", (source.id,))]
        owners += [("highlight", h.id) for h in
                   s.highlights.find("source_id = ?", (source.id,))]
        owners += [("quote", q.id) for q in s.quotes.find("source_id = ?", (source.id,))]
        owners += links.owners_of("source", source.id, "note")
        for otype, oid in owners:
            found.update(t for _, t in links.targets_of(otype, oid, "concept"))
        return found

    # -- concept ---------------------------------------------------------
    def concept_view(self, concept: Concept) -> ConceptView:
        links, s = self.store.links, self.store
        owners = links.owners_of("concept", concept.id)
        question_ids = [i for t, i in owners if t == "question"]
        questions = [q for q in (s.questions.get(i) for i in question_ids) if q]
        source_ids = {t_id for _, t_id in links.targets_of("concept", concept.id, "source")}
        for etype in ("highlight", "quote"):
            for otype, oid in owners:
                if otype == etype:
                    item = s.by_type[etype].get(oid)
                    if item:
                        source_ids.add(item.source_id)  # type: ignore[attr-defined]
        for q in questions:
            if q.source_id:
                source_ids.add(q.source_id)
        for otype, oid in owners:
            if otype == "note":
                source_ids.update(t for _, t in links.targets_of("note", oid, "source"))
        titles = sorted(x.title for x in (s.sources.get(i) for i in source_ids) if x)
        topics = [s.topics.get(i) for i in links.topic_ids_of_concept(concept.id)]
        return ConceptView(
            ref=concept.ref,
            name=concept.name,
            description=concept.description,
            sources=titles,
            questions=[(q.ref, q.text, q.status.value) for q in questions],
            related=self.knowledge.related_concepts(concept),
            topics=sorted(t.name for t in topics if t),
            highlights=sum(1 for t, _ in owners if t == "highlight"),
            quotes=sum(1 for t, _ in owners if t == "quote"),
            notes=sum(1 for t, _ in owners if t == "note"),
        )

    # -- question / goal ---------------------------------------------------
    def question_view(self, question: Question) -> QuestionView:
        source = self.store.sources.get(question.source_id) if question.source_id else None
        answers = self.store.answers.find("question_id = ?", (question.id,))
        return QuestionView(ref=question.ref, text=question.text,
                            status=question.status.value,
                            source=source.title if source else None,
                            location=question.location_label(),
                            answers=[a.text for a in answers])

    def goal_view(self, goal: Goal) -> GoalView:
        s = self.store
        source_ids: set[str] = set()
        concept_ids: set[str] = set()
        for q in s.questions.find("goal_id = ?", (goal.id,)):
            if q.source_id:
                source_ids.add(q.source_id)
            concept_ids.update(t for _, t in s.links.targets_of("question", q.id, "concept"))
        for otype, oid in s.links.owners_of("goal", goal.id):
            source_ids.update(t for _, t in s.links.targets_of(otype, oid, "source"))
            concept_ids.update(t for _, t in s.links.targets_of(otype, oid, "concept"))
        return GoalView(
            ref=goal.ref, title=goal.title,
            sources=sorted(x.title for x in (s.sources.get(i) for i in source_ids) if x),
            concepts=sorted(c.name for c in (s.concepts.get(i) for i in concept_ids) if c),
            open_questions=s.questions.count("goal_id = ? AND status = 'open'", (goal.id,)),
            answered_questions=s.questions.count(
                "goal_id = ? AND status = 'answered'", (goal.id,)),
        )
