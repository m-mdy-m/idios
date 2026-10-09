"""SQLite connection and schema.

One local file, standard library only. ``PRAGMA user_version`` tracks the
schema version; migrations are added only when the schema actually changes.

Every entity table has:
  seq  INTEGER PRIMARY KEY AUTOINCREMENT  -- display number, never reused
  id   TEXT UNIQUE                        -- stable identity
"""
from __future__ import annotations

import os
import sqlite3
from pathlib import Path

SCHEMA_VERSION = 2

_LOCATION = """
    chapter TEXT, page TEXT, section TEXT, timestamp TEXT,
"""

_STAMPS = """
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
"""


def _entity(table: str, columns: str) -> str:
    return f"""
    CREATE TABLE {table} (
        seq INTEGER PRIMARY KEY AUTOINCREMENT,
        id TEXT NOT NULL UNIQUE,
        {columns}
    );"""


SCHEMA_V1 = [
    _entity("categories", f"""
        name TEXT NOT NULL,
        parent_id TEXT REFERENCES categories(id) ON DELETE SET NULL,
        {_STAMPS}"""),
    _entity("topics", f"""
        name TEXT NOT NULL,
        parent_id TEXT REFERENCES topics(id) ON DELETE SET NULL,
        {_STAMPS}"""),
    _entity("tags", f"name TEXT NOT NULL, {_STAMPS}"),
    _entity("people", f"name TEXT NOT NULL, {_STAMPS}"),
    _entity("goals", f"title TEXT NOT NULL, description TEXT, {_STAMPS}"),
    _entity("sources", f"""
        title TEXT NOT NULL,
        type TEXT NOT NULL,
        path TEXT,
        url TEXT,
        description TEXT,
        category_id TEXT REFERENCES categories(id) ON DELETE SET NULL,
        {_STAMPS}"""),
    _entity("sessions", f"""
        started_at TEXT NOT NULL,
        ended_at TEXT,
        goal_id TEXT REFERENCES goals(id) ON DELETE SET NULL,
        source_id TEXT REFERENCES sources(id) ON DELETE SET NULL,
        {_LOCATION}
        activity_count INTEGER NOT NULL DEFAULT 0,
        {_STAMPS}"""),
    _entity("questions", f"""
        text TEXT NOT NULL,
        status TEXT NOT NULL DEFAULT 'open',
        goal_id TEXT REFERENCES goals(id) ON DELETE SET NULL,
        source_id TEXT REFERENCES sources(id) ON DELETE SET NULL,
        {_LOCATION}
        {_STAMPS}"""),
    _entity("answers", f"""
        question_id TEXT NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
        text TEXT NOT NULL,
        {_STAMPS}"""),
    _entity("notes", f"""
        text TEXT NOT NULL,
        {_LOCATION}
        {_STAMPS}"""),
    _entity("highlights", f"""
        source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
        text TEXT NOT NULL,
        {_LOCATION}
        {_STAMPS}"""),
    _entity("quotes", f"""
        source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
        text TEXT NOT NULL,
        {_LOCATION}
        {_STAMPS}"""),
    _entity("concepts", f"name TEXT NOT NULL, description TEXT, {_STAMPS}"),
    """CREATE TABLE source_authors (
        source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
        person_id TEXT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
        PRIMARY KEY (source_id, person_id)
    );""",
    """CREATE TABLE concept_topics (
        concept_id TEXT NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
        topic_id TEXT NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
        PRIMARY KEY (concept_id, topic_id)
    );""",
    # Tags are labels attached to any entity. They are not relations.
    """CREATE TABLE entity_tags (
        tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
        entity_type TEXT NOT NULL,
        entity_id TEXT NOT NULL,
        PRIMARY KEY (tag_id, entity_type, entity_id)
    );""",
    # Mentions: "this note / question / highlight is about that entity".
    """CREATE TABLE refs (
        owner_type TEXT NOT NULL,
        owner_id TEXT NOT NULL,
        target_type TEXT NOT NULL,
        target_id TEXT NOT NULL,
        PRIMARY KEY (owner_type, owner_id, target_type, target_id)
    );""",
    # Typed, directed graph edges. Independent from tags and categories.
    """CREATE TABLE relations (
        id TEXT PRIMARY KEY,
        source_type TEXT NOT NULL,
        source_id TEXT NOT NULL,
        target_type TEXT NOT NULL,
        target_id TEXT NOT NULL,
        type TEXT NOT NULL,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        UNIQUE (source_type, source_id, target_type, target_id, type)
    );""",
    "CREATE TABLE context (key TEXT PRIMARY KEY, value TEXT);",
    "CREATE UNIQUE INDEX idx_people_name ON people(lower(name));",
    "CREATE UNIQUE INDEX idx_goals_title ON goals(lower(title));",
    "CREATE UNIQUE INDEX idx_concepts_name ON concepts(lower(name));",
    "CREATE UNIQUE INDEX idx_topics_name ON topics(lower(name));",
    "CREATE UNIQUE INDEX idx_categories_name ON categories(lower(name));",
    "CREATE UNIQUE INDEX idx_tags_name ON tags(lower(name));",
    "CREATE INDEX idx_refs_target ON refs(target_type, target_id);",
    "CREATE INDEX idx_relations_source ON relations(source_type, source_id);",
    "CREATE INDEX idx_relations_target ON relations(target_type, target_id);",
]

SCHEMA_V2 = [
    _entity("tasks", f"""
        text TEXT NOT NULL,
        due_date TEXT NOT NULL,
        status TEXT NOT NULL DEFAULT 'open',
        done_at TEXT,
        goal_id TEXT REFERENCES goals(id) ON DELETE SET NULL,
        source_id TEXT REFERENCES sources(id) ON DELETE SET NULL,
        {_STAMPS}"""),
    "CREATE INDEX idx_tasks_due ON tasks(due_date, status);",
]

MIGRATIONS = {1: SCHEMA_V1, 2: SCHEMA_V2}

TABLES = [
    "categories", "topics", "tags", "people", "goals", "sources", "sessions",
    "questions", "answers", "notes", "highlights", "quotes", "concepts",
    "source_authors", "concept_topics", "entity_tags", "refs", "relations", "tasks",
]


def default_home() -> Path:
    """Where IDIOS keeps its own data. Override with IDIOS_HOME."""
    env = os.environ.get("IDIOS_HOME")
    if env:
        return Path(env).expanduser()
    return Path.home() / ".idios"


def connect(db_path: Path | str) -> sqlite3.Connection:
    """Open (and migrate) the database. ``":memory:"`` is allowed for tests."""
    if str(db_path) != ":memory:":
        Path(db_path).parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(str(db_path))
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    _migrate(conn)
    return conn


def _migrate(conn: sqlite3.Connection) -> None:
    """Apply each missing schema step in order; existing data is never touched."""
    version = conn.execute("PRAGMA user_version").fetchone()[0]
    for target in range(version + 1, SCHEMA_VERSION + 1):
        with conn:
            for statement in MIGRATIONS[target]:
                conn.execute(statement)
            conn.execute(f"PRAGMA user_version = {target}")
