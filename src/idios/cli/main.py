"""Command line entry point.

``idios`` opens the interactive learning environment, which is the main
interface. The few subcommands below are conveniences for use from scripts
and other shells; everything they do is also possible inside the shell.
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path
from typing import Optional, Sequence

from idios import __version__
from idios.domain.errors import IdiosError
from idios.services.app import App
from idios.shell import render
from idios.shell.io import ConsoleIO, ScriptIO
from idios.shell.shell import Shell


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="idios",
        description="IDIOS: a terminal notebook for learning. "
                    "Run with no arguments to start.",
    )
    parser.add_argument("--version", action="version", version=f"idios {__version__}")
    parser.add_argument("--home", type=Path, help="data folder to use instead of ~/.idios")
    sub = parser.add_subparsers(dest="command")

    search = sub.add_parser("search", help="search everything you have recorded")
    search.add_argument("query", nargs="+")

    show = sub.add_parser("show", help="show a source, concept, question or goal")
    show.add_argument("what", nargs="+", help="a name or an id such as q3")

    sub.add_parser("shelf", help="list your registered sources")

    sub.add_parser("plan", help="show what you planned: overdue, today and the week ahead")
    sub.add_parser("review", help="settle today's open tasks: done, tomorrow, or skip")
    sub.add_parser("remind", help="send today's reminder (this is what the scheduler runs)")

    sched = sub.add_parser("schedule", help="set up the daily reminder with your OS scheduler")
    sched.add_argument("action", nargs="?", default="status", choices=["status", "install", "remove", "test"])
    sched.add_argument("--at", metavar="HH:MM", help="review time, 24-hour (default 21:00)")
    sched.add_argument("--dry-run", action="store_true", help="show what would be installed")
    sched.add_argument("--yes", "-y", action="store_true", help="do not ask for confirmation")

    run = sub.add_parser("run", help="play a script of inputs, like the files in examples/")
    run.add_argument("file", type=Path)

    export = sub.add_parser("export", help="write your knowledge to a file")
    export.add_argument("--format", "-f", default="markdown", choices=["markdown", "md", "json"])
    export.add_argument("--output", "-o", type=Path, help="where to write (default: ~/.idios/exports)")
    return parser


def main(argv: Optional[Sequence[str]] = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        app = App(args.home)
    except OSError as err:
        print(f"Could not open the IDIOS data folder.\n\n{err}", file=sys.stderr)
        return 1
    try:
        return _dispatch(app, args)
    except IdiosError as err:
        print(err.render(), file=sys.stderr)
        return 1
    except BrokenPipeError:  # e.g. `idios search x | head`
        return 0
    finally:
        app.close()


def _dispatch(app: App, args: argparse.Namespace) -> int:
    if args.command is None:
        io = ConsoleIO(app.home / "history")
        try:
            Shell(app, io, interactive=sys.stdin.isatty()).run()
        finally:
            io.close()
        return 0

    if args.command == "remind":
        return _remind(app)
    if args.command == "schedule":
        return _schedule(app, args)
    if args.command == "review":
        shell = Shell(app, ConsoleIO(), interactive=sys.stdin.isatty())
        shell.handle(":review")
        return 0
    if args.command == "plan":
        Shell(app, ConsoleIO(), interactive=False).handle(":plan")
        return 0

    if args.command == "run":
        try:
            lines = args.file.read_text(encoding="utf-8").splitlines()
        except OSError as err:
            raise IdiosError(f"Could not read {args.file}.", hint=str(err)) from err
        Shell(app, ScriptIO(lines), interactive=False).run()
        return 0

    io = ConsoleIO()
    shell = Shell(app, io, interactive=False)
    if args.command == "search":
        shell.handle(":search " + " ".join(args.query))
    elif args.command == "show":
        shell.handle(":show " + " ".join(args.what))
    elif args.command == "shelf":
        shell.handle(":shelf")
    elif args.command == "export":
        path = app.exporter.export(args.format, args.output)
        print(f"{render.OK} Exported to {path}")
    return 0


def _remind(app: App) -> int:
    """Run by the OS scheduler. Silent when there is nothing open."""
    reminder = app.plan.reminder()
    if reminder is None:
        return 0
    title, body = reminder
    if not app.notify(title, body):  # no notification tool: stdout (cron mails it)
        print(f"{title}\n{body}")
    return 0


def _schedule(app: App, args: argparse.Namespace) -> int:
    plan, sched = app.plan, app.scheduler
    if args.at:
        plan.set_review_time(args.at)
    command = app.reminder_command()
    if args.action == "install":
        line = sched.preview(plan.review_time, command)
        if args.dry_run:
            print(line)
            return 0
        if not args.yes:
            print(f"This adds a daily job at {plan.review_time}:\n  {line}")
            if input("Install it? [y/N] ").strip().lower() not in ("y", "yes"):
                print("Nothing was installed.")
                return 0
        print(f"{render.OK} Reminder on · {sched.install(plan.review_time, command)}")
    elif args.action == "remove":
        print(f"{render.OK} Reminder removed" if sched.remove() else "No reminder was installed.")
    elif args.action == "test":
        ok = app.notify("IDIOS", "This is how your daily reminder will look.")
        print(f"{render.OK} Notification sent" if ok else "No notification tool found on this computer.")
    else:
        st = sched.status()
        print(render.schedule_status(plan.review_time, st.installed, st.time or "", st.detail,
                                     sched.preview(plan.review_time, command)))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
