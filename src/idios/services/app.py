"""The application: one object that wires the services over one database."""
from __future__ import annotations

from pathlib import Path
from typing import Optional

from idios.services.context import ContextService
from idios.services.export import ExportService
from idios.services.knowledge import KnowledgeService
from idios.services.learning import LearningService
from idios.services.search import SearchService
from idios.services.sources import SourceService
from idios.services.views import ViewService
from idios.storage.db import connect, default_home
from idios.storage.repos import Store


class App:
    def __init__(self, home: Optional[Path] = None) -> None:
        self.home = Path(home) if home else default_home()
        self.store = Store(connect(self.home / "idios.db"))
        self.context = ContextService(self.store)
        self.knowledge = KnowledgeService(self.store, self.context)
        self.sources = SourceService(self.store, self.context, self.knowledge)
        self.learning = LearningService(self.store, self.context)
        self.search = SearchService(self.store)
        self.views = ViewService(self.store, self.context, self.knowledge, self.sources)
        self.exporter = ExportService(self.store, self.knowledge, self.sources, self.home)

    def close(self) -> None:
        self.context.end_session()
        self.store.close()
