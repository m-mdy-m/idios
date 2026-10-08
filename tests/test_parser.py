import pytest

from idios.shell.parser import parse


@pytest.mark.parametrize("line, kind, text", [
    ("", "empty", ""),
    (":search register", "directive", "search"),
    ("quit", "directive", "quit"),
    ("note: Registers are fast", "note", "Registers are fast"),
    ("highlight: Registers can be accessed faster.", "highlight", "Registers can be accessed faster."),
    ("quote: \"Memory is slow\"", "quote", "\"Memory is slow\""),
    ("concept: CPU Register", "concept", "CPU Register"),
    ("goal: Learn Assembly", "goal", "Learn Assembly"),
    ("source: Programming from the Ground Up", "source", "Programming from the Ground Up"),
    ("book: Programming from the Ground Up", "new_source", "Programming from the Ground Up"),
    ("Why does the CPU need registers?", "question", "Why does the CPU need registers?"),
    ("registers are important", "statement", "registers are important"),
    ("Why registers matter in assembly", "maybe_question", "Why registers matter in assembly"),
    ("source add", "source_add", ""),
])
def test_kinds(line, kind, text):
    intent = parse(line)
    assert intent.kind == kind
    assert intent.text == text


def test_link_forms():
    i = parse("link CPU Register to Memory")
    assert (i.kind, i.text, i.arg, i.extra) == ("link", "CPU Register", "Memory", "related_to")
    i = parse("link CPU to Register as contains")
    assert (i.text, i.arg, i.extra) == ("CPU", "Register", "contains")


def test_location_words():
    for line, field, value in [("chapter 2", "chapter", "2"), ("page 14", "page", "14"),
                               ("section 2.3", "section", "2.3"), ("chapter: Registers", "chapter", "Registers")]:
        i = parse(line)
        assert (i.kind, i.text, i.arg) == ("location", field, value)


def test_ordinary_sentences_are_not_commands():
    assert parse("page numbers are annoying").kind == "statement"
    assert parse("chapter two is long").kind == "statement"


def test_bare_goal_and_source():
    i = parse("goal Learn Assembly")
    assert (i.kind, i.text, i.arg) == ("goal", "Learn Assembly", "bare")
    assert parse("source Programming from the Ground Up").kind == "source"


def test_answer_cues():
    assert parse("Because registers are inside the CPU.").extra == "answer"
    assert parse("A small storage location inside the CPU.").extra == "answer"
    assert parse("registers are important").extra == ""


def test_answer_with_reference():
    i = parse("answer q3: it is fast")
    assert (i.kind, i.text, i.arg) == ("answer", "it is fast", "q3")


def test_highlight_this_has_no_text():
    i = parse("highlight this")
    assert (i.kind, i.text, i.extra) == ("highlight", "", "this")
