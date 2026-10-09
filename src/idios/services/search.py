"""Local search: BM25 over everything the user has recorded.

The index is rebuilt from the database for each query. A personal knowledge
base is small, so this stays fast and can never go stale.
"""
from __future__ import annotations

import math
import re
from collections import Counter
from dataclasses import dataclass
from typing import Optional

from idios.domain.models import ENTITY_TYPES
from idios.storage.repos import Store

K1 = 1.5
B = 0.75
_STOP = frozenset(
    "a an the is it in of to and or for on at by with from as be was are this that "
    "do does did not but if so i we you they".split()
)
_WORD = re.compile(r"\w+", re.UNICODE)

#: a tie between two results is broken by this order (most useful first)
_TYPE_ORDER = ["concept", "question", "answer", "note", "highlight", "quote",
               "source", "goal", "topic", "person", "tag", "category"]


def stem(word: str) -> str:
    """A deliberately tiny stemmer: registers/register, queries/query."""
    if len(word) > 4 and word.endswith("ies"):
        return word[:-3] + "y"
    if len(word) > 4 and word.endswith("es") and word[-3] in "sxz":
        return word[:-2]
    if len(word) > 3 and word.endswith("s") and not word.endswith("ss"):
        return word[:-1]
    return word


def tokenize(text: str) -> list[str]:
    words = (w.lower() for w in _WORD.findall(text))
    return [stem(w) for w in words if w not in _STOP and len(w) > 1]


@dataclass
class Doc:
    entity_type: str
    entity_id: str
    ref: str
    title: str            # what to show for the hit
    body: str = ""        # extra searchable text (not shown)
    context: str = ""     # e.g. the source title, shown after the hit


@dataclass
class Hit:
    doc: Doc
    score: float

    @property
    def label(self) -> str:
        return ENTITY_TYPES[self.doc.entity_type][3]


class SearchService:
    def __init__(self, store: Store) -> None:
        self.store = store

    def search(self, query: str, limit: int = 15,
               entity_type: Optional[str] = None) -> list[Hit]:
        terms = tokenize(query)
        if not terms:
            return []
        docs = [d for d in self._documents()
                if entity_type is None or d.entity_type == entity_type]
        if not docs:
            return []
        tokens = [tokenize(d.title) * 2 + tokenize(d.body) for d in docs]  # titles count double
        n = len(docs)
        avg = (sum(len(t) for t in tokens) / n) or 1.0
        df: Counter[str] = Counter()
        for toks in tokens:
            df.update(set(toks))
        hits = []
        for doc, toks in zip(docs, tokens):
            if not toks:
                continue
            freq = Counter(toks)
            score = 0.0
            for term in set(terms):
                tf = freq.get(term, 0)
                if not tf:
                    continue
                idf = math.log(1 + (n - df[term] + 0.5) / (df[term] + 0.5))
                score += idf * tf * (K1 + 1) / (tf + K1 * (1 - B + B * len(toks) / avg))
            if score > 0:
                hits.append(Hit(doc, score))
        order = {t: i for i, t in enumerate(_TYPE_ORDER)}
        hits.sort(key=lambda h: (-round(h.score, 9), order.get(h.doc.entity_type, 99),
                                 h.doc.ref))
        return hits[:limit]

    # -- documents ---------------------------------------------------------
    def _documents(self) -> list[Doc]:
        s = self.store
        links = s.links
        source_title = {x.id: x.title for x in s.sources.all()}
        docs: list[Doc] = []

        def tag_text(etype: str, eid: str) -> str:
            tags = (s.tags.get(i) for i in links.tag_ids(etype, eid))
            return " ".join(t.name for t in tags if t)

        for src in s.sources.all():
            authors = " ".join(p.name for p in
                               (s.people.get(i) for i in links.author_ids(src.id)) if p)
            cat = s.categories.get(src.category_id) if src.category_id else None
            body = " ".join(filter(None, [authors, src.description, src.type.value,
                                          cat.name if cat else "",
                                          tag_text("source", src.id)]))
            docs.append(Doc("source", src.id, src.ref, src.title, body))
        for q in s.questions.all():
            docs.append(Doc("question", q.id, q.ref, q.text,
                            context=source_title.get(q.source_id or "", "")))
        question_text = {q.id: q.text for q in s.questions.all()}
        for a in s.answers.all():
            docs.append(Doc("answer", a.id, a.ref, a.text,
                            context=question_text.get(a.question_id, "")))
        for n in s.notes.all():
            docs.append(Doc("note", n.id, n.ref, n.text, tag_text("note", n.id)))
        for h in s.highlights.all():
            docs.append(Doc("highlight", h.id, h.ref, h.text,
                            context=source_title.get(h.source_id, "")))
        for qu in s.quotes.all():
            docs.append(Doc("quote", qu.id, qu.ref, qu.text,
                            context=source_title.get(qu.source_id, "")))
        for c in s.concepts.all():
            docs.append(Doc("concept", c.id, c.ref, c.name,
                            " ".join(filter(None, [c.description,
                                                   tag_text("concept", c.id)]))))
        for g in s.goals.all():
            docs.append(Doc("goal", g.id, g.ref, g.title, g.description or ""))
        for p in s.people.all():
            docs.append(Doc("person", p.id, p.ref, p.name))
        for t in s.tags.all():
            docs.append(Doc("tag", t.id, t.ref, t.name))
        for t in s.topics.all():
            docs.append(Doc("topic", t.id, t.ref, t.name))
        return docs
