param([string]$Version = "")

$ErrorActionPreference = "Stop"

# SQLite is bundled by github.com/mattn/go-sqlite3, so a C compiler on PATH is enough:
# no separate SQLite headers or libraries are needed.
$env:CGO_ENABLED = "1"
New-Item -ItemType Directory -Force -Path "dist" | Out-Null
$version = if ($Version) { $Version -replace '^v', '' } elseif ($env:GITHUB_REF_NAME) { $env:GITHUB_REF_NAME -replace '^v', '' } elseif ($env:IDIOS_VERSION) { $env:IDIOS_VERSION -replace '^v', '' } else { "3.0.0" }
if (-not (Get-Command pnpm -ErrorAction SilentlyContinue)) {
    throw "pnpm is required to build the bundled IDIOS web UI. Install pnpm 9.x and retry."
}
Push-Location clients/web
try {
    pnpm install --no-frozen-lockfile
    if ($LASTEXITCODE -ne 0) { throw "pnpm install failed with exit code $LASTEXITCODE." }
    pnpm run build
    if ($LASTEXITCODE -ne 0) { throw "Web UI build failed with exit code $LASTEXITCODE." }
} finally {
    Pop-Location
}

$cliLdflags = "-s -w -X idios/clients/cli.Version=$version -X idios/internal/updater.Version=$version"

go build -tags=embed_web -trimpath -ldflags "$cliLdflags" -o "dist/idios.exe" ./cmd/idios
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Built IDIOS v${version}: embedded Web UI + dist/idios.exe"
