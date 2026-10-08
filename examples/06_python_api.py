"""Use IDIOS from Python instead of the shell.

Run:  python examples/06_python_api.py
The same services power the shell, so everything here behaves identically.
"""
import tempfile
from pathlib import Path

from idios.services.app import App

with tempfile.TemporaryDirectory() as tmp:
    app = App(Path(tmp))                     # normally App() uses ~/.idios

    goal, _ = app.learning.goal("Learn Assembly")
    app.context.set_goal(goal)
    book = app.sources.add("Programming from the Ground Up",
                           author="Jonathan Bartlett",
                           location="~/Shelf/Books/Assembly/pgu.pdf",
                           category="Assembly")
    app.context.set_source(book)
    app.context.set_location("chapter", "2")

    question = app.learning.ask("What is a register?")
    app.learning.answer(question, "A small storage location inside the CPU.")
    app.learning.highlight("Registers can be accessed much faster than memory.")
    concept, _ = app.knowledge.concept("CPU Register")
    app.knowledge.link("CPU Register", "Memory", "related_to")

    print("Search 'register':")
    for hit in app.search.search("register"):
        print(f"  {hit.label:<10} {hit.doc.title}")

    view = app.views.concept_view(concept)
    print(f"\n{view.name}: sources={view.sources} related={view.related}")
    print("Shelf:", {t.value: list(g) for t, g in app.sources.shelf().items()})
    app.close()
