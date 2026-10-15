from pathlib import Path
import subprocess

from idios.services.schedule import ScheduleService


def test_windows_schedule_is_interactive_for_toast_delivery() -> None:
    service = ScheduleService(platform="win32", which=lambda _name: "schtasks")
    preview = service.preview("21:00", ["python.exe", "-m", "idios", "remind"])
    assert "/IT" in preview

    calls = []

    def fake_run(command, **kwargs):
        calls.append((command, kwargs))
        return subprocess.CompletedProcess(command, 0, stdout="", stderr="")

    service = ScheduleService(run=fake_run, platform="win32", which=lambda _name: "schtasks")
    result = service.install("21:00", ["python.exe", "-m", "idios", "remind"])
    assert "Task Scheduler" in result
    assert "/IT" in calls[0][0]
