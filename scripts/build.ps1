param([string]$Version = "")

$ErrorActionPreference = "Stop"

# SQLite is bundled by github.com/mattn/go-sqlite3, so a C compiler on PATH is enough:
# no separate SQLite headers or libraries are needed.
$env:CGO_ENABLED = "1"
New-Item -ItemType Directory -Force -Path "dist" | Out-Null
$version = if ($Version) { $Version -replace '^v', '' } elseif ($env:GITHUB_REF_NAME) { $env:GITHUB_REF_NAME -replace '^v', '' } elseif ($env:IDIOS_VERSION) { $env:IDIOS_VERSION -replace '^v', '' } else { "1.0.2" }
$cliLdflags = "-s -w -X idios/internal/cli.Version=$version -X idios/internal/updater.Version=$version"
$tuiLdflags = "-s -w -X idios/internal/updater.Version=$version"

go build -trimpath -ldflags "$cliLdflags" -o "dist/idios.exe" ./cmd/idios
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go build -trimpath -ldflags "$tuiLdflags" -o "dist/idios-tui.exe" ./cmd/idios-tui
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go build -trimpath -ldflags "$tuiLdflags" -o "dist/idios-tui-demo.exe" ./cmd/idios-tui-demo
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Built IDIOS v${version}: dist/idios.exe, dist/idios-tui.exe, and dist/idios-tui-demo.exe"
