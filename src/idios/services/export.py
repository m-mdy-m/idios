"""Export everything to JSON (lossless) or Markdown (readable, Obsidian-friendly)."""
from __future__ import annotations

import json
from pathlib import Path
from typing import Optional

from idios.domain.ids import now
from idios.domain.errors import Invalid
from idios.services.knowledge import KnowledgeService
from idios.services.sources import SourceService
from idios.storage.db import TABLES
from idios.storage.repos import Store


class ExportService:
    def __init__(self, store: Store, knowledge: KnowledgeService,
                 sources: SourceService, home: Path) -> None:
        self.store = store
        self.knowledge = knowledge
        self.sources = sources
        self.home = home

    def export(self, fmt: str = "markdown", out: Optional[Path] = None) -> Path:
        fmt = fmt.lower()
        if fmt in ("md", "markdown"):
            text, ext = self.markdown(), "md"
        elif fmt == "json":
            text, ext = self.json(), "json"
        else:
            raise Invalid(f"I can't export as '{fmt}'.", hint="idios export --format markdown|json")
        if out is None:
            stamp = now().replace(":", "").replace("-", "")[:15]
            out = self.home / "exports" / f"idios-{stamp}.{ext}"
        out = Path(out).expanduser()
        out.parent.mkdir(parents=True, exist_ok=True)
        out.write_text(text, encoding="utf-8")
        return out

    def json(self) -> str:
        data = {"exported_at": now(), "version": 1}
        for table in TABLES:
            rows = self.store.conn.execute(f"SELECT * FROM {table}").fetchall()
            data[table] = [dict(r) for r in rows]
        return json.dumps(data, indent=2, ensure_ascii=False)

    def markdown(self) -> str:
        s = self.store
        out = ["# IDIOS export", "", f"_Exported {now()}_", ""]

        out += ["## Goals", ""]
        out += [f"- {g.title}" for g in s.goals.all()] or ["_none_"]

        out += ["", "## Sources", ""]
        for src in s.sources.find(order="lower(title)"):
            authors = ", ".join(p.name for p in self.sources.authors(src))
            out.append(f"### {src.title}")
            meta = [src.type.value] + ([authors] if authors else [])
            out.append("_" + " · ".join(meta) + "_")
            if src.path or src.url:
                out.append(f"Reference: `{src.path or src.url}`")
            for h in s.highlights.find("source_id = ?", (src.id,)):
                out.append(f"- **Highlight** {_loc(h)}: {h.text}")
            for q in s.quotes.find("source_id = ?", (src.id,)):
                out.append(f"> {q.text}" + (f"  \n> — {_loc(q)}" if _loc(q) else ""))
            out.append("")

        out += ["## Questions", ""]
        for q in s.questions.all():
            mark = "x" if q.status.value == "answered" else " "
            out.append(f"- [{mark}] {q.text}")
            for a in s.answers.find("question_id = ?", (q.id,)):
                out.append(f"    - {a.text}")

        out += ["", "## Plan", ""]
        for task in s.tasks.find(order="due_date, seq"):
            mark = {"done": "x", "skipped": "-"}.get(task.status.value, " ")
            out.append(f"- [{mark}] {task.due_date} {task.text}")

        out += ["", "## Notes", ""]
        out += [f"- {n.text}" for n in s.notes.all()] or ["_none_"]

        out += ["", "## Concepts", ""]
        for c in s.concepts.find(order="lower(name)"):
            related = ", ".join(f"[[{n}]]" for n in self.knowledge.related_concepts(c))
            out.append(f"- [[{c.name}]]" + (f" — {related}" if related else ""))

        out += ["", "## Relations", ""]
        for r in s.links.all_relations():
            out.append(f"- {self.knowledge.entity_name(r.source_type, r.source_id)} "
                       f"**{r.type}** {self.knowledge.entity_name(r.target_type, r.target_id)}")
        return "\n".join(out) + "\n"


def _loc(item) -> str:
    return f"({item.location_label()})" if item.location_label() else ""
