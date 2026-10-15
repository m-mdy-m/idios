"""Modern Windows Toasts and click-to-open activation for IDIOS.

The Windows provider uses the WinRT ``ToastNotification`` API with the
``ToastGeneric`` template. It creates a per-user Start Menu shortcut with an
explicit AppUserModelID (required by classic desktop Toasts), and registers an
``idios:`` URI handler so clicking a Toast launches the schedule window even
when the scheduled reminder process has already exited.
"""
from __future__ import annotations

import html
import os
import shutil
import subprocess
import sys
from pathlib import Path
from typing import Optional

from .activation import PROTOCOL, activation_uri

APP_ID = "IDIOS.Desktop"


# This small helper uses the Windows Shell property store to set
# System.AppUserModel.ID on a .lnk. WScript.Shell can create shortcuts, but it
# cannot set that shell property itself.
_SHORTCUT_PROPERTY_HELPER = r'''
using System;
using System.Runtime.InteropServices;

[StructLayout(LayoutKind.Sequential)]
public struct IdiosPropertyKey
{
    public Guid formatId;
    public uint propertyId;
}

[StructLayout(LayoutKind.Explicit, Size = 24)]
public struct IdiosPropVariant
{
    [FieldOffset(0)] public ushort variantType;
    [FieldOffset(8)] public IntPtr wideString;
}

[ComImport]
[Guid("886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99")]
[InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
public interface IdiosIPropertyStore
{
    [PreserveSig] int GetCount(out uint count);
    [PreserveSig] int GetAt(uint index, out IdiosPropertyKey key);
    [PreserveSig] int GetValue(ref IdiosPropertyKey key, out IdiosPropVariant value);
    [PreserveSig] int SetValue(ref IdiosPropertyKey key, ref IdiosPropVariant value);
    [PreserveSig] int Commit();
}

public static class IdiosToastShortcut
{
    [DllImport("shell32.dll", CharSet = CharSet.Unicode, PreserveSig = true)]
    private static extern int SHGetPropertyStoreFromParsingName(
        string path, IntPtr bindContext, uint flags,
        ref Guid interfaceId, out IdiosIPropertyStore propertyStore);

    public static void SetAppUserModelId(string shortcutPath, string appId)
    {
        Guid interfaceId = new Guid("886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99");
        IdiosIPropertyStore propertyStore;
        int result = SHGetPropertyStoreFromParsingName(
            shortcutPath, IntPtr.Zero, 2, ref interfaceId, out propertyStore); // GPS_READWRITE
        if (result < 0) Marshal.ThrowExceptionForHR(result);

        IdiosPropertyKey key = new IdiosPropertyKey();
        key.formatId = new Guid("9F4C2855-9F79-4B39-A8D0-E1D42DE1D5F3");
        key.propertyId = 5; // PKEY_AppUserModel_ID
        IntPtr appIdPointer = Marshal.StringToCoTaskMemUni(appId);
        IdiosPropVariant value = new IdiosPropVariant();
        value.variantType = 31; // VT_LPWSTR
        value.wideString = appIdPointer;
        try
        {
            result = propertyStore.SetValue(ref key, ref value);
            if (result < 0) Marshal.ThrowExceptionForHR(result);
            result = propertyStore.Commit();
            if (result < 0) Marshal.ThrowExceptionForHR(result);
        }
        finally
        {
            Marshal.FreeCoTaskMem(appIdPointer);
            if (Marshal.IsComObject(propertyStore)) Marshal.ReleaseComObject(propertyStore);
        }
    }
}
'''


def _ps_quote(value: str) -> str:
    """Quote a value for a PowerShell single-quoted literal."""
    return "'" + value.replace("'", "''") + "'"


def register_protocol_handler() -> bool:
    """Register a per-user URI handler; no administrator privileges are needed."""
    if sys.platform != "win32":
        return False
    try:
        import winreg
    except ImportError:
        return False

    python = Path(sys.executable)
    windowless = python.with_name("pythonw.exe")
    launcher = windowless if windowless.exists() else python
    command = f'"{launcher}" -m idios --_open-schedule "%1"'
    try:
        with winreg.CreateKey(winreg.HKEY_CURRENT_USER, rf"Software\Classes\{PROTOCOL}") as key:
            winreg.SetValueEx(key, None, 0, winreg.REG_SZ, "URL:IDIOS Schedule")
            winreg.SetValueEx(key, "URL Protocol", 0, winreg.REG_SZ, "")
        with winreg.CreateKey(
            winreg.HKEY_CURRENT_USER,
            rf"Software\Classes\{PROTOCOL}\shell\open\command",
        ) as key:
            winreg.SetValueEx(key, None, 0, winreg.REG_SZ, command)
    except OSError:
        return False
    return True


def _powershell_executable() -> Optional[str]:
    found = shutil.which("powershell.exe")
    if found:
        return found
    root = Path(os.environ.get("SystemRoot", r"C:\Windows"))
    fallback = root / "System32" / "WindowsPowerShell" / "v1.0" / "powershell.exe"
    return str(fallback) if fallback.exists() else None


def _toast_xml(title: str, body: str, uri: str) -> str:
    """Create safe, single-line ToastGeneric XML with body and button activation."""
    title_xml = html.escape(title, quote=False)
    body_xml = html.escape(body, quote=False)
    uri_xml = html.escape(uri, quote=True)
    return (
        f'<toast activationType="protocol" launch="{uri_xml}" duration="long">'
        '<visual><binding template="ToastGeneric">'
        f'<text>{title_xml}</text><text hint-maxLines="3">{body_xml}</text>'
        '</binding></visual>'
        '<actions><action activationType="protocol" content="Open schedule" '
        f'arguments="{uri_xml}" /></actions>'
        '<audio src="ms-winsoundevent:Notification.Default" />'
        '</toast>'
    )


def _powershell_script(title: str, body: str, uri: str, launcher: Path) -> str:
    """Build the native Toast + shortcut-registration script."""
    working_directory = str(Path(sys.executable).resolve().parent)
    toast_xml = _toast_xml(title, body, uri)
    shortcut_args = "-m idios --_open-schedule idios://schedule/open"
    return f"""$ErrorActionPreference = 'Stop'
try {{
    $startMenu = Join-Path $env:APPDATA 'Microsoft\\Windows\\Start Menu\\Programs'
    New-Item -ItemType Directory -Path $startMenu -Force | Out-Null
    $shortcutPath = Join-Path $startMenu 'IDIOS Schedule.lnk'
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($shortcutPath)
    $shortcut.TargetPath = {_ps_quote(str(launcher))}
    $shortcut.Arguments = {_ps_quote(shortcut_args)}
    $shortcut.WorkingDirectory = {_ps_quote(working_directory)}
    $shortcut.Description = 'IDIOS learning schedule'
    $shortcut.Save()

    Add-Type -TypeDefinition @'
{_SHORTCUT_PROPERTY_HELPER}
'@
    [IdiosToastShortcut]::SetAppUserModelId($shortcutPath, {_ps_quote(APP_ID)})

    [Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
    [Windows.UI.Notifications.ToastNotification, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
    [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
    $toastXmlText = @'
{toast_xml}
'@
    $toastXml = New-Object Windows.Data.Xml.Dom.XmlDocument
    $toastXml.LoadXml($toastXmlText)
    $toast = [Windows.UI.Notifications.ToastNotification]::new($toastXml)
    $notifier = [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier({_ps_quote(APP_ID)})
    $notifier.Show($toast)
    exit 0
}} catch {{
    [Console]::Error.WriteLine($_.Exception.Message)
    exit 1
}}
"""


def send_toast(title: str, body: str, home: Optional[Path]) -> bool:
    """Display a modern WinRT Toast with persistent click activation."""
    if sys.platform != "win32":
        return False
    powershell = _powershell_executable()
    if not powershell or not register_protocol_handler():
        return False

    python = Path(sys.executable)
    windowless = python.with_name("pythonw.exe")
    launcher = windowless if windowless.exists() else python
    uri = activation_uri(home)
    script = _powershell_script(title, body, uri, launcher)
    try:
        kwargs = {
            "check": True,
            "timeout": 25,
            "capture_output": True,
            "text": True,
        }
        no_window = getattr(subprocess, "CREATE_NO_WINDOW", 0)
        if no_window:
            kwargs["creationflags"] = no_window
        subprocess.run([powershell, "-NoProfile", "-NonInteractive", "-Command", script], **kwargs)
    except (OSError, subprocess.SubprocessError):
        return False
    return True
