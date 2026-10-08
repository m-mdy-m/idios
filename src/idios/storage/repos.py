"""Repositories: the only code that talks SQL.

A small generic ``Repo`` maps flat dataclasses to tables by field name.
Anything that is not plain CRUD (links, relations, context) lives in
explicit, separate classes below it.
"""
from __future__ import annotations

import sqlite3
from dataclasses import fields
from typing import Any, Generic, Optional, Sequence, TypeVar

from idios.domain.ids import now
from idios.domain.models import (
    Answer, Category, Concept, Context, Entity, Goal, Highlight, Note, Person,
    Question, Quote, Relation, Session, Source, Tag, Topic,
)

T = TypeVar("T", bound=Entity)


class Repo(Generic[T]):
    table = ""
    model: type = Entity

    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn
        self._columns = [f.name for f in fields(self.model) if f.name != "seq"]

    # -- mapping ---------------------------------------------------------
    def _to_row(self, obj: T) -> dict[str, Any]:
        row = {}
        for name in self._columns:
            value = getattr(obj, name)
            row[name] = getattr(value, "value", value)  # enums -> plain str
        return row

    def _from_row(self, row: sqlite3.Row) -> T:
        return self.model(**{k: row[k] for k in row.keys()})

    # -- CRUD ------------------------------------------------------------
    def add(self, obj: T) -> T:
        row = self._to_row(obj)
        names = ", ".join(row)
        marks = ", ".join(f":{k}" for k in row)
        with self.conn:
            cur = self.conn.execute(
                f"INSERT INTO {self.table} ({names}) VALUES ({marks})", row
            )
        obj.seq = cur.lastrowid
        return obj

    def update(self, obj: T) -> T:
        obj.updated_at = now()
        row = self._to_row(obj)
        sets = ", ".join(f"{k} = :{k}" for k in row if k != "id")
        with self.conn:
            self.conn.execute(f"UPDATE {self.table} SET {sets} WHERE id = :id", row)
        return obj

    def get(self, id: str) -> Optional[T]:
        return self.one("id = ?", (id,))

    def get_by_seq(self, seq: int) -> Optional[T]:
        return self.one("seq = ?", (seq,))

    def delete(self, id: str) -> None:
        with self.conn:
            self.conn.execute(f"DELETE FROM {self.table} WHERE id = ?", (id,))

    def one(self, where: str, params: Sequence[Any] = ()) -> Optional[T]:
        row = self.conn.execute(
            f"SELECT * FROM {self.table} WHERE {where} LIMIT 1", params
        ).fetchone()
        return self._from_row(row) if row else None

    def find(self, where: str = "1=1", params: Sequence[Any] = (),
             order: str = "seq") -> list[T]:
        rows = self.conn.execute(
            f"SELECT * FROM {self.table} WHERE {where} ORDER BY {order}", params
        ).fetchall()
        return [self._from_row(r) for r in rows]

    def all(self) -> list[T]:
        return self.find()

    def count(self, where: str = "1=1", params: Sequence[Any] = ()) -> int:
        return self.conn.execute(
            f"SELECT COUNT(*) FROM {self.table} WHERE {where}", params
        ).fetchone()[0]


class NamedRepo(Repo[T]):
    """Repo for entities identified to the user by a unique, case-insensitive name."""

    name_column = "name"

    def by_name(self, name: str) -> Optional[T]:
        return self.one(f"lower({self.name_column}) = lower(?)", (name.strip(),))

    def like(self, text: str) -> list[T]:
        pattern = f"%{_escape_like(text.strip())}%"
        return self.find(f"{self.name_column} LIKE ? ESCAPE '\\' COLLATE NOCASE", (pattern,))


def _escape_like(text: str) -> str:
    return text.replace("\\", "\\\\").replace("%", "\\%").replace("_", "\\_")


class SourceRepo(NamedRepo[Source]):
    table, model, name_column = "sources", Source, "title"


class PersonRepo(NamedRepo[Person]):
    table, model = "people", Person


class GoalRepo(NamedRepo[Goal]):
    table, model, name_column = "goals", Goal, "title"


class SessionRepo(Repo[Session]):
    table, model = "sessions", Session


class QuestionRepo(Repo[Question]):
    table, model = "questions", Question


class AnswerRepo(Repo[Answer]):
    table, model = "answers", Answer


class NoteRepo(Repo[Note]):
    table, model = "notes", Note


class HighlightRepo(Repo[Highlight]):
    table, model = "highlights", Highlight


class QuoteRepo(Repo[Quote]):
    table, model = "quotes", Quote


class ConceptRepo(NamedRepo[Concept]):
    table, model = "concepts", Concept


class TopicRepo(NamedRepo[Topic]):
    table, model = "topics", Topic


class CategoryRepo(NamedRepo[Category]):
    table, model = "categories", Category


class TagRepo(NamedRepo[Tag]):
    table, model = "tags", Tag


class Links:
    """Junction tables: authors, tags, topics, mentions (refs) and relations."""

    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn

    # -- authors ---------------------------------------------------------
    def add_author(self, source_id: str, person_id: str) -> None:
        with self.conn:
            self.conn.execute(
                "INSERT OR IGNORE INTO source_authors VALUES (?, ?)",
                (source_id, person_id),
            )

    def author_ids(self, source_id: str) -> list[str]:
        rows = self.conn.execute(
            "SELECT person_id FROM source_authors WHERE source_id = ?", (source_id,)
        ).fetchall()
        return [r[0] for r in rows]

    def source_ids_of_person(self, person_id: str) -> list[str]:
        rows = self.conn.execute(
            "SELECT source_id FROM source_authors WHERE person_id = ?", (person_id,)
        ).fetchall()
        return [r[0] for r in rows]

    # -- topics ----------------------------------------------------------
    def add_concept_topic(self, concept_id: str, topic_id: str) -> None:
        with self.conn:
            self.conn.execute(
                "INSERT OR IGNORE INTO concept_topics VALUES (?, ?)",
                (concept_id, topic_id),
            )

    def topic_ids_of_concept(self, concept_id: str) -> list[str]:
        rows = self.conn.execute(
            "SELECT topic_id FROM concept_topics WHERE concept_id = ?", (concept_id,)
        ).fetchall()
        return [r[0] for r in rows]

    # -- tags ------------------------------------------------------------
    def tag(self, tag_id: str, entity_type: str, entity_id: str) -> None:
        with self.conn:
            self.conn.execute(
                "INSERT OR IGNORE INTO entity_tags VALUES (?, ?, ?)",
                (tag_id, entity_type, entity_id),
            )

    def tag_ids(self, entity_type: str, entity_id: str) -> list[str]:
        rows = self.conn.execute(
            "SELECT tag_id FROM entity_tags WHERE entity_type = ? AND entity_id = ?",
            (entity_type, entity_id),
        ).fetchall()
        return [r[0] for r in rows]

    def tagged(self, tag_id: str) -> list[tuple[str, str]]:
        rows = self.conn.execute(
            "SELECT entity_type, entity_id FROM entity_tags WHERE tag_id = ?", (tag_id,)
        ).fetchall()
        return [(r[0], r[1]) for r in rows]

    # -- mentions --------------------------------------------------------
    def add_ref(self, owner_type: str, owner_id: str,
                target_type: str, target_id: str) -> None:
        with self.conn:
            self.conn.execute(
                "INSERT OR IGNORE INTO refs VALUES (?, ?, ?, ?)",
                (owner_type, owner_id, target_type, target_id),
            )

    def owners_of(self, target_type: str, target_id: str,
                  owner_type: Optional[str] = None) -> list[tuple[str, str]]:
        sql = "SELECT owner_type, owner_id FROM refs WHERE target_type = ? AND target_id = ?"
        params: list[Any] = [target_type, target_id]
        if owner_type:
            sql += " AND owner_type = ?"
            params.append(owner_type)
        return [(r[0], r[1]) for r in self.conn.execute(sql, params).fetchall()]

    def targets_of(self, owner_type: str, owner_id: str,
                   target_type: Optional[str] = None) -> list[tuple[str, str]]:
        sql = "SELECT target_type, target_id FROM refs WHERE owner_type = ? AND owner_id = ?"
        params: list[Any] = [owner_type, owner_id]
        if target_type:
            sql += " AND target_type = ?"
            params.append(target_type)
        return [(r[0], r[1]) for r in self.conn.execute(sql, params).fetchall()]

    # -- relations -------------------------------------------------------
    def add_relation(self, rel: Relation) -> bool:
        """Insert a relation. Returns False if the same edge already exists."""
        with self.conn:
            cur = self.conn.execute(
                "INSERT OR IGNORE INTO relations VALUES (?,?,?,?,?,?,?,?)",
                (rel.id, rel.source_type, rel.source_id, rel.target_type,
                 rel.target_id, rel.type, rel.created_at, rel.updated_at),
            )
        return cur.rowcount > 0

    def relations_of(self, entity_type: str, entity_id: str) -> list[Relation]:
        rows = self.conn.execute(
            "SELECT * FROM relations WHERE (source_type = ? AND source_id = ?) "
            "OR (target_type = ? AND target_id = ?) ORDER BY created_at, id",
            (entity_type, entity_id, entity_type, entity_id),
        ).fetchall()
        return [Relation(**dict(r)) for r in rows]

    def all_relations(self) -> list[Relation]:
        rows = self.conn.execute("SELECT * FROM relations ORDER BY created_at, id")
        return [Relation(**dict(r)) for r in rows.fetchall()]

    def count_relations(self) -> int:
        return self.conn.execute("SELECT COUNT(*) FROM relations").fetchone()[0]

    # -- cleanup ---------------------------------------------------------
    def purge(self, entity_type: str, entity_id: str) -> None:
        """Remove every polymorphic link that points at or comes from an entity."""
        with self.conn:
            self.conn.execute(
                "DELETE FROM entity_tags WHERE entity_type = ? AND entity_id = ?",
                (entity_type, entity_id))
            self.conn.execute(
                "DELETE FROM refs WHERE (owner_type = ? AND owner_id = ?) "
                "OR (target_type = ? AND target_id = ?)",
                (entity_type, entity_id, entity_type, entity_id))
            self.conn.execute(
                "DELETE FROM relations WHERE (source_type = ? AND source_id = ?) "
                "OR (target_type = ? AND target_id = ?)",
                (entity_type, entity_id, entity_type, entity_id))


class ContextStore:
    """The active context, persisted as simple key/value rows."""

    KEYS = ("goal_id", "source_id", "concept_id", "question_id", "session_id",
            "chapter", "page", "section", "timestamp")

    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn

    def flag(self, name: str) -> bool:
        row = self.conn.execute("SELECT 1 FROM context WHERE key = ?", (f"flag:{name}",)).fetchone()
        return row is not None

    def set_flag(self, name: str) -> None:
        with self.conn:
            self.conn.execute("INSERT OR IGNORE INTO context VALUES (?, '1')", (f"flag:{name}",))

    def load(self) -> Context:
        rows = self.conn.execute("SELECT key, value FROM context").fetchall()
        data = {r[0]: r[1] for r in rows if r[0] in self.KEYS}
        return Context(**data)

    def save(self, ctx: Context) -> None:
        with self.conn:
            for key in self.KEYS:
                value = getattr(ctx, key)
                if value is None:
                    self.conn.execute("DELETE FROM context WHERE key = ?", (key,))
                else:
                    self.conn.execute(
                        "INSERT INTO context VALUES (?, ?) "
                        "ON CONFLICT(key) DO UPDATE SET value = excluded.value",
                        (key, value),
                    )


class Store:
    """All repositories over one connection."""

    def __init__(self, conn: sqlite3.Connection) -> None:
        self.conn = conn
        self.sources = SourceRepo(conn)
        self.people = PersonRepo(conn)
        self.goals = GoalRepo(conn)
        self.sessions = SessionRepo(conn)
        self.questions = QuestionRepo(conn)
        self.answers = AnswerRepo(conn)
        self.notes = NoteRepo(conn)
        self.highlights = HighlightRepo(conn)
        self.quotes = QuoteRepo(conn)
        self.concepts = ConceptRepo(conn)
        self.topics = TopicRepo(conn)
        self.categories = CategoryRepo(conn)
        self.tags = TagRepo(conn)
        self.links = Links(conn)
        self.context = ContextStore(conn)
        self.by_type: dict[str, Repo] = {
            "source": self.sources, "person": self.people, "goal": self.goals,
            "session": self.sessions, "question": self.questions,
            "answer": self.answers, "note": self.notes,
            "highlight": self.highlights, "quote": self.quotes,
            "concept": self.concepts, "topic": self.topics,
            "category": self.categories, "tag": self.tags,
        }

    def close(self) -> None:
        self.conn.close()
