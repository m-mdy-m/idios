#define AppName "IDIOS"

#ifndef AppTag
  #define AppTag GetEnv("GITHUB_REF_NAME")
#endif
#ifndef AppArch
  #define AppArch GetEnv("ASSET")
#endif

#if AppArch == "windows-amd64"
  #define AppAllowedArch "x64compatible and not arm64"
  #define AppInstallMode "x64compatible"
#elif AppArch == "windows-arm64"
  #define AppAllowedArch "arm64"
  #define AppInstallMode "arm64"
#else
  #error "ASSET must be windows-amd64 or windows-arm64"
#endif

[Setup]
AppId={{B16D2F4D-459F-4B41-8D98-AD3BFBAC8340}
AppName={#AppName}
AppVersion={#AppTag}
AppPublisher=IDIOS contributors
AppPublisherURL=https://github.com/m-mdy-m/idios
AppSupportURL=https://github.com/m-mdy-m/idios/issues
DefaultDirName={localappdata}\Programs\IDIOS
DefaultGroupName=IDIOS
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ChangesEnvironment=yes
ArchitecturesAllowed={#AppAllowedArch}
ArchitecturesInstallIn64BitMode={#AppInstallMode}
LicenseFile=..\..\LICENSE
OutputBaseFilename=idios-{#AppTag}-{#AppArch}-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\idios-tui.exe
CloseApplications=no
RestartApplications=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "startmenu"; Description: "Create Start Menu shortcuts"; GroupDescription: "Additional shortcuts:"; Flags: checkedonce

[Files]
Source: "..\..\dist\idios.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\dist\idios-tui.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\README.md"; DestDir: "{app}\docs"; Flags: ignoreversion
Source: "..\..\LICENSE"; DestDir: "{app}\docs"; Flags: ignoreversion

[Registry]
; Track whether this setup added the path, so uninstall does not remove a pre-existing user choice.
Root: HKCU; Subkey: "Software\IDIOS"; ValueType: dword; ValueName: "PathAddedByInstaller"; ValueData: "1"; Flags: uninsdeletevalue; Check: NeedsAddPath(ExpandConstant('{app}'))
; Add the user-scoped install directory to PATH so `idios` works in a terminal.
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; Flags: preservestringtype; Check: NeedsAddPath(ExpandConstant('{app}'))

[Icons]
Name: "{userprograms}\IDIOS\IDIOS CLI Help"; Filename: "{app}\idios.exe"; Parameters: "help"; Tasks: startmenu
Name: "{userprograms}\IDIOS\IDIOS Terminal Workspace"; Filename: "{app}\idios-tui.exe"; Tasks: startmenu

[Code]
function NeedsAddPath(Param: string): Boolean;
var
  ExistingPath: string;
  WrappedPath: string;
  WrappedParam: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', ExistingPath) then
  begin
    Result := True;
    exit;
  end;
  WrappedPath := ';' + Lowercase(ExistingPath) + ';';
  WrappedParam := ';' + Lowercase(RemoveBackslash(Param)) + ';';
  Result := Pos(WrappedParam, WrappedPath) = 0;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  ExistingPath: string;
  RemainingPath: string;
  Entry: string;
  AppPath: string;
  Separator: Integer;
  PathAdded: Cardinal;
begin
  if CurUninstallStep <> usUninstall then
    exit;
  if not RegQueryDWordValue(HKEY_CURRENT_USER, 'Software\IDIOS', 'PathAddedByInstaller', PathAdded) then
    exit;
  if PathAdded <> 1 then
    exit;
  if not RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', ExistingPath) then
    exit;

  AppPath := Lowercase(ExpandConstant('{app}'));
  RemainingPath := '';
  while ExistingPath <> '' do
  begin
    Separator := Pos(';', ExistingPath);
    if Separator = 0 then
    begin
      Entry := ExistingPath;
      ExistingPath := '';
    end
    else
    begin
      Entry := Copy(ExistingPath, 1, Separator - 1);
      Delete(ExistingPath, 1, Separator);
    end;
    if (Lowercase(RemoveBackslash(Trim(Entry))) <> RemoveBackslash(AppPath)) and (Trim(Entry) <> '') then
    begin
      if RemainingPath <> '' then
        RemainingPath := RemainingPath + ';';
      RemainingPath := RemainingPath + Entry;
    end;
  end;
  RegWriteStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', RemainingPath);
end;
