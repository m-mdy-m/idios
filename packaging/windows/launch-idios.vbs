Option Explicit

Dim shell, files, installDir, executable
Set shell = CreateObject("WScript.Shell")
Set files = CreateObject("Scripting.FileSystemObject")

installDir = files.GetParentFolderName(WScript.ScriptFullName)
executable = files.BuildPath(installDir, "idios.exe")

If Not files.FileExists(executable) Then
    MsgBox "IDIOS executable was not found in " & installDir, vbCritical, "IDIOS"
    WScript.Quit 1
End If

shell.CurrentDirectory = installDir
shell.Run Chr(34) & executable & Chr(34) & " serve --open", 0, False
