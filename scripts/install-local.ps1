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
$PathMarker = Join-Path $InstallDir '.idios-path-added'
if ($Uninstall) {
    foreach ($process in (Get-CimInstance Win32_Process | Where-Object { ($_.Name -in @('idios.exe', 'idios-tui.exe')) -and $_.ExecutablePath -and $_.ExecutablePath.StartsWith($InstallDir, [System.StringComparison]::OrdinalIgnoreCase) })) { throw "Close $($process.Name) before uninstalling IDIOS." }
    Remove-Item -LiteralPath $CliPath, (Join-Path $InstallDir 'idios-tui.exe'), (Join-Path $InstallDir 'idios-tui-demo.exe') -Force -ErrorAction SilentlyContinue
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
Remove-Item -LiteralPath $Backup, (Join-Path $InstallDir 'idios-tui.exe'), (Join-Path $InstallDir 'idios-tui-demo.exe') -Force -ErrorAction SilentlyContinue
$UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$Entries = @($UserPath -split ';' | Where-Object { $_ })
if (-not ($Entries | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') })) { $Entries += $InstallDir; [Environment]::SetEnvironmentVariable('Path', ($Entries -join ';'), 'User'); New-Item -ItemType File -Force -Path $PathMarker | Out-Null }
if (-not (($env:PATH -split ';') | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') })) { $env:PATH = "$InstallDir;$env:PATH" }
Write-Host "Installed IDIOS local build to $InstallDir" -ForegroundColor Green
Write-Host 'Open a new terminal if PATH does not refresh, then run: idios serve --open'
