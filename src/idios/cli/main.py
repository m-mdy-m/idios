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
        description="IDIOS: a local-first personal learning environment. "
                    "Run with no arguments to start learning.",
    )
    parser.add_argument("--version", action="version", version=f"idios {__version__}")
    parser.add_argument("--home", type=Path, help="data folder to use instead of ~/.idios")
    sub = parser.add_subparsers(dest="command")

    search = sub.add_parser("search", help="search everything you have recorded")
    search.add_argument("query", nargs="+")

    show = sub.add_parser("show", help="show a source, concept, question or goal")
    show.add_argument("what", nargs="+", help="a name or an id such as q3")

    sub.add_parser("shelf", help="list your registered sources")

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


if __name__ == "__main__":
    raise SystemExit(main())
