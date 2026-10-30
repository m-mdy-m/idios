param([string]$Version = "3.0.0")

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$port = 43170

# Stop only an IDIOS executable that currently owns the local API port. Do not terminate
# unrelated programs if a different application has claimed the port.
$connections = @(Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue)
$processIds = @($connections | Select-Object -ExpandProperty OwningProcess -Unique)
foreach ($processId in $processIds) {
    $process = Get-CimInstance Win32_Process -Filter "ProcessId = $processId" -ErrorAction SilentlyContinue
    if ($null -eq $process) { continue }
    if ($process.Name -notmatch '^idios(\.exe)?$') {
        throw "Port $port is occupied by '$($process.Name)' (PID $processId). It was not stopped. Close that process or change IDIOS server port first."
    }
    Write-Host "Stopping the previous IDIOS server (PID $processId) so the rebuilt UI and API are from one executable..."
    Stop-Process -Id $processId -Force
}

# Wait briefly for Windows to release the listening socket.
for ($attempt = 0; $attempt -lt 30; $attempt++) {
    $remaining = @(Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue)
    if ($remaining.Count -eq 0) { break }
    Start-Sleep -Milliseconds 200
}
if (@(Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue).Count -gt 0) {
    throw "Port $port is still occupied after stopping the old IDIOS process."
}

$buildScript = Join-Path $repoRoot "scripts/build.ps1"
Write-Host "Building IDIOS $Version with the current Shelf PDF Reader and embedded UI..."
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $buildScript -Version $Version
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$binary = Join-Path $repoRoot "dist/idios.exe"
if (-not (Test-Path $binary -PathType Leaf)) { throw "Build completed without producing $binary" }
Write-Host "Starting the exact newly-built executable: $binary"
Write-Host "The server logs below belong to this build. Press Ctrl+C to stop it."
& $binary serve --open
exit $LASTEXITCODE
