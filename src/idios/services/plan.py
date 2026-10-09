"""Day planning: log what you did, plan what's next, review at the end of the day.

This is deliberately tiny. A *task* is one line of text with a day and a state
(open, done, skipped). It is not a project manager: the point is the evening
loop that closes the day.

    night      did: ...          what I did today          (saved as done)
               plan: ...         what I'll do tomorrow     (saved as open)
    next eve   idios remind      desktop notification, from the OS scheduler
               :review           done / not yet (→ tomorrow) / skip
"""
from __future__ import annotations

from dataclasses import dataclass
from datetime import date, datetime, timedelta
from typing import Callable, Optional

from idios.domain.errors import Invalid
from idios.domain.ids import now
from idios.domain.models import Note, Task, TaskStatus
from idios.services.context import ContextService
from idios.services.dates import parse_clock
from idios.storage.repos import Store

Clock = Callable[[], datetime]
DEFAULT_REVIEW_TIME = "21:00"


@dataclass
class DayView:
    """Tasks grouped by day for ``:plan``."""

    overdue: list[Task]
    days: list[tuple[date, list[Task]]]


@dataclass
class Review:
    done: list[Task]
    carried: list[Task]
    skipped: list[Task]

    @property
    def total(self) -> int:
        return len(self.done) + len(self.carried) + len(self.skipped)


class PlanService:
    def __init__(self, store: Store, context: ContextService, clock: Clock = datetime.now) -> None:
        self.store = store
        self.context = context
        self.clock = clock

    # -- time ---------------------------------------------------------------
    def today(self) -> date:
        return self.clock().date()

    @property
    def review_time(self) -> str:
        return self.store.context.setting("review_time", DEFAULT_REVIEW_TIME)

    def set_review_time(self, text: str) -> str:
        value = parse_clock(text)
        self.store.context.set_setting("review_time", value)
        return value

    def past_review_time(self) -> bool:
        return self.clock().strftime("%H:%M") >= self.review_time

    # -- creating -----------------------------------------------------------
    def add(self, text: str, day: Optional[date] = None) -> Task:
        """Plan something. Default day is tomorrow: planning is a night-before habit."""
        return self._create(text, day or self.today() + timedelta(days=1), TaskStatus.OPEN)

    def did(self, text: str, day: Optional[date] = None) -> Task:
        """Record something already done (today unless told otherwise)."""
        task = self._create(text, day or self.today(), TaskStatus.DONE)
        task.done_at = now()
        self.store.tasks.update(task)
        return task

    def _create(self, text: str, day: date, status: TaskStatus) -> Task:
        text = " ".join(text.split())
        if not text:
            raise Invalid("What is it?", hint="plan: Read chapter 3      did: Finished the exercises")
        ctx = self.context.ctx
        task = Task(text=text, due_date=day.isoformat(), status=status,
                    goal_id=ctx.goal_id, source_id=ctx.source_id)
        self.store.tasks.add(task)
        self.context.record_activity()
        return task

    # -- changing state -----------------------------------------------------
    def complete(self, task: Task) -> Task:
        task.status, task.done_at = TaskStatus.DONE, now()
        return self.store.tasks.update(task)

    def skip(self, task: Task) -> Task:
        task.status, task.done_at = TaskStatus.SKIPPED, None
        return self.store.tasks.update(task)

    def move(self, task: Task, day: date) -> Task:
        task.due_date, task.status, task.done_at = day.isoformat(), TaskStatus.OPEN, None
        return self.store.tasks.update(task)

    # -- reading ------------------------------------------------------------
    def view(self, ahead: int = 7) -> DayView:
        today = self.today()
        overdue = [t for t in self.store.tasks.open_through(today.isoformat())
                   if t.due_date < today.isoformat()]
        tasks = self.store.tasks.between(today.isoformat(), (today + timedelta(days=ahead)).isoformat())
        by_day: dict[str, list[Task]] = {}
        for t in tasks:
            by_day.setdefault(t.due_date, []).append(t)
        return DayView(overdue, [(date.fromisoformat(d), ts) for d, ts in sorted(by_day.items())])

    def open_now(self) -> list[Task]:
        """Open tasks due today or earlier: what a review needs to resolve."""
        return self.store.tasks.open_through(self.today().isoformat())

    def open_count_today(self) -> int:
        return len(self.open_now())

    def checkin_due(self) -> list[Task]:
        """Tasks worth interrupting for: overdue ones, or today's once the review time passed."""
        today = self.today().isoformat()
        tasks = self.open_now()
        if self.past_review_time():
            return tasks
        return [t for t in tasks if t.due_date < today]

    def tomorrow_count(self) -> int:
        day = (self.today() + timedelta(days=1)).isoformat()
        return self.store.tasks.count("due_date = ? AND status = 'open'", (day,))

    # -- the reminder text (what `idios remind` sends) ----------------------
    def reminder(self) -> Optional[tuple[str, str]]:
        tasks = self.open_now()
        done = self.store.tasks.count("due_date = ? AND status = 'done'",
                                      (self.today().isoformat(),))
        if not tasks:
            return None
        n = len(tasks)
        title = f"IDIOS: {n} thing{'s' if n != 1 else ''} still open today"
        lines = [f"• {t.text}" for t in tasks[:5]] + ([f"+ {n - 5} more"] if n > 5 else [])
        body = "\n".join(lines) + f"\nDone so far: {done}. Open IDIOS to review."
        return title, body

    # -- review ---------------------------------------------------------------
    def save_review(self, review: Review) -> Optional[Note]:
        """Keep the day's outcome as a searchable note."""
        if review.total == 0:
            return None
        parts = [f"Daily review {self.today().isoformat()}."]
        if review.done:
            parts.append("Done: " + "; ".join(t.text for t in review.done) + ".")
        if review.carried:
            parts.append("Moved to tomorrow: " + "; ".join(t.text for t in review.carried) + ".")
        if review.skipped:
            parts.append("Skipped: " + "; ".join(t.text for t in review.skipped) + ".")
        note = Note(text=" ".join(parts))
        return self.store.notes.add(note)

    def done_today(self) -> list[Task]:
        return self.store.tasks.find("due_date = ? AND status = 'done'", (self.today().isoformat(),))
