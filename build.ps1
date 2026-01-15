# Build RavenForge
$ErrorActionPreference = "Stop"

Write-Host "=====================================" -ForegroundColor Cyan
Write-Host "   RavenForge Build Script" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan

# Check prerequisites
Write-Host "`n[Check] Verifying prerequisites..." -ForegroundColor Yellow

try {
    $goVersion = go version 2>&1
    Write-Host "✓ Go: $goVersion" -ForegroundColor Green
} catch {
    Write-Host "✗ Go not found. Install from https://go.dev/dl/" -ForegroundColor Red
    exit 1
}

try {
    $dockerVersion = docker --version 2>&1
    Write-Host "✓ Docker: $dockerVersion" -ForegroundColor Green
} catch {
    Write-Host "⚠ Docker not found (optional for building tools)" -ForegroundColor Yellow
}

# Build Go binaries
Write-Host "`n[1/3] Downloading Go dependencies..." -ForegroundColor Yellow
Set-Location "$PSScriptRoot\core"
go mod download
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "`n[2/3] Building Go binaries..." -ForegroundColor Yellow
New-Item -ItemType Directory -Force -Path bin | Out-Null

Write-Host "  → Building ravenforged..." -ForegroundColor Cyan
go build -o bin/ravenforged.exe ./cmd/ravenforged
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "  → Building ravenforge..." -ForegroundColor Cyan
go build -o bin/ravenforge.exe ./cmd/ravenforge
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

# Run tests
Write-Host "`n[3/3] Running tests..." -ForegroundColor Yellow
go test ./... -short
if ($LASTEXITCODE -ne 0) {
    Write-Host "⚠ Some tests failed" -ForegroundColor Yellow
} else {
    Write-Host "✓ All tests passed" -ForegroundColor Green
}

# Summary
Write-Host "`n=====================================" -ForegroundColor Cyan
Write-Host "   Build Completed Successfully!" -ForegroundColor Green
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host "`nBinaries:" -ForegroundColor Yellow
Write-Host "  → core\bin\ravenforged.exe" -ForegroundColor Cyan
Write-Host "  → core\bin\ravenforge.exe" -ForegroundColor Cyan

Write-Host "`nNext steps:" -ForegroundColor Yellow
Write-Host "  1. Copy config: copy core\config\ravenforge.example.yaml core\config\ravenforge.yaml"
Write-Host "  2. Start daemon: .\core\bin\ravenforged.exe --config core\config\ravenforge.yaml"
Write-Host "  3. Check status: .\core\bin\ravenforge.exe status"
Write-Host ""
