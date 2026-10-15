from datetime import date, datetime
from pathlib import Path

from idios.services.app import App
from idios.ui.schedule_dialog import _schedule_data, schedule_text


def test_schedule_dialog_text_contains_schedule_details(tmp_path: Path) -> None:
    now = datetime(2026, 10, 15, 21, 0)
    app = App(tmp_path, clock=lambda: now)
    try:
        app.plan.add("Overdue reading", date(2026, 10, 14))
        app.plan.add("Read chapter 4", date(2026, 10, 15))
        app.plan.add("Review notes", date(2026, 10, 17))
        app.plan.did("Finished exercises", date(2026, 10, 15))
        rendered = schedule_text(app)
        assert "Daily review: 21:00" in rendered
        assert "Overdue reading" in rendered
        assert "Read chapter 4" in rendered
        assert "Finished exercises" in rendered
        assert "Review notes" in rendered
        assert "read-only" in rendered.lower()
    finally:
        app.close()


def test_schedule_snapshot_groups_items_for_dashboard_cards(tmp_path: Path) -> None:
    now = datetime(2026, 10, 15, 21, 0)
    app = App(tmp_path, clock=lambda: now)
    try:
        app.plan.add("Late task", date(2026, 10, 14))
        app.plan.add("Today task", date(2026, 10, 15))
        app.plan.add("Upcoming task", date(2026, 10, 17))
        skipped = app.plan.add("Skipped task", date(2026, 10, 15))
        app.plan.skip(skipped)
        app.plan.did("Finished task", date(2026, 10, 15))

        snapshot = _schedule_data(app)

        assert [task.text for task in snapshot["overdue"]] == ["Late task"]
        assert [task.text for task in snapshot["due_today"]] == ["Today task"]
        assert [task.text for task in snapshot["upcoming"]] == ["Upcoming task"]
        assert [task.text for task in snapshot["skipped_today"]] == ["Skipped task"]
        assert [task.text for task in snapshot["completed_today"]] == ["Finished task"]
        assert snapshot["review_time"] == "21:00"
    finally:
        app.close()
