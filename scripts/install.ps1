$ErrorActionPreference = 'Stop'
$Owner = 'm-mdy-m'
$Repo = 'idios'
$InstallDir = if ($env:IDIOS_INSTALL_DIR) { $env:IDIOS_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\IDIOS' }
$ApiUrl = "https://api.github.com/repos/$Owner/$Repo/releases/latest"
$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $TempDir | Out-Null
try {
    $Release = Invoke-RestMethod -Uri $ApiUrl -Headers @{ 'Accept' = 'application/vnd.github+json'; 'User-Agent' = 'IDIOS-Installer' }
    $Arch = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) { 'X64' { 'amd64' } 'Arm64' { 'arm64' } default { throw "Unsupported CPU architecture: $($_)" } }
    $Asset = "idios-$($Release.tag_name)-windows-$Arch.zip"
    $BaseUrl = "https://github.com/$Owner/$Repo/releases/download/$($Release.tag_name)"
    $ArchivePath = Join-Path $TempDir $Asset
    $ChecksumsPath = Join-Path $TempDir 'SHA256SUMS'
    Invoke-WebRequest -Uri "$BaseUrl/$Asset" -OutFile $ArchivePath
    Invoke-WebRequest -Uri "$BaseUrl/SHA256SUMS" -OutFile $ChecksumsPath
    $Line = Get-Content -LiteralPath $ChecksumsPath | Where-Object { $Parts = $_ -split '\s+', 2; if ($Parts.Count -gt 1) { $Name = $Parts[1] -replace '^\*', ''; ($Name -eq $Asset) -or $Name.EndsWith('/' + $Asset) -or $Name.EndsWith('\' + $Asset) } } | Select-Object -First 1
    if (-not $Line) { throw "SHA256SUMS does not contain $Asset. Nothing was installed." }
    $Expected = ($Line -split '\s+')[0].ToLowerInvariant()
    $Actual = (Get-FileHash -LiteralPath $ArchivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($Expected -ne $Actual) { throw 'IDIOS checksum verification failed. Nothing was installed.' }
    $Unpacked = Join-Path $TempDir 'unpacked'
    Expand-Archive -LiteralPath $ArchivePath -DestinationPath $Unpacked -Force
    $Cli = Join-Path $Unpacked 'idios.exe'
    if (-not (Test-Path -LiteralPath $Cli)) { throw 'Release archive is missing idios.exe. Nothing was installed.' }
    if (Test-Path -LiteralPath $InstallDir) {
        $running = Get-CimInstance Win32_Process | Where-Object { ($_.Name -in @('idios.exe', 'idios-tui.exe')) -and $_.ExecutablePath -and $_.ExecutablePath.StartsWith($InstallDir, [System.StringComparison]::OrdinalIgnoreCase) }
        if ($running) { throw 'Close IDIOS before running the installer, then run this script again.' }
    }
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $CliPath = Join-Path $InstallDir 'idios.exe'
    $Stage = Join-Path $InstallDir '.idios.new.exe'
    $Backup = Join-Path $InstallDir '.idios.backup.exe'
    Copy-Item -LiteralPath $Cli -Destination $Stage -Force
    $HadOld = Test-Path -LiteralPath $CliPath
    try {
        if ($HadOld) { Move-Item -LiteralPath $CliPath -Destination $Backup -Force }
        Move-Item -LiteralPath $Stage -Destination $CliPath -Force
    } catch {
        Remove-Item -LiteralPath $CliPath -Force -ErrorAction SilentlyContinue
        if ($HadOld -and (Test-Path -LiteralPath $Backup)) { Move-Item -LiteralPath $Backup -Destination $CliPath -Force -ErrorAction SilentlyContinue }
        throw
    }
    Remove-Item -LiteralPath $Backup, (Join-Path $InstallDir 'idios-tui.exe'), (Join-Path $InstallDir 'idios-tui-demo.exe') -Force -ErrorAction SilentlyContinue
    $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $PathEntries = @($UserPath -split ';' | Where-Object { $_ })
    if (-not ($PathEntries | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') })) { $PathEntries += $InstallDir; [Environment]::SetEnvironmentVariable('Path', ($PathEntries -join ';'), 'User') }
    if (-not (($env:PATH -split ';') | Where-Object { $_.TrimEnd('\') -ieq $InstallDir.TrimEnd('\') })) { $env:PATH = "$InstallDir;$env:PATH" }
    Write-Host "Installed IDIOS $($Release.tag_name -replace '^v', '') to $InstallDir" -ForegroundColor Green
    Write-Host 'Run: idios serve --open'
} finally { Remove-Item -LiteralPath $TempDir -Recurse -Force -ErrorAction SilentlyContinue }
