from pathlib import Path
import subprocess

import pytest

from idios.services.notifications.activation import home_from_activation_uri
from idios.services.notify import command_for
from idios.services.notifications import clean
from idios.services.notifications.activation import activation_uri
from idios.services.notifications.desktop import _send_notify_send


def test_clean_single_lines_and_limits_body() -> None:
    assert clean("  hello\n world  ", 100) == "hello world"
    assert clean("x" * 10, 6) == "xxxxx…"
    assert clean("a]]>b", 20) == "a]]>b"


def test_activation_uri_round_trips_custom_home(tmp_path: Path) -> None:
    uri = activation_uri(tmp_path)
    assert uri.startswith("idios://schedule/open?")
    assert home_from_activation_uri(uri) == tmp_path.resolve()


def test_activation_uri_without_home_uses_default() -> None:
    assert home_from_activation_uri("idios://schedule/open") is None


@pytest.mark.parametrize("uri", [
    "https://example.com/schedule/open",
    "idios://something/open",
    "idios://schedule/delete",
    "idios://schedule/open?unknown=",
    "idios://schedule/open#fragment",
    "idios://schedule/open?home=",
    "idios://schedule/open/?home=%2Ftmp",
])
def test_activation_uri_rejects_unknown_targets(uri: str) -> None:
    with pytest.raises(ValueError):
        home_from_activation_uri(uri)


def test_windows_does_not_select_deprecated_balloon_api() -> None:
    assert command_for("title", "body", platform="win32", which=lambda _name: "powershell") is None


def test_notify_send_default_action_opens_schedule() -> None:
    calls = []
    shown = []

    def fake_run(command, **kwargs):
        calls.append(command)
        return subprocess.CompletedProcess(command, 0, stdout="default\n", stderr="")

    sent = _send_notify_send(
        "IDIOS", "Open schedule", fake_run, lambda name: "/usr/bin/notify-send" if name == "notify-send" else None,
        lambda: shown.append(True), 5,
    )
    assert sent is True
    assert shown == [True]
    assert "--action=default=Open IDIOS" in calls[0]
    assert "--wait" in calls[0]


def test_windows_toast_xml_escapes_content_and_uses_protocol_activation() -> None:
    from idios.services.notifications.windows import _toast_xml

    xml = _toast_xml('IDIOS & <plan>', 'Read "chapter" & review', 'idios://schedule/open?home=C%3A%5Cidios')
    assert '<text>IDIOS &amp; &lt;plan&gt;</text>' in xml
    assert '<text hint-maxLines="3">Read "chapter" &amp; review</text>' in xml
    assert 'activationType="protocol"' in xml
    assert 'idios://schedule/open?home=C%3A%5Cidios' in xml
    assert 'ShowBalloonTip' not in xml


def test_windows_toast_registers_start_menu_shortcut_and_app_id() -> None:
    from pathlib import Path
    from idios.services.notifications.windows import APP_ID, _powershell_script

    script = _powershell_script("IDIOS", "Schedule details", "idios://schedule/open", Path("C:/Python/pythonw.exe"))
    assert "IDIOS Schedule.lnk" in script
    assert "SetAppUserModelId" in script
    assert "IntPtr.Zero, 2, ref interfaceId" in script  # GPS_READWRITE
    assert APP_ID in script
    assert "ToastNotificationManager" in script
    assert "ToastGeneric" in script


def test_macos_terminal_notifier_cold_launches_schedule(monkeypatch, tmp_path: Path) -> None:
    import shlex
    from idios.services.notifications import desktop

    calls = []

    def fake_run(command, **kwargs):
        calls.append((command, kwargs))
        return subprocess.CompletedProcess(command, 0, stdout="", stderr="")

    monkeypatch.setattr(desktop.subprocess, "run", fake_run)
    home = tmp_path / "idios data folder"
    sent = desktop._send_terminal_notifier(
        "IDIOS", "Review now", home,
        lambda name: "/usr/local/bin/terminal-notifier" if name == "terminal-notifier" else None,
    )
    assert sent is True
    command = calls[0][0]
    assert command[0] == "/usr/local/bin/terminal-notifier"
    execute = command[command.index("-execute") + 1]
    arguments = shlex.split(execute)
    assert "--_open-schedule" in arguments
    uri = arguments[arguments.index("--_open-schedule") + 1]
    assert uri.startswith("idios://schedule/open?home=")
    assert "idios data folder" not in uri  # query component is URI-encoded
