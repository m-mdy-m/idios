param(
    [Parameter(Mandatory = $true)][ValidateSet('windows-amd64', 'windows-arm64')][string]$Target,
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$Tag
)
$ErrorActionPreference = 'Stop'
$Root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$ReleaseDir = Join-Path $Root 'release'
$OutputDir = Join-Path $Root 'dist/release'
New-Item -ItemType Directory -Force -Path $ReleaseDir, $OutputDir | Out-Null
Copy-Item -LiteralPath (Join-Path $Root 'dist/idios.exe'), (Join-Path $Root 'README.md'), (Join-Path $Root 'LICENSE') -Destination $ReleaseDir -Force
$Archive = Join-Path $OutputDir "idios-$Tag-$Target.zip"
Compress-Archive -Path (Join-Path $ReleaseDir '*') -DestinationPath $Archive -Force

$Compiler = $null
$Roots = @([Environment]::GetFolderPath('ProgramFilesX86'), [Environment]::GetFolderPath('ProgramFiles'))
foreach ($InstallRoot in $Roots) {
    foreach ($Name in @('Inno Setup 7', 'Inno Setup 6')) {
        $Candidate = Join-Path $InstallRoot "$Name\ISCC.exe"
        if (Test-Path $Candidate) { $Compiler = $Candidate; break }
    }
    if ($Compiler) { break }
}
if (-not $Compiler) {
    choco install innosetup --no-progress -y
    if ($LASTEXITCODE -ne 0) { throw 'Failed to install Inno Setup.' }
    foreach ($InstallRoot in $Roots) {
        foreach ($Name in @('Inno Setup 7', 'Inno Setup 6')) {
            $Candidate = Join-Path $InstallRoot "$Name\ISCC.exe"
            if (Test-Path $Candidate) { $Compiler = $Candidate; break }
        }
        if ($Compiler) { break }
    }
}
if (-not $Compiler) { throw 'ISCC.exe was not found.' }
Write-Host "[release] Windows target: $Target"
Write-Host "[release] Creating ZIP archive: $Archive"
Write-Host "[release] Inno Setup compiler: $Compiler"

$InstallerName = "idios-$Tag-$Target-setup.exe"
$Installer = Join-Path $OutputDir $InstallerName
$LegacyInstaller = Join-Path $Root "packaging/windows/Output/$InstallerName"

Push-Location $Root
try {
    # /O selects the output directory explicitly. Without it, Inno Setup defaults
    # to packaging/windows/Output because the .iss file lives in that directory.
    & $Compiler "/O$OutputDir" "/DAppTag=$Tag" "/DAppArch=$Target" 'packaging/windows/idios.iss'
    if ($LASTEXITCODE -ne 0) { throw "Inno Setup failed with exit code $LASTEXITCODE." }
} finally { Pop-Location }

# Keep compatibility with older Inno Setup configurations that ignore /O.
if (-not (Test-Path -LiteralPath $Installer) -and (Test-Path -LiteralPath $LegacyInstaller)) {
    Write-Host "[release] Moving installer from legacy Inno Setup output: $LegacyInstaller"
    Move-Item -LiteralPath $LegacyInstaller -Destination $Installer -Force
}
if (-not (Test-Path -LiteralPath $Installer)) {
    $Found = Get-ChildItem -LiteralPath (Join-Path $Root 'packaging/windows') -Filter $InstallerName -File -Recurse -ErrorAction SilentlyContinue |
        Select-Object -ExpandProperty FullName
    $FoundText = if ($Found) { $Found -join [Environment]::NewLine } else { '(no matching installer found)' }
    throw "Inno Setup returned success but the expected installer is missing: $Installer. Search results: $FoundText"
}

$InstallerInfo = Get-Item -LiteralPath $Installer
$InstallerHash = (Get-FileHash -LiteralPath $Installer -Algorithm SHA256).Hash
Write-Host "[release] Installer ready: $Installer ($($InstallerInfo.Length) bytes)"
Write-Host "[release] Installer SHA256: $InstallerHash"
Write-Host "[release] Windows packaging completed for $Target"
