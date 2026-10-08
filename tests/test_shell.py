from conftest import run_shell
from idios.services.app import App

MILESTONE = [
    "goal: Learn Assembly",
    "source: Programming from the Ground Up",
    "chapter 2",
    "What is a register?",
    "A small storage location inside the CPU.",
    "note: Registers are extremely fast.",
    "concept: CPU Register",
    ":questions",
    ":search register",
    ":quit",
]


def test_first_milestone_and_persistence(home):
    out = run_shell(home, MILESTONE)
    for expected in ("✓ Goal created", "✓ Source added and selected", "✓ Context updated",
                     "✓ Question saved", "✓ Answer saved", "✓ Note saved", "✓ Concept created",
                     "SEARCH: register", "Concept — CPU Register"):
        assert expected in out
    # everything survives a restart
    out2 = run_shell(home, [":status", "Whaat? is this?", ":search register", ":quit"])
    assert "Welcome back." in out2 and "Goal: Learn Assembly" in out2
    assert "Location: Chapter 2" in out2
    assert "Concept — CPU Register" in out2 and "Question — What is a register?" in out2


def test_answer_attaches_to_the_current_question(home):
    run_shell(home, ["source: Book", "Why are registers faster?", "Because they are inside the CPU."])
    app = App(home)
    q = app.store.questions.all()[0]
    assert q.status.value == "answered"
    assert app.learning.answers_to(q)[0].text.startswith("Because")
    app.close()


def test_context_is_applied_without_repeating_it(home):
    run_shell(home, ["source: Book", "chapter 2", "page 14", "highlight: Registers are fast", "quote: \"Memory is slow\""])
    app = App(home)
    h = app.store.highlights.all()[0]
    assert (h.chapter, h.page) == ("2", "14")
    assert app.store.quotes.all()[0].text == "Memory is slow"      # quote marks stripped
    app.close()


def test_ambiguous_statement_asks_and_never_guesses(home):
    out = run_shell(home, ["registers are important", "3"])
    assert "What should I save this as?" in out and "Ignored." in out
    app = App(home)
    assert app.store.notes.count() == 0 and app.store.answers.count() == 0
    app.close()


def test_ambiguous_statement_can_become_a_note(home):
    run_shell(home, ["registers are important", "1"])
    app = App(home)
    assert [n.text for n in app.store.notes.all()] == ["registers are important"]
    app.close()


def test_menu_offers_answer_only_when_a_question_is_open(home):
    out = run_shell(home, ["Why is memory slower than registers?", "registers are important", "2"])
    assert "2. Answer to current question" in out
    app = App(home)
    assert app.store.answers.count() == 1
    app.close()


def test_question_without_mark_asks_first(home):
    out = run_shell(home, ["Why registers matter in assembly", "1"])
    assert "1. Question" in out
    app = App(home)
    assert app.store.questions.count() == 1
    app.close()


def test_invalid_menu_choices_fall_back_to_ignore(home):
    out = run_shell(home, ["registers are important", "x", "y", "z"])
    assert out.count("Please choose") == 3
    app = App(home)
    assert app.store.notes.count() == 0
    app.close()


def test_link_creates_relation_and_concepts(home):
    out = run_shell(home, ["concept: CPU Register", "link CPU Register to Memory", "link CPU Register to Memory", ":graph CPU Register"])
    assert "✓ Relation created" in out and "(new concept: Memory)" in out
    assert "Already linked." in out
    assert "related_to → Memory" in out


def test_errors_are_human_readable(home):
    out = run_shell(home, ["highlight: nothing selected", ":nope", ":show zzz", "link A to B as banana"])
    assert "no source is selected" in out
    assert "I don't know :nope." in out
    assert "Could not find that." in out and "Try:" in out
    assert "banana" in out
    assert "Traceback" not in out and "Error" not in out


def test_directives_list_things(home):
    out = run_shell(home, [
        "goal: Learn Assembly", "book: Programming from the Ground Up",
        "author: Jonathan Bartlett", "category: Assembly", "tag: cpu, low-level",
        "highlight: Registers are fast", "quote: Hello", "note: n", "concept: CPU Register",
        "Why?", ":sources", ":concepts", ":highlights", ":quotes", ":notes", ":goals",
        ":shelf", ":show Programming from the Ground Up", ":questions", ":help",
    ])
    assert "Jonathan Bartlett" in out
    assert "BOOKS" in out and "Assembly\n  Programming from the Ground Up" in out
    assert "Highlights:\n  1" in out and "Tags:\n  cpu, low-level" in out
    assert "Open Questions" in out and "Why?" in out
    assert "Exit\n  :quit" in out


def test_show_concept_lists_sources_questions_and_relations(home):
    out = run_shell(home, ["source: Book", "Why are registers faster?", "concept: CPU Register",
                           "link CPU Register to Memory", ":show CPU Register"])
    view = out[out.rindex("CPU Register\n\nSources"):]
    assert "Book" in view and "Why are registers faster?  (open)" in view
    assert "Related Concepts\n  Memory" in view


def test_delete_asks_first(home):
    run_shell(home, ["Why?", ":delete q1", "n"])
    app = App(home)
    assert app.store.questions.count() == 1
    app.close()
    out = run_shell(home, [":delete q1", "y"])
    assert "Delete Question q1? [y/N]" in out and "✓ Deleted Question q1" in out
    app = App(home)
    assert app.store.questions.count() == 0
    app.close()


def test_resume_can_be_declined_in_interactive_mode(home):
    run_shell(home, ["goal: Learn Assembly"])
    out = run_shell(home, ["n", ":status"], interactive=True)
    assert "Continue? [Y/n]" in out
    assert "Goal:\n  (none)" in out


def test_interactive_source_registration_asks_for_details(home):
    run_shell(home, ["", "source add", "Programming from the Ground Up", "Jonathan Bartlett", "book",
                     "~/Shelf/Books/Assembly/pgu.pdf", "Assembly"], interactive=True)
    app = App(home)
    s = app.store.sources.all()[0]
    assert s.path == "~/Shelf/Books/Assembly/pgu.pdf"
    assert app.sources.category_name(s) == "Assembly"
    assert app.sources.authors(s)[0].name == "Jonathan Bartlett"
    app.close()


def test_new_book_follow_ups_only_when_interactive(home):
    run_shell(home, ["book: Piped Book", "chapter 1"])
    app = App(home)
    assert app.context.ctx.chapter == "1"       # the 2nd line was a command, not an answer
    app.close()


def test_blank_input_and_eof_are_harmless(home):
    out = run_shell(home, ["", "   "])
    assert "Saved. See you next time." in out
