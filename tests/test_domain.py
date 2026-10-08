import pytest

from idios.domain.errors import Invalid
from idios.domain.models import QuestionStatus, SourceType


def test_ids_are_stable_and_independent_of_titles(app):
    concept, _ = app.knowledge.concept("CPU Register")
    assert concept.id.startswith("concept_") and "register" not in concept.id.lower()
    first = concept.id
    concept.name = "Register"
    app.store.concepts.update(concept)
    assert app.store.concepts.get(first).name == "Register"
    assert concept.created_at and concept.updated_at


def test_source_without_file_and_path_is_only_a_reference(app, tmp_path):
    bare = app.sources.add("Some Article", SourceType.ARTICLE, location="https://example.org/a")
    assert bare.url == "https://example.org/a" and bare.path is None
    book = app.sources.add("Programming from the Ground Up", location="'~/Shelf/Books/Assembly/book.pdf'")
    assert book.path == "~/Shelf/Books/Assembly/book.pdf"   # kept portable, quotes removed
    assert not list(app.home.rglob("*.pdf"))                  # never copied
    assert app.sources.add("Idea", SourceType.WEBSITE).path is None


def test_people_are_not_duplicated(app):
    a = app.sources.add("Book A", author="Jonathan Bartlett")
    b = app.sources.add("Book B", author="jonathan  bartlett")
    assert app.store.people.count() == 1
    assert app.sources.authors(a)[0].id == app.sources.authors(b)[0].id


def test_questions_open_then_answered(app):
    q = app.learning.ask("What is a register?")
    assert q.status is QuestionStatus.OPEN
    assert [x.id for x in app.learning.open_questions()] == [q.id]
    app.learning.answer(q, "Fast CPU-local storage")
    assert app.store.questions.get(q.id).status is QuestionStatus.ANSWERED
    assert app.learning.open_questions() == []
    assert [a.question_id for a in app.learning.answers_to(q)] == [q.id]


def test_unanswered_question_is_valid(app):
    app.learning.ask("Why?")
    assert len(app.learning.open_questions()) == 1


def test_highlight_and_quote_need_a_source(app):
    with pytest.raises(Invalid):
        app.learning.highlight("text")
    with pytest.raises(Invalid):
        app.learning.quote("text")


def test_records_carry_source_and_only_known_location(app):
    src = app.sources.add("Book")
    app.context.set_source(src)
    app.context.set_location("chapter", "2")
    h = app.learning.highlight("Registers are fast")
    assert (h.source_id, h.chapter, h.page) == (src.id, "2", None)
    assert h.location_label() == "Chapter 2"


def test_concepts_are_unique_case_insensitively(app):
    a, created_a = app.knowledge.concept("CPU Register")
    b, created_b = app.knowledge.concept("cpu  register")
    assert created_a and not created_b and a.id == b.id


def test_concept_can_exist_without_a_source(app):
    concept, _ = app.knowledge.concept("Memory")
    assert app.views.concept_view(concept).sources == []


def test_relations_are_typed_deduplicated_and_not_tags(app):
    rel, created, new = app.knowledge.link("CPU", "Register", "contains")
    assert created and sorted(new) == ["CPU", "Register"] and rel.type == "contains"
    _, again, _ = app.knowledge.link("CPU", "Register", "contains")
    assert not again
    _, other, _ = app.knowledge.link("CPU", "Register", "uses")
    assert other
    assert app.store.tags.count() == 0 and app.store.categories.count() == 0
    with pytest.raises(Invalid):
        app.knowledge.link("CPU", "Register", "banana")
    with pytest.raises(Invalid):
        app.knowledge.link("CPU", "cpu")


def test_tags_are_labels_not_relations(app):
    src = app.sources.add("Book")
    app.sources.tag(src, "assembly, #x86")
    assert app.sources.tags(src) == ["assembly", "x86"]
    assert app.store.links.count_relations() == 0


def test_deleting_a_source_removes_its_highlights_and_links(app):
    src = app.sources.add("Book")
    app.context.set_source(src)
    app.learning.highlight("x")
    c, _ = app.knowledge.concept("A")
    app.knowledge.link("A", "B")
    assert "1 highlight" in app.learning.impact(src)
    app.learning.delete(src)
    assert app.store.highlights.count() == 0
    assert app.context.ctx.source_id is None
    assert app.learning.impact(c) == "1 relation"
    app.learning.delete(c)
    assert app.store.links.count_relations() == 0


def test_display_numbers_are_never_reused(app):
    q1 = app.learning.ask("First?")
    app.learning.delete(q1)
    q2 = app.learning.ask("Second?")
    assert q2.seq > q1.seq
