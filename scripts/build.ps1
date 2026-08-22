# PowerShell Build Script for DAEGSA (§3, §5, §15)
[CmdletBinding()]
param (
    [string]$Version = "v0.1.0-dev",
    [string]$OutputDir = "bin",
    [switch]$RunDoctor,
    [switch]$RunSelfTest,
    [switch]$RunTests
)

$ErrorActionPreference = "Stop"

# Retrieve git commit hash
$Commit = "unknown"
try {
    $Commit = (git rev-parse --short=12 HEAD).Trim()
} catch {
    Write-Warning "Could not determine git commit SHA"
}

# Current UTC timestamp
$BuildDate = [DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")

Write-Host "================================================================================" -ForegroundColor Cyan
Write-Host "                       DAEGSA BUILD AUTOMATION (Windows)                        " -ForegroundColor Cyan
Write-Host "================================================================================" -ForegroundColor Cyan
Write-Host "  Version    : $Version"
Write-Host "  Commit     : $Commit"
Write-Host "  Build Date : $BuildDate"
Write-Host "  Output Dir : $OutputDir"
Write-Host ""

if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
}

$BinaryPath = Join-Path $OutputDir "daegsa.exe"

$LdFlags = "-s -w " +
    "-X github.com/charleszardd/daegsa/internal/cli.Version=$Version " +
    "-X github.com/charleszardd/daegsa/internal/cli.Commit=$Commit " +
    "-X github.com/charleszardd/daegsa/internal/cli.BuildDate=$BuildDate " +
    "-X github.com/charleszardd/daegsa/internal/report.DefaultDaegsaVersion=$Version " +
    "-X github.com/charleszardd/daegsa/internal/report.DefaultCommit=$Commit " +
    "-X github.com/charleszardd/daegsa/internal/report.DefaultBuildDate=$BuildDate"

Write-Host "Compiling standalone Windows binary (-trimpath)..." -ForegroundColor Yellow
go build -trimpath -ldflags $LdFlags -o $BinaryPath ./cmd/daegsa

if ($LASTEXITCODE -ne 0) {
    Write-Error "Build failed with exit code $LASTEXITCODE"
    exit $LASTEXITCODE
}

Write-Host "Build SUCCESS: $BinaryPath" -ForegroundColor Green

if ($RunTests) {
    Write-Host "`nRunning test suite..." -ForegroundColor Yellow
    go test -count=1 ./...
}

if ($RunDoctor) {
    Write-Host "`nRunning daegsa doctor..." -ForegroundColor Yellow
    & $BinaryPath doctor
}

if ($RunSelfTest) {
    Write-Host "`nRunning daegsa self-test..." -ForegroundColor Yellow
    & $BinaryPath self-test
}

Write-Host "`nDone." -ForegroundColor Green
