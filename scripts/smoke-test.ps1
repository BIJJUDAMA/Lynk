# ==============================================================================
# Lynk Platform — End-to-End Integration Smoke Test Suite (PowerShell)
#
# Exercises the complete MVP lifecycle against running local infrastructure:
#   1. Infrastructure Health Checks (Go API, SuperTokens Core, MinIO S3)
#   2. Institutional .edu Registration Gate Assertion (Non-.edu rejected)
#   3. SuperTokens User Provisioning & Campus Verification
#   4. User Auth Synchronization (POST /api/v1/auth/sync)
#   5. Student & Employer Profile Updates (PUT /api/v1/profile/student & employer)
#   6. Resume Upload (Multipart PDF) & Presigned S3 Download URL Fetch
#   7. Job Creation by Verified Employer (POST /api/v1/jobs)
#   8. Job Filtering, Search & Detail Retrieval
#   9. Campus Email Verification Gate Enforcement (HTTP 403 EMAIL_NOT_VERIFIED)
#  10. Verified Student Job Application Submission
#  11. Employer Application Review & Acceptance (Atomic Contract Generation)
#  12. Contract Status Progression (Active -> Completed)
#  13. Peer Review Submission & Duplicate Conflict Assertion (HTTP 409)
#
# Invariants:
#   - Exit code 0 on all assertions passing.
#   - Non-zero exit code (exit 1) with colored diagnostic logs on any assertion failure.
#   - Accepts pre-acquired tokens via environment variables OR acquires them
#     via SuperTokens APIs with automated member provisioning & verification.
# ==============================================================================

[CmdletBinding()]
param (
    [string]$ApiBaseUrl = $(if ($env:API_BASE_URL) { $env:API_BASE_URL } else { "http://127.0.0.1:8080" }),
    [string]$SuperTokensUrl = $(if ($env:SUPERTOKENS_URL) { $env:SUPERTOKENS_URL } else { "http://127.0.0.1:3567" }),
    [string]$SuperTokensApiKey = $(if ($env:SUPERTOKENS_API_KEY) { $env:SUPERTOKENS_API_KEY } else { "lynk-supertokens-secret-api-key-2026" }),
    [string]$MinioUrl = $(if ($env:MINIO_URL) { $env:MINIO_URL } else { "http://127.0.0.1:9000" }),
    [string]$EmployerToken = $env:EMPLOYER_TOKEN,
    [string]$VerifiedStudentToken = $env:VERIFIED_STUDENT_TOKEN,
    [string]$UnverifiedStudentToken = $env:UNVERIFIED_STUDENT_TOKEN,
    [string]$EmployerEmail = $(if ($env:EMPLOYER_EMAIL) { $env:EMPLOYER_EMAIL } elseif ($env:EMPLOYER_USERNAME) { $env:EMPLOYER_USERNAME } else { "poster@campus.edu" }),
    [string]$EmployerPassword = $(if ($env:EMPLOYER_PASSWORD) { $env:EMPLOYER_PASSWORD } else { "password123" }),
    [string]$VerifiedStudentEmail = $(if ($env:VERIFIED_STUDENT_EMAIL) { $env:VERIFIED_STUDENT_EMAIL } elseif ($env:VERIFIED_STUDENT_USERNAME) { $env:VERIFIED_STUDENT_USERNAME } else { "applicant@campus.edu" }),
    [string]$VerifiedStudentPassword = $(if ($env:VERIFIED_STUDENT_PASSWORD) { $env:VERIFIED_STUDENT_PASSWORD } else { "password123" }),
    [string]$UnverifiedStudentEmail = $(if ($env:UNVERIFIED_STUDENT_EMAIL) { $env:UNVERIFIED_STUDENT_EMAIL } elseif ($env:UNVERIFIED_STUDENT_USERNAME) { $env:UNVERIFIED_STUDENT_USERNAME } else { "unverified@campus.edu" }),
    [string]$UnverifiedStudentPassword = $(if ($env:UNVERIFIED_STUDENT_PASSWORD) { $env:UNVERIFIED_STUDENT_PASSWORD } else { "password123" }),
    [string]$UnauthorizedEmail = "unauthorized@gmail.com",
    [switch]$SkipInfraHealth = $(if ($env:SKIP_INFRA_HEALTH -eq "1") { $true } else { $false })
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

# ------------------------------------------------------------------------------
# Logging Helpers
# ------------------------------------------------------------------------------
function Log-Info ($msg) {
    Write-Host "[INFO] $msg" -ForegroundColor Cyan
}

function Log-Step ($num, $title) {
    Write-Host ""
    Write-Host ("=" * 70) -ForegroundColor Blue
    Write-Host "[STEP $num/13] $title" -ForegroundColor Magenta
    Write-Host ("=" * 70) -ForegroundColor Blue
}

function Log-Substep ($msg) {
    Write-Host "  --> $msg" -ForegroundColor Cyan
}

function Log-Pass ($msg) {
    Write-Host "  [PASS] $msg" -ForegroundColor Green
}

function Log-Fail ($msg) {
    Write-Host "  [FAIL] $msg" -ForegroundColor Red
}

function Log-Warn ($msg) {
    Write-Host "  [WARN] $msg" -ForegroundColor Yellow
}

# ------------------------------------------------------------------------------
# Temporary Directory Setup
# ------------------------------------------------------------------------------
$TmpDir = [System.IO.Path]::Combine([System.IO.Path]::GetTempPath(), [System.Guid]::NewGuid().ToString("N"))
[System.IO.Directory]::CreateDirectory($TmpDir) | Out-Null

function Cleanup {
    if (Test-Path $TmpDir) {
        Remove-Item -Path $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# ------------------------------------------------------------------------------
# HTTP Request Dispatcher
# ------------------------------------------------------------------------------
function Invoke-ApiRequest {
    param (
        [string]$Method,
        [string]$Endpoint,
        [string]$Token,
        [string]$Body,
        [int]$ExpectedStatusCode = 200,
        [string]$ContentType = "application/json"
    )

    $url = "$ApiBaseUrl$Endpoint"
    $headers = @{}
    if ($Token) {
        $headers["Authorization"] = "Bearer $Token"
        $headers["st-auth-mode"] = "header"
        $headers["Cookie"] = "sAccessToken=$Token"
    }

    $responseStatusCode = 0
    $responseBody = ""

    try {
        $params = @{
            Uri = $url
            Method = $Method
            Headers = $headers
            ErrorAction = "Stop"
        }
        if ($Body -and ($Method -in @("POST", "PUT", "PATCH"))) {
            $params["Body"] = $Body
            $params["ContentType"] = $ContentType
        }

        $res = Invoke-WebRequest @params
        $responseStatusCode = [int]$res.StatusCode
        $responseBody = $res.Content
    }
    catch [System.Net.WebException] {
        $webEx = $_.Exception
        if ($webEx.Response) {
            $responseStatusCode = [int]$webEx.Response.StatusCode
            $stream = $webEx.Response.GetResponseStream()
            if ($stream) {
                $reader = New-Object System.IO.StreamReader($stream)
                $responseBody = $reader.ReadToEnd()
                $reader.Close()
            }
        } else {
            $responseStatusCode = 0
            $responseBody = $webEx.Message
        }
    }
    catch {
        # Catch for PowerShell 7 / HttpResponseException
        if ($_.Exception.Response) {
            $responseStatusCode = [int]$_.Exception.Response.StatusCode
        }
        $responseBody = $_.ToString()
    }

    if ($responseStatusCode -ne $ExpectedStatusCode) {
        Log-Fail "Request: $Method $Endpoint"
        Log-Fail "Expected HTTP $ExpectedStatusCode, got HTTP $responseStatusCode"
        Log-Fail "Response body: $responseBody"
        Cleanup
        exit 1
    }

    return $responseBody
}

# ------------------------------------------------------------------------------
# SuperTokens Authentication Helpers
# ------------------------------------------------------------------------------
function Register-SuperTokensUser {
    param (
        [string]$Email,
        [string]$Password
    )

    $url = "$ApiBaseUrl/api/v1/auth/signup"
    $headers = @{
        "rid" = "emailpassword"
        "st-auth-mode" = "header"
        "Content-Type" = "application/json"
    }
    $body = @{
        formFields = @(
            @{ id = "email"; value = $Email },
            @{ id = "password"; value = $Password }
        )
    } | ConvertTo-Json -Depth 5

    $statusCode = 0
    $respBody = ""
    $respHeaders = @{}

    try {
        $res = Invoke-WebRequest -Uri $url -Method Post -Headers $headers -Body $body -ErrorAction Stop
        $statusCode = [int]$res.StatusCode
        $respBody = $res.Content
        $respHeaders = $res.Headers
    }
    catch [System.Net.WebException] {
        $webEx = $_.Exception
        if ($webEx.Response) {
            $statusCode = [int]$webEx.Response.StatusCode
            $respHeaders = $webEx.Response.Headers
            $stream = $webEx.Response.GetResponseStream()
            if ($stream) {
                $reader = New-Object System.IO.StreamReader($stream)
                $respBody = $reader.ReadToEnd()
                $reader.Close()
            }
        } else {
            $respBody = $webEx.Message
        }
    }
    catch {
        if ($_.Exception.Response) {
            $statusCode = [int]$_.Exception.Response.StatusCode
            $respHeaders = $_.Exception.Response.Headers
        }
        $respBody = $_.ToString()
    }

    $json = $null
    try {
        $json = $respBody | ConvertFrom-Json
    } catch { }

    $token = $null
    if ($respHeaders) {
        if ($respHeaders["st-access-token"]) {
            $token = $respHeaders["st-access-token"]
            if ($token -is [array]) { $token = $token[0] }
        }
        if (-not $token -and $respHeaders["Set-Cookie"]) {
            $cookieStr = [string]$respHeaders["Set-Cookie"]
            if ($cookieStr -match "sAccessToken=([^;]+)") {
                $token = $Matches[1]
            }
        }
    }

    $userId = $null
    if ($json -and $json.user -and $json.user.id) {
        $userId = $json.user.id
    }

    return @{
        StatusCode = $statusCode
        Body = $respBody
        Json = $json
        Token = $token
        UserId = $userId
    }
}

function Login-SuperTokensUser {
    param (
        [string]$Email,
        [string]$Password
    )

    $url = "$ApiBaseUrl/api/v1/auth/signin"
    $headers = @{
        "rid" = "emailpassword"
        "st-auth-mode" = "header"
        "Content-Type" = "application/json"
    }
    $body = @{
        formFields = @(
            @{ id = "email"; value = $Email },
            @{ id = "password"; value = $Password }
        )
    } | ConvertTo-Json -Depth 5

    $statusCode = 0
    $respBody = ""
    $respHeaders = @{}

    try {
        $res = Invoke-WebRequest -Uri $url -Method Post -Headers $headers -Body $body -ErrorAction Stop
        $statusCode = [int]$res.StatusCode
        $respBody = $res.Content
        $respHeaders = $res.Headers
    }
    catch [System.Net.WebException] {
        $webEx = $_.Exception
        if ($webEx.Response) {
            $statusCode = [int]$webEx.Response.StatusCode
            $respHeaders = $webEx.Response.Headers
            $stream = $webEx.Response.GetResponseStream()
            if ($stream) {
                $reader = New-Object System.IO.StreamReader($stream)
                $respBody = $reader.ReadToEnd()
                $reader.Close()
            }
        } else {
            $respBody = $webEx.Message
        }
    }
    catch {
        if ($_.Exception.Response) {
            $statusCode = [int]$_.Exception.Response.StatusCode
            $respHeaders = $_.Exception.Response.Headers
        }
        $respBody = $_.ToString()
    }

    $json = $null
    try {
        $json = $respBody | ConvertFrom-Json
    } catch { }

    $token = $null
    if ($respHeaders) {
        if ($respHeaders["st-access-token"]) {
            $token = $respHeaders["st-access-token"]
            if ($token -is [array]) { $token = $token[0] }
        }
        if (-not $token -and $respHeaders["Set-Cookie"]) {
            $cookieStr = [string]$respHeaders["Set-Cookie"]
            if ($cookieStr -match "sAccessToken=([^;]+)") {
                $token = $Matches[1]
            }
        }
    }

    $userId = $null
    if ($json -and $json.user -and $json.user.id) {
        $userId = $json.user.id
    }

    return @{
        StatusCode = $statusCode
        Body = $respBody
        Json = $json
        Token = $token
        UserId = $userId
    }
}

function Set-SuperTokensEmailVerified {
    param (
        [string]$UserId,
        [string]$Email
    )

    Log-Substep "Verifying campus email in SuperTokens Core for $Email ($UserId)..."

    $url = "$SuperTokensUrl/recipe/user/email/verify"
    $headers = @{
        "api-key" = $SuperTokensApiKey
        "Content-Type" = "application/json"
    }
    $body = @{
        userId = $UserId
        email = $Email
    } | ConvertTo-Json -Depth 5

    try {
        $res = Invoke-WebRequest -Uri $url -Method Post -Headers $headers -Body $body -ErrorAction Stop
        $json = $res.Content | ConvertFrom-Json
        if ($json.status -eq "OK" -or $json.status -eq "EMAIL_ALREADY_VERIFIED_ERROR") {
            Log-Pass "Campus email verified in SuperTokens: $Email"
            return $true
        }
        Log-Warn "SuperTokens email verify returned unexpected status: $($json.status)"
        return $false
    }
    catch {
        Log-Warn "Failed to verify email via SuperTokens Core: $_"
        return $false
    }
}

function Resolve-SuperTokensUser {
    param (
        [string]$EnvToken,
        [string]$Email,
        [string]$Password,
        [bool]$Verified
    )

    if ($EnvToken) {
        return @{ Token = $EnvToken; UserId = $null }
    }

    Log-Substep "Resolving SuperTokens test member: $Email (verified=$Verified)..."

    $reg = Register-SuperTokensUser -Email $Email -Password $Password
    $token = $reg.Token
    $userId = $reg.UserId

    if ($reg.Json -and $reg.Json.status -eq "EMAIL_ALREADY_EXISTS_ERROR") {
        Log-Substep "Member $Email already registered; signing in..."
        $login = Login-SuperTokensUser -Email $Email -Password $Password
        if ($login.Json -and $login.Json.status -eq "OK") {
            $token = $login.Token
            $userId = $login.UserId
        } else {
            Log-Fail "Failed to sign in existing member $($Email): $($login.Body)"
            Cleanup
            exit 1
        }
    } elseif ($reg.Json -and $reg.Json.status -ne "OK") {
        Log-Fail "Failed to register member $($Email): $($reg.Body)"
        Cleanup
        exit 1
    }

    if (-not $token) {
        Log-Fail "Could not extract SuperTokens access token for $Email"
        Cleanup
        exit 1
    }

    if ($Verified) {
        if (-not $userId) {
            $syncRes = Invoke-ApiRequest -Method "POST" -Endpoint "/api/v1/auth/sync" -Token $token -Body '{}' -ExpectedStatusCode 200
            $syncJson = $syncRes | ConvertFrom-Json
            $userId = $syncJson.data.id
        }
        if ($userId) {
            Set-SuperTokensEmailVerified -UserId $userId -Email $Email | Out-Null
        }
    }

    return @{ Token = $token; UserId = $userId }
}

# ------------------------------------------------------------------------------
# Multipart Form Upload Helper
# ------------------------------------------------------------------------------
function Upload-ResumeFile {
    param (
        [string]$FilePath,
        [string]$Token
    )

    $url = "$ApiBaseUrl/api/v1/profile/student/resume"

    # Use curl.exe if available (native on Windows 10+ and standard across environments)
    $curlCmd = Get-Command "curl.exe" -ErrorAction SilentlyContinue
    if ($curlCmd) {
        $result = & $curlCmd.Source -s -S -w "`n%{http_code}" -X POST $url `
            -H "Authorization: Bearer $Token" `
            -H "Cookie: sAccessToken=$Token" `
            -H "st-auth-mode: header" `
            -F "resume=@$FilePath;type=application/pdf" 2>&1
        $lines = $result -split "`n"
        $statusCode = [int]($lines[-1].Trim())
        $body = ($lines[0..($lines.Count - 2)] -join "`n").Trim()
        return @{ StatusCode = $statusCode; Body = $body }
    }

    # Fallback to .NET HttpClient
    Add-Type -AssemblyName System.Net.Http
    $client = New-Object System.Net.Http.HttpClient
    $client.DefaultRequestHeaders.Authorization = New-Object System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", $Token)
    $client.DefaultRequestHeaders.Add("Cookie", "sAccessToken=$Token")
    $client.DefaultRequestHeaders.Add("st-auth-mode", "header")

    $content = New-Object System.Net.Http.MultipartFormDataContent
    $fileBytes = [System.IO.File]::ReadAllBytes($FilePath)
    $byteContent = New-Object System.Net.Http.ByteArrayContent($fileBytes)
    $byteContent.Headers.ContentType = New-Object System.Net.Http.Headers.MediaTypeHeaderValue("application/pdf")
    $content.Add($byteContent, "resume", [System.IO.Path]::GetFileName($FilePath))

    $response = $client.PostAsync($url, $content).GetAwaiter().GetResult()
    $statusCode = [int]$response.StatusCode
    $body = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
    $client.Dispose()

    return @{ StatusCode = $statusCode; Body = $body }
}

# ==============================================================================
# Execution Entry Point
# ==============================================================================

try {
    Log-Info "Starting Lynk MVP End-to-End Integration Smoke Test Suite (PowerShell)"
    Log-Info "Target API:          $ApiBaseUrl"
    Log-Info "SuperTokens URL:     $SuperTokensUrl"
    Log-Info "MinIO URL:           $MinioUrl"

    # --------------------------------------------------------------------------
    # STEP 1: Health Check Verification
    # --------------------------------------------------------------------------
    Log-Step "1" "Infrastructure & API Health Checks"

    Log-Substep "Verifying Lynk Go REST API (/health)..."
    try {
        $apiHealthRes = Invoke-WebRequest -Uri "$ApiBaseUrl/health" -Method Get -ErrorAction Stop
        $apiHealth = $apiHealthRes.Content | ConvertFrom-Json
        if ($apiHealth.status -ne "ok") {
            Log-Fail "Go API health check did not return status: ok. Body: $($apiHealthRes.Content)"
            exit 1
        }
        Log-Pass "Go API is healthy: HTTP 200 (status: ok)"
    }
    catch {
        Log-Fail "Go API health check failed. Is Lynk API running on $ApiBaseUrl?"
        exit 1
    }

    if (-not $SkipInfraHealth) {
        Log-Substep "Verifying SuperTokens Core health (/hello)..."
        try {
            $stRes = Invoke-WebRequest -Uri "$SuperTokensUrl/hello" -Method Get -ErrorAction Stop
            if ([int]$stRes.StatusCode -ne 200 -or -not ($stRes.Content -like "*Hello*")) {
                Log-Fail "SuperTokens Core health check failed with HTTP $($stRes.StatusCode). Body: $($stRes.Content)"
                exit 1
            }
            Log-Pass "SuperTokens Core service is accessible: HTTP 200 (Hello)"
        }
        catch {
            Log-Fail "SuperTokens Core check failed. Is SuperTokens running on $SuperTokensUrl?"
            exit 1
        }

        Log-Substep "Verifying MinIO S3 health check (/minio/health/live)..."
        try {
            $minioRes = Invoke-WebRequest -Uri "$MinioUrl/minio/health/live" -Method Get -ErrorAction Stop
            if ([int]$minioRes.StatusCode -ne 200) {
                Log-Fail "MinIO health check failed with HTTP $($minioRes.StatusCode)"
                exit 1
            }
            Log-Pass "MinIO S3 service is healthy: HTTP 200"
        }
        catch {
            Log-Fail "MinIO health check failed. Is MinIO running on $MinioUrl?"
            exit 1
        }
    }

    # --------------------------------------------------------------------------
    # STEP 2: Institutional .edu Registration Gate Assertion (Hard Invariant)
    # --------------------------------------------------------------------------
    Log-Step "2" "Institutional .edu Registration Gate Assertion (Non-.edu Rejected)"

    Log-Substep "Attempting signup with unauthorized non-.edu address ($UnauthorizedEmail)..."
    $unauthReg = Register-SuperTokensUser -Email $UnauthorizedEmail -Password "Password123!"

    if ($unauthReg.Json -and $unauthReg.Json.status -eq "GENERAL_ERROR") {
        $errMsg = $unauthReg.Json.message
        if ($errMsg -like "*Registration rejected: only institutional .edu email addresses are permitted*") {
            Log-Pass "Non-.edu registration rejected as expected with GENERAL_ERROR: '$errMsg'"
        } else {
            Log-Fail "Expected rejection message to mention institutional .edu addresses, got: '$errMsg'"
            exit 1
        }
    } else {
        Log-Fail "Non-.edu address ($UnauthorizedEmail) was NOT rejected! Response: $($unauthReg.Body)"
        exit 1
    }

    # --------------------------------------------------------------------------
    # STEP 3: User Provisioning & Authentication (SuperTokens)
    # --------------------------------------------------------------------------
    Log-Step "3" "User Provisioning & Campus Authentication"

    Log-Substep "Resolving Employer user ($EmployerEmail)..."
    $empRes = Resolve-SuperTokensUser -EnvToken $EmployerToken -Email $EmployerEmail -Password $EmployerPassword -Verified $true
    $EmployerToken = $empRes.Token
    Log-Pass "Employer token acquired & campus email verified ($EmployerEmail)"

    Log-Substep "Resolving Verified Student user ($VerifiedStudentEmail)..."
    $stuRes = Resolve-SuperTokensUser -EnvToken $VerifiedStudentToken -Email $VerifiedStudentEmail -Password $VerifiedStudentPassword -Verified $true
    $VerifiedStudentToken = $stuRes.Token
    Log-Pass "Verified Student token acquired & campus email verified ($VerifiedStudentEmail)"

    Log-Substep "Resolving Unverified Student user ($UnverifiedStudentEmail)..."
    $unvRes = Resolve-SuperTokensUser -EnvToken $UnverifiedStudentToken -Email $UnverifiedStudentEmail -Password $UnverifiedStudentPassword -Verified $false
    $UnverifiedStudentToken = $unvRes.Token
    Log-Pass "Unverified Student token acquired (unverified for gate enforcement)"

    # --------------------------------------------------------------------------
    # STEP 4: Auth Sync for Test Users (POST /api/v1/auth/sync)
    # --------------------------------------------------------------------------
    Log-Step "4" "User Auth Synchronization (POST /api/v1/auth/sync)"

    Log-Substep "Syncing Employer member..."
    $empSyncResp = Invoke-ApiRequest -Method "POST" -Endpoint "/api/v1/auth/sync" -Token $EmployerToken -Body '{"role":"member"}' -ExpectedStatusCode 200
    $empSyncData = ($empSyncResp | ConvertFrom-Json).data
    $EmployerUserId = $empSyncData.id
    Log-Pass "Employer synced: ID $EmployerUserId"

    Log-Substep "Syncing Verified Student member..."
    $stuSyncResp = Invoke-ApiRequest -Method "POST" -Endpoint "/api/v1/auth/sync" -Token $VerifiedStudentToken -Body '{"role":"member"}' -ExpectedStatusCode 200
    $stuSyncData = ($stuSyncResp | ConvertFrom-Json).data
    $VerifiedStudentUserId = $stuSyncData.id
    Log-Pass "Verified student synced: ID $VerifiedStudentUserId"

    Log-Substep "Syncing Unverified Student member..."
    $unvSyncResp = Invoke-ApiRequest -Method "POST" -Endpoint "/api/v1/auth/sync" -Token $UnverifiedStudentToken -Body '{"role":"member"}' -ExpectedStatusCode 200
    $unvSyncData = ($unvSyncResp | ConvertFrom-Json).data
    $UnverifiedStudentUserId = $unvSyncData.id
    Log-Pass "Unverified student synced: ID $UnverifiedStudentUserId"

    # --------------------------------------------------------------------------
    # STEP 5: Student & Employer Profile Updates
    # --------------------------------------------------------------------------
    Log-Step "5" "Profile Configuration (PUT /api/v1/profile/student & employer)"

    Log-Substep "Updating student profile for verified student..."
    $studentProfilePayload = @{
        first_name = "Jordan"
        last_name = "Lee"
        bio = "CS undergraduate specializing in cloud infrastructure and distributed Go systems."
        department = "Computer Science"
        graduation_year = 2026
        skills = @("Go", "Docker", "PostgreSQL", "Next.js", "Kubernetes")
        portfolio_links = @("https://github.com/jordanlee", "https://jordanlee.dev")
    } | ConvertTo-Json -Depth 5

    $stuProfResp = Invoke-ApiRequest -Method "PUT" -Endpoint "/api/v1/profile/student" -Token $VerifiedStudentToken -Body $studentProfilePayload -ExpectedStatusCode 200
    $stuProfData = ($stuProfResp | ConvertFrom-Json).data
    if ($stuProfData.first_name -ne "Jordan") {
        Log-Fail "Student profile first_name mismatch (expected Jordan, got '$($stuProfData.first_name)')"
        exit 1
    }
    Log-Pass "Student profile updated successfully (First Name: $($stuProfData.first_name))"

    Log-Substep "Updating employer profile..."
    $employerProfilePayload = @{
        company_or_org = "Lynk Campus Innovations"
        contact_name = "Dr. Aris Thorne"
        description = "University research lab building student freelance marketplace platforms."
        website = "https://lynk.campus.edu"
    } | ConvertTo-Json -Depth 5

    $empProfResp = Invoke-ApiRequest -Method "PUT" -Endpoint "/api/v1/profile/employer" -Token $EmployerToken -Body $employerProfilePayload -ExpectedStatusCode 200
    $empProfData = ($empProfResp | ConvertFrom-Json).data
    if ($empProfData.company_or_org -ne "Lynk Campus Innovations") {
        Log-Fail "Employer profile company mismatch (expected Lynk Campus Innovations, got '$($empProfData.company_or_org)')"
        exit 1
    }
    Log-Pass "Employer profile updated successfully (Company: $($empProfData.company_or_org))"

    # --------------------------------------------------------------------------
    # STEP 6: Resume Upload & Presigned URL Fetch
    # --------------------------------------------------------------------------
    Log-Step "6" "Resume Upload & Presigned S3 Download URL Fetch"

    $SamplePdf = [System.IO.Path]::Combine($TmpDir, "sample_resume.pdf")
    $pdfContent = @"
%PDF-1.4
1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj
2 0 obj << /Type /Pages /Kids [3 0 R] /Count 1 >> endobj
3 0 obj << /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R >> endobj
4 0 obj << /Length 55 >> stream
BT
/F1 12 Tf
72 712 Td
(Jordan Lee - Student Resume) Tj
ET
endstream endobj
xref
0 5
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000214 00000 n 
trailer << /Size 5 /Root 1 0 R >>
startxref
318
%%EOF
"@
    [System.IO.File]::WriteAllText($SamplePdf, $pdfContent)

    Log-Substep "Uploading PDF resume for verified student..."
    $uploadResult = Upload-ResumeFile -FilePath $SamplePdf -Token $VerifiedStudentToken
    if ($uploadResult.StatusCode -ne 200) {
        Log-Fail "Resume upload failed: HTTP $($uploadResult.StatusCode). Body: $($uploadResult.Body)"
        exit 1
    }

    $uploadJson = $uploadResult.Body | ConvertFrom-Json
    $ResumeKey = $uploadJson.data.resume_key
    if (-not $ResumeKey) {
        Log-Fail "Resume upload did not return data.resume_key: $($uploadResult.Body)"
        exit 1
    }
    Log-Pass "Resume uploaded to MinIO bucket 'resumes'. Key: $ResumeKey"

    Log-Substep "Fetching presigned download URL for uploaded resume..."
    $resumeUrlResp = Invoke-ApiRequest -Method "GET" -Endpoint "/api/v1/profile/student/resume" -Token $VerifiedStudentToken -ExpectedStatusCode 200
    $resumeUrlJson = $resumeUrlResp | ConvertFrom-Json
    $downloadUrl = if ($resumeUrlJson.data.download_url) { $resumeUrlJson.data.download_url } else { $resumeUrlJson.data.url }

    if (-not $downloadUrl -or -not $downloadUrl.StartsWith("http")) {
        Log-Fail "Presigned download URL invalid or missing: $resumeUrlResp"
        exit 1
    }
    Log-Pass "Presigned S3 download URL generated successfully (Valid 15m)"

    # SEC-03 Assertion: Verify presigned download URL does not expose internal Docker hostname (minio:9000)
    Log-Substep "Asserting presigned URL public endpoint rewriting (SEC-03)..."
    if ($downloadUrl -match "://minio:9000") {
        Log-Fail "SEC-03: Presigned URL contains internal Docker hostname 'minio:9000': $downloadUrl"
        exit 1
    }
    Log-Pass "SEC-03: Presigned URL properly uses public authority: $downloadUrl"

    # --------------------------------------------------------------------------
    # STEP 7: Job Creation by Employer (POST /api/v1/jobs)
    # --------------------------------------------------------------------------
    Log-Step "7" "Job Creation by Employer (POST /api/v1/jobs)"

    $jobPayload = @{
        title = "Campus Marketplace Go & Next.js Engineer"
        description = "Looking for an energetic student engineer to develop high-trust freelance marketplace features with Go, Chi, PostgreSQL, and Next.js."
        budget = 950.00
        pay_type = "fixed"
        required_skills = @("Go", "PostgreSQL", "Docker", "Next.js")
        department = "Computer Science"
    } | ConvertTo-Json -Depth 5

    $jobCreateResp = Invoke-ApiRequest -Method "POST" -Endpoint "/api/v1/jobs" -Token $EmployerToken -Body $jobPayload -ExpectedStatusCode 201
    $jobCreateData = ($jobCreateResp | ConvertFrom-Json).data
    $JobId = $jobCreateData.id
    $JobStatus = $jobCreateData.status

    if (-not $JobId -or $JobStatus -ne "open") {
        Log-Fail "Job creation failed. Response: $jobCreateResp"
        exit 1
    }
    Log-Pass "Job created successfully: UUID $JobId (Status: $JobStatus)"

    # --------------------------------------------------------------------------
    # STEP 8: Job Filtering & Detail Retrieval
    # --------------------------------------------------------------------------
    Log-Step "8" "Job Search, Filtering & Detail Retrieval"

    Log-Substep "Filtering jobs with query params (search=Marketplace&department=Computer+Science)..."
    $jobFilterResp = Invoke-ApiRequest -Method "GET" -Endpoint "/api/v1/jobs?search=Marketplace&department=Computer+Science&skill=Go" -ExpectedStatusCode 200
    if (-not ($jobFilterResp -like "*$JobId*")) {
        Log-Fail "Created job $JobId was not found in filtered listings response: $jobFilterResp"
        exit 1
    }
    Log-Pass "Job discovered via public search & filter query"

    Log-Substep "Retrieving single job detail (GET /api/v1/jobs/$JobId)..."
    $jobDetailResp = Invoke-ApiRequest -Method "GET" -Endpoint "/api/v1/jobs/$JobId" -ExpectedStatusCode 200
    $jobDetailData = ($jobDetailResp | ConvertFrom-Json).data
    if ($jobDetailData.id -ne $JobId) {
        Log-Fail "Job detail ID mismatch: expected $JobId, got '$($jobDetailData.id)'"
        exit 1
    }
    Log-Pass "Job details retrieved successfully: Budget `$$($jobDetailData.budget)"

    # --------------------------------------------------------------------------
    # STEP 9: Campus Email Verification Gate Enforcement (HTTP 403 EMAIL_NOT_VERIFIED)
    # --------------------------------------------------------------------------
    Log-Step "9" "Campus Email Verification Gate Enforcement (HTTP 403 EMAIL_NOT_VERIFIED)"

    Log-Substep "Assertion 9a: Unverified student attempts job creation (Must return 403 EMAIL_NOT_VERIFIED)..."
    $unvJobPayload = @{
        title = "Unauthorized Opportunity"
        description = "Should be blocked by campus verification gate."
        budget = 100.00
        pay_type = "fixed"
        required_skills = @("Go")
        department = "Computer Science"
    } | ConvertTo-Json -Depth 5

    $gateJobResp = Invoke-ApiRequest -Method "POST" -Endpoint "/api/v1/jobs" -Token $UnverifiedStudentToken -Body $unvJobPayload -ExpectedStatusCode 403
    $gateJobJson = $gateJobResp | ConvertFrom-Json
    if ($gateJobJson.error.code -ne "EMAIL_NOT_VERIFIED") {
        Log-Fail "Expected error code EMAIL_NOT_VERIFIED on job creation, got '$($gateJobJson.error.code)'"
        exit 1
    }
    if (-not ($gateJobJson.error.message -like "*Campus verification*")) {
        Log-Fail "Expected error message to mention 'Campus verification', got '$($gateJobJson.error.message)'"
        exit 1
    }
    Log-Pass "Job creation strictly blocked for unverified member: HTTP 403 (EMAIL_NOT_VERIFIED: $($gateJobJson.error.message))"

    Log-Substep "Assertion 9b: Unverified student attempts resume upload (Must return 403 EMAIL_NOT_VERIFIED)..."
    $gateUploadResult = Upload-ResumeFile -FilePath $SamplePdf -Token $UnverifiedStudentToken
    if ($gateUploadResult.StatusCode -ne 403) {
        Log-Fail "Email Gate failed on resume upload: Expected HTTP 403, got HTTP $($gateUploadResult.StatusCode)"
        Log-Fail "Body: $($gateUploadResult.Body)"
        exit 1
    }
    $gateUploadJson = $gateUploadResult.Body | ConvertFrom-Json
    if ($gateUploadJson.error.code -ne "EMAIL_NOT_VERIFIED") {
        Log-Fail "Email Gate returned wrong error code on resume upload: expected EMAIL_NOT_VERIFIED, got '$($gateUploadJson.error.code)'"
        exit 1
    }
    Log-Pass "Resume upload strictly blocked for unverified member: HTTP 403 (EMAIL_NOT_VERIFIED)"

    Log-Substep "Assertion 9c: Unverified student attempts job apply (Must return 403 EMAIL_NOT_VERIFIED)..."
    $unvApplyPayload = @{
        cover_letter = "Attempting application without verified university email."
    } | ConvertTo-Json

    $gateApplyResp = Invoke-ApiRequest -Method "POST" -Endpoint "/api/v1/jobs/$JobId/applications" -Token $UnverifiedStudentToken -Body $unvApplyPayload -ExpectedStatusCode 403
    $gateApplyJson = $gateApplyResp | ConvertFrom-Json
    if ($gateApplyJson.error.code -ne "EMAIL_NOT_VERIFIED") {
        Log-Fail "Email Gate returned wrong error code on job apply: expected EMAIL_NOT_VERIFIED, got '$($gateApplyJson.error.code)'"
        exit 1
    }
    Log-Pass "Job application strictly blocked for unverified member: HTTP 403 (EMAIL_NOT_VERIFIED)"

    # --------------------------------------------------------------------------
    # STEP 10: Verified Student Job Application (POST /api/v1/jobs/{id}/applications)
    # --------------------------------------------------------------------------
    Log-Step "10" "Verified Student Job Application Submission"

    $applyPayload = @{
        cover_letter = "I have extensive experience building scalable microservices in Go and full-stack Next.js web applications."
        resume_key = $ResumeKey
    } | ConvertTo-Json

    $applyResp = Invoke-ApiRequest -Method "POST" -Endpoint "/api/v1/jobs/$JobId/applications" -Token $VerifiedStudentToken -Body $applyPayload -ExpectedStatusCode 201
    $applyData = ($applyResp | ConvertFrom-Json).data
    $ApplicationId = $applyData.id
    $ApplicationStatus = $applyData.status

    if (-not $ApplicationId -or $ApplicationStatus -ne "pending") {
        Log-Fail "Application submission failed: $applyResp"
        exit 1
    }
    Log-Pass "Application submitted successfully: UUID $ApplicationId (Status: $ApplicationStatus)"

    # SEC-07 Assertion: Job poster downloads applicant resume via profile route after application
    Log-Substep "Fetching applicant resume via GET /api/v1/profile/{id}/resume (SEC-07)..."
    $memberResumeResp = Invoke-ApiRequest -Method "GET" -Endpoint "/api/v1/profile/$VerifiedStudentUserId/resume" -Token $EmployerToken -ExpectedStatusCode 200
    $memberResumeJson = $memberResumeResp | ConvertFrom-Json
    $presigned = $memberResumeJson.data.download_url
    if (-not $presigned) { $presigned = $memberResumeJson.data.url }
    if (-not $presigned) { throw "SEC-07: missing presigned download URL in JSON body: $memberResumeResp" }
    $dl = Invoke-WebRequest -Uri $presigned -Method GET -MaximumRedirection 0 -SkipHttpErrorCheck
    if ($dl.StatusCode -ne 200) { throw "SEC-07: presigned GET expected 200, got $($dl.StatusCode)" }
    Log-Pass "SEC-07: Job poster retrieved applicant resume via /profile/{id}/resume and GET presigned URL"

    # --------------------------------------------------------------------------
    # STEP 11: Employer Applicant Review & Acceptance (Atomic Contract Generation)
    # --------------------------------------------------------------------------
    Log-Step "11" "Employer Applicant Review & Acceptance"

    Log-Substep "Employer lists applications for job $JobId..."
    $appListResp = Invoke-ApiRequest -Method "GET" -Endpoint "/api/v1/jobs/$JobId/applications" -Token $EmployerToken -ExpectedStatusCode 200
    if (-not ($appListResp -like "*$ApplicationId*")) {
        Log-Fail "Submitted application $ApplicationId not found in employer job applications: $appListResp"
        exit 1
    }
    Log-Pass "Application visible in employer candidate list"

    Log-Substep "Employer accepts application $ApplicationId (triggers atomic contract creation)..."
    $acceptResp = Invoke-ApiRequest -Method "PATCH" -Endpoint "/api/v1/applications/$ApplicationId/status" -Token $EmployerToken -Body '{"status":"accepted"}' -ExpectedStatusCode 200
    $acceptData = ($acceptResp | ConvertFrom-Json).data
    $ContractId = $acceptData.contract.id
    $ContractStatus = $acceptData.contract.status

    if ($acceptData.status -ne "accepted") {
        Log-Fail "Expected application status 'accepted', got '$($acceptData.status)'"
        exit 1
    }

    if (-not $ContractId -or $ContractStatus -ne "active") {
        Log-Fail "Atomic contract creation failed. Response: $acceptResp"
        exit 1
    }
    Log-Pass "Application accepted and atomic Contract created: UUID $ContractId (Status: $ContractStatus)"

    # --------------------------------------------------------------------------
    # STEP 12: Contract Status Progression (Active -> Completed)
    # --------------------------------------------------------------------------
    Log-Step "12" "Contract State Machine Progression (active -> completed)"

    Log-Substep "Inspecting active contract detail (GET /api/v1/contracts/$ContractId)..."
    $contractGetResp = Invoke-ApiRequest -Method "GET" -Endpoint "/api/v1/contracts/$ContractId" -Token $EmployerToken -ExpectedStatusCode 200
    $contractGetData = ($contractGetResp | ConvertFrom-Json).data

    if ($contractGetData.status -ne "active") {
        Log-Fail "Expected initial contract status 'active', got '$($contractGetData.status)'"
        exit 1
    }
    Log-Pass "Contract verified in active state (Agreed Budget: `$$($contractGetData.agreed_budget))"

    # SEC-06 Assertion: Freelancer attempting to mark contract completed must receive 403 Forbidden
    Log-Substep "Asserting freelancer cannot mark contract completed (SEC-06)..."
    $freelancerCompleteResp = Invoke-ApiRequest -Method "PATCH" -Endpoint "/api/v1/contracts/$ContractId/status" -Token $VerifiedStudentToken -Body '{"status":"completed"}' -ExpectedStatusCode 403
    Log-Pass "SEC-06: Freelancer completion attempt rejected with HTTP 403 Forbidden"

    Log-Substep "Transitioning contract status to 'completed' as client..."
    $completeResp = Invoke-ApiRequest -Method "PATCH" -Endpoint "/api/v1/contracts/$ContractId/status" -Token $EmployerToken -Body '{"status":"completed"}' -ExpectedStatusCode 200
    $completeData = ($completeResp | ConvertFrom-Json).data

    if ($completeData.status -ne "completed") {
        Log-Fail "Contract transition to completed failed. Response: $completeResp"
        exit 1
    }
    Log-Pass "Contract successfully transitioned to terminal state: completed"

    # SEC-05 Assertion: Parent job automatically transitioned to 'closed'
    Log-Substep "Asserting parent job status automatically transitioned to 'closed' (SEC-05)..."
    $jobCheckResp = Invoke-ApiRequest -Method "GET" -Endpoint "/api/v1/jobs/$JobId" -Token $EmployerToken -ExpectedStatusCode 200
    $jobCheckData = ($jobCheckResp | ConvertFrom-Json).data
    if ($jobCheckData.status -ne "closed") {
        Log-Fail "SEC-05: Expected job status 'closed', got '$($jobCheckData.status)'"
        exit 1
    }
    Log-Pass "SEC-05: Parent job atomically transitioned to 'closed' upon contract completion"

    # --------------------------------------------------------------------------
    # STEP 13: Peer Review Submission & Duplicate Conflict Assertion
    # --------------------------------------------------------------------------
    Log-Step "13" "Peer Review Submission & Duplicate Review Conflict Check"

    Log-Substep "Employer submits 5-star review for student on completed contract..."
    $empReviewPayload = @{
        rating = 5
        comment = "Exceptional work! Jordan delivered clean Go code, thorough unit tests, and excellent communication throughout the project."
    } | ConvertTo-Json

    $empRevResp = Invoke-ApiRequest -Method "POST" -Endpoint "/api/v1/contracts/$ContractId/reviews" -Token $EmployerToken -Body $empReviewPayload -ExpectedStatusCode 201
    $empRevData = ($empRevResp | ConvertFrom-Json).data
    if ($empRevData.rating -ne 5) {
        Log-Fail "Employer review rating mismatch: expected 5, got '$($empRevData.rating)'"
        exit 1
    }
    Log-Pass "Employer review submitted successfully: Rating 5/5"

    Log-Substep "Student submits 5-star review for employer on completed contract..."
    $stuReviewPayload = @{
        rating = 5
        comment = "Outstanding employer to work with! Clear requirements, clear deliverables, and prompt feedback."
    } | ConvertTo-Json

    $stuRevResp = Invoke-ApiRequest -Method "POST" -Endpoint "/api/v1/contracts/$ContractId/reviews" -Token $VerifiedStudentToken -Body $stuReviewPayload -ExpectedStatusCode 201
    $stuRevData = ($stuRevResp | ConvertFrom-Json).data
    if ($stuRevData.rating -ne 5) {
        Log-Fail "Student review rating mismatch: expected 5, got '$($stuRevData.rating)'"
        exit 1
    }
    Log-Pass "Student review submitted successfully: Rating 5/5"

    Log-Substep "Duplicate Conflict Assertion: Employer attempts duplicate review on same contract (Must return 409)..."
    $dupReviewPayload = @{
        rating = 4
        comment = "Attempting duplicate review submission."
    } | ConvertTo-Json

    $dupResp = Invoke-ApiRequest -Method "POST" -Endpoint "/api/v1/contracts/$ContractId/reviews" -Token $EmployerToken -Body $dupReviewPayload -ExpectedStatusCode 409
    $dupJson = $dupResp | ConvertFrom-Json
    if ($dupJson.error.code -ne "DUPLICATE_REVIEW") {
        Log-Fail "Expected error code DUPLICATE_REVIEW, got '$($dupJson.error.code)'"
        exit 1
    }
    Log-Pass "Duplicate review blocked with HTTP 409 Conflict (DUPLICATE_REVIEW)"

    Log-Substep "Verifying contract reviews list (GET /api/v1/contracts/$ContractId/reviews)..."
    $reviewsResp = Invoke-ApiRequest -Method "GET" -Endpoint "/api/v1/contracts/$ContractId/reviews" -ExpectedStatusCode 200
    $reviewsData = ($reviewsResp | ConvertFrom-Json).data
    if ($reviewsData.Count -lt 2) {
        Log-Fail "Contract reviews list incomplete: count is $($reviewsData.Count)"
        exit 1
    }
    Log-Pass "Contract reviews verified: Both counterparties recorded"

    # SEC-01 Assertion: Reviews return profile names via joined profiles table
    Log-Substep "Asserting reviewer profile joins return names without SQL errors (SEC-01)..."
    foreach ($rev in $reviewsData) {
        if (-not $rev.reviewer -or -not $rev.reviewer.first_name) {
            Log-Fail "SEC-01: Review missing reviewer profile first_name: $($rev | ConvertTo-Json -Depth 3)"
            exit 1
        }
    }
    Log-Pass "SEC-01: Reviews successfully joined profiles table returning member names"

    # --------------------------------------------------------------------------
    # SUMMARY
    # --------------------------------------------------------------------------
    Write-Host ""
    Write-Host ("=" * 70) -ForegroundColor Green
    Write-Host "  ALL 13 END-TO-END SMOKE TEST PHASES PASSED SUCCESSFULLY!            " -ForegroundColor Green
    Write-Host ("=" * 70) -ForegroundColor Green
    Write-Host "  Job ID:         $JobId" -ForegroundColor Cyan
    Write-Host "  Application ID: $ApplicationId" -ForegroundColor Cyan
    Write-Host "  Contract ID:    $ContractId" -ForegroundColor Cyan
    Write-Host "  Resume Key:     $ResumeKey" -ForegroundColor Cyan
    Write-Host "  Exit code:      0 (Verified)`n" -ForegroundColor Green

    Cleanup
    exit 0
}
catch {
    Log-Fail "Smoke test encountered an unexpected exception: $_"
    Cleanup
    exit 1
}
