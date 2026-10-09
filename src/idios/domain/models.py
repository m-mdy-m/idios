"""Domain model: plain typed dataclasses, no behaviour tied to storage.

Every persistent entity has a stable ``id`` (never derived from a title),
a display number ``seq`` assigned by the database, and timestamps.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum
from typing import Optional

from idios.domain.ids import new_id, now


class SourceType(str, Enum):
    BOOK = "book"
    ARTICLE = "article"
    PAPER = "paper"
    VIDEO = "video"
    WEBSITE = "website"


class QuestionStatus(str, Enum):
    OPEN = "open"
    ANSWERED = "answered"


class TaskStatus(str, Enum):
    OPEN = "open"
    DONE = "done"
    SKIPPED = "skipped"


# Relation types. Deliberately small; this is a lightweight graph.
RELATION_TYPES = (
    "related_to",
    "depends_on",
    "uses",
    "contains",
    "part_of",
    "explains",
    "contrasts_with",
    "prerequisite_of",
)

# entity type -> (table, id prefix, short reference letters, display label)
ENTITY_TYPES: dict[str, tuple[str, str, str, str]] = {
    "source": ("sources", "src", "s", "Source"),
    "person": ("people", "person", "p", "Person"),
    "goal": ("goals", "goal", "g", "Goal"),
    "session": ("sessions", "session", "x", "Session"),
    "question": ("questions", "question", "q", "Question"),
    "answer": ("answers", "answer", "a", "Answer"),
    "note": ("notes", "note", "n", "Note"),
    "highlight": ("highlights", "highlight", "h", "Highlight"),
    "quote": ("quotes", "quote", "qt", "Quote"),
    "concept": ("concepts", "concept", "c", "Concept"),
    "topic": ("topics", "topic", "t", "Topic"),
    "category": ("categories", "category", "k", "Category"),
    "tag": ("tags", "tag", "tg", "Tag"),
    "task": ("tasks", "task", "tk", "Task"),
}


@dataclass(kw_only=True)
class Entity:
    id: str = ""
    seq: Optional[int] = None
    created_at: str = ""
    updated_at: str = ""

    #: set by subclasses
    entity_type = ""

    def __post_init__(self) -> None:
        if not self.id:
            self.id = new_id(ENTITY_TYPES[self.entity_type][1])
        if not self.created_at:
            self.created_at = now()
        if not self.updated_at:
            self.updated_at = self.created_at

    @property
    def ref(self) -> str:
        """Short reference shown to the user, e.g. ``q12``."""
        letters = ENTITY_TYPES[self.entity_type][2]
        return f"{letters}{self.seq}" if self.seq is not None else self.id


@dataclass(kw_only=True)
class Located(Entity):
    """Mixin for records that remember where in a source they happened.

    Only fields the user actually provided are stored.
    """

    chapter: Optional[str] = None
    page: Optional[str] = None
    section: Optional[str] = None
    timestamp: Optional[str] = None

    def location_label(self) -> str:
        return format_location(self.chapter, self.page, self.section, self.timestamp)


def format_location(
    chapter: Optional[str],
    page: Optional[str],
    section: Optional[str],
    timestamp: Optional[str],
) -> str:
    parts = []
    if chapter:
        parts.append(f"Chapter {chapter}")
    if section:
        parts.append(f"Section {section}")
    if page:
        parts.append(f"Page {page}")
    if timestamp:
        parts.append(f"@ {timestamp}")
    return " · ".join(parts)


@dataclass(kw_only=True)
class Source(Entity):
    entity_type = "source"
    title: str
    type: SourceType = SourceType.BOOK
    path: Optional[str] = None  # reference to a local file; never copied
    url: Optional[str] = None
    description: Optional[str] = None
    category_id: Optional[str] = None

    def __post_init__(self) -> None:
        super().__post_init__()
        self.type = SourceType(self.type)


@dataclass(kw_only=True)
class Person(Entity):
    entity_type = "person"
    name: str


@dataclass(kw_only=True)
class Goal(Entity):
    entity_type = "goal"
    title: str
    description: Optional[str] = None


@dataclass(kw_only=True)
class Session(Located):
    """Internal learning state. The user never has to manage one."""

    entity_type = "session"
    started_at: str = ""
    ended_at: Optional[str] = None
    goal_id: Optional[str] = None
    source_id: Optional[str] = None
    activity_count: int = 0

    def __post_init__(self) -> None:
        super().__post_init__()
        if not self.started_at:
            self.started_at = self.created_at


@dataclass(kw_only=True)
class Question(Located):
    entity_type = "question"
    text: str
    status: QuestionStatus = QuestionStatus.OPEN
    goal_id: Optional[str] = None
    source_id: Optional[str] = None

    def __post_init__(self) -> None:
        super().__post_init__()
        self.status = QuestionStatus(self.status)


@dataclass(kw_only=True)
class Answer(Entity):
    entity_type = "answer"
    question_id: str
    text: str


@dataclass(kw_only=True)
class Note(Located):
    entity_type = "note"
    text: str


@dataclass(kw_only=True)
class Highlight(Located):
    entity_type = "highlight"
    source_id: str
    text: str


@dataclass(kw_only=True)
class Quote(Located):
    entity_type = "quote"
    source_id: str
    text: str


@dataclass(kw_only=True)
class Concept(Entity):
    entity_type = "concept"
    name: str
    description: Optional[str] = None


@dataclass(kw_only=True)
class Topic(Entity):
    entity_type = "topic"
    name: str
    parent_id: Optional[str] = None


@dataclass(kw_only=True)
class Category(Entity):
    entity_type = "category"
    name: str
    parent_id: Optional[str] = None


@dataclass(kw_only=True)
class Tag(Entity):
    entity_type = "tag"
    name: str


@dataclass(kw_only=True)
class Task(Entity):
    """Something to do on a given day, or something done (``did:``).

    ``due_date`` is a local calendar date, ``YYYY-MM-DD``.
    """

    entity_type = "task"
    text: str
    due_date: str
    status: TaskStatus = TaskStatus.OPEN
    done_at: Optional[str] = None
    goal_id: Optional[str] = None
    source_id: Optional[str] = None

    def __post_init__(self) -> None:
        super().__post_init__()
        self.status = TaskStatus(self.status)


@dataclass(kw_only=True)
class Relation:
    """A typed, directed edge between two entities. Not a tag, not a category."""

    source_type: str
    source_id: str
    target_type: str
    target_id: str
    type: str = "related_to"
    id: str = field(default_factory=lambda: new_id("rel"))
    created_at: str = field(default_factory=now)
    updated_at: str = ""

    def __post_init__(self) -> None:
        if not self.updated_at:
            self.updated_at = self.created_at


@dataclass
class Context:
    """What the user is working on right now. Persisted between runs."""

    goal_id: Optional[str] = None
    source_id: Optional[str] = None
    concept_id: Optional[str] = None
    question_id: Optional[str] = None
    session_id: Optional[str] = None
    chapter: Optional[str] = None
    page: Optional[str] = None
    section: Optional[str] = None
    timestamp: Optional[str] = None

    def location_label(self) -> str:
        return format_location(self.chapter, self.page, self.section, self.timestamp)

    def clear_location(self) -> None:
        self.chapter = self.page = self.section = self.timestamp = None

    def is_empty(self) -> bool:
        return not (self.goal_id or self.source_id or self.location_label())
