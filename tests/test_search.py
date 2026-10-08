from idios.services.search import stem, tokenize


def seeded(app):
    src = app.sources.add("Programming from the Ground Up", author="Jonathan Bartlett")
    app.context.set_source(src)
    app.knowledge.concept("CPU Register")
    q = app.learning.ask("Why does the CPU need registers?")
    app.learning.answer(q, "Because registers provide very fast CPU-local storage.")
    app.learning.note("Memory is much slower than the CPU")
    app.learning.highlight("Registers can be accessed much faster than memory.")
    app.learning.quote("Simplicity is prerequisite for reliability")
    return src


def test_stemming_is_small_and_predictable():
    assert stem("registers") == stem("register")
    assert stem("queries") == "query"
    assert stem("class") == "class"
    assert tokenize("The CPU needs registers!") == ["cpu", "need", "register"]


def test_search_spans_entity_types_and_ranks(app):
    seeded(app)
    hits = app.search.search("register")
    kinds = {h.label for h in hits}
    assert {"Concept", "Question", "Answer", "Highlight"} <= kinds
    assert hits[0].label == "Concept"            # short, exact title wins
    scores = [h.score for h in hits]
    assert scores == sorted(scores, reverse=True)


def test_search_finds_sources_authors_and_quotes(app):
    seeded(app)
    assert {h.label for h in app.search.search("bartlett")} == {"Person", "Source"}
    assert app.search.search("simplicity")[0].label == "Quote"
    assert app.search.search("assembly zebra") == []


def test_search_is_deterministic_and_filterable(app):
    seeded(app)
    first = [h.doc.entity_id for h in app.search.search("cpu")]
    assert first == [h.doc.entity_id for h in app.search.search("cpu")]
    only_notes = app.search.search("cpu", entity_type="note")
    assert only_notes and {h.label for h in only_notes} == {"Note"}


def test_empty_and_stopword_queries(app):
    seeded(app)
    assert app.search.search("") == []
    assert app.search.search("the of") == []
