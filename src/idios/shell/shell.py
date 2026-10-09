"""The interactive learning environment.

The shell is a thin interpreter: parse a line, call an application service,
print a short confirmation. Business rules live in the services; text layout
lives in ``render``.
"""
from __future__ import annotations

import os
import re
from datetime import date, timedelta
from typing import Callable, Optional

from idios.domain.errors import Invalid, IdiosError
from idios.domain.models import Entity, SourceType
from idios.services.app import App
from idios.services.plan import Review
from idios.services.lookup import lookup
from idios.shell import intro, render
from idios.shell.io import IO
from idios.services.dates import label as day_label, parse_when
from idios.shell.parser import Intent, parse
from idios.shell.style import style

_QUOTE_CHARS = "\"'“”‘’«»"


class Shell:
    def __init__(self, app: App, io: IO, interactive: bool = True) -> None:
        """``interactive`` is False when input is piped or scripted. Optional
        follow-up questions (author, path, resume) are then skipped so they
        never swallow the next command."""
        self.app = app
        self.io = io
        self.interactive = interactive
        self._directives: dict[str, Callable[[str], None]] = {
            "help": self._help, "h": self._help, "?": self._help,
            "status": self._status,
            "search": self._search, "find": self._search,
            "sources": self._sources,
            "questions": self._questions,
            "concepts": self._concepts,
            "highlights": lambda _: self._items("highlight", "Highlights"),
            "quotes": lambda _: self._items("quote", "Quotes"),
            "notes": lambda _: self._items("note", "Notes"),
            "goals": self._goals,
            "graph": self._graph,
            "shelf": self._shelf,
            "show": self._show,
            "delete": self._delete,
            "clear": self._clear_screen, "cls": self._clear_screen,
            "forget": self._forget,
            "plan": self._plan_view, "review": lambda _: self._review(),
            "done": lambda a: self._task_mark(Intent("task_mark", a, "done")),
            "skip": lambda a: self._task_mark(Intent("task_mark", a, "skip")),
            "schedule": self._schedule,
            "intro": self._intro,
        }
        self._handlers: dict[str, Callable[[Intent], None]] = {
            "question": self._question, "maybe_question": self._maybe_question,
            "statement": self._statement, "answer": self._answer,
            "note": self._note, "highlight": self._highlight, "quote": self._quote,
            "concept": self._concept, "topic": self._topic,
            "category": self._category, "tag": self._tag, "author": self._author, "reference": self._reference,
            "did": self._did, "plan": self._plan, "task_mark": self._task_mark,
            "goal": self._goal, "source": self._source, "new_source": self._new_source,
            "source_add": self._source_add, "location": self._location,
            "link": self._link,
        }

    # ------------------------------------------------------------------ loop
    def run(self) -> None:
        ctx = self.app.context
        self._first_run_intro()
        if not ctx.ctx.is_empty():
            goal, source = ctx.goal, ctx.source
            self.say(render.welcome_back(goal.title if goal else None,
                                         source.title if source else None,
                                         ctx.ctx.location_label()))
            if self.interactive and not self._confirm("Continue?", default=True):
                ctx.reset()
        self.say(self._header())
        ctx.start_session()
        if self.interactive:
            self._plan_checkin()
        while True:
            try:
                line = self.io.read(render.prompt())
            except KeyboardInterrupt:
                self.say("\n(:quit to leave)")
                continue
            if line is None or not self.handle(line):
                break
        self._goodbye_hint()
        self.say(style.dim("Saved. See you next time."))

    def _first_run_intro(self) -> None:
        """Interactive starts show the logo; a brand-new user gets the full welcome once.

        Set IDIOS_NO_LOGO=1 to skip the logo on later starts.
        """
        flags = self.app.store.context
        first = not flags.flag("intro_seen")
        if first:
            flags.set_flag("intro_seen")
        if not self.interactive:
            return  # scripts and pipes never see it
        if first and self.app.context.ctx.is_empty():
            self.io.clear()
            self.say(intro.banner())
            self.io.read(style.dim("  Press Enter to begin… "))
            self.io.clear()
        elif not os.environ.get("IDIOS_NO_LOGO"):
            self.say(intro.logo())

    def handle(self, line: str) -> bool:
        """Process one line. Returns False when the user asked to quit."""
        intent = parse(line)
        try:
            if intent.kind == "empty":
                return True
            if intent.kind == "directive":
                if intent.text in ("quit", "exit", "q"):
                    return False
                action = self._directives.get(intent.text)
                if action is None:
                    raise Invalid(f"I don't know :{intent.text}.", hint=":help")
                action(intent.arg)
            else:
                self._handlers[intent.kind](intent)
        except IdiosError as err:
            self.io.write(render.error(err.message, err.hint))
        except Exception:  # never dump internals on the learner
            if os.environ.get("IDIOS_DEBUG"):
                raise
            self.io.write(render.error("Something went wrong and that was not saved.\n"
                                       "Your other data is safe. (IDIOS_DEBUG=1 shows details.)"))
        return True

    # ------------------------------------------------------------- learning
    def _question(self, i: Intent) -> None:
        q = self.app.learning.ask(i.text)
        self.say(f"{render.OK} Question saved · {q.ref}")

    def _answer(self, i: Intent) -> None:
        question = self.app.context.question
        if i.arg:
            target = lookup(self.app.store, i.arg)
            if target is None or target.entity_type != "question":
                raise Invalid(f"I couldn't find a question called {i.arg}.", hint=":questions")
            question = target  # type: ignore[assignment]
        if question is None:
            raise Invalid("There is no question to answer yet.",
                          hint="ask one first, or:  answer q3: <your answer>")
        ans = self.app.learning.answer(question, i.text)
        self.app.context.set_question(question)
        self.say(f"{render.OK} Answer saved · {ans.ref}\n  to: {render.shorten(question.text, 70)}")

    def _note(self, i: Intent) -> None:
        n = self.app.learning.note(i.text)
        self.say(f"{render.OK} Note saved · {n.ref}")

    def _highlight(self, i: Intent) -> None:
        h = self.app.learning.highlight(self._text_or_ask(i, "highlight"))
        self.say(f"{render.OK} Highlight saved · {h.ref}")

    def _quote(self, i: Intent) -> None:
        text = self._text_or_ask(i, "quote").strip().strip(_QUOTE_CHARS).strip()
        q = self.app.learning.quote(text)
        self.say(f"{render.OK} Quote saved · {q.ref}")

    def _text_or_ask(self, i: Intent, what: str) -> str:
        if i.text:
            return i.text
        if self.interactive:
            return (self.io.read("Text: ") or "").strip()
        raise Invalid(f"What should the {what} say?", hint=f"{what}: <text from the source>")

    # ---------------------------------------------------- ambiguous input
    def _statement(self, i: Intent) -> None:
        question = self.app.context.question
        open_question = question if question and question.status.value == "open" else None
        if i.extra == "answer" and open_question:
            self._answer(Intent("answer", i.text))
            return
        options = ["note"] + (["answer"] if open_question else []) + ["ignore"]
        choice = self._choose(options)
        if choice == "note":
            self._note(Intent("note", i.text))
        elif choice == "answer":
            self._answer(Intent("answer", i.text))
        else:
            self.say("Ignored.")

    def _maybe_question(self, i: Intent) -> None:
        choice = self._choose(["question", "note", "ignore"])
        if choice == "question":
            self._question(i)
        elif choice == "note":
            self._note(Intent("note", i.text))
        else:
            self.say("Ignored.")

    def _choose(self, options: list[str]) -> str:
        labels = {"note": "Note", "answer": "Answer to current question",
                  "question": "Question", "ignore": "Ignore"}
        self.say(style.bold("What should I save this as?") + "\n")
        for n, opt in enumerate(options, 1):
            self.say(f"{style.dim(f'{n}.')} {labels[opt]}")
        for _ in range(3):
            reply = self.io.read(render.prompt())
            if reply is None:
                break
            reply = reply.strip().lower()
            if reply.isdigit() and 1 <= int(reply) <= len(options):
                return options[int(reply) - 1]
            if reply in options:
                return reply
            self.say(f"Please choose 1-{len(options)}.")
        return "ignore"

    # -------------------------------------------------------------- context
    def _goal(self, i: Intent) -> None:
        goal = self.app.learning.find_goal(i.text) if i.arg == "bare" else None
        created = False
        if goal is None:
            if i.arg == "bare" and self.interactive and not self._confirm(
                    f'Create the goal "{i.text}"?', default=True):
                self.say("Cancelled.")
                return
            goal, created = self.app.learning.goal(i.text)
        self.app.context.set_goal(goal)
        self.say(f"{render.OK} Goal {'created' if created else 'selected'}")

    def _source(self, i: Intent) -> None:
        self._select_or_add(i.text, SourceType.BOOK, confirm=i.arg == "bare")

    def _new_source(self, i: Intent) -> None:
        self._select_or_add(i.text, SourceType(i.arg), confirm=False)

    def _select_or_add(self, title: str, stype: SourceType, confirm: bool) -> None:
        if not title.strip():
            raise Invalid("Which source?", hint="source: Programming from the Ground Up")
        source = self.app.sources.find(title)
        if source is not None:
            self.app.context.set_source(source)
            self.say(f"{render.OK} Source selected")
            return
        if confirm and self.interactive and not self._confirm(
                f'"{title}" is not registered. Add it as a {stype.value}?', default=True):
            self.say("Cancelled.")
            return
        source = self.app.sources.add(title, stype)
        self.app.context.set_source(source)
        self.say(f"{render.OK} Source added and selected")
        if self.interactive:
            self._source_details(source)

    def _source_details(self, source) -> None:
        """Optional follow-ups after registering a source; Enter skips each."""
        author = (self.io.read("Author (Enter to skip): ") or "").strip()
        if author:
            self.app.sources.add_authors(source, author)
        where = (self.io.read("Path / URL (Enter to skip): ") or "").strip()
        if where:
            self.app.sources.set_location(source, where)

    def _source_add(self, i: Intent) -> None:
        if not self.interactive:
            raise Invalid("`source add` asks questions, so it needs a terminal.",
                          hint="book: <title>")
        title = (self.io.read("Title: ") or "").strip()
        if not title:
            self.say("Cancelled.")
            return
        author = (self.io.read("Author: ") or "").strip()
        kinds = "/".join(t.value for t in SourceType)
        raw_type = (self.io.read(f"Type ({kinds}) [book]: ") or "").strip().lower() or "book"
        if raw_type not in {t.value for t in SourceType}:
            raise Invalid(f"'{raw_type}' is not a source type.", hint=f"one of: {kinds}")
        where = (self.io.read("Path / URL: ") or "").strip()
        category = (self.io.read("Shelf (e.g. Assembly): ") or "").strip()
        source = self.app.sources.add(title, SourceType(raw_type), author or None,
                                      where or None, category or None)
        self.app.context.set_source(source)
        self.say(f"{render.OK} Source added and selected")

    def _location(self, i: Intent) -> None:
        self.app.context.set_location(i.text, i.arg)
        self.say(f"{render.OK} Context updated")

    # ------------------------------------------------------------ knowledge
    def _concept(self, i: Intent) -> None:
        concept, created = self.app.knowledge.concept(i.text)
        adopted = self.app.knowledge.adopt_question(concept)
        self.app.context.set_concept(concept)
        extra = f"\n  linked to: {render.shorten(adopted, 70)}" if adopted else ""
        self.say(f"{render.OK} Concept {'created' if created else 'selected'}{extra}")

    def _topic(self, i: Intent) -> None:
        topic, created = self.app.knowledge.topic(i.text)
        concept = self.app.context.concept
        note = ""
        if concept:
            self.app.knowledge.file_under_topic(concept, topic)
            note = f"\n  filed {concept.name} under it"
        self.say(f"{render.OK} Topic {'created' if created else 'selected'}{note}")

    def _need_source(self, what: str):
        source = self.app.context.source
        if source is None:
            raise Invalid(f"Which source is this {what} for? None is selected.",
                          hint="source: <title>")
        return source

    def _category(self, i: Intent) -> None:
        source = self._need_source("shelf")
        self.app.sources.set_category(source, i.text)
        self.say(f"{render.OK} Shelved under {i.text.strip()}")

    def _tag(self, i: Intent) -> None:
        source = self._need_source("tag")
        names = self.app.sources.tag(source, i.text)
        self.say(f"{render.OK} Tagged: {', '.join(names)}")

    def _author(self, i: Intent) -> None:
        source = self._need_source("author")
        people = self.app.sources.add_authors(source, i.text)
        self.say(f"{render.OK} Author added: {', '.join(p.name for p in people)}")

    # ------------------------------------------------------------- planning
    def _did(self, i: Intent) -> None:
        plan = self.app.plan
        day = parse_when(i.arg, plan.today()) if i.arg else None
        task = plan.did(i.text, day)
        self.say(f"{render.OK} Done saved · {task.ref}\n  {day_label(date.fromisoformat(task.due_date), plan.today())}")

    def _plan(self, i: Intent) -> None:
        plan = self.app.plan
        day = parse_when(i.arg, plan.today()) if i.arg else None
        task = plan.add(i.text, day)
        self.say(f"{render.OK} Planned · {task.ref}\n  "
                 f"{day_label(date.fromisoformat(task.due_date), plan.today())}")

    def _plan_view(self, _: str) -> None:
        plan = self.app.plan
        view = plan.view()
        self.say(render.plan_view(view.overdue, view.days, plan.today(), plan.review_time))

    def _task_mark(self, i: Intent) -> None:
        refs = [r for r in re.split(r"[\s,]+", i.text) if r]
        if not refs:
            raise Invalid("Which task?", hint="done tk1 tk2      skip tk3      (see :plan)")
        tasks = []
        for ref in refs:  # resolve all first so a typo changes nothing
            found = lookup(self.app.store, ref)
            if found is None or found.entity_type != "task":
                raise Invalid(f"I couldn't find a task called {ref}.", hint=":plan")
            tasks.append(found)
        for task in tasks:
            (self.app.plan.complete if i.arg == "done" else self.app.plan.skip)(task)  # type: ignore[arg-type]
        word = "done" if i.arg == "done" else "skipped"
        self.say("\n".join(f"{render.OK} Marked {word}: {render.shorten(t.text, 60)}" for t in tasks))  # type: ignore[attr-defined]

    def _review(self) -> None:
        """Go through what is still open for today and settle each item."""
        plan = self.app.plan
        tasks = plan.open_now()
        if not tasks:
            self.say(style.dim("Nothing open for today.") + f"  Done so far: {len(plan.done_today())}.")
            return
        self.say(style.heading("Review") + style.dim(f"  {len(tasks)} open"))
        done, carried, skipped = [], [], []
        tomorrow = plan.today() + timedelta(days=1)
        for task in tasks:
            answer = self._ask_task(task)
            if answer is None:
                break  # input ended: leave the rest untouched
            if answer == "l":
                continue
            if answer == "y":
                plan.complete(task); done.append(task)
            elif answer == "n":
                plan.move(task, tomorrow); carried.append(task)
            else:
                plan.skip(task); skipped.append(task)
        review = Review(done, carried, skipped)
        if review.total == 0:
            return
        plan.save_review(review)
        self.say(render.review_summary(done, carried, skipped, plan.tomorrow_count()))
        self.say(f"\n{render.OK} Review saved as a note")

    def _ask_task(self, task) -> Optional[str]:
        self.say(f"\n{render.task_row(task, show_day=True)}")
        for _ in range(3):
            reply = self.io.read(style.dim("  done? ") + style.command("[y]") + style.dim("es · ")
                                 + style.command("[n]") + style.dim("ot yet → tomorrow · ")
                                 + style.command("[s]") + style.dim("kip: "))
            if reply is None:
                return None
            r = reply.strip().lower()
            if r in ("y", "yes", "d", "done"):
                return "y"
            if r in ("n", "no", "not yet", "later", "t", "tomorrow"):
                return "n"
            if r in ("s", "skip"):
                return "s"
            self.say("Please answer y, n or s.")
        return "l"  # no usable answer: leave it open and move on

    def _schedule(self, arg: str) -> None:
        plan, sched, word = self.app.plan, self.app.scheduler, arg.strip().lower()
        command = self.app.reminder_command()
        if word in ("install", "on"):
            line = sched.preview(plan.review_time, command)
            if not self._confirm(f"Add a daily job at {plan.review_time}?\n  {line}\n", default=False):
                self.say("Nothing was installed.")
                return
            self.say(f"{render.OK} Reminder on · {sched.install(plan.review_time, command)}")
        elif word in ("remove", "off"):
            removed = sched.remove()
            self.say(f"{render.OK} Reminder removed" if removed else "No reminder was installed.")
        elif word == "test":
            if self.app.notify("IDIOS", "This is how your daily reminder will look."):
                self.say(f"{render.OK} Notification sent")
            else:
                self.say("This computer has no notification tool, so IDIOS will print the reminder instead.")
        elif word:
            at = plan.set_review_time(word)
            note = "  (run :schedule install again to move the job)" if sched.status().installed else ""
            self.say(f"{render.OK} Review time set to {at}{note}")
        else:
            st = sched.status()
            self.say(render.schedule_status(plan.review_time, st.installed, st.time or "", st.detail,
                                            sched.preview(plan.review_time, command)))

    def _plan_checkin(self) -> None:
        """On launch: offer a review when tasks are overdue or the day's review time has passed."""
        plan = self.app.plan
        due = plan.checkin_due()
        if due:
            n = len(due)
            self.say(style.warn(f"Plan check-in: {n} thing{'s' if n != 1 else ''} still open."))
            for t in due[:5]:
                self.say("  " + render.task_row(t, show_day=True))
            if self._confirm("Review now?", default=True):
                self._review()
                self.say("")
        elif plan.open_count_today():
            self.say(style.dim(f"Today's plan: {plan.open_count_today()} open · :plan"))

    def _goodbye_hint(self) -> None:
        """Once, after planning tomorrow: mention the reminder if it is not set up."""
        store = self.app.store.context
        if not self.interactive or store.flag("schedule_hint") or not self.app.plan.tomorrow_count():
            return
        if self.app.scheduler.status().installed:
            return
        store.set_flag("schedule_hint")
        self.say(style.dim(f"Tomorrow: {self.app.plan.tomorrow_count()} planned. Want a reminder at "
                           f"{self.app.plan.review_time}? ") + style.command(":schedule"))

    def _reference(self, i: Intent) -> None:
        source = self._need_source("reference")
        if not i.text.strip():
            raise Invalid("Where is it?", hint="url: https://...   or   path: ~/Shelf/Books/book.pdf")
        self.app.sources.set_location(source, i.text)
        self.say(f"{render.OK} Reference saved")

    def _link(self, i: Intent) -> None:
        rel, created, new = self.app.knowledge.link(i.text, i.arg, i.extra)
        if not created:
            self.say("Already linked.")
            return
        lines = [f"{render.OK} Relation created"]
        k = self.app.knowledge
        lines.append(f"  {k.entity_name(rel.source_type, rel.source_id)} "
                     f"—{rel.type}→ {k.entity_name(rel.target_type, rel.target_id)}")
        if new:
            lines.append(f"  (new concept: {', '.join(new)})")
        self.say("\n".join(lines))

    # ----------------------------------------------------------- directives
    def _help(self, topic: str) -> None:
        self.say(render.help_overview(topic))

    def _status(self, _: str) -> None:
        self.say(render.status(self.app.views.status()))

    def _search(self, query: str) -> None:
        if not query:
            raise Invalid("Search for what?", hint=":search register")
        self.say(render.search_results(query, self.app.search.search(query)))

    def _sources(self, _: str) -> None:
        rows = []
        for s in self.app.store.sources.find(order="lower(title)"):
            authors = ", ".join(p.name for p in self.app.sources.authors(s))
            meta = " · ".join(x for x in (s.type.value.capitalize(), authors) if x)
            rows.append((s.ref, s.title, meta))
        self.say(render.sources_list(rows))

    def _questions(self, arg: str) -> None:
        store = self.app.store
        show_all = arg.strip().lower() == "all"
        questions = store.questions.all() if show_all else self.app.learning.open_questions()
        titles = {s.id: s.title for s in store.sources.all()}
        self.say(render.question_list(questions, titles,
                                      "Questions" if show_all else "Open Questions"))

    def _concepts(self, _: str) -> None:
        rows = [(c.ref, c.name) for c in self.app.store.concepts.find(order="lower(name)")]
        self.say(render.listing("Concepts", rows, "None yet. Try:  concept: CPU Register", "concept"))

    def _goals(self, _: str) -> None:
        rows = [(g.ref, g.title) for g in self.app.store.goals.all()]
        self.say(render.listing("Goals", rows, "None yet. Try:  goal: Learn Assembly", "goal"))

    def _items(self, etype: str, title: str) -> None:
        store = self.app.store
        titles = {s.id: s.title for s in store.sources.all()}
        rows = []
        for item in store.by_type[etype].all():
            text = item.text  # type: ignore[attr-defined]
            source = titles.get(getattr(item, "source_id", None) or "")
            rows.append((item.ref, f"{text}" + (f"  — {source}" if source else "")))
        self.say(render.listing(title, rows, "None yet.", etype))

    def _graph(self, arg: str) -> None:
        root = None
        if arg:
            root = self.app.store.concepts.by_name(arg) or (lookup(self.app.store, arg))
            if root is None or root.entity_type != "concept":
                raise Invalid(f"I couldn't find a concept called {arg}.", hint=":concepts")
        else:
            root = self.app.context.concept
        lines = self.app.knowledge.tree(root)  # type: ignore[arg-type]
        self.say(render.graph(lines) if lines else
                 style.dim("No relations yet. Try:  link CPU Register to Memory"))

    def _shelf(self, _: str) -> None:
        self.say(render.shelf(self.app.sources.shelf()))

    def _show(self, arg: str) -> None:
        entity = self._find(arg)
        views, etype = self.app.views, entity.entity_type
        if etype == "source":
            self.say(render.source_view(views.source_view(entity)))  # type: ignore[arg-type]
        elif etype == "concept":
            self.say(render.concept_view(views.concept_view(entity)))  # type: ignore[arg-type]
        elif etype == "question":
            self.say(render.question_view(views.question_view(entity)))  # type: ignore[arg-type]
        elif etype == "goal":
            self.say(render.goal_view(views.goal_view(entity)))  # type: ignore[arg-type]
        else:
            text = getattr(entity, "text", None) or getattr(entity, "name", "")
            where = entity.location_label() if hasattr(entity, "location_label") else ""
            self.say(text + (f"\n({where})" if where else ""))

    def _delete(self, arg: str) -> None:
        entity = self._find(arg)
        if entity.entity_type == "session":
            raise Invalid("Sessions are managed automatically.")
        label = f"{entity.entity_type.capitalize()} {entity.ref}"
        impact = self.app.learning.impact(entity)
        prompt = f"Delete {label}?" + (f" This also removes {impact}." if impact else "")
        if not self._confirm(prompt, default=False):
            self.say("Nothing was deleted.")
            return
        self.app.learning.delete(entity)
        self.say(f"{render.OK} Deleted {label}")

    def _forget(self, _: str) -> None:
        self.app.context.clear_focus()
        self.say(f"{render.OK} Forgot the current concept and question")

    def _clear_screen(self, _: str) -> None:
        """Wipe the screen, then redraw where you are so you keep your bearings."""
        self.io.clear()
        self.say(self._header())

    def _intro(self, _: str) -> None:
        self.say(intro.banner())

    # -------------------------------------------------------------- helpers
    def _find(self, text: str) -> Entity:
        if not text.strip():
            raise Invalid("Which one?", hint=":show q3   or   :show CPU Register")
        entity = lookup(self.app.store, text)
        if entity is None:
            raise Invalid("Could not find that.", hint=":sources   :concepts   :questions")
        return entity

    def _confirm(self, prompt: str, default: bool) -> bool:
        hint = "[Y/n]" if default else "[y/N]"
        reply = self.io.read(f"{prompt} {hint} ")
        if reply is None:
            return default
        reply = reply.strip().lower()
        if not reply:
            return default
        return reply in ("y", "yes")

    def _header(self) -> str:
        c = self.app.context
        goal, source, concept = c.goal, c.source, c.concept
        return render.header(goal.title if goal else None, source.title if source else None,
                             c.ctx.location_label(), concept.name if concept else None)

    def say(self, text: str = "") -> None:
        self.io.write(render.decorate(text))
