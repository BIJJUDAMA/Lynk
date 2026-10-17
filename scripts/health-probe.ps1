param (
    [string]$HostName = "localhost"
)

$ErrorActionPreference = "Continue"

Write-Host "===============================================" -ForegroundColor Cyan
Write-Host "   Lynk Multi-Container Synthetic Health Probe " -ForegroundColor Cyan
Write-Host "===============================================" -ForegroundColor Cyan
Write-Host "Testing connectivity on host: $HostName`n"

$services = @(
    @{ Name = "PostgreSQL (Raw Pool)"; Port = 5432; Type = "TCP" },
    @{ Name = "PgBouncer (Tx Pool)";   Port = 6432; Type = "TCP" },
    @{ Name = "SuperTokens IAM Core";  Port = 3567; Type = "HTTP"; Endpoint = "http://${HostName}:3567/hello" },
    @{ Name = "MinIO Object Storage";  Port = 9000; Type = "HTTP"; Endpoint = "http://${HostName}:9000/minio/health/live" },
    @{ Name = "Redis Cache & Pub/Sub"; Port = 6379; Type = "TCP" },
    @{ Name = "Go REST API & SSE";     Port = 8080; Type = "HTTP"; Endpoint = "http://${HostName}:8080/health" },
    @{ Name = "FastAPI AI Subsystem";  Port = 8000; Type = "HTTP"; Endpoint = "http://${HostName}:8000/health" }
)

$passed = 0
$total = $services.Count

foreach ($svc in $services) {
    $ok = $false
    if ($svc.Type -eq "TCP") {
        try {
            $tcp = New-Object System.Net.Sockets.TcpClient
            $async = $tcp.BeginConnect($HostName, $svc.Port, $null, $null)
            $wait = $async.AsyncWaitHandle.WaitOne(1000, $false)
            if ($wait -and $tcp.Connected) {
                $tcp.EndConnect($async)
                $ok = $true
            }
            $tcp.Close()
        } catch {}
    } else {
        try {
            $req = [System.Net.WebRequest]::Create($svc.Endpoint)
            $req.Timeout = 1500
            $resp = $req.GetResponse()
            if ($resp.StatusCode -eq 200 -or $resp.StatusCode -eq "OK") {
                $ok = $true
            }
            $resp.Close()
        } catch {
            if ($_.Exception -and $_.Exception.Response -and ($_.Exception.Response.StatusCode -eq 200 -or $_.Exception.Response.StatusCode -eq "OK")) {
                $ok = $true
            }
        }
    }

    if ($ok) {
        Write-Host " [PASS] $($svc.Name) on port $($svc.Port)" -ForegroundColor Green
        $passed++
    } else {
        Write-Host " [DOWN] $($svc.Name) on port $($svc.Port) (offline or unmapped)" -ForegroundColor Yellow
    }
}

Write-Host "`nProbe result: $passed / $total services responded." -ForegroundColor Cyan
