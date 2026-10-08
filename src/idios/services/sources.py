"""Sources: books, articles, papers, videos and websites.

A source is a *reference*. A local path or URL is stored as given; the file
itself is never copied into IDIOS.
"""
from __future__ import annotations

from pathlib import Path
from typing import Optional

from idios.domain.errors import Invalid
from idios.domain.models import Person, Source, SourceType
from idios.services.context import ContextService
from idios.services.knowledge import KnowledgeService
from idios.storage.repos import Store

#: ``None`` category sorts last on the shelf
UNSORTED = "Unsorted"


def clean_path(raw: str) -> str:
    """Trim whitespace and the quotes a terminal drag-and-drop adds."""
    return raw.strip().strip("'\"").strip()


def looks_like_url(text: str) -> bool:
    return text.lower().startswith(("http://", "https://"))


def path_exists(path: str) -> bool:
    return Path(path).expanduser().exists()


class SourceService:
    def __init__(self, store: Store, context: ContextService,
                 knowledge: KnowledgeService) -> None:
        self.store = store
        self.context = context
        self.knowledge = knowledge

    def add(self, title: str, type: SourceType | str = SourceType.BOOK,
            author: Optional[str] = None, location: Optional[str] = None,
            category: Optional[str] = None, description: Optional[str] = None) -> Source:
        """Register a source. ``location`` is a URL or a local path."""
        title = " ".join(title.split())
        if not title:
            raise Invalid("A source needs a title.", hint="book: Programming from the Ground Up")
        source = Source(title=title, type=SourceType(type), description=description)
        if location:
            location = clean_path(location)
            if looks_like_url(location):
                source.url = location
            else:
                source.path = location
        if category:
            source.category_id = self.knowledge.category(category).id
        self.store.sources.add(source)
        if author:
            self.add_authors(source, author)
        return source

    def find(self, text: str) -> Optional[Source]:
        """Find a source by exact title or unique fragment."""
        text = " ".join(text.split())
        if not text:
            return None
        exact = self.store.sources.by_name(text)
        if exact:
            return exact
        hits = self.store.sources.like(text)
        if len(hits) == 1:
            return hits[0]
        if len(hits) > 1:
            names = "\n  ".join(h.title for h in hits[:8])
            raise Invalid(f"More than one source matches '{text}':\n  {names}",
                          hint="be a little more specific")
        return None

    def add_authors(self, source: Source, names: str) -> list[Person]:
        people = []
        for name in _split_names(names):
            person = self.knowledge.person(name)
            self.store.links.add_author(source.id, person.id)
            people.append(person)
        return people

    def authors(self, source: Source) -> list[Person]:
        people = [self.store.people.get(i) for i in self.store.links.author_ids(source.id)]
        return [p for p in people if p]

    def set_category(self, source: Source, name: str) -> None:
        source.category_id = self.knowledge.category(name).id
        self.store.sources.update(source)

    def set_location(self, source: Source, location: str) -> None:
        location = clean_path(location)
        if looks_like_url(location):
            source.url = location
        else:
            source.path = location
        self.store.sources.update(source)

    def tag(self, source: Source, names: str) -> list[str]:
        tags = [self.knowledge.tag(n) for n in _split_names(names)]
        for tag in tags:
            self.store.links.tag(tag.id, "source", source.id)
        return [t.name for t in tags]

    def tags(self, source: Source) -> list[str]:
        names = [self.store.tags.get(i) for i in self.store.links.tag_ids("source", source.id)]
        return sorted(t.name for t in names if t)

    def category_name(self, source: Source) -> Optional[str]:
        if not source.category_id:
            return None
        cat = self.store.categories.get(source.category_id)
        return cat.name if cat else None

    # -- shelf -----------------------------------------------------------
    def shelf(self) -> dict[SourceType, dict[str, list[Source]]]:
        """A logical index of registered sources: type -> category -> sources."""
        shelf: dict[SourceType, dict[str, list[Source]]] = {}
        for source in self.store.sources.find(order="lower(title)"):
            group = self.category_name(source) or UNSORTED
            shelf.setdefault(source.type, {}).setdefault(group, []).append(source)
        return shelf


def _split_names(text: str) -> list[str]:
    parts = [p.strip() for chunk in text.split(";") for p in chunk.split(",")]
    parts = [" ".join(p.split()) for p in parts if p.strip()]
    if not parts:
        raise Invalid("I didn't catch a name there.")
    return parts
