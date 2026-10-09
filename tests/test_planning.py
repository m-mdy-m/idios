"""did / plan / review, the reminder, and the scheduler. All offline, with a fake clock."""
import sqlite3
import subprocess
import sys
from datetime import date, datetime

import pytest

from conftest import run_shell
from idios.cli.main import main
from idios.domain.errors import IdiosError, Invalid
from idios.services import notify
from idios.services.app import App
from idios.services.dates import label, parse_clock, parse_when
from idios.services.schedule import ScheduleService, cron_line
from idios.shell.parser import parse
from idios.storage import db

THU = date(2026, 10, 8)                       # a Thursday


def at(hour, minute=0, day=8):
    return lambda: datetime(2026, 10, day, hour, minute)


# -- dates ------------------------------------------------------------------
@pytest.mark.parametrize("word, expected", [
    ("", date(2026, 10, 9)), ("tomorrow", date(2026, 10, 9)), ("today", THU),
    ("yesterday", date(2026, 10, 7)), ("monday", date(2026, 10, 12)),
    ("thursday", date(2026, 10, 15)),          # same weekday means next week
    ("fri", date(2026, 10, 9)), ("+3d", date(2026, 10, 11)), ("+2", date(2026, 10, 10)),
    ("2026-12-25", date(2026, 12, 25)),
])
def test_parse_when(word, expected):
    assert parse_when(word, THU) == expected


@pytest.mark.parametrize("bad", ["someday", "2026-13-40", "next week"])
def test_parse_when_rejects_nonsense(bad):
    with pytest.raises(Invalid):
        parse_when(bad, THU)


def test_labels_and_clock_times():
    assert label(THU, THU) == "Today · Thu 8 Oct"
    assert label(date(2026, 10, 9), THU) == "Tomorrow · Fri 9 Oct"
    assert label(date(2026, 10, 12), THU) == "Mon 12 Oct"
    assert parse_clock("9:05") == "09:05"
    for bad in ("25:00", "9", "nine"):
        with pytest.raises(Invalid):
            parse_clock(bad)


# -- parser -----------------------------------------------------------------
def test_parser_task_forms():
    for line, kind, text, arg in [
        ("did: Finished chapter 2", "did", "Finished chapter 2", ""),
        ("did yesterday: Read paper", "did", "Read paper", "yesterday"),
        ("plan: Read chapter 3", "plan", "Read chapter 3", ""),
        ("plan monday: Review", "plan", "Review", "monday"),
        ("Plan +2d: Review", "plan", "Review", "+2d"),
    ]:
        i = parse(line)
        assert (i.kind, i.text, i.arg) == (kind, text, arg), line
    i = parse("done tk1 tk2")
    assert (i.kind, i.text, i.arg) == ("task_mark", "tk1 tk2", "done")
    assert parse("skip tk3, tk4").arg == "skip"
    assert parse("done with the chapter").kind == "statement"      # ordinary sentences stay sentences
    assert parse("plan to read more tomorrow").kind == "statement"


# -- service ----------------------------------------------------------------
def test_plan_defaults_to_tomorrow_and_did_is_done_today(home):
    app = App(home, clock=at(22))
    planned = app.plan.add("Read chapter 3")
    done = app.plan.did("Finished exercises")
    assert planned.due_date == "2026-10-09" and planned.status.value == "open"
    assert done.due_date == "2026-10-08" and done.status.value == "done" and done.done_at
    assert app.plan.tomorrow_count() == 1
    app.close()


def test_empty_text_is_refused(app):
    with pytest.raises(Invalid):
        app.plan.add("   ")


def test_view_groups_by_day_and_separates_overdue(home):
    app = App(home, clock=at(10, day=8))
    app.plan.add("old", date(2026, 10, 6))
    app.plan.add("today", THU)
    app.plan.add("later", date(2026, 10, 12))
    app.plan.add("far away", date(2026, 12, 1))
    view = app.plan.view()
    assert [t.text for t in view.overdue] == ["old"]
    assert [(d, [t.text for t in ts]) for d, ts in view.days] == [(THU, ["today"]), (date(2026, 10, 12), ["later"])]
    app.close()


def test_checkin_only_after_review_time_or_when_overdue(home):
    for hour, expected in ((9, 0), (21, 1)):
        app = App(home, clock=at(hour))
        if hour == 9:
            app.plan.add("today", THU)
        assert len(app.plan.checkin_due()) == expected, hour
        app.close()
    app = App(home, clock=at(8, day=9))                     # the next morning: yesterday is overdue
    assert [t.text for t in app.plan.checkin_due()] == ["today"]
    app.close()


def test_review_time_is_configurable_and_validated(home):
    app = App(home, clock=at(20, 30))
    app.plan.add("x", THU)
    assert app.plan.review_time == "21:00" and not app.plan.past_review_time()
    app.plan.set_review_time("20:15")
    assert app.plan.past_review_time()
    with pytest.raises(Invalid):
        app.plan.set_review_time("late")
    app.close()


def test_reminder_lists_open_items_and_counts_done(home):
    app = App(home, clock=at(21))
    assert app.plan.reminder() is None                      # nothing planned: silence
    app.plan.add("Read chapter 3", THU)
    app.plan.add("Write notes", THU)
    app.plan.did("Exercises")
    title, body = app.plan.reminder()
    assert title == "IDIOS: 2 things still open today"
    assert "• Read chapter 3" in body and "Done so far: 1" in body
    app.close()


# -- shell: night routine and review ------------------------------------------
def test_night_before_session(home):
    out = run_shell(home, ["did: Finished chapter 2 exercises", "plan: Read chapter 3",
                           "plan monday: Review stack frames", ":plan"], clock=at(22))
    assert "✓ Done saved · tk1\n  Today · Thu 8 Oct" in out
    assert "✓ Planned · tk2\n  Tomorrow · Fri 9 Oct" in out
    assert "Mon 12 Oct\ntk3" in out and "tk1   ✓ Finished chapter 2 exercises" in out
    assert "tk2   ○ Read chapter 3" in out


def test_bad_day_and_empty_text_explain_themselves(home):
    out = run_shell(home, ["plan someday: x", "plan:", "done tk99"], clock=at(22))
    assert "I don't understand the day 'someday'" in out and "plan tomorrow:" in out
    assert "What is it?" in out
    assert "I couldn't find a task called tk99." in out


def test_done_and_skip_several_at_once_and_typos_change_nothing(home):
    run_shell(home, ["plan today: a", "plan today: b", "plan today: c", "done tk1 tk2", "skip tk3"], clock=at(10))
    app = App(home, clock=at(10))
    assert [t.status.value for t in app.store.tasks.all()] == ["done", "done", "skipped"]
    app.close()
    run_shell(home, ["plan today: d", "done tk4 tk99"], clock=at(10))
    app = App(home, clock=at(10))
    assert app.store.tasks.get_by_seq(4).status.value == "open"
    app.close()


def test_review_settles_each_item_and_saves_a_note(home):
    out = run_shell(home, ["plan today: Read", "plan today: Practise", "plan today: Tidy", ":review", "y", "n", "s"],
                    clock=at(21, 30))
    assert "Review  3 open" in out and "1 of 3 done" in out
    assert "→ Practise" in out and "Review saved as a note" in out
    app = App(home, clock=at(21, 30))
    by_text = {t.text: t for t in app.store.tasks.all()}
    assert by_text["Read"].status.value == "done" and by_text["Read"].done_at
    assert by_text["Practise"].due_date == "2026-10-09" and by_text["Practise"].status.value == "open"
    assert by_text["Tidy"].status.value == "skipped"
    note = app.store.notes.all()[-1].text
    assert "Done: Read." in note and "Moved to tomorrow: Practise." in note and "Skipped: Tidy." in note
    app.close()


def test_review_stops_cleanly_when_input_ends_and_leaves_the_rest(home):
    run_shell(home, ["plan today: a", "plan today: b", ":review", "y"], clock=at(21, 30))
    app = App(home, clock=at(21, 30))
    assert [t.status.value for t in app.store.tasks.all()] == ["done", "open"]
    app.close()


def test_review_with_nothing_open(home):
    out = run_shell(home, ["did: x", ":review"], clock=at(21))
    assert "Nothing open for today." in out and "Done so far: 1" in out


def test_review_ignores_garbage_then_leaves_task_open(home):
    out = run_shell(home, ["plan today: a", ":review", "what", "huh", "eh"], clock=at(21))
    assert out.count("Please answer y, n or s.") == 3
    app = App(home, clock=at(21))
    assert app.store.tasks.all()[0].status.value == "open"
    app.close()


def test_status_shows_open_tasks_for_today(home):
    assert "Open for today:\n  1" in run_shell(home, ["plan today: x", ":status"], clock=at(10))


# -- check-in on launch --------------------------------------------------------
def seed_open_task(home, clock):
    run_shell(home, ["plan today: Read chapter 3"], clock=clock)       # also marks intro as seen


def test_launch_after_review_time_offers_the_review(home):
    seed_open_task(home, at(10))
    out = run_shell(home, ["y", "y", ":quit"], interactive=True, clock=at(21, 30))
    assert "Plan check-in: 1 thing still open." in out and "Review now? [Y/n]" in out
    assert "1 of 1 done" in out
    app = App(home)
    assert app.store.tasks.all()[0].status.value == "done"
    app.close()


def test_launch_before_review_time_only_mentions_the_plan(home):
    seed_open_task(home, at(10))
    out = run_shell(home, [":quit"], interactive=True, clock=at(11))
    assert "Today's plan: 1 open" in out and "Review now?" not in out


def test_launch_the_next_day_finds_yesterdays_leftovers(home):
    seed_open_task(home, at(10))
    out = run_shell(home, ["n", ":quit"], interactive=True, clock=at(8, day=9))
    assert "Plan check-in" in out and "2026-10-08" in out
    assert "Review now? [Y/n] n" in out


def test_scripts_are_never_interrupted_by_a_check_in(home):
    seed_open_task(home, at(10))
    out = run_shell(home, [":quit"], interactive=False, clock=at(22))
    assert "Plan check-in" not in out


def test_schedule_hint_appears_once_after_planning_tomorrow(home):
    run_shell(home, ["plan: x"], clock=at(22))
    first = run_shell(home, [":quit"], interactive=True, clock=at(22))
    second = run_shell(home, [":quit"], interactive=True, clock=at(22))
    assert "Want a reminder at 21:00?" in first and "Want a reminder" not in second


# -- notifications --------------------------------------------------------------
def fake_which(*tools):
    return lambda name: f"/usr/bin/{name}" if name in tools else None


def test_notification_commands_per_platform():
    assert notify.command_for("T", "B", "linux", fake_which("notify-send"))[0] == "notify-send"
    mac = notify.command_for('He said "hi"', "B", "darwin", fake_which("osascript"))
    assert mac[0] == "osascript" and '\\"hi\\"' in mac[2]
    win = notify.command_for("It's", "B", "win32", fake_which("powershell"))
    assert win[0] == "powershell" and "It''s" in win[3]
    assert notify.command_for("T", "B", "linux", fake_which()) is None


def test_send_reports_failure_instead_of_raising():
    calls = []
    assert notify.send("T", "B", run=lambda *a, **k: calls.append(a), platform="linux",
                       which=fake_which("notify-send")) is True and calls
    def boom(*a, **k):
        raise subprocess.CalledProcessError(1, "x")
    assert notify.send("T", "B", run=boom, platform="linux", which=fake_which("notify-send")) is False
    assert notify.send("T", "B", platform="linux", which=fake_which()) is False


def test_notification_text_is_sanitised():
    cmd = notify.command_for("T\x1b[31m", "B\x00" + "x" * 1000, "linux", fake_which("notify-send"))
    assert "\x1b" not in cmd[2] and "\x00" not in cmd[3] and len(cmd[3]) <= 400


# -- scheduler (fake crontab) --------------------------------------------------
class FakeCron:
    def __init__(self, lines=None, available=True):
        self.lines, self.available = list(lines or []), available
        self.calls = []

    def run(self, cmd, **kw):
        self.calls.append(cmd)
        if cmd[:2] == ["crontab", "-l"]:
            if not self.lines:
                return subprocess.CompletedProcess(cmd, 1, "", "no crontab for root")
            return subprocess.CompletedProcess(cmd, 0, "\n".join(self.lines) + "\n", "")
        if cmd[:2] == ["crontab", "-"]:
            self.lines = kw["input"].splitlines()
            return subprocess.CompletedProcess(cmd, 0, "", "")
        raise AssertionError(cmd)

    def service(self):
        return ScheduleService(self.run, "linux", fake_which("crontab") if self.available else fake_which())


CMD = ["/usr/bin/python3", "-m", "idios", "remind"]


def test_cron_install_keeps_other_jobs_and_never_duplicates():
    cron = FakeCron(["0 3 * * * backup.sh"])
    svc = cron.service()
    assert svc.status().installed is False
    svc.install("21:00", CMD)
    svc.install("20:30", CMD)                                # moving the time replaces the line
    assert cron.lines[0] == "0 3 * * * backup.sh"
    assert [l for l in cron.lines if "idios-reminder" in l] == ["30 20 * * * /usr/bin/python3 -m idios remind # idios-reminder"]
    st = svc.status()
    assert st.installed and st.time == "20:30"
    assert svc.remove() is True and cron.lines == ["0 3 * * * backup.sh"]
    assert svc.remove() is False


def test_cron_on_a_fresh_machine_and_missing_cron():
    cron = FakeCron()
    cron.service().install("21:00", CMD)
    assert len(cron.lines) == 1
    with pytest.raises(IdiosError) as err:
        FakeCron(available=False).service().install("21:00", CMD)
    assert "crontab is not available" in err.value.message and "--dry-run" in err.value.hint


def test_cron_line_quotes_paths_with_spaces():
    line = cron_line("07:05", ["/my py/python", "-m", "idios", "--home", "/data/my notes", "remind"])
    assert line.startswith("5 7 * * * ") and "'/my py/python'" in line and "'/data/my notes'" in line


def test_windows_uses_task_scheduler():
    calls = []
    svc = ScheduleService(lambda cmd, **kw: calls.append(cmd) or subprocess.CompletedProcess(cmd, 0, "", ""),
                          "win32", fake_which("schtasks"))
    svc.install("21:00", ["C:\\Python\\python.exe", "-m", "idios", "remind"])
    assert calls[0][:2] == ["schtasks", "/Create"] and "/ST" in calls[0] and "21:00" in calls[0]
    assert "schtasks /Create" in svc.preview("21:00", CMD)


def test_shell_schedule_flow_asks_before_installing(home, monkeypatch):
    cron = FakeCron()
    monkeypatch.setattr(App, "__init__", _with_fake_scheduler(App.__init__, cron))
    out = run_shell(home, [":schedule 20:45", ":schedule install", "n", ":schedule install", "y", ":schedule", ":schedule remove"],
                    clock=at(22))
    assert "Review time set to 20:45" in out
    assert "Nothing was installed." in out and "✓ Reminder on" in out
    assert "Reminder: on  daily at 20:45" in out and "✓ Reminder removed" in out
    assert cron.lines == []


def _with_fake_scheduler(original, cron):
    def init(self, *a, **k):
        original(self, *a, **k)
        self.scheduler = cron.service()
    return init


def test_bad_review_time_in_shell(home):
    assert "'late' is not a time." in run_shell(home, [":schedule late"])


# -- CLI --------------------------------------------------------------------------
@pytest.fixture
def sent(monkeypatch):
    messages = []
    monkeypatch.setattr(notify, "send", lambda title, body, **k: messages.append((title, body)) or True)
    return messages


def test_remind_notifies_when_something_is_open(home, sent, capsys):
    a = App(home)
    a.plan.add("Read chapter 3", a.plan.today())
    a.close()
    assert main(["--home", str(home), "remind"]) == 0
    assert len(sent) == 1 and "still open today" in sent[0][0] and "Read chapter 3" in sent[0][1]
    assert capsys.readouterr().out == ""                    # cron stays quiet when the popup worked


def test_remind_is_silent_when_nothing_is_open(home, sent, capsys):
    App(home).close()
    assert main(["--home", str(home), "remind"]) == 0
    assert sent == [] and capsys.readouterr().out == ""


def test_remind_falls_back_to_text_without_a_notifier(home, monkeypatch, capsys):
    monkeypatch.setattr(notify, "send", lambda *a, **k: False)
    a = App(home)
    a.plan.add("Read", a.plan.today())
    a.close()
    main(["--home", str(home), "remind"])
    assert "still open today" in capsys.readouterr().out


def test_schedule_cli_dry_run_and_validation(home, capsys):
    assert main(["--home", str(home), "schedule", "install", "--at", "20:30", "--dry-run"]) == 0
    line = capsys.readouterr().out.strip()
    if sys.platform == "win32":                 # Task Scheduler instead of cron
        assert line.startswith("schtasks /Create /SC DAILY /ST 20:30") and "remind" in line
    else:
        assert line.startswith("30 20 * * *") and "--home" in line and line.endswith("remind # idios-reminder")
    assert main(["--home", str(home), "schedule", "--at", "25:00"]) == 1
    assert "is not a time" in capsys.readouterr().err


def test_plan_cli_and_status(home, capsys):
    a = App(home)
    a.plan.add("Read chapter 3")
    a.close()
    assert main(["--home", str(home), "plan"]) == 0
    assert "Read chapter 3" in capsys.readouterr().out
    assert main(["--home", str(home), "schedule"]) == 0
    assert "Daily reminder" in capsys.readouterr().out


# -- storage ---------------------------------------------------------------------
def test_upgrading_a_version_1_database_keeps_everything(tmp_path):
    path = tmp_path / "old.db"
    conn = sqlite3.connect(path)
    for statement in db.SCHEMA_V1:
        conn.execute(statement)
    conn.execute("PRAGMA user_version = 1")
    conn.execute("INSERT INTO sources (id, title, type, created_at, updated_at) "
                 "VALUES ('src_x', 'Old Book', 'book', '2026-01-01', '2026-01-01')")
    conn.commit()
    conn.close()
    upgraded = db.connect(path)
    assert upgraded.execute("PRAGMA user_version").fetchone()[0] == db.SCHEMA_VERSION
    assert upgraded.execute("SELECT title FROM sources").fetchone()[0] == "Old Book"
    assert upgraded.execute("SELECT COUNT(*) FROM tasks").fetchone()[0] == 0
    upgraded.close()


def test_tasks_are_searchable_exportable_and_deletable(home):
    app = App(home, clock=at(22))
    app.plan.add("Practise pointer arithmetic")
    app.plan.did("Finished stack chapter")
    assert {h.label for h in app.search.search("pointer")} == {"Task"}
    md = app.exporter.markdown()
    assert "## Plan" in md and "- [ ] 2026-10-09 Practise pointer arithmetic" in md
    assert "- [x] 2026-10-08 Finished stack chapter" in md
    assert "tasks" in app.exporter.json()
    task = app.store.tasks.all()[0]
    app.learning.delete(task)
    assert app.store.tasks.count() == 1
    app.close()


def test_wsl_notifications_use_windows_powershell():
    from idios.services import notify
    which = lambda name: "/x/" + name if name == "powershell.exe" else None  # noqa: E731
    cmd = notify.command_for("t", "b", platform="linux", which=which, wsl=True)
    assert cmd and cmd[0] == "powershell.exe"
    assert notify.command_for("t", "b", platform="linux", which=lambda n: None, wsl=True) is None


def test_session_env_points_cron_at_the_desktop_session():
    from idios.services import notify
    env = notify.session_env({"PATH": "/bin"})
    assert env["DISPLAY"] == ":0" and env["PATH"] == "/bin"
