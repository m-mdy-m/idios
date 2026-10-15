"""A polished, dark schedule window opened from a desktop notification."""
from __future__ import annotations

from datetime import timedelta
from typing import TYPE_CHECKING, Any
try:
    import tkinter as tk
except ImportError:  # Tk can be omitted from minimal Python/Linux installations.
    tk = None  # type: ignore[assignment]

from idios.domain.models import Task, TaskStatus
from idios.ui.theme import (
    COLORS,
    FONTS,
    ScrollableFrame,
    configure_window,
    make_brand_mark,
    make_button,
    make_card,
    make_label,
    make_pill,
)

if TYPE_CHECKING:
    from idios.services.app import App


def _task_context(app: "App", task: Task) -> str:
    """Return the useful context attached to a task, if it still exists."""
    parts: list[str] = []
    if task.goal_id:
        goal = app.store.goals.get(task.goal_id)
        if goal:
            parts.append(f"Goal · {goal.title}")
    if task.source_id:
        source = app.store.sources.get(task.source_id)
        if source:
            parts.append(f"Source · {source.title}")
    return "   /   ".join(parts)


def _schedule_data(app: "App") -> dict[str, Any]:
    """Return a UI-independent schedule snapshot used by all IDIOS surfaces."""
    today = app.plan.today()
    today_text = today.isoformat()
    tomorrow_text = (today + timedelta(days=1)).isoformat()
    end_text = (today + timedelta(days=7)).isoformat()
    open_tasks = app.plan.open_now()
    return {
        "today": today,
        "review_time": app.plan.review_time,
        "overdue": [task for task in open_tasks if task.due_date < today_text],
        "due_today": [task for task in open_tasks if task.due_date == today_text],
        "completed_today": app.plan.done_today(),
        "skipped_today": app.store.tasks.find(
            "due_date = ? AND status = 'skipped'", (today_text,), order="seq"
        ),
        "upcoming": [
            task for task in app.store.tasks.between(tomorrow_text, end_text)
            if task.status == TaskStatus.OPEN
        ],
    }


def schedule_text(app: "App") -> str:
    """Build schedule details as plain text for terminal/fallback use and tests."""
    data = _schedule_data(app)
    today = data["today"]
    overdue = data["overdue"]
    due_today = data["due_today"]
    completed_today = data["completed_today"]
    skipped_today = data["skipped_today"]
    upcoming = data["upcoming"]
    lines = [
        "IDIOS · LEARNING SCHEDULE",
        f"Date: {today.strftime('%A, %B %d, %Y')}",
        f"Daily review: {data['review_time']}",
        "",
        f"Overview: {len(overdue)} overdue · {len(due_today)} due today · "
        f"{len(completed_today)} completed today · {len(upcoming)} upcoming",
        "",
    ]

    def add_tasks(heading: str, tasks: list[Task], *, show_due: bool = False) -> None:
        lines.extend([heading, "─" * len(heading)])
        if not tasks:
            lines.extend(["Nothing here.", ""])
            return
        for task in tasks:
            due = f" [{task.due_date}]" if show_due else ""
            lines.append(f"• {task.text}{due}")
            context = _task_context(app, task)
            if context:
                lines.append(f"  {context}")
        lines.append("")

    add_tasks("Needs attention · overdue", overdue, show_due=True)
    add_tasks("Needs attention · due today", due_today)
    add_tasks("Completed today", completed_today)
    add_tasks("Skipped today", skipped_today)
    add_tasks("Coming up · next 7 days", upcoming, show_due=True)
    lines.extend([
        "About this view",
        "──────────────",
        "Each task is shown with its saved text and due date. Goal/source context is "
        "shown when linked. This window is read-only; use :review in IDIOS to settle "
        "today's open tasks.",
    ])
    return "\n".join(lines)


def _center_window(root: tk.Tk, width: int, height: int) -> None:
    root.update_idletasks()
    screen_width, screen_height = root.winfo_screenwidth(), root.winfo_screenheight()
    x = max(0, (screen_width - width) // 2)
    y = max(0, (screen_height - height) // 2)
    root.geometry(f"{width}x{height}+{x}+{y}")


def _add_task_rows(
    parent: tk.Misc,
    app: "App",
    tasks: list[Task],
    *,
    accent: str,
    empty_title: str,
    empty_detail: str,
    show_due: bool = False,
    wraplength: int = 310,
) -> None:
    if not tasks:
        empty = tk.Frame(parent, bg=COLORS["item"], padx=12, pady=12)
        empty.pack(fill="x", pady=(10, 0))
        make_label(
            empty, "✓   " + empty_title, font=FONTS["body_bold"], color=accent,
            background=COLORS["item"], anchor="w",
        ).pack(fill="x")
        make_label(
            empty, empty_detail, font=FONTS["small"], color=COLORS["muted"],
            background=COLORS["item"], anchor="w", wraplength=wraplength,
            justify="left",
        ).pack(fill="x", pady=(5, 0))
        return

    for index, task in enumerate(tasks):
        row = tk.Frame(parent, bg=COLORS["item"], padx=11, pady=10)
        row.pack(fill="x", pady=(9 if index == 0 else 6, 0))
        row.grid_columnconfigure(1, weight=1)
        marker = tk.Frame(row, bg=accent, width=3, height=32)
        marker.grid(row=0, column=0, rowspan=2, sticky="nsw", padx=(0, 10))
        marker.grid_propagate(False)
        title = make_label(
            row, task.text, font=FONTS["body_bold"], background=COLORS["item"],
            wraplength=wraplength, justify="left", anchor="w",
        )
        title.grid(row=0, column=1, sticky="ew")
        if show_due:
            make_pill(
                row, task.due_date, foreground=COLORS["muted"],
                background=COLORS["surface"],
            ).grid(row=0, column=2, sticky="ne", padx=(8, 0))
        context = _task_context(app, task)
        if context:
            make_label(
                row, context, font=FONTS["tiny"], color=COLORS["muted"],
                background=COLORS["item"], wraplength=wraplength, justify="left",
                anchor="w",
            ).grid(row=1, column=1, columnspan=2, sticky="ew", pady=(5, 0))


def _section(
    parent: tk.Misc,
    app: "App",
    *,
    title: str,
    subtitle: str,
    tasks: list[Task],
    accent: str,
    soft: str,
    empty_title: str,
    empty_detail: str,
    show_due: bool = False,
    wraplength: int = 310,
) -> tk.Frame:
    card, body = make_card(parent, padding=15)
    top = tk.Frame(body, bg=COLORS["card"])
    top.pack(fill="x")
    top.grid_columnconfigure(0, weight=1)
    make_label(top, title, font=FONTS["heading"], background=COLORS["card"]).grid(
        row=0, column=0, sticky="w"
    )
    make_pill(top, str(len(tasks)), foreground=accent, background=soft).grid(
        row=0, column=1, sticky="e"
    )
    make_label(
        body, subtitle, font=FONTS["small"], color=COLORS["muted"],
        background=COLORS["card"], anchor="w",
    ).pack(fill="x", pady=(4, 0))
    _add_task_rows(
        body, app, tasks, accent=accent, empty_title=empty_title,
        empty_detail=empty_detail, show_due=show_due, wraplength=wraplength,
    )
    return card


def _metric_card(parent: tk.Misc, label: str, value: int, note: str, accent: str, soft: str) -> tk.Frame:
    card, body = make_card(parent, padding=14)
    make_label(
        body, label.upper(), font=FONTS["eyebrow"], color=COLORS["muted"],
        background=COLORS["card"], anchor="w",
    ).pack(fill="x")
    bottom = tk.Frame(body, bg=COLORS["card"])
    bottom.pack(fill="x", pady=(5, 0))
    make_label(bottom, str(value), font=FONTS["metric"], color=accent, background=COLORS["card"]).pack(
        side="left", anchor="s"
    )
    make_pill(bottom, "TODAY" if label == "Completed" else "OPEN", foreground=accent, background=soft).pack(
        side="right", anchor="center", pady=(5, 0)
    )
    make_label(
        body, note, font=FONTS["tiny"], color=COLORS["faint"],
        background=COLORS["card"], anchor="w",
    ).pack(fill="x", pady=(2, 0))
    return card


def show_schedule_dialog(app: "App") -> bool:
    """Show a modal, dark schedule window. Returns False without a desktop GUI."""
    if tk is None:
        return False
    try:
        root = tk.Tk()
    except (tk.TclError, OSError):
        return False

    try:
        data = _schedule_data(app)
        today = data["today"]
        overdue = data["overdue"]
        due_today = data["due_today"]
        completed_today = data["completed_today"]
        skipped_today = data["skipped_today"]
        upcoming = data["upcoming"]

        configure_window(root, "IDIOS — Learning space")
        root.minsize(720, 620)
        _center_window(root, 960, 790)
        root.grid_columnconfigure(0, weight=1)
        root.grid_rowconfigure(0, weight=1)

        shell = tk.Frame(root, bg=COLORS["background"], padx=26, pady=22)
        shell.grid(row=0, column=0, sticky="nsew")
        shell.grid_columnconfigure(0, weight=1)
        shell.grid_rowconfigure(2, weight=1)

        # Reusable brand header: icon + clear wordmark + local-first status.
        header = tk.Frame(shell, bg=COLORS["background"])
        header.grid(row=0, column=0, sticky="ew")
        header.grid_columnconfigure(1, weight=1)
        mark = make_brand_mark(header, 44)
        mark.grid(row=0, column=0, rowspan=2, sticky="w", padx=(0, 12))
        make_label(header, "IDIOS", font=FONTS["brand"], background=COLORS["background"]).grid(
            row=0, column=1, sticky="sw"
        )
        make_label(
            header, "PERSONAL LEARNING SPACE", font=FONTS["eyebrow"],
            color=COLORS["muted"], background=COLORS["background"],
        ).grid(row=1, column=1, sticky="nw", pady=(1, 0))
        make_pill(
            header, "LOCAL  ·  PRIVATE", foreground=COLORS["teal"],
            background=COLORS["teal_soft"],
        ).grid(row=0, column=2, rowspan=2, sticky="e")

        divider = tk.Frame(shell, bg=COLORS["border_subtle"], height=1)
        divider.grid(row=1, column=0, sticky="ew", pady=(18, 18))
        divider.grid_propagate(False)

        # Intro and counters remain visible while the task groups scroll.
        intro = tk.Frame(shell, bg=COLORS["background"])
        intro.grid(row=2, column=0, sticky="nsew")
        intro.grid_columnconfigure(0, weight=1)
        intro.grid_rowconfigure(2, weight=1)
        intro_top = tk.Frame(intro, bg=COLORS["background"])
        intro_top.grid(row=0, column=0, sticky="ew")
        intro_top.grid_columnconfigure(0, weight=1)
        make_label(
            intro_top, "Your learning, in focus.", font=FONTS["title"],
            background=COLORS["background"], anchor="w",
        ).grid(row=0, column=0, sticky="w")
        make_label(
            intro_top, today.strftime("%A, %B %d, %Y"), font=FONTS["body"],
            color=COLORS["muted"], background=COLORS["background"], anchor="w",
        ).grid(row=1, column=0, sticky="w", pady=(4, 0))
        review = tk.Frame(intro_top, bg=COLORS["accent_soft"], padx=12, pady=9)
        review.grid(row=0, column=1, rowspan=2, sticky="e", padx=(14, 0))
        make_label(
            review, "DAILY REVIEW", font=FONTS["eyebrow"], color=COLORS["accent"],
            background=COLORS["accent_soft"],
        ).pack(anchor="e")
        make_label(
            review, data["review_time"], font=FONTS["heading"],
            background=COLORS["accent_soft"],
        ).pack(anchor="e", pady=(2, 0))

        metrics = tk.Frame(intro, bg=COLORS["background"])
        metrics.grid(row=1, column=0, sticky="ew", pady=(16, 15))
        for column in range(4):
            metrics.grid_columnconfigure(column, weight=1, uniform="metric")
        metric_specs = [
            ("Overdue", len(overdue), "Needs a little love", COLORS["red"], COLORS["red_soft"]),
            ("Due today", len(due_today), "On today's list", COLORS["amber"], COLORS["amber_soft"]),
            ("Next 7 days", len(upcoming), "Coming into view", COLORS["teal"], COLORS["teal_soft"]),
            ("Completed", len(completed_today), "Progress made", COLORS["green"], COLORS["green_soft"]),
        ]
        for column, spec in enumerate(metric_specs):
            card = _metric_card(metrics, *spec)
            card.grid(row=0, column=column, sticky="nsew", padx=(0 if column == 0 else 7, 0 if column == 3 else 7))

        scroll = ScrollableFrame(intro, background=COLORS["background"])
        scroll.frame.grid(row=2, column=0, sticky="nsew")
        scroll.content.grid_columnconfigure(0, weight=1, uniform="schedule")
        scroll.content.grid_columnconfigure(1, weight=1, uniform="schedule")
        scroll.content.grid_rowconfigure(0, weight=0)
        sections = [
            _section(
                scroll.content, app, title="Overdue", subtitle="Past due · sorted by your plan",
                tasks=overdue, accent=COLORS["red"], soft=COLORS["red_soft"],
                empty_title="You're caught up", empty_detail="No overdue tasks. Keep that momentum.",
                show_due=True, wraplength=280,
            ),
            _section(
                scroll.content, app, title="Due today", subtitle="Your focus for today",
                tasks=due_today, accent=COLORS["amber"], soft=COLORS["amber_soft"],
                empty_title="A little breathing room", empty_detail="Nothing is scheduled for today.",
                wraplength=280,
            ),
            _section(
                scroll.content, app, title="Completed today", subtitle="Small steps still count",
                tasks=completed_today, accent=COLORS["green"], soft=COLORS["green_soft"],
                empty_title="Your first win is waiting", empty_detail="Completed tasks will show up here.",
                wraplength=280,
            ),
            _section(
                scroll.content, app, title="Skipped today", subtitle="Left for another time",
                tasks=skipped_today, accent=COLORS["muted"], soft=COLORS["surface"],
                empty_title="Nothing skipped", empty_detail="No tasks were skipped today.",
                wraplength=280,
            ),
            _section(
                scroll.content, app, title="Coming up", subtitle="The next seven days · dates included",
                tasks=upcoming, accent=COLORS["teal"], soft=COLORS["teal_soft"],
                empty_title="Your calendar is clear", empty_detail="New tasks for the week will appear here.",
                show_due=True, wraplength=690,
            ),
        ]
        for index, card in enumerate(sections):
            row, column = divmod(index, 2)
            if index == 4:
                card.grid(row=2, column=0, columnspan=2, sticky="nsew", pady=(10, 0))
            else:
                card.grid(row=row, column=column, sticky="nsew", padx=(0, 7) if column == 0 else (7, 0), pady=(0 if row == 0 else 10, 0))
        scroll.bind_mousewheel_to_children()

        footer = tk.Frame(shell, bg=COLORS["background"])
        footer.grid(row=3, column=0, sticky="ew", pady=(14, 0))
        footer.grid_columnconfigure(0, weight=1)
        make_label(
            footer, "READ-ONLY OVERVIEW    ·    YOUR DATA STAYS ON THIS DEVICE",
            font=FONTS["eyebrow"], color=COLORS["faint"],
            background=COLORS["background"],
        ).grid(row=0, column=0, sticky="w")
        close_button = make_button(footer, "Close window", root.destroy, primary=True)
        close_button.grid(row=0, column=1, sticky="e")

        root.protocol("WM_DELETE_WINDOW", root.destroy)
        root.bind("<Escape>", lambda _event: root.destroy())
        root.bind("<Control-w>", lambda _event: root.destroy())
        root.after(120, close_button.focus_set)
        root.lift()
        try:
            root.attributes("-topmost", True)
            root.after(500, lambda: root.attributes("-topmost", False))
        except tk.TclError:
            pass
        root.grab_set()
        root.mainloop()
        return True
    except (tk.TclError, OSError):
        try:
            root.destroy()
        except (NameError, tk.TclError):
            pass
        return False
