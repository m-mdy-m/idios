$ErrorActionPreference = 'Stop'

# Install or replace the latest IDIOS binaries for the current Windows user.
$Owner = 'm-mdy-m'
$Repo = 'idios'
$InstallDir = if ($env:IDIOS_INSTALL_DIR) { $env:IDIOS_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\IDIOS' }
$ApiUrl = "https://api.github.com/repos/$Owner/$Repo/releases/latest"
$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $TempDir | Out-Null

try {
    $Release = Invoke-RestMethod -Uri $ApiUrl -Headers @{ 'Accept' = 'application/vnd.github+json'; 'User-Agent' = 'IDIOS-Installer' }
    $Arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
        'X64' { 'amd64' }
        'Arm64' { 'arm64' }
        default { throw "Unsupported CPU architecture: $($_)" }
    }
    $Asset = "idios-$($Release.tag_name)-windows-$Arch.zip"
    $BaseUrl = "https://github.com/$Owner/$Repo/releases/download/$($Release.tag_name)"
    $ArchivePath = Join-Path $TempDir $Asset
    $ChecksumsPath = Join-Path $TempDir 'SHA256SUMS.txt'
    Invoke-WebRequest -Uri "$BaseUrl/$Asset" -OutFile $ArchivePath
    Invoke-WebRequest -Uri "$BaseUrl/SHA256SUMS.txt" -OutFile $ChecksumsPath

    $Line = Get-Content -LiteralPath $ChecksumsPath | Where-Object { $_ -match "\s+\*?$([regex]::Escape($Asset))$" } | Select-Object -First 1
    if (-not $Line) { throw "SHA256SUMS.txt does not contain $Asset. Nothing was installed." }
    $Expected = ($Line -split '\s+')[0].ToLowerInvariant()
    $Actual = (Get-FileHash -LiteralPath $ArchivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($Expected -ne $Actual) { throw 'IDIOS release checksum verification failed. Nothing was installed.' }

    $Unpacked = Join-Path $TempDir 'unpacked'
    Expand-Archive -LiteralPath $ArchivePath -DestinationPath $Unpacked -Force
    $Cli = Join-Path $Unpacked 'idios.exe'
    $Tui = Join-Path $Unpacked 'idios-tui.exe'
    if (-not (Test-Path -LiteralPath $Cli) -or -not (Test-Path -LiteralPath $Tui)) {
        throw 'Release archive is missing idios.exe or idios-tui.exe. Nothing was installed.'
    }

    if (Test-Path -LiteralPath $InstallDir) {
        $running = Get-CimInstance Win32_Process | Where-Object {
            ($_.Name -in @('idios.exe', 'idios-tui.exe')) -and $_.ExecutablePath -and $_.ExecutablePath.StartsWith($InstallDir, [System.StringComparison]::OrdinalIgnoreCase)
        }
        if ($running) { throw 'Close IDIOS before running the installer, then run this script again.' }
    }
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $CliStage = Join-Path $InstallDir '.idios.new.exe'
    $TuiStage = Join-Path $InstallDir '.idios-tui.new.exe'
    Copy-Item -LiteralPath $Cli -Destination $CliStage -Force
    Copy-Item -LiteralPath $Tui -Destination $TuiStage -Force

    $BackupCli = Join-Path $InstallDir '.idios.backup.exe'
    $BackupTui = Join-Path $InstallDir '.idios-tui.backup.exe'
    $HadCli = Test-Path -LiteralPath (Join-Path $InstallDir 'idios.exe')
    $HadTui = Test-Path -LiteralPath (Join-Path $InstallDir 'idios-tui.exe')
    $InstalledCli = $false
    $InstalledTui = $false
    try {
        if ($HadCli) { Move-Item -LiteralPath (Join-Path $InstallDir 'idios.exe') -Destination $BackupCli -Force }
        if ($HadTui) { Move-Item -LiteralPath (Join-Path $InstallDir 'idios-tui.exe') -Destination $BackupTui -Force }
        Move-Item -LiteralPath $CliStage -Destination (Join-Path $InstallDir 'idios.exe') -Force
        $InstalledCli = $true
        Move-Item -LiteralPath $TuiStage -Destination (Join-Path $InstallDir 'idios-tui.exe') -Force
        $InstalledTui = $true
        Remove-Item -LiteralPath $BackupCli, $BackupTui -Force -ErrorAction SilentlyContinue
    } catch {
        if ($InstalledCli) { Remove-Item -LiteralPath (Join-Path $InstallDir 'idios.exe') -Force -ErrorAction SilentlyContinue }
        if ($InstalledTui) { Remove-Item -LiteralPath (Join-Path $InstallDir 'idios-tui.exe') -Force -ErrorAction SilentlyContinue }
        if ($HadCli -and (Test-Path -LiteralPath $BackupCli)) { Move-Item -LiteralPath $BackupCli -Destination (Join-Path $InstallDir 'idios.exe') -Force -ErrorAction SilentlyContinue }
        if ($HadTui -and (Test-Path -LiteralPath $BackupTui)) { Move-Item -LiteralPath $BackupTui -Destination (Join-Path $InstallDir 'idios-tui.exe') -Force -ErrorAction SilentlyContinue }
        throw
    }

    $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $PathEntries = @($UserPath -split ';' | Where-Object { $_ })
    if (-not ($PathEntries | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') })) {
        $PathEntries += $InstallDir
        [Environment]::SetEnvironmentVariable('Path', ($PathEntries -join ';'), 'User')
    }
    if (-not (($env:PATH -split ';') | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') })) {
        $env:PATH = "$InstallDir;$env:PATH"
    }
    Write-Host "Installed IDIOS $($Release.tag_name -replace '^v', '') to $InstallDir" -ForegroundColor Green
    Write-Host 'Open a new terminal if PATH is not updated in an existing shell, then run: idios'
} finally {
    Remove-Item -LiteralPath $TempDir -Recurse -Force -ErrorAction SilentlyContinue
}
