"""Terminal text for every view. Pure functions: data in, string out.

No business logic here, and no internal terminology: the user should feel
they are learning, not managing a database. Colour comes from ``style`` and
vanishes automatically when output is piped, so the text is always readable.
"""
from __future__ import annotations

import re
from typing import Optional, Sequence

from idios.domain.models import Question, SourceType
from idios.services.search import Hit, stem, tokenize
from idios.services.sources import UNSORTED
from idios.services.views import (
    ConceptView, GoalView, QuestionView, SourceView, StatusView,
)
from idios.shell import help as help_text
from idios.shell.style import style

OK = "✓"
_WORD = re.compile(r"\w+", re.UNICODE)


def rule(width: int = 40) -> str:
    return style.dim("─" * width)


def decorate(text: str) -> str:
    """Colour feedback: a leading ✓ turns green, and the indented detail
    lines that follow it are dimmed. Text without ✓ is returned untouched."""
    if not style.enabled or OK not in text:
        return text
    out, after_ok = [], False
    for line in text.split("\n"):
        if line.startswith(OK):
            out.append(style.ok(OK) + line[len(OK):])
            after_ok = True
        elif after_ok and line.startswith("  "):
            out.append(style.dim(line))
        else:
            out.append(line)
            after_ok = False
    return "\n".join(out)


def prompt() -> str:
    return style.accent("> ")


def help_overview(topic: str = "") -> str:
    return help_text.topic(topic)


def error(message: str, hint: str = "") -> str:
    out = style.error(message)
    if hint:
        out += f"\n\n{style.warn('Try:')}\n  {style.command(hint)}"
    return out


def heading(title: str) -> str:
    return f"{style.heading(title)}\n{style.dim('─' * max(len(title), 8))}"


def shorten(text: str, width: int = 90) -> str:
    text = " ".join(text.split())
    return text if len(text) <= width else text[: width - 1].rstrip() + "…"


def _pad(ref: str, width: int = 5) -> str:
    return style.ref(f"{ref:<{width}}")


def header(goal: Optional[str], source: Optional[str], location: str,
           concept: Optional[str] = None) -> str:
    lines = [f"{style.accent('IDIOS')}  {style.dim('learn · connect · recall')}", rule(), ""]
    if goal:
        lines.append(f"{style.label('Goal:')} {style.kind('goal', goal, bold=True)}")
    if source:
        lines.append(f"{style.label('Source:')} {style.kind('source', source, bold=True)}")
    if location:
        lines.append(f"{style.label('Location:')} {location}")
    if concept:
        lines.append(f"{style.label('Concept:')} {style.kind('concept', concept)}")
    if not (goal or source):
        lines += [style.warn("Nothing selected yet. Start with:"),
                  f"  {style.command('goal: Learn Assembly')}",
                  f"  {style.command('source: Programming from the Ground Up')}",
                  f"  {style.dim('(')}{style.command(':help')}{style.dim(' shows everything, with examples)')}"]
    lines.append("")
    return "\n".join(lines)


def welcome_back(goal: Optional[str], source: Optional[str], location: str) -> str:
    lines = [style.accent("Welcome back."), "", style.bold("Last context:")]
    if goal:
        lines.append(f"{style.label('Goal:')} {style.kind('goal', goal)}")
    if source:
        lines.append(f"{style.label('Source:')} {style.kind('source', source)}")
    if location:
        lines.append(f"{style.label('Location:')} {location}")
    lines.append("")
    return "\n".join(lines)


def status(v: StatusView) -> str:
    def block(label: str, value) -> list[str]:
        shown = style.number(value) if isinstance(value, int) else value
        return [style.label(f"{label}:"), f"  {shown}", ""]

    none = style.dim("(none)")
    out = [style.accent("IDIOS STATUS"), ""]
    out += block("Goal", style.kind("goal", v.goal, True) if v.goal else none)
    out += block("Source", style.kind("source", v.source, True) if v.source else none)
    out += block("Location", v.location or none)
    if v.concept:
        out += block("Concept", style.kind("concept", v.concept))
    out += block("Open questions", v.open_questions)
    out += block("Concepts", v.concepts)
    out += block("Notes", v.notes)
    out += block("Highlights", v.highlights)
    out += block("Quotes", v.quotes)
    return "\n".join(out).rstrip()


def source_view(v: SourceView) -> str:
    out = [style.kind("source", v.title, True), ""]
    if v.authors:
        out += [style.label("Author:" if len(v.authors) == 1 else "Authors:"),
                "  " + ", ".join(v.authors), ""]
    out += [style.label("Type:"), f"  {v.type}", ""]
    if v.category:
        out += [style.label("Shelf:"), f"  {v.category}", ""]
    if v.path:
        missing = style.warn("   (file not found)") if v.path_missing else ""
        out += [style.label("Path:"), f"  {v.path}{missing}", ""]
    if v.url:
        out += [style.label("URL:"), f"  {v.url}", ""]
    if v.tags:
        out += [style.label("Tags:"), "  " + ", ".join(style.kind("tag", t) for t in v.tags), ""]
    for label, n in (("Questions", v.questions), ("Highlights", v.highlights),
                     ("Quotes", v.quotes), ("Concepts", v.concepts), ("Notes", v.notes)):
        out += [style.label(f"{label}:"), f"  {style.number(n)}", ""]
    return "\n".join(out).rstrip()


def concept_view(v: ConceptView) -> str:
    out = [style.kind("concept", v.name, True), ""]
    if v.description:
        out += [v.description, ""]
    if v.sources:
        out += [style.label("Sources")] + [f"  {style.kind('source', s)}" for s in v.sources] + [""]
    if v.questions:
        out += [style.label("Questions")] + [
            f"  {shorten(text)}" + ("" if state == "answered" else style.warn("  (open)"))
            for _, text, state in v.questions] + [""]
    if v.related:
        out += [style.label("Related Concepts")] + [
            f"  {style.kind('concept', c)}" for c in v.related] + [""]
    if v.topics:
        out += [style.label("Topics")] + [f"  {style.kind('topic', t)}" for t in v.topics] + [""]
    out += [style.label("Highlights"), f"  {style.number(v.highlights)}", "",
            style.label("Quotes"), f"  {style.number(v.quotes)}", "",
            style.label("Notes"), f"  {style.number(v.notes)}"]
    return "\n".join(out)


def question_view(v: QuestionView) -> str:
    state = style.ok(v.status) if v.status == "answered" else style.warn(v.status)
    out = [style.kind("question", v.text, True), "", f"{style.label('Status:')} {state}"]
    if v.source:
        out.append(f"{style.label('Source:')} {v.source}" + (f" · {v.location}" if v.location else ""))
    if v.answers:
        out += ["", style.label("Answers")] + [f"  {style.kind('answer', a)}" for a in v.answers]
    return "\n".join(out)


def goal_view(v: GoalView) -> str:
    out = [style.kind("goal", v.title, True), ""]
    if v.sources:
        out += [style.label("Sources")] + [f"  {style.kind('source', s)}" for s in v.sources] + [""]
    if v.concepts:
        out += [style.label("Concepts")] + [f"  {style.kind('concept', c)}" for c in v.concepts] + [""]
    out += [style.label("Questions"),
            f"  {style.number(v.open_questions)} open · {style.number(v.answered_questions)} answered"]
    return "\n".join(out)


def question_list(questions: Sequence[Question], sources: dict[str, str],
                  title: str = "Open Questions") -> str:
    if not questions:
        return (f"{heading(title)}\nNone. Ask one by typing it, e.g. "
                f"{style.command('Why does the CPU need registers?')}")
    lines = [heading(title)]
    for q in questions:
        where = sources.get(q.source_id or "", "")
        tail = " · ".join(x for x in (where, q.location_label()) if x)
        mark = "" if q.status.value == "open" else style.ok("  ✓")
        lines.append(f"{_pad(q.ref)} {style.kind('question', shorten(q.text))}{mark}")
        if tail:
            lines.append(f"      {style.dim(tail)}")
    return "\n".join(lines)


def listing(title: str, rows: Sequence[tuple[str, str]], empty: str,
            kind: str = "") -> str:
    """rows are (ref, text)."""
    if not rows:
        return f"{heading(title)}\n{style.dim(empty)}"
    lines = [heading(title)]
    for ref, text in rows:
        shown = shorten(text)
        lines.append(f"{_pad(ref)} {style.kind(kind, shown) if kind else shown}")
    return "\n".join(lines)


def _highlight_terms(text: str, terms: set[str]) -> str:
    if not style.enabled or not terms:
        return text

    def mark(m: re.Match) -> str:
        return style.match(m.group(0)) if stem(m.group(0).lower()) in terms else m.group(0)

    return _WORD.sub(mark, text)


def search_results(query: str, hits: Sequence[Hit]) -> str:
    if not hits:
        return style.warn(f'Nothing found for "{query}".')
    terms = set(tokenize(query))
    lines = [f"{style.label('SEARCH:')} {style.bold(query)}", ""]
    for i, hit in enumerate(hits, 1):
        d = hit.doc
        title = _highlight_terms(shorten(d.title, 70), terms)
        extra = f" {style.dim('· ' + shorten(d.context, 40))}" if d.context else ""
        kind = style.kind(d.entity_type, hit.label, True)
        lines.append(f"{style.dim(f'{i}.')} {kind} {style.dim('—')} {title}{extra}  "
                     f"{style.ref(f'[{d.ref}]')}")
    return "\n".join(lines)


def shelf(data: dict[SourceType, dict[str, list]]) -> str:
    if not data:
        return (f"{style.dim('The shelf is empty. Add a source with:')}  "
                f"{style.command('book: Programming from the Ground Up')}")
    plural = {SourceType.BOOK: "BOOKS", SourceType.ARTICLE: "ARTICLES",
              SourceType.PAPER: "PAPERS", SourceType.VIDEO: "VIDEOS",
              SourceType.WEBSITE: "WEBSITES"}
    blocks = []
    for stype in SourceType:
        groups = data.get(stype)
        if not groups:
            continue
        lines = [style.heading(plural[stype])]
        names = sorted((g for g in groups if g != UNSORTED), key=str.lower)
        if UNSORTED in groups:
            names.append(UNSORTED)
        for group in names:
            lines += ["", style.label(group)] + [
                f"  {style.kind('source', s.title)}" for s in groups[group]]
        blocks.append("\n".join(lines))
    return "\n\n".join(blocks)


def sources_list(rows: Sequence[tuple[str, str, str]]) -> str:
    """rows are (ref, title, 'Type · Author')."""
    if not rows:
        return (f"{style.dim('No sources yet. Try:')}  "
                f"{style.command('book: Programming from the Ground Up')}")
    out = [heading("Sources")]
    for ref, title, meta in rows:
        out.append(f"{_pad(ref)} {style.kind('source', shorten(title, 60))}"
                   + (f"  {style.dim('—  ' + meta)}" if meta else ""))
    return "\n".join(out)


def graph(lines: Sequence[str]) -> str:
    """Colour a plain tree from KnowledgeService.tree()."""
    out = []
    for line in lines:
        m = re.match(r"^([│├└─ ]*)(←?)\s*([a-z_]+)\s*([→←])\s*(.*)$", line)
        if m:
            guide, _, rel, arrow, name = m.groups()
            out.append(f"{style.dim(guide)}{style.command(rel)} {style.dim(arrow)} "
                       f"{style.kind('concept', name)}")
        else:
            out.append(style.kind("concept", line, True) if line else line)
    return "\n".join(out)
