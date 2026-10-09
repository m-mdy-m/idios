"""Install, inspect and remove the OS job that runs ``idios remind`` every day.

IDIOS has no background process of its own. The operating system's scheduler
(cron on Linux/macOS, Task Scheduler on Windows) runs a short command at the
review time. Nothing is installed unless the user asks, and ``dry_run`` shows
exactly what would be written.
"""
from __future__ import annotations

import shlex
import shutil
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, Optional

from idios.domain.errors import IdiosError
from idios.services.dates import parse_clock

TAG = "# idios-reminder"
TASK_NAME = "IDIOS-Reminder"
Runner = Callable[..., "subprocess.CompletedProcess[str]"]


@dataclass
class JobStatus:
    installed: bool
    time: Optional[str] = None
    detail: str = ""


def job_command(home: Optional[Path] = None) -> list[str]:
    """The exact command the scheduler runs."""
    cmd = [sys.executable, "-m", "idios"]
    if home is not None:
        cmd += ["--home", str(home)]
    return cmd + ["remind"]


def cron_line(hhmm: str, command: list[str]) -> str:
    hour, minute = hhmm.split(":")
    return f"{int(minute)} {int(hour)} * * * {shlex.join(command)} {TAG}"


class ScheduleService:
    def __init__(self, run: Runner = subprocess.run, platform: str = sys.platform,
                 which: Callable[[str], Optional[str]] = shutil.which) -> None:
        self._run, self.platform, self._which = run, platform, which

    @property
    def windows(self) -> bool:
        return self.platform == "win32"

    # -- what would be written ------------------------------------------------
    def preview(self, at: str, command: list[str]) -> str:
        at = parse_clock(at)
        if self.windows:
            return (f'schtasks /Create /SC DAILY /ST {at} /TN {TASK_NAME} '
                    f'/TR "{subprocess.list2cmdline(command)}" /F')
        return cron_line(at, command)

    # -- doing it ---------------------------------------------------------------
    def install(self, at: str, command: list[str]) -> str:
        at = parse_clock(at)
        if self.windows:
            self._need("schtasks")
            self._check(self._run(["schtasks", "/Create", "/SC", "DAILY", "/ST", at, "/TN", TASK_NAME,
                                   "/TR", subprocess.list2cmdline(command), "/F"],
                                  capture_output=True, text=True))
            return f"Task Scheduler: {TASK_NAME} daily at {at}"
        lines = [l for l in self._crontab() if TAG not in l]
        lines.append(cron_line(at, command))
        self._write_crontab(lines)
        return f"crontab: daily at {at}"

    def remove(self) -> bool:
        """Returns True if a job was removed."""
        if self.windows:
            self._need("schtasks")
            proc = self._run(["schtasks", "/Delete", "/TN", TASK_NAME, "/F"],
                             capture_output=True, text=True)
            return proc.returncode == 0
        lines = self._crontab()
        kept = [l for l in lines if TAG not in l]
        if len(kept) == len(lines):
            return False
        self._write_crontab(kept)
        return True

    def status(self) -> JobStatus:
        try:
            if self.windows:
                if not self._which("schtasks"):
                    return JobStatus(False, detail="Task Scheduler not found")
                proc = self._run(["schtasks", "/Query", "/TN", TASK_NAME, "/FO", "LIST"],
                                 capture_output=True, text=True)
                return JobStatus(proc.returncode == 0, detail="Task Scheduler")
            for line in self._crontab():
                if TAG in line:
                    minute, hour = line.split()[:2]
                    return JobStatus(True, f"{int(hour):02d}:{int(minute):02d}", "crontab")
        except IdiosError:
            return JobStatus(False, detail="crontab not available")
        return JobStatus(False)

    # -- internals ------------------------------------------------------------
    def _need(self, tool: str) -> None:
        if not self._which(tool):
            raise IdiosError(f"{tool} is not available on this computer.",
                             hint="idios schedule install --dry-run   (shows the line to add yourself)")

    def _check(self, proc) -> None:
        if proc.returncode != 0:
            raise IdiosError("The scheduler refused the job.", hint=(proc.stderr or "").strip()[:200])

    def _crontab(self) -> list[str]:
        self._need("crontab")
        proc = self._run(["crontab", "-l"], capture_output=True, text=True)
        if proc.returncode != 0:  # "no crontab for user" is normal on a fresh machine
            return []
        return [l for l in proc.stdout.splitlines()]

    def _write_crontab(self, lines: list[str]) -> None:
        text = "\n".join(lines).rstrip("\n")
        text = text + "\n" if text else ""
        proc = self._run(["crontab", "-"], input=text, capture_output=True, text=True)
        self._check(proc)
