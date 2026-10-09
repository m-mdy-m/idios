"""The application: one object that wires the services over one database."""
from __future__ import annotations

from datetime import datetime
from pathlib import Path
from typing import Callable, Optional

from idios.services.context import ContextService
from idios.services.export import ExportService
from idios.services.knowledge import KnowledgeService
from idios.services.learning import LearningService
from idios.services.plan import PlanService
from idios.services.schedule import ScheduleService, job_command
from idios.services import notify
from idios.services.search import SearchService
from idios.services.sources import SourceService
from idios.services.views import ViewService
from idios.storage.db import connect, default_home
from idios.storage.repos import Store


class App:
    def __init__(self, home: Optional[Path] = None,
                 clock: Callable[[], datetime] = datetime.now) -> None:
        """``clock`` exists so tests can say what time it is."""
        self.home = Path(home) if home else default_home()
        self.store = Store(connect(self.home / "idios.db"))
        self.context = ContextService(self.store)
        self.knowledge = KnowledgeService(self.store, self.context)
        self.sources = SourceService(self.store, self.context, self.knowledge)
        self.learning = LearningService(self.store, self.context)
        self.plan = PlanService(self.store, self.context, clock)
        self.scheduler = ScheduleService()
        self.notify = notify.send
        self.search = SearchService(self.store)
        self.views = ViewService(self.store, self.context, self.knowledge, self.sources,
                                 plan_open=self.plan.open_now)
        self.exporter = ExportService(self.store, self.knowledge, self.sources, self.home)

    def reminder_command(self) -> list[str]:
        """What the OS scheduler should run; pins the data folder only if it is not the default."""
        custom = None if self.home == default_home() else self.home
        return job_command(custom)

    def close(self) -> None:
        self.context.end_session()
        self.store.close()
