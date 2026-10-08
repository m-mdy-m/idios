from idios.services.app import App


def test_context_survives_restart(home):
    a = App(home)
    g, _ = a.learning.goal("Learn Assembly")
    a.context.set_goal(g)
    a.context.set_source(a.sources.add("Programming from the Ground Up"))
    a.context.set_location("chapter", "2")
    a.close()

    b = App(home)
    assert b.context.goal.title == "Learn Assembly"
    assert b.context.source.title == "Programming from the Ground Up"
    assert b.context.ctx.location_label() == "Chapter 2"
    b.close()


def test_changing_source_clears_location_and_chapter_clears_page(app):
    s1, s2 = app.sources.add("One"), app.sources.add("Two")
    app.context.set_source(s1)
    app.context.set_location("chapter", "2")
    app.context.set_location("page", "14")
    app.context.set_location("chapter", "3")
    assert app.context.ctx.page is None and app.context.ctx.chapter == "3"
    app.context.set_source(s1)                      # same source keeps location
    assert app.context.ctx.chapter == "3"
    app.context.set_source(s2)
    assert app.context.ctx.location_label() == ""


def test_session_is_created_automatically_and_closed(app):
    session = app.context.start_session()
    app.learning.ask("Why?")
    app.context.end_session()
    stored = app.store.sessions.get(session.id)
    assert stored.ended_at and stored.activity_count == 1
    assert app.store.sessions.count() == 1


def test_forget_clears_deleted_entities(app):
    g, _ = app.learning.goal("X")
    app.context.set_goal(g)
    app.learning.delete(g)
    assert app.context.ctx.goal_id is None
