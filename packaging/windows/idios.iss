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

[Icons]
Name: "{userprograms}\IDIOS\IDIOS CLI Help"; Filename: "{app}\idios.exe"; Parameters: "help"; Tasks: startmenu
Name: "{userprograms}\IDIOS\IDIOS Terminal Workspace"; Filename: "{app}\idios-tui.exe"; Tasks: startmenu
