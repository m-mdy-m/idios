"""Interactive REPL for IDIOS Learning Core.

A context-aware command loop. No AI, no model calls — pure Python + rich.

Context model
─────────────
  Root context   → goals list, search, graph overview, status
  Goal context   → questions, tasks, evidence, session, progress
  Session active → stage shown in prompt; advance/reflect commands live

Command dispatch
────────────────
  Every command is a method prefixed with _cmd_. The dispatcher strips
  the first token, looks it up in _COMMANDS (root) or _GOAL_COMMANDS
  (goal context), and calls it with the remaining tokens as args.

  Unknown commands never crash; they print a contextual hint.
"""
from __future__ import annotations

import sys
from datetime import datetime, timezone
from typing import Callable

from rich import box
from rich.columns import Columns
from rich.console import Console
from rich.panel import Panel
from rich.prompt import Confirm, Prompt
from rich.rule import Rule
from rich.table import Table
from rich.text import Text

from idios.graph.engine import KnowledgeGraph
from idios.graph.models import EDGE_LABELS, EdgeType, NodeType
from idios.learning.engine import LearningEngine
from idios.learning.models import (
    Concept,
    ConceptStatus,
    CuriosityDisposition,
    EvidenceKind,
    Goal,
    GoalStatus,
    LearningSession,
    Question,
    QuestionStatus,
    SessionStage,
    Task,
    TaskStatus,
)
from idios.retrieval.engine import RetrievalEngine
from idios.retrieval.models import KIND_ICONS, ResultKind
from idios.storage.base import StorageProvider

console = Console()

# ── style constants ────────────────────────────────────────────────────────────

_GOAL_ICONS = {
    GoalStatus.ACTIVE: "🟢",
    GoalStatus.PAUSED: "🔵",
    GoalStatus.DONE:   "✅",
}
_Q_ICONS = {
    QuestionStatus.OPEN:     "❓",
    QuestionStatus.ANSWERED: "✅",
    QuestionStatus.PARKED:   "🔵",
}
_TASK_ICONS = {
    TaskStatus.OPEN:        "⬜",
    TaskStatus.IN_PROGRESS: "🔄",
    TaskStatus.BLOCKED:     "🚫",
    TaskStatus.DONE:        "✅",
}
_CONCEPT_ICONS = {
    ConceptStatus.UNKNOWN:     "⬜",
    ConceptStatus.PROVISIONAL: "🔶",
    ConceptStatus.VALIDATED:   "✅",
}
_STAGE_ORDER = list(SessionStage)


# ── helpers ────────────────────────────────────────────────────────────────────

def _truncate(s: str, n: int = 70) -> str:
    return s if len(s) <= n else s[:n - 1] + "…"


def _ago(dt_str: str) -> str:
    """Human-readable age of an ISO timestamp."""
    try:
        dt = datetime.fromisoformat(dt_str.replace("Z", "+00:00"))
        delta = datetime.now(timezone.utc) - dt
        s = int(delta.total_seconds())
        if s < 60:
            return "just now"
        if s < 3600:
            return f"{s // 60}m ago"
        if s < 86400:
            return f"{s // 3600}h ago"
        return f"{s // 86400}d ago"
    except Exception:
        return ""


# ── REPL ───────────────────────────────────────────────────────────────────────

class LearningREPL:
    """Main interactive loop. Instantiate once; call .run()."""

    def __init__(
        self,
        engine: LearningEngine,
        graph: KnowledgeGraph,
        retrieval: RetrievalEngine,
        storage: StorageProvider,
    ) -> None:
        self._engine = engine
        self._graph = graph
        self._retrieval = retrieval
        self._storage = storage

        self._active_goal: Goal | None = None
        self._active_session: LearningSession | None = None
        self._breadcrumb: list[str] = ["🧠 IDIOS"]

    # ── entry point ───────────────────────────────────────────────────────

    def run(self) -> None:
        console.clear()
        self._print_header()
        self._print_hint("Type [bold cyan]help[/bold cyan] to see available commands.")

        while True:
            try:
                self._print_prompt()
                raw = input().strip()
            except (KeyboardInterrupt, EOFError):
                console.print("\n[dim]Type [bold]quit[/bold] to exit.[/dim]" if sys.exc_info()[0] is KeyboardInterrupt else "")
                if sys.exc_info()[0] is EOFError:
                    break
                continue

            if not raw:
                continue

            parts = raw.split(None, 3)
            cmd  = parts[0].lower()
            args = parts[1:]
            self._dispatch(cmd, args)

    # ── dispatcher ────────────────────────────────────────────────────────

    def _dispatch(self, cmd: str, args: list[str]) -> None:  # noqa: C901
        # global commands — always available
        if cmd in ("quit", "exit", "q"):
            console.print("\n[dim]Goodbye 👋[/dim]\n")
            sys.exit(0)

        if cmd in ("help", "h", "?"):
            self._cmd_help()
            return
        if cmd in ("clear", "cls"):
            console.clear()
            self._print_header()
            return
        if cmd in ("status",):
            self._cmd_status()
            return
        if cmd in ("goals", "g"):
            self._cmd_goals(args)
            return
        if cmd in ("search", "s"):
            self._cmd_search(args)
            return
        if cmd in ("graph", "gx"):
            self._cmd_graph(args)
            return
        if cmd in ("back", "b"):
            self._cmd_back()
            return

        # goal-context commands
        if self._active_goal:
            if cmd in ("question", "q"):
                self._cmd_question(args)
                return
            if cmd in ("task", "t"):
                self._cmd_task(args)
                return
            if cmd in ("evidence", "e"):
                self._cmd_evidence(args)
                return
            if cmd in ("session",):
                self._cmd_session(args)
                return
            if cmd in ("reflect", "r"):
                self._cmd_reflect()
                return
            if cmd in ("curiosity", "c"):
                self._cmd_curiosity(args)
                return
            if cmd in ("parking",):
                self._cmd_parking()
                return
            if cmd in ("progress", "p"):
                self._cmd_progress()
                return

        console.print(f"[red]Unknown command:[/red] [bold]{cmd}[/bold]  — type [cyan]help[/cyan]")

    # ── display helpers ────────────────────────────────────────────────────

    def _print_header(self) -> None:
        goals   = self._load_goals()
        active  = sum(1 for g in goals if g.status == GoalStatus.ACTIVE)
        n_c     = len(self._storage.list_keys("learning:concept:"))
        n_e     = len(self._storage.list_keys("learning:evidence:"))
        n_nodes = len(self._storage.list_keys("graph:node:"))
        n_edges = len(self._storage.list_keys("graph:edge:"))

        header = (
            f"[bold cyan]🧠  IDIOS Learning Core[/bold cyan]\n"
            f"[dim]  {active} active goal{'s' if active != 1 else ''}"
            f"  ·  {n_c} concept{'s' if n_c != 1 else ''}"
            f"  ·  {n_e} evidence record{'s' if n_e != 1 else ''}"
            f"  ·  graph {n_nodes}N/{n_edges}E[/dim]"
        )
        console.print(Panel(header, border_style="cyan", padding=(0, 2)))

    def _print_prompt(self) -> None:
        bc = " [dim]›[/dim] ".join(self._breadcrumb)
        suffix = ""
        if self._active_session:
            suffix = f" [yellow]⚡{self._active_session.stage.value}[/yellow]"
        console.print(f"\n{bc}{suffix}")
        console.print("[bold cyan]>[/bold cyan] ", end="")

    def _print_hint(self, msg: str) -> None:
        console.print(f"[dim]{msg}[/dim]")

    def _print_ok(self, msg: str) -> None:
        console.print(f"[green]✓[/green] {msg}")

    def _print_warn(self, msg: str) -> None:
        console.print(f"[yellow]⚠[/yellow]  {msg}")

    def _print_err(self, msg: str) -> None:
        console.print(f"[red]✗[/red]  {msg}")

    # ── help ─────────────────────────────────────────────────────────────

    def _cmd_help(self) -> None:
        t = Table(show_header=False, box=box.SIMPLE, padding=(0, 2), show_edge=False)
        t.add_column("cmd", style="bold cyan", min_width=22, no_wrap=True)
        t.add_column("desc", style="")

        rows = [
            ("goals  [g]",                "List goals; goals new; goals <n> to select"),
            ("search [s] <query>",         "Full-text search across everything"),
            ("graph  [gx] <sub>",          "Knowledge graph (show / add / path / prereq)"),
            ("status",                     "Overall stats"),
            ("clear",                      "Clear terminal"),
            ("back   [b]",                 "Deselect active goal / session"),
            ("quit",                       "Exit"),
        ]
        for cmd, desc in rows:
            t.add_row(cmd, desc)

        console.print(Rule("[bold]Global commands[/bold]"))
        console.print(t)

        if self._active_goal:
            t2 = Table(show_header=False, box=box.SIMPLE, padding=(0, 2), show_edge=False)
            t2.add_column("cmd", style="bold green", min_width=22, no_wrap=True)
            t2.add_column("desc")
            goal_rows = [
                ("question [q] <sub>",     "new / list / answer <n> / park <n>"),
                ("task     [t] <sub>",     "new / list / done <n> / block <n>"),
                ("evidence [e] <sub>",     "new / list"),
                ("session  <sub>",         "start / advance / status"),
                ("reflect  [r]",           "Record what you understood / what's unclear"),
                ("curiosity [c]",          "Capture a curiosity item"),
                ("parking",                "Show parked curiosities"),
                ("progress [p]",           "Goal progress bars"),
            ]
            for cmd, desc in goal_rows:
                t2.add_row(cmd, desc)
            console.print(Rule(f"[bold]Goal context[/bold]  [dim]({self._active_goal.title})[/dim]"))
            console.print(t2)

        console.print(Rule("[bold]graph sub-commands[/bold]"))
        gt = Table(show_header=False, box=box.SIMPLE, padding=(0, 2), show_edge=False)
        gt.add_column("sub", style="bold", min_width=22, no_wrap=True)
        gt.add_column("desc")
        graph_rows = [
            ("graph show [concept]",         "Overview or detail for one concept"),
            ("graph add",                    "Add a relation (interactive or inline)"),
            ("graph add A prereq B",         "Shorthand: A is prerequisite for B"),
            ("graph path A to B",            "Shortest learning path A → B"),
            ("graph prereq <concept>",       "What must you learn before <concept>?"),
            ("graph missing <concept>",      "Prerequisites not yet validated"),
            ("graph del A prereq B",         "Remove a relation"),
        ]
        for cmd, desc in graph_rows:
            gt.add_row(cmd, desc)
        console.print(gt)

    # ── goals ──────────────────────────────────────────────────────────────

    def _cmd_goals(self, args: list[str]) -> None:
        if not args:
            self._list_goals()
            return

        sub = args[0].lower()

        if sub == "new":
            self._create_goal_interactive()
        elif sub == "pause" and len(args) > 1 and args[1].isdigit():
            self._set_goal_status(int(args[1]), GoalStatus.PAUSED)
        elif sub == "done" and len(args) > 1 and args[1].isdigit():
            self._set_goal_status(int(args[1]), GoalStatus.DONE)
        elif sub == "resume" and len(args) > 1 and args[1].isdigit():
            self._set_goal_status(int(args[1]), GoalStatus.ACTIVE)
        elif sub.isdigit():
            self._select_goal(int(sub))
        else:
            self._list_goals()

    def _list_goals(self) -> None:
        goals = self._load_goals()
        if not goals:
            console.print("\n[dim]No goals yet.[/dim]")
            self._print_hint("Create one with: [bold cyan]goals new[/bold cyan]")
            return

        t = Table(
            box=box.ROUNDED,
            border_style="cyan",
            header_style="bold cyan",
            show_lines=False,
            padding=(0, 1),
        )
        t.add_column("#",        width=3,  justify="right", style="dim")
        t.add_column("",         width=3)
        t.add_column("Title",    min_width=24)
        t.add_column("Project",  style="dim", max_width=14)
        t.add_column("Q",        justify="center", width=6, style="dim")
        t.add_column("Concepts", justify="center", width=8, style="dim")
        t.add_column("Status",   justify="center", width=10)

        for i, goal in enumerate(goals, 1):
            q_keys = self._storage.list_keys("learning:question:")
            open_q = sum(
                1 for k in q_keys
                if (r := self._storage.get(k)) and r.get("goal_id") == goal.id and r.get("status") == "open"
            )
            total_q = sum(
                1 for k in q_keys
                if (r := self._storage.get(k)) and r.get("goal_id") == goal.id
            )
            node_count = sum(
                1 for n in self._graph.all_nodes()
                if goal.id in n.goal_ids
            )
            validated_count = sum(
                1 for n in self._graph.all_nodes()
                if goal.id in n.goal_ids
                and (raw := self._storage.get(f"learning:concept:{n.name}"))
                and raw.get("status") == "validated"
            )

            status_style = {"active": "green", "paused": "blue", "done": "dim"}.get(goal.status.value, "")
            concept_str = f"{validated_count}✅/{node_count}" if node_count else "—"

            t.add_row(
                str(i),
                _GOAL_ICONS[goal.status],
                f"[bold]{goal.title}[/bold]",
                goal.project or "—",
                f"{open_q}/{total_q}" if total_q else "—",
                concept_str,
                f"[{status_style}]{goal.status.value}[/{status_style}]",
            )

        console.print(t)
        self._print_hint("goals <n>  ·  goals new  ·  goals pause/done/resume <n>")

    def _select_goal(self, idx: int) -> None:
        goals = self._load_goals()
        if not (1 <= idx <= len(goals)):
            self._print_err(f"No goal #{idx}")
            return
        self._active_goal = goals[idx - 1]
        self._breadcrumb = ["🧠 IDIOS", f"🎯 {_truncate(self._active_goal.title, 30)}"]
        self._show_goal_detail()

    def _show_goal_detail(self) -> None:
        goal = self._active_goal
        assert goal is not None

        icon = _GOAL_ICONS[goal.status]
        lines = [f"{icon}  [bold]{goal.title}[/bold]"]
        if goal.project:
            lines.append(f"   project: [cyan]{goal.project}[/cyan]")
        if goal.scope:
            lines.append(f"   scope:   [dim]{', '.join(goal.scope)}[/dim]")
        console.print(Panel("\n".join(lines), border_style="green", padding=(0, 2)))

        # DoD
        if goal.definition_of_done:
            dod = goal.definition_of_done
            console.print("\n[bold]Definition of Done[/bold]")
            for c in dod.criteria:
                mark = "[green]✓[/green]" if c in dod.met else "[dim]·[/dim]"
                console.print(f"  {mark} {c}")

        # Questions summary
        q_keys  = self._storage.list_keys("learning:question:")
        g_qs    = [r for k in q_keys if (r := self._storage.get(k)) and r.get("goal_id") == goal.id]
        open_qs = [q for q in g_qs if q.get("status") == "open"]
        if g_qs:
            console.print(f"\n[bold]Questions[/bold]  [dim]({len(open_qs)} open / {len(g_qs)} total)[/dim]")
            for i, q in enumerate(g_qs[:5], 1):
                icon = _Q_ICONS.get(QuestionStatus(q.get("status", "open")), "·")
                status_style = {"open": "white", "answered": "dim", "parked": "blue"}.get(q.get("status", ""), "")
                console.print(f"  [{i}] {icon} [{status_style}]{_truncate(q.get('text', ''), 65)}[/{status_style}]")
            if len(g_qs) > 5:
                self._print_hint(f"  … and {len(g_qs) - 5} more — run [bold]question list[/bold]")

        # Concepts
        nodes = self._graph.nodes_for_goal(goal.id)
        if nodes:
            console.print(f"\n[bold]Concepts in graph[/bold]  [dim]({len(nodes)})[/dim]")
            for n in nodes[:8]:
                raw_c = self._storage.get(f"learning:concept:{n.name}")
                status = raw_c.get("status", "unknown") if raw_c else "unknown"
                icon = _CONCEPT_ICONS.get(ConceptStatus(status), "·")
                gaps = raw_c.get("open_gaps", []) if raw_c else []
                gap_str = f"  [dim]gaps: {', '.join(gaps[:2])}[/dim]" if gaps else ""
                console.print(f"  {icon} [bold]{n.name}[/bold] [{status}]{gap_str}")
            if len(nodes) > 8:
                self._print_hint(f"  … {len(nodes) - 8} more")

        console.print()
        self._print_hint("question · task · evidence · session · reflect · progress · graph · back")

    def _create_goal_interactive(self) -> None:
        console.print(Rule("[bold cyan]New Goal[/bold cyan]"))
        title = Prompt.ask("  Title")
        if not title.strip():
            self._print_err("Cancelled.")
            return

        project  = Prompt.ask("  Project [dim](optional)[/dim]", default="") or None
        scope_in = Prompt.ask("  Scope / tags [dim](comma-separated, optional)[/dim]", default="")
        scope    = [s.strip() for s in scope_in.split(",") if s.strip()]

        criteria: list[str] = []
        if Confirm.ask("\n  Add Definition-of-Done criteria?", default=True):
            self._print_hint("  Enter criteria one per line. Empty line to finish.")
            while True:
                c = Prompt.ask("  Criterion [dim](blank to stop)[/dim]", default="")
                if not c.strip():
                    break
                criteria.append(c.strip())

        goal = self._engine.create_goal(title.strip(), project=project, scope=scope)
        if criteria:
            self._engine.set_definition_of_done(goal, criteria)
            goal = self._reload_goal(goal.id)   # reload to get dod attached

        self._print_ok(f"Goal created: [bold]{goal.title}[/bold]")
        self._active_goal = goal
        self._breadcrumb  = ["🧠 IDIOS", f"🎯 {_truncate(goal.title, 30)}"]
        self._show_goal_detail()

    def _set_goal_status(self, idx: int, status: GoalStatus) -> None:
        goals = self._load_goals()
        if not (1 <= idx <= len(goals)):
            self._print_err(f"No goal #{idx}")
            return
        goal = goals[idx - 1]
        goal.status = status
        self._storage.set(f"learning:goal:{goal.id}", goal.model_dump(mode="json"))
        self._print_ok(f"Goal [bold]{goal.title}[/bold] → {status.value}")

    def _cmd_back(self) -> None:
        if self._active_session:
            self._active_session = None
            self._print_hint("Session deactivated.")
        elif self._active_goal:
            self._active_goal = None
            self._breadcrumb  = ["🧠 IDIOS"]
            self._print_hint("Returned to root.")
        else:
            self._print_hint("Already at root.")

    # ── questions ─────────────────────────────────────────────────────────

    def _cmd_question(self, args: list[str]) -> None:
        if not self._active_goal:
            self._print_err("Select a goal first: [cyan]goals <n>[/cyan]")
            return
        sub = (args[0].lower() if args else "list")

        if sub == "new":
            self._create_question_interactive()
        elif sub == "list":
            self._list_questions()
        elif sub == "answer" and len(args) > 1 and args[1].isdigit():
            self._set_question_status(int(args[1]), QuestionStatus.ANSWERED)
        elif sub == "park" and len(args) > 1 and args[1].isdigit():
            self._set_question_status(int(args[1]), QuestionStatus.PARKED)
        elif sub == "open" and len(args) > 1 and args[1].isdigit():
            self._set_question_status(int(args[1]), QuestionStatus.OPEN)
        elif sub.isdigit():
            self._list_questions()
        else:
            self._list_questions()

    def _list_questions(self) -> None:
        qs = self._goal_questions()
        if not qs:
            console.print("\n[dim]No questions yet.[/dim]")
            self._print_hint("Add one: [cyan]question new[/cyan]")
            return

        t = Table(box=box.SIMPLE, show_header=True, header_style="bold dim", padding=(0, 1))
        t.add_column("#",      width=3,  justify="right", style="dim")
        t.add_column("",       width=3)
        t.add_column("Question")
        t.add_column("Status", width=10, justify="center")
        t.add_column("Added",  width=10, style="dim")

        for i, q in enumerate(qs, 1):
            status = QuestionStatus(q.get("status", "open"))
            style  = {"open": "white", "answered": "dim", "parked": "blue"}.get(status.value, "")
            t.add_row(
                str(i),
                _Q_ICONS[status],
                f"[{style}]{_truncate(q.get('text', ''), 65)}[/{style}]",
                f"[{style}]{status.value}[/{style}]",
                _ago(q.get("created_at", "")),
            )

        console.print(t)
        self._print_hint("question new  ·  question answer <n>  ·  question park <n>")

    def _create_question_interactive(self) -> None:
        text = Prompt.ask("\n  Question")
        if not text.strip():
            return
        self._engine.create_question(self._active_goal, text.strip())
        self._print_ok(f"Question added: [dim]{_truncate(text.strip(), 60)}[/dim]")

    def _set_question_status(self, idx: int, status: QuestionStatus) -> None:
        qs = self._goal_questions()
        if not (1 <= idx <= len(qs)):
            self._print_err(f"No question #{idx}")
            return
        q = qs[idx - 1]
        q["status"] = status.value
        self._storage.set(f"learning:question:{q['id']}", q)
        self._print_ok(f"Question #{idx} → [bold]{status.value}[/bold]")

    def _goal_questions(self) -> list[dict]:
        goal = self._active_goal
        assert goal is not None
        keys = self._storage.list_keys("learning:question:")
        rows = [r for k in keys if (r := self._storage.get(k)) and r.get("goal_id") == goal.id]
        rows.sort(key=lambda x: x.get("created_at", ""))
        return rows

    # ── tasks ─────────────────────────────────────────────────────────────

    def _cmd_task(self, args: list[str]) -> None:
        if not self._active_goal:
            self._print_err("Select a goal first.")
            return
        sub = (args[0].lower() if args else "list")

        if sub == "new":
            self._create_task_interactive()
        elif sub == "done" and len(args) > 1 and args[1].isdigit():
            self._set_task_status(int(args[1]), TaskStatus.DONE)
        elif sub == "block" and len(args) > 1 and args[1].isdigit():
            self._set_task_status(int(args[1]), TaskStatus.BLOCKED)
        elif sub == "start" and len(args) > 1 and args[1].isdigit():
            self._set_task_status(int(args[1]), TaskStatus.IN_PROGRESS)
        else:
            self._list_tasks()

    def _list_tasks(self) -> None:
        tasks = self._goal_tasks()
        if not tasks:
            console.print("\n[dim]No tasks yet.[/dim]")
            self._print_hint("Add one: [cyan]task new[/cyan]")
            return

        t = Table(box=box.SIMPLE, show_header=True, header_style="bold dim", padding=(0, 1))
        t.add_column("#",      width=3,  justify="right", style="dim")
        t.add_column("",       width=3)
        t.add_column("Task")
        t.add_column("Status", width=12, justify="center")
        t.add_column("Added",  width=10, style="dim")

        for i, task in enumerate(tasks, 1):
            status = TaskStatus(task.get("status", "open"))
            style  = {"open": "white", "in_progress": "yellow", "blocked": "red", "done": "dim"}.get(status.value, "")
            t.add_row(
                str(i),
                _TASK_ICONS[status],
                f"[{style}]{_truncate(task.get('title', ''), 65)}[/{style}]",
                f"[{style}]{status.value}[/{style}]",
                _ago(task.get("created_at", "")),
            )

        console.print(t)
        self._print_hint("task new  ·  task start/done/block <n>")

    def _create_task_interactive(self) -> None:
        console.print(Rule("[bold green]New Task[/bold green]"))
        title = Prompt.ask("  Title")
        if not title.strip():
            return

        # optionally link to an open question
        open_qs = [q for q in self._goal_questions() if q.get("status") == "open"]
        linked_q: Question | None = None
        if open_qs:
            console.print("\n  Open questions:")
            for i, q in enumerate(open_qs, 1):
                console.print(f"    [{i}] {_truncate(q['text'], 65)}")
            choice = Prompt.ask("  Link to question # [dim](blank to skip)[/dim]", default="")
            if choice.isdigit():
                idx = int(choice) - 1
                if 0 <= idx < len(open_qs):
                    linked_q = Question.model_validate(open_qs[idx])

        task = self._engine.create_task(self._active_goal, title.strip(), question=linked_q)
        self._print_ok(f"Task created: [bold]{task.title}[/bold]")

    def _set_task_status(self, idx: int, status: TaskStatus) -> None:
        tasks = self._goal_tasks()
        if not (1 <= idx <= len(tasks)):
            self._print_err(f"No task #{idx}")
            return
        task = tasks[idx - 1]
        task["status"] = status.value
        self._storage.set(f"learning:task:{task['id']}", task)
        self._print_ok(f"Task #{idx} → [bold]{status.value}[/bold]")

    def _goal_tasks(self) -> list[dict]:
        goal = self._active_goal
        assert goal is not None
        keys = self._storage.list_keys("learning:task:")
        rows = [r for k in keys if (r := self._storage.get(k)) and r.get("goal_id") == goal.id]
        rows.sort(key=lambda x: x.get("created_at", ""))
        return rows

    # ── evidence ──────────────────────────────────────────────────────────

    def _cmd_evidence(self, args: list[str]) -> None:
        if not self._active_goal:
            self._print_err("Select a goal first.")
            return
        sub = (args[0].lower() if args else "new")
        if sub == "list":
            self._list_evidence()
        else:
            self._record_evidence_interactive()

    def _record_evidence_interactive(self) -> None:
        console.print(Rule("[bold green]Record Evidence[/bold green]"))

        concept = Prompt.ask("  Concept / topic")
        if not concept.strip():
            return
        concept = concept.strip()

        # show EvidenceKind options
        kinds = list(EvidenceKind)
        console.print("\n  Evidence kinds:")
        for i, k in enumerate(kinds, 1):
            console.print(f"    [{i}] {k.value}")
        kind_in = Prompt.ask("  Kind # or name", default="experiment")
        if kind_in.isdigit() and 1 <= int(kind_in) <= len(kinds):
            kind = kinds[int(kind_in) - 1]
        else:
            try:
                kind = EvidenceKind(kind_in.lower())
            except ValueError:
                kind = EvidenceKind.OTHER

        description = Prompt.ask("  Description")
        if not description.strip():
            return

        # optionally link to a task
        open_tasks = [t for t in self._goal_tasks() if t.get("status") in ("open", "in_progress")]
        linked_task: Task | None = None
        if open_tasks:
            console.print("\n  Open tasks:")
            for i, task in enumerate(open_tasks, 1):
                console.print(f"    [{i}] {_truncate(task['title'], 65)}")
            t_choice = Prompt.ask("  Link to task # [dim](blank to skip)[/dim]", default="")
            if t_choice.isdigit():
                idx = int(t_choice) - 1
                if 0 <= idx < len(open_tasks):
                    linked_task = Task.model_validate(open_tasks[idx])

        self._engine.record_evidence(
            kind, description.strip(), concept=concept, task=linked_task
        )

        # auto-enrich the graph node for this concept
        self._graph.add_node(
            concept,
            goal_id=self._active_goal.id,
            description=description.strip()[:120],
        )

        self._print_ok(f"Evidence recorded → [bold]{concept}[/bold]  [{kind.value}]")

        # offer validation if any evidence now exists
        if self._engine.has_required_evidence(concept):
            if Confirm.ask(f"\n  Validate concept [bold]{concept}[/bold] now?", default=False):
                gaps_in = Prompt.ask("  Open gaps [dim](comma-separated, blank = none)[/dim]", default="")
                gaps    = [g.strip() for g in gaps_in.split(",") if g.strip()]
                c = self._engine.validate_concept(concept, open_gaps=gaps)
                self._print_ok(f"Concept [bold]{c.name}[/bold] validated!")

    def _list_evidence(self) -> None:
        keys  = self._storage.list_keys("learning:evidence:")
        items = [r for k in keys if (r := self._storage.get(k))]
        if not items:
            console.print("[dim]No evidence recorded yet.[/dim]")
            return

        t = Table(box=box.SIMPLE, header_style="bold dim", padding=(0, 1))
        t.add_column("Concept", style="bold", min_width=14)
        t.add_column("Kind",    width=22)
        t.add_column("Description")
        t.add_column("When", width=10, style="dim")
        for e in sorted(items, key=lambda x: x.get("created_at", ""), reverse=True)[:30]:
            t.add_row(
                e.get("concept") or "—",
                e.get("kind", ""),
                _truncate(e.get("description", ""), 55),
                _ago(e.get("created_at", "")),
            )
        console.print(t)

    # ── sessions ──────────────────────────────────────────────────────────

    def _cmd_session(self, args: list[str]) -> None:
        if not self._active_goal:
            self._print_err("Select a goal first.")
            return
        sub = (args[0].lower() if args else "start")

        if sub == "start":
            self._start_session()
        elif sub in ("advance", "next"):
            self._advance_session()
        elif sub == "status":
            self._show_session_status()
        else:
            self._show_session_status()

    def _start_session(self) -> None:
        if self._active_session:
            if not Confirm.ask(f"  Session at [bold]{self._active_session.stage.value}[/bold] is active. Start a new one?", default=False):
                return

        open_tasks = [t for t in self._goal_tasks() if t.get("status") in ("open", "in_progress")]
        linked_task: Task | None = None
        if open_tasks:
            console.print("\n  Tasks:")
            for i, task in enumerate(open_tasks, 1):
                s = task.get("status", "")
                console.print(f"    [{i}] {_TASK_ICONS.get(TaskStatus(s), '·')} {_truncate(task['title'], 55)}")
            t_choice = Prompt.ask("  Session for task # [dim](blank = goal-level)[/dim]", default="")
            if t_choice.isdigit():
                idx = int(t_choice) - 1
                if 0 <= idx < len(open_tasks):
                    linked_task = Task.model_validate(open_tasks[idx])
                    linked_task.status = TaskStatus.IN_PROGRESS
                    self._storage.set(f"learning:task:{linked_task.id}", linked_task.model_dump(mode="json"))

        session = self._engine.start_session(self._active_goal, task=linked_task)
        self._active_session = session
        self._print_ok(f"Session started — stage: [bold]{session.stage.value}[/bold]")
        self._render_stage_bar(session.stage)

    def _advance_session(self) -> None:
        if not self._active_session:
            self._print_err("No active session.  Run [cyan]session start[/cyan]")
            return

        cur_idx   = _STAGE_ORDER.index(self._active_session.stage)
        remaining = _STAGE_ORDER[cur_idx + 1:]
        if not remaining:
            self._print_hint("All stages complete for this session.")
            return

        console.print("\n  Next stages:")
        for i, stage in enumerate(remaining, 1):
            console.print(f"    [{i}] {stage.value}")

        choice = Prompt.ask("  Advance to stage # or name")
        if not choice.strip():
            return

        target: SessionStage | None = None
        if choice.isdigit():
            idx = int(choice) - 1
            if 0 <= idx < len(remaining):
                target = remaining[idx]
        else:
            try:
                target = SessionStage(choice.lower().replace(" ", "_"))
            except ValueError:
                self._print_err(f"Unknown stage: {choice}")
                return

        if target:
            try:
                self._active_session = self._engine.advance(self._active_session, target)
                self._print_ok(f"Advanced → [bold]{target.value}[/bold]")
                self._render_stage_bar(target)
            except ValueError as exc:
                self._print_err(str(exc))

    def _show_session_status(self) -> None:
        if not self._active_session:
            self._print_hint("No active session.")
            return
        self._render_stage_bar(self._active_session.stage)

    def _render_stage_bar(self, current: SessionStage) -> None:
        stages = _STAGE_ORDER
        cur_idx = stages.index(current)
        parts: list[str] = []
        for i, s in enumerate(stages):
            label = s.value.replace("_", " ")
            if i < cur_idx:
                parts.append(f"[green dim]{label}[/green dim]")
            elif i == cur_idx:
                parts.append(f"[bold yellow]▶ {label}[/bold yellow]")
            else:
                parts.append(f"[dim]{label}[/dim]")
        console.print("\n  " + "  →  ".join(parts) + "\n")

    # ── reflect ───────────────────────────────────────────────────────────

    def _cmd_reflect(self) -> None:
        if not self._active_session:
            self._print_err("No active session.  Run [cyan]session start[/cyan]")
            return

        console.print(Rule("[bold green]Reflection[/bold green]"))
        understood_in = Prompt.ask("  What did you understand? [dim](comma-separated)[/dim]")
        unclear_in    = Prompt.ask("  What's still unclear?   [dim](comma-separated)[/dim]")

        understood = [s.strip() for s in understood_in.split(",") if s.strip()]
        unclear    = [s.strip() for s in unclear_in.split(",")    if s.strip()]

        self._engine.reflect(self._active_session, understood=understood, unclear=unclear)
        self._print_ok("Reflection saved.")

        if unclear and Confirm.ask("\n  Park unclear items as curiosities?", default=True):
            for item in unclear:
                self._engine.capture_curiosity(item, CuriosityDisposition.USEFUL_LATER)
            self._print_ok(f"{len(unclear)} curiosit{'y' if len(unclear) == 1 else 'ies'} parked.")

    # ── curiosity ─────────────────────────────────────────────────────────

    def _cmd_curiosity(self, args: list[str]) -> None:
        sub = (args[0].lower() if args else "new")
        if sub == "list":
            self._cmd_parking()
        else:
            self._capture_curiosity()

    def _capture_curiosity(self) -> None:
        console.print(Rule("[bold]Capture Curiosity[/bold]"))
        question = Prompt.ask("  What are you curious about?")
        if not question.strip():
            return

        console.print("\n  Disposition:")
        console.print("    [1] relevant_now  — explore this now")
        console.print("    [2] useful_later  — park it, don't lose it  [default]")
        console.print("    [3] unrelated     — interesting but off-topic")
        choice = Prompt.ask("  Choose", choices=["1", "2", "3"], default="2")
        disposition = {
            "1": CuriosityDisposition.RELEVANT_NOW,
            "2": CuriosityDisposition.USEFUL_LATER,
            "3": CuriosityDisposition.UNRELATED,
        }[choice]

        c = self._engine.capture_curiosity(question.strip(), disposition)
        tag = "[blue]parked[/blue]" if c.parked else "[yellow]active[/yellow]"
        self._print_ok(f"Curiosity captured → {tag}")

    def _cmd_parking(self) -> None:
        parked = self._engine.parking_lot()
        if not parked:
            console.print("[dim]Parking lot is empty.[/dim]")
            return

        t = Table(title="🅿️  Parking Lot", box=box.ROUNDED, border_style="blue", padding=(0, 1))
        t.add_column("#",           width=3,  justify="right", style="dim")
        t.add_column("Question",    min_width=40)
        t.add_column("Disposition", width=14, justify="center")
        t.add_column("Added",       width=10, style="dim")

        for i, c in enumerate(parked, 1):
            t.add_row(
                str(i),
                c.question,
                c.disposition.value if c.disposition else "—",
                _ago(str(c.created_at)),
            )
        console.print(t)

    # ── progress ──────────────────────────────────────────────────────────

    def _cmd_progress(self) -> None:
        if not self._active_goal:
            self._print_err("Select a goal first.")
            return
        goal = self._active_goal

        tasks     = self._goal_tasks()
        done_t    = sum(1 for t in tasks if t.get("status") == "done")
        total_t   = len(tasks)

        nodes     = self._graph.nodes_for_goal(goal.id)
        validated = sum(
            1 for n in nodes
            if (raw := self._storage.get(f"learning:concept:{n.name}"))
            and raw.get("status") == "validated"
        )
        total_n = len(nodes)

        evidence  = self._storage.list_keys("learning:evidence:")
        n_evidence = len(evidence)

        dod      = goal.definition_of_done
        dod_met  = len(dod.met)      if dod else 0
        dod_tot  = len(dod.criteria) if dod else 0

        def bar(done: int, total: int, width: int = 24) -> str:
            if total == 0:
                return f"[dim]{'░' * width}[/dim] [dim]—[/dim]"
            pct   = done / total
            filled = int(pct * width)
            color  = "green" if pct >= 1.0 else ("yellow" if pct >= 0.5 else "cyan")
            b = f"[{color}]{'█' * filled}[/{color}][dim]{'░' * (width - filled)}[/dim]"
            return f"{b} [bold]{done}/{total}[/bold]  [dim]{pct*100:.0f}%[/dim]"

        console.print(f"\n[bold]Progress:[/bold] {_GOAL_ICONS[goal.status]} {goal.title}\n")
        console.print(f"  Tasks      {bar(done_t, total_t)}")
        console.print(f"  Concepts   {bar(validated, total_n)}")
        if dod_tot:
            console.print(f"  DoD        {bar(dod_met, dod_tot)}")
        console.print(f"\n  Evidence records: [bold]{n_evidence}[/bold]")

        # open gaps
        all_gaps: list[str] = []
        for n in nodes:
            raw_c = self._storage.get(f"learning:concept:{n.name}")
            if raw_c and raw_c.get("open_gaps"):
                all_gaps.extend(f"{n.name}: {g}" for g in raw_c["open_gaps"])
        if all_gaps:
            console.print(f"\n  [yellow]Open gaps:[/yellow]")
            for g in all_gaps[:5]:
                console.print(f"    · {g}")
            if len(all_gaps) > 5:
                self._print_hint(f"  … and {len(all_gaps) - 5} more")

    # ── search ────────────────────────────────────────────────────────────

    def _cmd_search(self, args: list[str]) -> None:
        query = " ".join(args).strip() if args else ""
        if not query:
            query = Prompt.ask("\n  Search query")
        if not query.strip():
            return

        results = self._retrieval.search(query.strip(), top_k=12)

        if not results:
            console.print(f"\n[dim]No results for:[/dim] [bold]{query}[/bold]")
            return

        console.print(f"\n[bold]Results for[/bold] [cyan]{query}[/cyan]  [dim]({len(results)} hits)[/dim]\n")

        t = Table(box=box.SIMPLE, show_header=True, header_style="bold dim", padding=(0, 1))
        t.add_column("",       width=3)
        t.add_column("Kind",   width=11)
        t.add_column("Result", min_width=40)
        t.add_column("Score",  width=7, justify="right", style="dim")

        for r in results:
            icon = KIND_ICONS.get(r.kind, "·")
            t.add_row(icon, r.kind.value, _truncate(r.snippet, 75), f"{r.score:.2f}")

        console.print(t)

    # ── graph ─────────────────────────────────────────────────────────────

    def _cmd_graph(self, args: list[str]) -> None:
        if not args:
            self._graph_overview()
            return

        sub = args[0].lower()

        if sub == "show":
            concept = " ".join(args[1:]).strip() or None
            if concept:
                self._graph_show_concept(concept)
            else:
                self._graph_overview()

        elif sub == "add":
            # graph add A <edge_type> B  or interactive
            rest = args[1:]
            if len(rest) >= 3:
                self._graph_add_inline(rest[0], rest[1], rest[2], note=" ".join(rest[3:]))
            else:
                self._graph_add_interactive()

        elif sub == "del":
            rest = args[1:]
            if len(rest) >= 3:
                self._graph_delete_inline(rest[0], rest[1], rest[2])
            else:
                self._print_hint("Usage: graph del <A> <edge_type> <B>")

        elif sub == "path":
            # graph path A to B
            rest = args[1:]
            if len(rest) >= 3 and rest[1].lower() == "to":
                self._graph_path(rest[0], rest[2])
            elif len(rest) == 2:
                self._graph_path(rest[0], rest[1])
            else:
                self._graph_path_interactive()

        elif sub == "prereq":
            concept = " ".join(args[1:]).strip() or None
            self._graph_prereq(concept)

        elif sub == "missing":
            concept = " ".join(args[1:]).strip() or None
            self._graph_missing(concept)

        else:
            self._graph_overview()

    def _graph_overview(self) -> None:
        nodes = self._graph.all_nodes()
        stats = self._graph.stats()

        if not nodes:
            console.print("\n[dim]Knowledge graph is empty.[/dim]")
            self._print_hint(
                "Add relations: [cyan]graph add <A> prereq <B>[/cyan]\n"
                "  or record evidence to auto-populate concepts."
            )
            return

        console.print(f"\n[bold]Knowledge Graph[/bold]  "
                      f"[dim]{stats['nodes']} nodes  ·  {stats['edges']} edges[/dim]\n")

        # node table
        t = Table(box=box.SIMPLE, header_style="bold dim", padding=(0, 1))
        t.add_column("Concept",    style="bold", min_width=18)
        t.add_column("Type",       width=10)
        t.add_column("Out",        width=4,  justify="center", style="dim")
        t.add_column("In",         width=4,  justify="center", style="dim")
        t.add_column("Learn status", width=14)
        t.add_column("Goals",      width=6,  justify="center", style="dim")

        for node in nodes:
            out_n   = len(self._graph.edges_from(node.name))
            in_n    = len(self._graph.edges_to(node.name))
            raw_c   = self._storage.get(f"learning:concept:{node.name}")
            status  = raw_c.get("status", "unknown") if raw_c else "unknown"
            s_icon  = _CONCEPT_ICONS.get(ConceptStatus(status), "·")
            t.add_row(
                node.name,
                node.node_type.value,
                str(out_n),
                str(in_n),
                f"{s_icon} {status}",
                str(len(node.goal_ids)),
            )
        console.print(t)

        # sample edges
        all_edge_keys = self._storage.list_keys("graph:edge:")
        if all_edge_keys:
            console.print(f"\n[bold dim]Relations[/bold dim]  [dim]({len(all_edge_keys)} total)[/dim]")
            from idios.graph.models import GraphEdge
            shown = 0
            for k in all_edge_keys:
                raw = self._storage.get(k)
                if not raw:
                    continue
                try:
                    e = GraphEdge.model_validate(raw)
                    lbl = EDGE_LABELS.get(e.edge_type, f"─{e.edge_type.value}→")
                    note_str = f"  [dim italic]{e.note}[/dim italic]" if e.note else ""
                    console.print(f"  [cyan]{e.from_node}[/cyan] [dim]{lbl}[/dim] [cyan]{e.to_node}[/cyan]{note_str}")
                    shown += 1
                    if shown >= 20:
                        break
                except Exception:
                    pass
            if len(all_edge_keys) > 20:
                self._print_hint(f"  … {len(all_edge_keys) - 20} more edges")

        console.print()
        self._print_hint(
            "graph show <concept>  ·  graph add  ·  graph path A to B  ·  graph prereq <concept>"
        )

    def _graph_show_concept(self, name: str) -> None:
        node = self._graph.node(name)
        if not node:
            self._print_err(f"Concept not in graph: [bold]{name}[/bold]")
            return

        console.print(Panel(
            f"[bold]{node.name}[/bold]  [dim]({node.node_type.value})[/dim]"
            + (f"\n{node.description}" if node.description else ""),
            border_style="cyan", padding=(0, 2),
        ))

        # learning state
        raw_c = self._storage.get(f"learning:concept:{name}")
        if raw_c:
            status = raw_c.get("status", "unknown")
            icon   = _CONCEPT_ICONS.get(ConceptStatus(status), "·")
            console.print(f"\n  Learning status: {icon} [bold]{status}[/bold]")
            if raw_c.get("open_gaps"):
                console.print(f"  Open gaps:")
                for g in raw_c["open_gaps"]:
                    console.print(f"    · {g}")

        # outgoing
        edges_out = self._graph.edges_from(name)
        if edges_out:
            console.print(f"\n  [bold]Outgoing[/bold] ({len(edges_out)})")
            for e in edges_out:
                lbl = EDGE_LABELS.get(e.edge_type, f"─{e.edge_type.value}→")
                note = f"  [dim]{e.note}[/dim]" if e.note else ""
                console.print(f"    [dim]{lbl}[/dim] [cyan bold]{e.to_node}[/cyan bold]{note}")

        # incoming
        edges_in = self._graph.edges_to(name)
        if edges_in:
            console.print(f"\n  [bold]Incoming[/bold] ({len(edges_in)})")
            for e in edges_in:
                lbl = EDGE_LABELS.get(e.edge_type, f"─{e.edge_type.value}→")
                note = f"  [dim]{e.note}[/dim]" if e.note else ""
                console.print(f"    [cyan bold]{e.from_node}[/cyan bold] [dim]{lbl}[/dim]{note}")

        # neighbourhood depth 2
        related = self._graph.related(name, depth=2)
        if related:
            console.print(f"\n  [bold]Related concepts[/bold] (depth ≤ 2)")
            for depth, pairs in sorted(related.items()):
                for node_r, edge in pairs:
                    raw_r  = self._storage.get(f"learning:concept:{node_r.name}")
                    status = raw_r.get("status", "unknown") if raw_r else "unknown"
                    icon   = _CONCEPT_ICONS.get(ConceptStatus(status), "·")
                    console.print(
                        f"    [dim]d{depth}[/dim] {icon} [cyan]{node_r.name}[/cyan]"
                        f"  [dim]via {edge.edge_type.value}[/dim]"
                    )

        # evidence
        e_keys = self._storage.list_keys("learning:evidence:")
        concept_evidence = [
            r for k in e_keys if (r := self._storage.get(k)) and r.get("concept") == name
        ]
        if concept_evidence:
            console.print(f"\n  [bold]Evidence[/bold] ({len(concept_evidence)} records)")
            for ev in concept_evidence[:4]:
                console.print(f"    📌 [{ev.get('kind', '')}] {_truncate(ev.get('description', ''), 55)}")

    def _graph_add_interactive(self) -> None:
        console.print(Rule("[bold]Add Relation[/bold]"))
        from_name = Prompt.ask("  From concept")
        if not from_name.strip():
            return

        types = list(EdgeType)
        console.print("\n  Edge types:")
        for i, et in enumerate(types, 1):
            console.print(f"    [{i}] {et.value:18}  {EDGE_LABELS[et]}")
        et_in = Prompt.ask("  Relation # or name", default="related")
        if et_in.isdigit() and 1 <= int(et_in) <= len(types):
            edge_type = types[int(et_in) - 1]
        else:
            try:
                edge_type = EdgeType(et_in.lower())
            except ValueError:
                self._print_err(f"Unknown edge type: {et_in}")
                return

        to_name = Prompt.ask("  To concept")
        if not to_name.strip():
            return

        note = Prompt.ask("  Note [dim](optional)[/dim]", default="")
        self._graph_add_inline(from_name.strip(), edge_type.value, to_name.strip(), note=note)

    def _graph_add_inline(self, from_name: str, edge_type_str: str, to_name: str, note: str = "") -> None:
        try:
            edge_type = EdgeType(edge_type_str.lower())
        except ValueError:
            self._print_err(f"Unknown edge type: [bold]{edge_type_str}[/bold]")
            valid = ", ".join(et.value for et in EdgeType)
            self._print_hint(f"Valid types: {valid}")
            return

        edge = self._graph.add_relation(from_name, edge_type, to_name, note=note)

        # link to active goal if any
        if self._active_goal:
            self._graph.add_node(from_name, goal_id=self._active_goal.id)
            self._graph.add_node(to_name,   goal_id=self._active_goal.id)

        lbl = EDGE_LABELS.get(edge_type, f"─{edge_type.value}→")
        self._print_ok(
            f"[cyan]{from_name}[/cyan] [dim]{lbl}[/dim] [cyan]{to_name}[/cyan]"
            + (f"  [dim italic]{note}[/dim italic]" if note else "")
        )

    def _graph_delete_inline(self, from_name: str, edge_type_str: str, to_name: str) -> None:
        try:
            edge_type = EdgeType(edge_type_str.lower())
        except ValueError:
            self._print_err(f"Unknown edge type: {edge_type_str}")
            return
        removed = self._graph.delete_relation(from_name, edge_type, to_name)
        if removed:
            self._print_ok(f"Removed: {from_name} ─[{edge_type_str}]→ {to_name}")
        else:
            self._print_warn(f"Relation not found: {from_name} ─[{edge_type_str}]→ {to_name}")

    def _graph_path(self, from_name: str, to_name: str) -> None:
        path = self._graph.learning_path(from_name, to_name)
        if path is None:
            self._print_warn(
                f"No directed path from [cyan]{from_name}[/cyan] to [cyan]{to_name}[/cyan]\n"
                "  (only PREREQUISITE / ENABLES edges are traversed)"
            )
            return

        console.print(f"\n[bold]Learning path[/bold]  [dim]({len(path) - 1} hop{'s' if len(path) != 2 else ''})[/dim]\n")
        for step in path:
            if step.edge_label:
                console.print(f"    [dim]{step.edge_label}[/dim]")
            raw_c = self._storage.get(f"learning:concept:{step.node}")
            status = raw_c.get("status", "unknown") if raw_c else "unknown"
            icon   = _CONCEPT_ICONS.get(ConceptStatus(status), "·")
            console.print(f"  {icon} [cyan bold]{step.node}[/cyan bold]  [dim]{status}[/dim]")

    def _graph_path_interactive(self) -> None:
        from_name = Prompt.ask("  From concept")
        to_name   = Prompt.ask("  To concept")
        if from_name.strip() and to_name.strip():
            self._graph_path(from_name.strip(), to_name.strip())

    def _graph_prereq(self, concept: str | None) -> None:
        if not concept:
            concept = Prompt.ask("  Concept name").strip()
        if not concept:
            return

        prereqs = self._graph.prerequisites(concept)
        if not prereqs:
            console.print(f"[dim]No direct prerequisites for:[/dim] [bold]{concept}[/bold]")
            return

        console.print(f"\n[bold]Direct prerequisites for[/bold] [cyan]{concept}[/cyan]")
        for p in prereqs:
            raw_c  = self._storage.get(f"learning:concept:{p.name}")
            status = raw_c.get("status", "unknown") if raw_c else "unknown"
            icon   = _CONCEPT_ICONS.get(ConceptStatus(status), "·")
            console.print(f"  {icon} [bold]{p.name}[/bold]  [{status}]")

        all_p = self._graph.all_prerequisites(concept)
        if len(all_p) > len(prereqs):
            console.print(f"\n  [dim]Full transitive chain ({len(all_p)} concepts):[/dim]")
            for name in all_p:
                raw_c  = self._storage.get(f"learning:concept:{name}")
                status = raw_c.get("status", "unknown") if raw_c else "unknown"
                icon   = _CONCEPT_ICONS.get(ConceptStatus(status), "·")
                console.print(f"    {icon} {name}")

    def _graph_missing(self, concept: str | None) -> None:
        if not concept:
            concept = Prompt.ask("  Goal concept").strip()
        if not concept:
            return

        validated_names = set()
        for k in self._storage.list_keys("learning:concept:"):
            raw = self._storage.get(k)
            if raw and raw.get("status") == "validated":
                validated_names.add(raw.get("name", ""))

        missing = self._graph.missing_prerequisites(concept, validated_names)
        if not missing:
            self._print_ok(f"All prerequisites for [bold]{concept}[/bold] are validated!")
            return

        console.print(f"\n[yellow]Missing prerequisites for[/yellow] [cyan]{concept}[/cyan]  [dim](learn these first)[/dim]")
        for name in missing:
            raw_c  = self._storage.get(f"learning:concept:{name}")
            status = raw_c.get("status", "unknown") if raw_c else "unknown"
            icon   = _CONCEPT_ICONS.get(ConceptStatus(status), "·")
            console.print(f"  {icon} {name}  [dim]{status}[/dim]")

    # ── status ────────────────────────────────────────────────────────────

    def _cmd_status(self) -> None:
        goals     = self._load_goals()
        active_g  = sum(1 for g in goals if g.status == GoalStatus.ACTIVE)
        stats     = self._retrieval.index_stats()
        validated = sum(
            1 for k in self._storage.list_keys("learning:concept:")
            if (r := self._storage.get(k)) and r.get("status") == "validated"
        )
        graph_s = self._graph.stats()

        t = Table(box=box.ROUNDED, border_style="cyan", show_header=False, padding=(0, 2))
        t.add_column("",   style="dim",  min_width=20)
        t.add_column("",   style="bold", min_width=16)

        t.add_row("Active goals",         str(active_g))
        t.add_row("Total goals",          str(len(goals)))
        t.add_row("Questions",            str(stats["questions"]))
        t.add_row("Tasks",                str(stats["tasks"]))
        t.add_row("Evidence records",     str(stats["evidence"]))
        t.add_row("Concepts (validated)", f"{validated} / {stats['concepts']}")
        t.add_row("Graph nodes",          str(graph_s["nodes"]))
        t.add_row("Graph edges",          str(graph_s["edges"]))
        t.add_row("Parked curiosities",   str(len(self._engine.parking_lot())))
        if self._active_goal:
            t.add_row("— active goal —",  self._active_goal.title)
        if self._active_session:
            t.add_row("— active session —", self._active_session.stage.value)

        console.print(t)

    # ── storage helpers ───────────────────────────────────────────────────

    def _load_goals(self) -> list[Goal]:
        keys  = self._storage.list_keys("learning:goal:")
        goals = []
        for k in keys:
            raw = self._storage.get(k)
            if raw:
                try:
                    goals.append(Goal.model_validate(raw))
                except Exception:
                    pass
        goals.sort(key=lambda g: g.created_at)
        return goals

    def _reload_goal(self, goal_id: str) -> Goal:
        raw = self._storage.get(f"learning:goal:{goal_id}")
        return Goal.model_validate(raw)
