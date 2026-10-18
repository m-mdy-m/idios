$ErrorActionPreference = "Stop"

# SQLite is bundled by github.com/mattn/go-sqlite3, so a C compiler on PATH is enough:
# no separate SQLite headers or libraries are needed.
$env:CGO_ENABLED = "1"
New-Item -ItemType Directory -Force -Path "dist" | Out-Null

go build -trimpath -o "dist/idios.exe" ./cmd/idios
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go build -trimpath -o "dist/idios-tui.exe" ./cmd/idios-tui
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Built dist/idios.exe and dist/idios-tui.exe"
