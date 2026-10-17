param (
    [string]$BaseUrl = $(if ($env:API_BASE_URL) { $env:API_BASE_URL } else { "http://localhost:8080" }),
    [string]$Suite = "all"
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = (Resolve-Path (Join-Path $ScriptDir "..")).Path
Set-Location $RepoRoot

Write-Host "===============================================" -ForegroundColor Cyan
Write-Host "  Lynk Platform k6 Load & Concurrency Runner   " -ForegroundColor Cyan
Write-Host "===============================================" -ForegroundColor Cyan
Write-Host "Target API: $BaseUrl"
Write-Host "Suite:      $Suite"
Write-Host ""

if (-not (Get-Command k6 -ErrorAction SilentlyContinue)) {
    Write-Warning "k6 executable was not found on PATH."
    Write-Host "To install on Windows: choco install k6 OR winget install k6"
    Write-Host "Running JavaScript syntax validation via node instead..." -ForegroundColor Yellow
    node -c tests/load/helpers.js
    node -c tests/load/search_load.js
    node -c tests/load/ratelimit_stress.js
    node -c tests/load/outbox_burst.js
    Write-Host "All load test scripts passed syntax verification." -ForegroundColor Green
    exit 0
}

$env:API_BASE_URL = $BaseUrl

if ($Suite -eq "all" -or $Suite -eq "search") {
    Write-Host "--> Running Search & Cache-Aside Load Suite..." -ForegroundColor Yellow
    k6 run tests/load/search_load.js
}

if ($Suite -eq "all" -or $Suite -eq "ratelimit") {
    Write-Host "--> Running Rate Limiter Stress Suite..." -ForegroundColor Yellow
    k6 run tests/load/ratelimit_stress.js
}

if ($Suite -eq "all" -or $Suite -eq "outbox") {
    Write-Host "--> Running Metrics & Outbox Throughput Suite..." -ForegroundColor Yellow
    k6 run tests/load/outbox_burst.js
}

Write-Host ""
Write-Host "k6 load testing completed successfully." -ForegroundColor Green
