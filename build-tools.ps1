# Build Docker Images for RavenForge Tools
$ErrorActionPreference = "Stop"

Write-Host "=====================================" -ForegroundColor Cyan
Write-Host "   Building RavenForge Tool Images" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan

$toolsDir = "$PSScriptRoot\tools"
$tools = @(
    @{Name="ingest-jsonlines"; Category="ingest"}
    @{Name="detect-simple-rules"; Category="detect"}
    @{Name="enrich-geoip"; Category="enrich"}
    @{Name="correlate-events"; Category="correlate"}
    @{Name="report-generate"; Category="report"}
    @{Name="triage-prioritize"; Category="triage"}
)

$builtCount = 0
$failedCount = 0

foreach ($tool in $tools) {
    $toolName = $tool.Name
    $category = $tool.Category
    $toolPath = Join-Path $toolsDir "$category\$toolName"
    $dockerfilePath = Join-Path $toolPath "Dockerfile"
    
    if (-not (Test-Path $dockerfilePath)) {
        Write-Host "Skipping $toolName - Dockerfile not found" -ForegroundColor Yellow
        continue
    }
    
    Write-Host ""
    Write-Host "Building $toolName..." -ForegroundColor Cyan
    
    $imageName = "ravenforge/${toolName}:1.0.0"
    # Build from root directory with context
    docker build -t $imageName -f $dockerfilePath .
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Built $imageName" -ForegroundColor Green
        $builtCount++
    } else {
        Write-Host "Failed to build $toolName" -ForegroundColor Red
        $failedCount++
    }
}

Write-Host ""
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host "   Build Summary" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host "Successfully built: $builtCount" -ForegroundColor Green
if ($failedCount -gt 0) {
    Write-Host "Failed: $failedCount" -ForegroundColor Red
}

Write-Host ""
Write-Host "Verifying images:" -ForegroundColor Yellow
docker images ravenforge/*

Write-Host ""
Write-Host "Done!" -ForegroundColor Green
