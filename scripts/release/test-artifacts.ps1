$ErrorActionPreference = 'Stop'
$TargetId = $env:TARGET_ID
$ArtifactDir = $env:PIPELINE_ARTIFACT_TEST_DIR
if (-not $TargetId -or -not $ArtifactDir) { throw 'TARGET_ID and PIPELINE_ARTIFACT_TEST_DIR are required.' }

function Assert-Condition([bool]$Condition, [string]$Message) {
    if (-not $Condition) { throw "FAIL [$TargetId] $Message" }
}
function Wait-ForHealth([string]$Url) {
    for ($i = 0; $i -lt 100; $i++) {
        try {
            $health = Invoke-RestMethod -Uri "$Url/api/health" -TimeoutSec 2
            if ($health.status -eq 'ok') { return $true }
        } catch {}
        Start-Sleep -Milliseconds 200
    }
    return $false
}

$Zip = Get-ChildItem -LiteralPath $ArtifactDir -Filter "idios-v*-$TargetId.zip" -File | Select-Object -First 1
$Setup = Get-ChildItem -LiteralPath $ArtifactDir -Filter "idios-v*-$TargetId-setup.exe" -File | Select-Object -First 1
Assert-Condition ($null -ne $Zip -and $null -ne $Setup) 'expected portable ZIP and Setup.exe were not downloaded.'

$Work = Join-Path $env:RUNNER_TEMP ("idios-artifact-test-" + [Guid]::NewGuid().ToString('N'))
$Portable = Join-Path $Work 'portable'
$InstallDir = Join-Path $Work 'installed'
$ShimDir = Join-Path $Work 'shim'
New-Item -ItemType Directory -Force -Path $Portable, $InstallDir, $ShimDir | Out-Null
$ServerStarted = $false

try {
    Expand-Archive -LiteralPath $Zip.FullName -DestinationPath $Portable -Force
    Assert-Condition (Test-Path (Join-Path $Portable 'idios.exe')) 'portable ZIP is missing idios.exe.'
    Assert-Condition (Test-Path (Join-Path $Portable 'launch-idios.vbs')) 'portable ZIP is missing launch-idios.vbs.'
    Assert-Condition (Test-Path (Join-Path $Portable 'idios.ico')) 'portable ZIP is missing idios.ico.'
    $VbsText = Get-Content -LiteralPath (Join-Path $Portable 'launch-idios.vbs') -Raw
    Assert-Condition ($VbsText.Contains(' serve --open')) 'VBS launcher does not invoke idios.exe serve --open.'

    $InstallArguments = '/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /DIR="' + $InstallDir + '"'
    $SetupProcess = Start-Process -FilePath $Setup.FullName -ArgumentList $InstallArguments -Wait -PassThru
    Assert-Condition ($SetupProcess.ExitCode -eq 0) "Inno Setup returned exit code $($SetupProcess.ExitCode)."
    $Cli = Join-Path $InstallDir 'idios.exe'
    $Launcher = Join-Path $InstallDir 'launch-idios.vbs'
    $Icon = Join-Path $InstallDir 'idios.ico'
    Assert-Condition (Test-Path $Cli) 'installed package is missing idios.exe.'
    Assert-Condition (Test-Path $Launcher) 'installed package is missing launch-idios.vbs.'
    Assert-Condition (Test-Path $Icon) 'installed package is missing idios.ico.'

    $DesktopLink = Join-Path ([Environment]::GetFolderPath('DesktopDirectory')) 'IDIOS.lnk'
    Assert-Condition (Test-Path $DesktopLink) 'installer did not create the IDIOS desktop shortcut.'
    $Shell = New-Object -ComObject WScript.Shell
    $Shortcut = $Shell.CreateShortcut($DesktopLink)
    Assert-Condition ($Shortcut.TargetPath.ToLowerInvariant().EndsWith('\wscript.exe')) 'desktop shortcut does not use the Windows Script Host.'
    Assert-Condition ($Shortcut.Arguments.Contains($Launcher)) 'desktop shortcut does not point to the installed VBS launcher.'

    $ShimSource = Join-Path $Work 'browser-shim.go'
    $ShimCode = @'
package main

import (
    "os"
    "strings"
)

func main() {
    path := os.Getenv("IDIOS_BROWSER_TEST_LOG")
    if path == "" { os.Exit(2) }

    // Every shortcut click is a separate browser-open request. Append each
    // invocation instead of overwriting the first call's diagnostic record.
    log, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
    if err != nil { os.Exit(3) }
    defer log.Close()
    if _, err := log.WriteString(strings.Join(os.Args[1:], "\n")+"\n"); err != nil {
        os.Exit(4)
    }
}
'@
    [System.IO.File]::WriteAllText($ShimSource, $ShimCode, [System.Text.UTF8Encoding]::new($false))
    $ShimExe = Join-Path $ShimDir 'rundll32.exe'
    & go build -trimpath -o $ShimExe $ShimSource
    if ($LASTEXITCODE -ne 0) { throw 'Could not build the test-only rundll32 browser shim.' }

    $Port = 43178
    $BaseUrl = "http://127.0.0.1:$Port"
    $ConfigPath = Join-Path $Work 'config.json'
    $BrowserLog = Join-Path $Work 'browser.log'
    [System.IO.File]::WriteAllText($ConfigPath, '{"server":{"host":"127.0.0.1","port":43178,"open_browser":false}}', [System.Text.UTF8Encoding]::new($false))
    $env:IDIOS_CONFIG = $ConfigPath
    $env:IDIOS_DB = Join-Path $Work 'engine.sqlite3'
    $env:IDIOS_BROWSER_TEST_LOG = $BrowserLog
    $env:PATH = "$ShimDir;$env:PATH"

    # Run the real .lnk through the Windows shell, not just the VBS payload.
    Start-Process -FilePath $DesktopLink
    Assert-Condition (Wait-ForHealth $BaseUrl) "server did not become healthy at $BaseUrl."
    $ServerStarted = $true

    $BrowserOpened = $false
    for ($i = 0; $i -lt 50; $i++) {
        if ((Test-Path $BrowserLog) -and (Select-String -LiteralPath $BrowserLog -SimpleMatch $BaseUrl -Quiet)) {
            $BrowserOpened = $true
            break
        }
        Start-Sleep -Milliseconds 100
    }
    Assert-Condition $BrowserOpened "rundll32 was not called with $BaseUrl."
    $BrowserArgs = Get-Content -LiteralPath $BrowserLog -Raw
    Assert-Condition ($BrowserArgs.Contains('url.dll,FileProtocolHandler')) 'Windows browser handler arguments were not preserved.'

    # Clicking the shortcut again must open a second tab while reusing the first server.
    Start-Process -FilePath $DesktopLink
    $SecondOpen = $false
    for ($i = 0; $i -lt 50; $i++) {
        $Calls = if (Test-Path $BrowserLog) { (Get-Content -LiteralPath $BrowserLog | Where-Object { $_ -match [regex]::Escape($BaseUrl) }).Count } else { 0 }
        if ($Calls -ge 2) { $SecondOpen = $true; break }
        Start-Sleep -Milliseconds 100
    }
    Assert-Condition $SecondOpen 'second desktop shortcut click did not reopen the existing workspace.'

    & $Cli server stop | Out-Null
    $ServerStarted = $false
    Write-Host "PASS [$TargetId] downloaded ZIP and Setup.exe, installed desktop shortcut, VBS launcher, server startup and browser URL"
}
finally {
    if ($ServerStarted) {
        try { & (Join-Path $InstallDir 'idios.exe') server stop | Out-Null } catch {}
    }
    Remove-Item -LiteralPath $Work -Recurse -Force -ErrorAction SilentlyContinue
}
