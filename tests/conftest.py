import pytest

from idios.services.app import App
from idios.shell.io import ScriptedIO
from idios.shell.shell import Shell


@pytest.fixture
def home(tmp_path):
    return tmp_path / "idios-home"


@pytest.fixture
def app(home):
    a = App(home)
    yield a
    a.close()


def run_shell(home, lines, interactive=False):
    """Run a whole IDIOS session against ``home`` and return the transcript."""
    app = App(home)
    io = ScriptedIO(lines)
    try:
        Shell(app, io, interactive=interactive).run()
    finally:
        app.close()
    return io.text
