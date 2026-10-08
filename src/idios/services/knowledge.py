"""Knowledge objects (concepts, topics, categories, tags, people) and the graph."""
from __future__ import annotations

from typing import Optional

from idios.domain.errors import Invalid
from idios.domain.models import (
    RELATION_TYPES, Category, Concept, Entity, Person, Relation, Tag, Topic,
)
from idios.services.context import ContextService
from idios.storage.repos import Store


def _clean(name: str, what: str) -> str:
    name = " ".join(name.split())
    if not name:
        raise Invalid(f"The {what} needs a name.")
    return name


class KnowledgeService:
    def __init__(self, store: Store, context: ContextService) -> None:
        self.store = store
        self.context = context

    # -- get-or-create ----------------------------------------------------
    def person(self, name: str) -> Person:
        name = _clean(name, "person")
        return self.store.people.by_name(name) or self.store.people.add(Person(name=name))

    def tag(self, name: str) -> Tag:
        name = _clean(name, "tag").lstrip("#").lower()
        return self.store.tags.by_name(name) or self.store.tags.add(Tag(name=name))

    def category(self, name: str) -> Category:
        name = _clean(name, "category")
        return self.store.categories.by_name(name) or self.store.categories.add(
            Category(name=name))

    def topic(self, name: str) -> tuple[Topic, bool]:
        name = _clean(name, "topic")
        existing = self.store.topics.by_name(name)
        if existing:
            return existing, False
        return self.store.topics.add(Topic(name=name)), True

    def concept(self, name: str) -> tuple[Concept, bool]:
        """Return (concept, created). Never creates a duplicate name."""
        name = _clean(name, "concept")
        existing = self.store.concepts.by_name(name)
        if existing:
            return existing, False
        concept = self.store.concepts.add(Concept(name=name))
        source = self.context.source
        if source:  # remember which source this concept came from
            self.store.links.add_ref("concept", concept.id, "source", source.id)
        return concept, True

    def adopt_question(self, concept: Concept) -> Optional[str]:
        """Link the current question to this concept if it has none yet.

        Typing ``concept: X`` right after asking a question almost always
        names what the question is about. Returns the question text if linked.
        """
        question = self.context.question
        if question is None:
            return None
        links = self.store.links
        if links.targets_of("question", question.id, "concept"):
            return None
        links.add_ref("question", question.id, "concept", concept.id)
        return question.text

    def file_under_topic(self, concept: Concept, topic: Topic) -> None:
        self.store.links.add_concept_topic(concept.id, topic.id)

    # -- graph ------------------------------------------------------------
    def link(self, a: str, b: str, rel_type: str = "related_to") -> tuple[Relation, bool, list[str]]:
        """Relate two entities by name. Unknown names become new concepts.

        Returns (relation, created, names_of_new_concepts).
        """
        rel_type = rel_type.strip().lower().replace(" ", "_").replace("-", "_")
        if rel_type not in RELATION_TYPES:
            raise Invalid(
                f"'{rel_type}' is not a relation type I know.",
                hint="one of: " + ", ".join(RELATION_TYPES),
            )
        new: list[str] = []
        left = self._resolve_or_create(a, new)
        right = self._resolve_or_create(b, new)
        if left.id == right.id:
            raise Invalid("A thing can't be linked to itself.")
        rel = Relation(source_type=left.entity_type, source_id=left.id,
                       target_type=right.entity_type, target_id=right.id, type=rel_type)
        created = self.store.links.add_relation(rel)
        return rel, created, new

    def _resolve_or_create(self, text: str, new: list[str]) -> Entity:
        text = _clean(text, "concept")
        for repo in (self.store.concepts, self.store.topics, self.store.sources,
                     self.store.people):
            hit = repo.by_name(text)
            if hit:
                return hit
        concept, created = self.concept(text)
        if created:
            new.append(concept.name)
        return concept

    def entity_name(self, entity_type: str, entity_id: str) -> str:
        entity = self.store.by_type[entity_type].get(entity_id)
        if entity is None:
            return "(deleted)"
        return getattr(entity, "name", None) or getattr(entity, "title", None) or entity.id

    def neighbours(self, concept: Concept) -> list[tuple[str, str, str, str]]:
        """Edges touching a concept as (direction, relation type, entity type, name)."""
        out = []
        for rel in self.store.links.relations_of("concept", concept.id):
            if rel.source_id == concept.id:
                out.append(("out", rel.type, rel.target_type,
                            self.entity_name(rel.target_type, rel.target_id)))
            else:
                out.append(("in", rel.type, rel.source_type,
                            self.entity_name(rel.source_type, rel.source_id)))
        return out

    def related_concepts(self, concept: Concept) -> list[str]:
        seen: list[str] = []
        for _, _, etype, name in self.neighbours(concept):
            if etype == "concept" and name not in seen:
                seen.append(name)
        return seen

    def tree(self, root: Optional[Concept], depth: int = 3) -> list[str]:
        """Render the graph as indented lines. From a concept, or every edge."""
        links = self.store.links
        lines: list[str] = []

        def children(etype: str, eid: str):
            return [r for r in links.relations_of(etype, eid)
                    if r.source_type == etype and r.source_id == eid]

        def walk(etype: str, eid: str, prefix: str, level: int, path: set[str]) -> None:
            kids = children(etype, eid)
            for i, rel in enumerate(kids):
                last = i == len(kids) - 1
                branch = "└──" if last else "├──"
                name = self.entity_name(rel.target_type, rel.target_id)
                lines.append(f"{prefix}{branch} {rel.type} → {name}")
                if level < depth and rel.target_id not in path:
                    walk(rel.target_type, rel.target_id,
                         prefix + ("    " if last else "│   "), level + 1,
                         path | {rel.target_id})

        if root is not None:
            lines.append(root.name)
            walk("concept", root.id, "", 1, {root.id})
            for rel in links.relations_of("concept", root.id):  # incoming edges
                if rel.target_id == root.id:
                    lines.append(f"← {rel.type} ← "
                                 f"{self.entity_name(rel.source_type, rel.source_id)}")
            return lines

        seen: set[tuple[str, str]] = set()
        for rel in links.all_relations():
            key = (rel.source_type, rel.source_id)
            if key in seen:
                continue
            seen.add(key)
            lines.append(self.entity_name(*key))
            walk(rel.source_type, rel.source_id, "", 1, {rel.source_id})
            lines.append("")
        return lines[:-1] if lines else lines
