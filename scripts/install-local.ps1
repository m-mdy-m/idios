param(
    [string]$SourceDir = 'dist',
    [string]$InstallDir = '',
    [switch]$Uninstall
)

$ErrorActionPreference = 'Stop'
if (-not $InstallDir) {
    if ($env:IDIOS_INSTALL_DIR) {
        $InstallDir = $env:IDIOS_INSTALL_DIR
    } elseif ($env:LOCALAPPDATA) {
        $InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\IDIOS'
    } else {
        throw 'LOCALAPPDATA is unavailable; pass -InstallDir explicitly.'
    }
}
$InstallDir = [System.IO.Path]::GetFullPath($InstallDir)
$CliPath = Join-Path $InstallDir 'idios.exe'
$TuiPath = Join-Path $InstallDir 'idios-tui.exe'
$PathMarker = Join-Path $InstallDir '.idios-path-added'

if ($Uninstall) {
    foreach ($process in (Get-CimInstance Win32_Process | Where-Object {
        ($_.Name -in @('idios.exe', 'idios-tui.exe')) -and $_.ExecutablePath -and
        $_.ExecutablePath.StartsWith($InstallDir, [System.StringComparison]::OrdinalIgnoreCase)
    })) {
        throw "Close $($process.Name) before uninstalling IDIOS."
    }
    Remove-Item -LiteralPath $CliPath, $TuiPath -Force -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $PathMarker) {
        $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        $Entries = @($UserPath -split ';' | Where-Object { $_ -and $_.TrimEnd('\') -ine $InstallDir.TrimEnd('\') })
        [Environment]::SetEnvironmentVariable('Path', ($Entries -join ';'), 'User')
        Remove-Item -LiteralPath $PathMarker -Force -ErrorAction SilentlyContinue
    }
    if (Test-Path -LiteralPath $InstallDir) {
        $remaining = Get-ChildItem -LiteralPath $InstallDir -Force -ErrorAction SilentlyContinue
        if (-not $remaining) { Remove-Item -LiteralPath $InstallDir -Force }
    }
    Write-Host "Uninstalled IDIOS from $InstallDir" -ForegroundColor Green
    exit 0
}

$SourceCli = Join-Path $SourceDir 'idios.exe'
$SourceTui = Join-Path $SourceDir 'idios-tui.exe'
if (-not (Test-Path -LiteralPath $SourceCli -PathType Leaf) -or -not (Test-Path -LiteralPath $SourceTui -PathType Leaf)) {
    throw "Expected idios.exe and idios-tui.exe in '$SourceDir'. Run make build first."
}
if (Test-Path -LiteralPath $InstallDir) {
    $running = Get-CimInstance Win32_Process | Where-Object {
        ($_.Name -in @('idios.exe', 'idios-tui.exe')) -and $_.ExecutablePath -and
        $_.ExecutablePath.StartsWith($InstallDir, [System.StringComparison]::OrdinalIgnoreCase)
    }
    if ($running) { throw 'Close IDIOS before installing from a local build, then run make install again.' }
}
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$StageCli = Join-Path $InstallDir '.idios.new.exe'
$StageTui = Join-Path $InstallDir '.idios-tui.new.exe'
$BackupCli = Join-Path $InstallDir '.idios.backup.exe'
$BackupTui = Join-Path $InstallDir '.idios-tui.backup.exe'
Copy-Item -LiteralPath $SourceCli -Destination $StageCli -Force
Copy-Item -LiteralPath $SourceTui -Destination $StageTui -Force
$HadCli = Test-Path -LiteralPath $CliPath
$HadTui = Test-Path -LiteralPath $TuiPath
$InstalledCli = $false
$InstalledTui = $false
try {
    if ($HadCli) { Move-Item -LiteralPath $CliPath -Destination $BackupCli -Force }
    if ($HadTui) { Move-Item -LiteralPath $TuiPath -Destination $BackupTui -Force }
    Move-Item -LiteralPath $StageCli -Destination $CliPath -Force
    $InstalledCli = $true
    Move-Item -LiteralPath $StageTui -Destination $TuiPath -Force
    $InstalledTui = $true
} catch {
    if ($InstalledCli) { Remove-Item -LiteralPath $CliPath -Force -ErrorAction SilentlyContinue }
    if ($InstalledTui) { Remove-Item -LiteralPath $TuiPath -Force -ErrorAction SilentlyContinue }
    if ($HadCli -and (Test-Path -LiteralPath $BackupCli)) { Move-Item -LiteralPath $BackupCli -Destination $CliPath -Force -ErrorAction SilentlyContinue }
    if ($HadTui -and (Test-Path -LiteralPath $BackupTui)) { Move-Item -LiteralPath $BackupTui -Destination $TuiPath -Force -ErrorAction SilentlyContinue }
    throw
}
Remove-Item -LiteralPath $BackupCli, $BackupTui -Force -ErrorAction SilentlyContinue

$UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$Entries = @($UserPath -split ';' | Where-Object { $_ })
if (-not ($Entries | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') })) {
    $Entries += $InstallDir
    [Environment]::SetEnvironmentVariable('Path', ($Entries -join ';'), 'User')
    New-Item -ItemType File -Force -Path $PathMarker | Out-Null
}
if (-not (($env:PATH -split ';') | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') })) {
    $env:PATH = "$InstallDir;$env:PATH"
}
Write-Host "Installed IDIOS local build to $InstallDir" -ForegroundColor Green
Write-Host 'Open a new terminal if PATH does not refresh, then run: idios'
