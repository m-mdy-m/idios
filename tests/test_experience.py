"""Colour, help and the runnable examples."""
import subprocess
import sys
from pathlib import Path

import pytest

from conftest import run_shell
from idios.cli.main import main
from idios.services.app import App
from idios.shell import help as help_text
from idios.shell import render
from idios.shell.parser import parse
from idios.shell.style import Style, set_enabled, style

ROOT = Path(__file__).resolve().parents[1]
EXAMPLES = sorted((ROOT / "examples").glob("[0-9]*.idios"))


@pytest.fixture
def colour():
    set_enabled(True)
    yield
    set_enabled(False)


@pytest.fixture(autouse=True)
def plain_by_default():
    set_enabled(False)


# -- style ---------------------------------------------------------------
def test_style_is_a_no_op_when_disabled():
    s = Style(enabled=False)
    assert s.ok("x") == "x" and s.heading("Title") == "Title"


def test_style_wraps_text_in_ansi_when_enabled():
    s = Style(enabled=True)
    assert s.error("boom") == "\033[91mboom\033[0m"
    assert s.strip(s.ok("done")) == "done"


def test_colour_environment_switches(monkeypatch):
    monkeypatch.setenv("IDIOS_COLOR", "always")
    assert Style().enabled
    monkeypatch.setenv("IDIOS_COLOR", "never")
    assert not Style().enabled
    monkeypatch.setenv("IDIOS_COLOR", "auto")
    monkeypatch.setenv("NO_COLOR", "1")
    assert not Style().enabled


def test_readline_prompt_escapes_are_wrapped():
    wrapped = Style(True).readline_safe(Style(True).accent("> "))
    assert "\001\033[" in wrapped and "\002" in wrapped


def test_plain_output_never_contains_escapes(home):
    assert "\033" not in run_shell(home, ["goal: X", "Why?", ":status", ":help", ":search x"])


def test_coloured_output_keeps_the_same_text(home, colour):
    out = run_shell(home, ["goal: Learn Assembly", "book: B", "What is a register?", ":search register"])
    assert "\033[" in out
    plain = style.strip(out)
    assert "✓ Goal created" in plain and "SEARCH: register" in plain
    assert "Question — What is a register?" in plain


def test_search_matches_are_highlighted(home, colour):
    out = run_shell(home, ["What is a register?", ":search registers"])
    assert style.match("register") in out


def test_errors_are_red_with_a_hint(home, colour):
    out = run_shell(home, [":nope"])
    assert style.error("I don't know :nope.") in out and style.strip(out).count("Try:") == 1


# -- help ----------------------------------------------------------------
def test_overview_is_short_and_shows_examples():
    text = help_text.topic()
    assert len(text.splitlines()) <= 36
    for needle in ("Why do registers matter?", "note: ...", "link CPU Register to Memory",
                   "goal: Learn Assembly", ":search register", ":help learn", ":quit"):
        assert needle in text
    assert " ~ " not in text and "`" not in text


@pytest.mark.parametrize("name", help_text.TOPIC_NAMES)
def test_every_topic_renders_with_input_and_result_examples(name):
    text = help_text.topic(name)
    assert text.count("> ") >= 2 and " ~ " not in text and "`" not in text


def test_help_topics_in_the_shell(home):
    out = run_shell(home, [":help learn", ":help sources", ":help nonsense", ":help :search"])
    assert "Questions, answers, notes" in out
    assert "Register what you read" in out
    assert "There is no help topic called 'nonsense'." in out and ":help learn | sources" in out
    assert "Search everything you have recorded" in out


def test_help_examples_are_true_to_the_real_output(home):
    """Every `> input` in the examples topic, replayed, must give its documented ✓ line."""
    lines = help_text.TOPICS["examples"].splitlines()
    inputs = [l[2:].split("  ~ ")[0] for l in lines if l.startswith("> ") and not l.startswith("> :")]
    out = run_shell(home, inputs)
    for expected in [l for l in lines if l.startswith("✓")]:
        assert expected in out, expected


# -- new directives --------------------------------------------------------
def test_url_and_path_directives(home):
    out = run_shell(home, ["article: A", "url: https://example.org/a", "path: ~/Shelf/a.pdf", "path:"])
    assert out.count("✓ Reference saved") == 2 and "Where is it?" in out
    assert parse("url: https://x.org").kind == "reference"


# -- script runner and examples -------------------------------------------
def test_there_are_examples():
    assert len(EXAMPLES) >= 5


@pytest.mark.parametrize("script", EXAMPLES, ids=lambda p: p.name)
def test_example_runs_clean(script, tmp_path, capsys):
    assert main(["--home", str(tmp_path), "run", str(script)]) == 0
    out = capsys.readouterr().out
    assert "✓" in out
    assert "Something went wrong" not in out and "Traceback" not in out
    assert "I don't know" not in out and "Could not find" not in out
    # narration comes through, commands are echoed
    assert "# " in out and "\n> " in out


def test_run_shows_comments_dimmed_and_does_not_execute_them(tmp_path, capsys):
    script = tmp_path / "s.idios"
    script.write_text("# narrate\n\ngoal: X\n", encoding="utf-8")
    main(["--home", str(tmp_path / "h"), "run", str(script)])
    out = capsys.readouterr().out
    assert "# narrate" in out and "✓ Goal created" in out and "Ignored" not in out


def test_run_reports_a_missing_file(tmp_path, capsys):
    assert main(["--home", str(tmp_path), "run", str(tmp_path / "nope.idios")]) == 1
    assert "Could not read" in capsys.readouterr().err


def test_python_example_runs():
    result = subprocess.run([sys.executable, str(ROOT / "examples" / "06_python_api.py")],
                            capture_output=True, text=True, timeout=60,
                            env={"PYTHONPATH": str(ROOT / "src"), "PATH": "/usr/bin:/bin"})
    assert result.returncode == 0, result.stderr
    assert "CPU Register" in result.stdout


# -- psx -------------------------------------------------------------------
def test_psx_config_is_consistent_with_the_project():
    yaml = pytest.importorskip("yaml")
    cfg = yaml.safe_load((ROOT / "psx.yml").read_text())
    rules = cfg["rules"]
    assert cfg["project"]["type"] == "python"
    # Every rule that is switched on must have its files present (what `psx check` verifies).
    expected = {
        "readme": "README.md", "license": "LICENSE", "gitignore": ".gitignore",
        "gitattributes": ".gitattributes", "changelog": "CHANGELOG.md",
        "env_example": ".env.example", "package_manager": "pyproject.toml",
        "src_folder": "src", "tests_folder": "tests", "docs_folder": "docs",
        "scripts_folder": "scripts", "makefile": "Makefile",
        "contributing": "docs/CONTRIBUTING.md", "security": "SECURITY.md",
        "code_of_conduct": "CODE_OF_CONDUCT.md",
        "architecture": "docs/ARCHITECTURE.md",
        "pull_request_template": ".github/PULL_REQUEST_TEMPLATE.md",
        "issue_templates": ".github/ISSUE_TEMPLATE", "ci_config": ".github/workflows",
        "release_workflow": ".github/workflows/release.yml",
        "dependency_updates": ".github/dependabot.yml",
        "workflow_ci": ".github/workflows/ci.yml", "workflow_release": ".github/workflows/release.yml",
        "workflow_codeql": ".github/workflows/codeql.yml",
        "workflow_secret_scan": ".github/workflows/secret-scan.yml",
        "editorconfig": ".editorconfig", "pre_commit": ".pre-commit-config.yaml",
        "code_owners": ".github/CODEOWNERS",
    }
    for rule, path in expected.items():
        assert rules.get(rule), f"{rule} should be enabled in psx.yml"
        assert (ROOT / path).exists(), f"{rule}: missing {path}"
    for rule, severity in rules.items():
        if severity:
            assert rule in expected, f"{rule} is enabled but not verified here"


# -- intro and clear -------------------------------------------------------
from idios.shell import intro  # noqa: E402


def test_banner_art_spells_idios_in_plain_ascii():
    rows = intro.art()
    assert len(rows) == 5 and rows[0].startswith(" ___")
    assert rows[4] == "|___||____/ |___| \\___/ |____/"
    assert all(ch.isascii() for row in rows for ch in row)


def test_intro_is_shown_once_to_a_new_interactive_user(home):
    first = run_shell(home, ["", ":quit"], interactive=True)
    assert "|____/" in first and "Press Enter to begin" in first
    assert first.count("<clear>") == 0 or "<clear>" in first       # screen cleared around it
    assert "A personal cognitive and learning operating system." in first
    second = run_shell(home, [":quit"], interactive=True)
    assert "A personal cognitive" not in second and "Press Enter" not in second
    assert "|____/" in second and "learn · connect · recall" in second   # logo on every start


def test_logo_is_shown_to_existing_users_and_can_be_turned_off(home, tmp_path, monkeypatch):
    run_shell(tmp_path / "old", ["goal: X"])
    assert "|____/" in run_shell(tmp_path / "old", ["y", ":quit"], interactive=True)
    monkeypatch.setenv("IDIOS_NO_LOGO", "1")
    assert "|____/" not in run_shell(tmp_path / "old", ["y", ":quit"], interactive=True)


def test_intro_is_skipped_for_scripts_and_existing_users(home, tmp_path):
    assert "A personal cognitive" not in run_shell(home, [":quit"])           # non-interactive
    run_shell(tmp_path / "old", ["goal: X"])                                  # has data, never saw intro
    assert "A personal cognitive" not in run_shell(tmp_path / "old", ["y", ":quit"], interactive=True)
    assert "|____/" not in run_shell(home, [":quit"])


def test_intro_can_be_replayed(home):
    out = run_shell(home, [":intro"])
    assert "A personal cognitive and learning operating system." in out and "Try it" in out


def test_clear_wipes_the_screen_and_redraws_the_context(home):
    out = run_shell(home, ["goal: Learn Assembly", "book: B", "clear", ":status"])
    after = out[out.index("<clear>"):]
    assert "Goal: Learn Assembly" in after and "Source: B" in after
    assert out.count("<clear>") == 1
    for word in (":clear", "cls"):
        assert "<clear>" in run_shell(home, [word])


def test_clear_changes_no_data(home):
    run_shell(home, ["goal: X", "Why?", "clear"])
    app = App(home)
    assert app.store.questions.count() == 1 and app.context.goal.title == "X"
    app.close()


def test_forget_replaces_the_old_clear_for_concept_and_question(home):
    run_shell(home, ["concept: A", "Why?", ":forget"])
    app = App(home)
    assert app.context.concept is None and app.context.question is None
    app.close()


def test_console_clear_only_writes_escapes_on_a_terminal(capsys):
    from idios.shell.io import ConsoleIO
    ConsoleIO().clear()                     # pytest captures stdout: not a tty
    assert capsys.readouterr().out == ""


def test_parser_reads_clear_as_a_directive():
    for word in ("clear", "cls", ":clear"):
        assert parse(word).kind == "directive"
    assert parse("clear skies are nice").kind == "statement"
