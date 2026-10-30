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
Push-Location $Root
try {
    & $Compiler "/DAppTag=$Tag" "/DAppArch=$Target" 'packaging/windows/idios.iss'
    if ($LASTEXITCODE -ne 0) { throw "Inno Setup failed with exit code $LASTEXITCODE." }
} finally { Pop-Location }
$Installer = Join-Path $Root "idios-$Tag-$Target-setup.exe"
if (-not (Test-Path $Installer)) { throw "Expected setup installer was not created: $Installer" }
Move-Item -LiteralPath $Installer -Destination $OutputDir -Force
