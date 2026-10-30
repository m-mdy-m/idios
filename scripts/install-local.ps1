param(
    [string]$SourceDir = 'dist',
    [string]$InstallDir = '',
    [switch]$Uninstall
)
$ErrorActionPreference = 'Stop'
if (-not $InstallDir) {
    if ($env:IDIOS_INSTALL_DIR) { $InstallDir = $env:IDIOS_INSTALL_DIR }
    elseif ($env:LOCALAPPDATA) { $InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\IDIOS' }
    else { throw 'LOCALAPPDATA is unavailable; pass -InstallDir explicitly.' }
}
$InstallDir = [System.IO.Path]::GetFullPath($InstallDir)
$CliPath = Join-Path $InstallDir 'idios.exe'
$Root = Split-Path -Parent $PSScriptRoot
$LauncherSource = Join-Path $Root 'packaging/windows/launch-idios.vbs'
$IconSource = Join-Path $Root 'clients/web/public/favicon.ico'
$LauncherPath = Join-Path $InstallDir 'launch-idios.vbs'
$IconPath = Join-Path $InstallDir 'idios.ico'
$PathMarker = Join-Path $InstallDir '.idios-path-added'
if ($Uninstall) {
    foreach ($process in (Get-CimInstance Win32_Process | Where-Object { ($_.Name -in @('idios.exe', 'idios-tui.exe')) -and $_.ExecutablePath -and $_.ExecutablePath.StartsWith($InstallDir, [System.StringComparison]::OrdinalIgnoreCase) })) { throw "Close $($process.Name) before uninstalling IDIOS." }
    Remove-Item -LiteralPath $CliPath, $LauncherPath, $IconPath, (Join-Path $InstallDir 'idios-tui.exe'), (Join-Path $InstallDir 'idios-tui-demo.exe') -Force -ErrorAction SilentlyContinue
    $Shell = New-Object -ComObject WScript.Shell
    $DesktopLink = Join-Path ([Environment]::GetFolderPath('DesktopDirectory')) 'IDIOS.lnk'
    $StartMenuLink = Join-Path ([Environment]::GetFolderPath('Programs')) 'IDIOS/IDIOS.lnk'
    foreach ($Link in @($DesktopLink, $StartMenuLink)) {
        if (Test-Path -LiteralPath $Link) {
            $Shortcut = $Shell.CreateShortcut($Link)
            if ($Shortcut.Arguments -like '*launch-idios.vbs*') { Remove-Item -LiteralPath $Link -Force }
        }
    }
    if (Test-Path -LiteralPath $PathMarker) {
        $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        $Entries = @($UserPath -split ';' | Where-Object { $_ -and $_.TrimEnd('\') -ine $InstallDir.TrimEnd('\') })
        [Environment]::SetEnvironmentVariable('Path', ($Entries -join ';'), 'User')
        Remove-Item -LiteralPath $PathMarker -Force -ErrorAction SilentlyContinue
    }
    if (Test-Path -LiteralPath $InstallDir) { $remaining = Get-ChildItem -LiteralPath $InstallDir -Force -ErrorAction SilentlyContinue; if (-not $remaining) { Remove-Item -LiteralPath $InstallDir -Force } }
    Write-Host "Uninstalled IDIOS from $InstallDir" -ForegroundColor Green
    exit 0
}
$SourceCli = Join-Path $SourceDir 'idios.exe'
if (-not (Test-Path -LiteralPath $SourceCli -PathType Leaf)) { throw "Expected idios.exe in '$SourceDir'. Run make build first." }
if (Test-Path -LiteralPath $InstallDir) {
    $running = Get-CimInstance Win32_Process | Where-Object { ($_.Name -in @('idios.exe', 'idios-tui.exe')) -and $_.ExecutablePath -and $_.ExecutablePath.StartsWith($InstallDir, [System.StringComparison]::OrdinalIgnoreCase) }
    if ($running) { throw 'Close IDIOS before installing from a local build, then run make install again.' }
}
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$Stage = Join-Path $InstallDir '.idios.new.exe'
$Backup = Join-Path $InstallDir '.idios.backup.exe'
Copy-Item -LiteralPath $SourceCli -Destination $Stage -Force
$HadOld = Test-Path -LiteralPath $CliPath
try { if ($HadOld) { Move-Item -LiteralPath $CliPath -Destination $Backup -Force }; Move-Item -LiteralPath $Stage -Destination $CliPath -Force }
catch { Remove-Item -LiteralPath $CliPath -Force -ErrorAction SilentlyContinue; if ($HadOld -and (Test-Path -LiteralPath $Backup)) { Move-Item -LiteralPath $Backup -Destination $CliPath -Force -ErrorAction SilentlyContinue }; throw }
if (-not (Test-Path -LiteralPath $LauncherSource)) { throw "Desktop launcher source is missing: $LauncherSource" }
if (-not (Test-Path -LiteralPath $IconSource)) { throw "IDIOS icon source is missing: $IconSource" }
Copy-Item -LiteralPath $LauncherSource -Destination $LauncherPath -Force
Copy-Item -LiteralPath $IconSource -Destination $IconPath -Force
Remove-Item -LiteralPath $Backup, (Join-Path $InstallDir 'idios-tui.exe'), (Join-Path $InstallDir 'idios-tui-demo.exe') -Force -ErrorAction SilentlyContinue
$UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$Entries = @($UserPath -split ';' | Where-Object { $_ })
if (-not ($Entries | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') })) { $Entries += $InstallDir; [Environment]::SetEnvironmentVariable('Path', ($Entries -join ';'), 'User'); New-Item -ItemType File -Force -Path $PathMarker | Out-Null }
if (-not (($env:PATH -split ';') | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') })) { $env:PATH = "$InstallDir;$env:PATH" }
$Shell = New-Object -ComObject WScript.Shell
$Desktop = [Environment]::GetFolderPath('DesktopDirectory')
$StartMenu = Join-Path ([Environment]::GetFolderPath('Programs')) 'IDIOS'
New-Item -ItemType Directory -Force -Path $StartMenu | Out-Null
foreach ($LinkPath in @((Join-Path $Desktop 'IDIOS.lnk'), (Join-Path $StartMenu 'IDIOS.lnk'))) {
    $Shortcut = $Shell.CreateShortcut($LinkPath)
    $Shortcut.TargetPath = Join-Path $env:SystemRoot 'System32\wscript.exe'
    $Shortcut.Arguments = '//B "' + $LauncherPath + '"'
    $Shortcut.WorkingDirectory = $InstallDir
    $Shortcut.IconLocation = $IconPath + ',0'
    $Shortcut.Description = 'Start IDIOS and open the local workspace'
    $Shortcut.Save()
}
Write-Host "Installed IDIOS local build to $InstallDir" -ForegroundColor Green
Write-Host 'Desktop and Start Menu shortcuts created.'
