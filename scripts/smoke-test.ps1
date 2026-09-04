# ==============================================================================
# Lynk Platform — End-to-End Integration Smoke Test Suite (PowerShell)
#
# Exercises the complete MVP lifecycle against running local infrastructure:
#   1. Infrastructure Health Checks (Go API, Keycloak realm, MinIO S3)
#   2. Auth Sync for Test Users (Employer, Verified Student, Unverified Student)
#   3. Student & Employer Profile Updates
#   4. Resume Upload (Multipart PDF) & Presigned Download URL Fetch
#   5. Job Creation by Employer
#   6. Job Filtering, Search & Detail Retrieval
#   7. Institutional Email Gate Enforcement (HTTP 403 EMAIL_NOT_VERIFIED)
#   8. Verified Student Job Application
#   9. Employer Application Review & Acceptance (Atomic Contract Generation)
#  10. Contract Status Progression (Active -> Completed)
#  11. Peer Review Submission & Duplicate Conflict Assertion (HTTP 409)
#
# Invariants:
#   - Exit code 0 on all assertions passing.
#   - Non-zero exit code (exit 1) with colored diagnostic logs on any assertion failure.
#   - Accepts pre-acquired tokens via environment variables OR acquires them
#     via Keycloak direct grant (with automated user provisioning fallback).
# ==============================================================================

[CmdletBinding()]
param (
    [string]$ApiBaseUrl = $(if ($env:API_BASE_URL) { $env:API_BASE_URL } else { "http://localhost:8080" }),
    [string]$KeycloakUrl = $(if ($env:KEYCLOAK_URL) { $env:KEYCLOAK_URL } else { "http://localhost:8081" }),
    [string]$KeycloakRealm = $(if ($env:KEYCLOAK_REALM) { $env:KEYCLOAK_REALM } else { "lynk" }),
    [string]$KeycloakClientId = $(if ($env:KEYCLOAK_CLIENT_ID) { $env:KEYCLOAK_CLIENT_ID } else { "lynk-frontend" }),
    [string]$MinioUrl = $(if ($env:MINIO_URL) { $env:MINIO_URL } else { "http://localhost:9000" }),
    [string]$EmployerToken = $env:EMPLOYER_TOKEN,
    [string]$VerifiedStudentToken = $env:VERIFIED_STUDENT_TOKEN,
    [string]$UnverifiedStudentToken = $env:UNVERIFIED_STUDENT_TOKEN,
    [string]$EmployerUsername = $(if ($env:EMPLOYER_USERNAME) { $env:EMPLOYER_USERNAME } else { "employer@lynk.test" }),
    [string]$EmployerPassword = $(if ($env:EMPLOYER_PASSWORD) { $env:EMPLOYER_PASSWORD } else { "password123" }),
    [string]$VerifiedStudentUsername = $(if ($env:VERIFIED_STUDENT_USERNAME) { $env:VERIFIED_STUDENT_USERNAME } else { "student.verified@stanford.edu" }),
    [string]$VerifiedStudentPassword = $(if ($env:VERIFIED_STUDENT_PASSWORD) { $env:VERIFIED_STUDENT_PASSWORD } else { "password123" }),
    [string]$UnverifiedStudentUsername = $(if ($env:UNVERIFIED_STUDENT_USERNAME) { $env:UNVERIFIED_STUDENT_USERNAME } else { "student.unverified@stanford.edu" }),
    [string]$UnverifiedStudentPassword = $(if ($env:UNVERIFIED_STUDENT_PASSWORD) { $env:UNVERIFIED_STUDENT_PASSWORD } else { "password123" }),
    [string]$KeycloakAdmin = $(if ($env:KEYCLOAK_ADMIN) { $env:KEYCLOAK_ADMIN } else { "admin" }),
    [string]$KeycloakAdminPassword = $(if ($env:KEYCLOAK_ADMIN_PASSWORD) { $env:KEYCLOAK_ADMIN_PASSWORD } else { "admin_password" }),
    [switch]$SkipInfraHealth = $(if ($env:SKIP_INFRA_HEALTH -eq "1") { $true } else { $false })
)

$ErrorActionPreference = "Stop"

# ------------------------------------------------------------------------------
# Logging Helpers
# ------------------------------------------------------------------------------
function Log-Info ($msg) {
    Write-Host "[INFO] $msg" -ForegroundColor Cyan
}

function Log-Step ($num, $title) {
    Write-Host ""
    Write-Host ("=" * 70) -ForegroundColor Blue
    Write-Host "[STEP $num/11] $title" -ForegroundColor Magenta
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
# Keycloak Token Acquisition & User Provisioning Helper
# ------------------------------------------------------------------------------
function Get-DirectGrantToken {
    param (
        [string]$Username,
        [string]$Password
    )

    $tokenUrl = "$KeycloakUrl/realms/$KeycloakRealm/protocol/openid-connect/token"
    $body = "grant_type=password&client_id=$KeycloakClientId&username=$([System.Uri]::EscapeDataString($Username))&password=$([System.Uri]::EscapeDataString($Password))"

    try {
        $res = Invoke-WebRequest -Uri $tokenUrl -Method Post -ContentType "application/x-www-form-urlencoded" -Body $body -ErrorAction Stop
        $json = $res.Content | ConvertFrom-Json
        if ($json.access_token) {
            return $json.access_token
        }
    }
    catch {
        return $null
    }
    return $null
}

function Provision-KeycloakUser {
    param (
        [string]$Username,
        [string]$Password,
        [string]$Email,
        [string]$Role,
        [bool]$Verified
    )

    Log-Substep "Provisioning Keycloak test user: $Username ($Role, verified=$Verified)..."

    $adminTokenUrl = "$KeycloakUrl/realms/master/protocol/openid-connect/token"
    $adminBody = "grant_type=password&client_id=admin-cli&username=$([System.Uri]::EscapeDataString($KeycloakAdmin))&password=$([System.Uri]::EscapeDataString($KeycloakAdminPassword))"

    $adminToken = $null
    try {
        $adminRes = Invoke-WebRequest -Uri $adminTokenUrl -Method Post -ContentType "application/x-www-form-urlencoded" -Body $adminBody -ErrorAction Stop
        $adminJson = $adminRes.Content | ConvertFrom-Json
        $adminToken = $adminJson.access_token
    }
    catch {
        Log-Warn "Unable to acquire Keycloak admin token. Automated provisioning skipped."
        return $false
    }

    if (-not $adminToken) {
        return $false
    }

    $adminHeaders = @{
        "Authorization" = "Bearer $adminToken"
    }

    # 1. Check if user already exists
    $userId = $null
    try {
        $userSearchUrl = "$KeycloakUrl/admin/realms/$KeycloakRealm/users?username=$([System.Uri]::EscapeDataString($Username))"
        $usersRes = Invoke-WebRequest -Uri $userSearchUrl -Method Get -Headers $adminHeaders -ErrorAction Stop
        $users = $usersRes.Content | ConvertFrom-Json
        if ($users -and $users.Count -gt 0) {
            $userId = $users[0].id
        }
    }
    catch { }

    # 2. Create user if not exists
    if (-not $userId) {
        $userPayload = @{
            username = $Username
            email = $Email
            emailVerified = $Verified
            enabled = $true
            credentials = @(
                @{
                    type = "password"
                    value = $Password
                    temporary = $false
                }
            )
        } | ConvertTo-Json -Depth 5

        try {
            Invoke-WebRequest -Uri "$KeycloakUrl/admin/realms/$KeycloakRealm/users" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $userPayload -ErrorAction Stop | Out-Null
        }
        catch { }

        # Query user ID again
        try {
            $usersRes = Invoke-WebRequest -Uri "$KeycloakUrl/admin/realms/$KeycloakRealm/users?username=$([System.Uri]::EscapeDataString($Username))" -Method Get -Headers $adminHeaders -ErrorAction Stop
            $users = $usersRes.Content | ConvertFrom-Json
            if ($users -and $users.Count -gt 0) {
                $userId = $users[0].id
            }
        }
        catch { }
    }

    if (-not $userId) {
        Log-Warn "Failed to resolve user ID for $Username after creation attempt."
        return $false
    }

    # 3. Map realm role
    try {
        $roleUrl = "$KeycloakUrl/admin/realms/$KeycloakRealm/roles/$Role"
        $roleRes = Invoke-WebRequest -Uri $roleUrl -Method Get -Headers $adminHeaders -ErrorAction Stop
        $roleObj = $roleRes.Content | ConvertFrom-Json
        if ($roleObj.id) {
            $rolePayload = @(
                @{
                    id = $roleObj.id
                    name = $Role
                }
            ) | ConvertTo-Json -Depth 5
            Invoke-WebRequest -Uri "$KeycloakUrl/admin/realms/$KeycloakRealm/users/$userId/role-mappings/realm" -Method Post -Headers $adminHeaders -ContentType "application/json" -Body $rolePayload -ErrorAction Stop | Out-Null
        }
    }
    catch { }

    return $true
}

function Resolve-Token {
    param (
        [string]$EnvToken,
        [string]$Username,
        [string]$Password,
        [string]$Email,
        [string]$Role,
        [bool]$Verified
    )

    if ($EnvToken) {
        return $EnvToken
    }

    $token = Get-DirectGrantToken -Username $Username -Password $Password
    if ($token) {
        return $token
    }

    $provisioned = Provision-KeycloakUser -Username $Username -Password $Password -Email $Email -Role $Role -Verified $Verified
    if ($provisioned) {
        $token = Get-DirectGrantToken -Username $Username -Password $Password
        if ($token) {
            return $token
        }
    }

    Log-Fail "Could not acquire JWT for $Username ($Role)."
    Log-Fail "Please ensure Keycloak is running at $KeycloakUrl or export token environment variable."
    Cleanup
    exit 1
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
        $result = & $curlCmd.Source -s -S -w "`n%{http_code}" -X POST $url -H "Authorization: Bearer $Token" -F "resume=@$FilePath;type=application/pdf" 2>&1
        $lines = $result -split "`n"
        $statusCode = [int]($lines[-1].Trim())
        $body = ($lines[0..($lines.Count - 2)] -join "`n").Trim()
        return @{ StatusCode = $statusCode; Body = $body }
    }

    # Fallback to .NET HttpClient
    Add-Type -AssemblyName System.Net.Http
    $client = New-Object System.Net.Http.HttpClient
    $client.DefaultRequestHeaders.Authorization = New-Object System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", $Token)

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
    Log-Info "Keycloak URL:        $KeycloakUrl (realm: $KeycloakRealm)"
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
        Log-Substep "Verifying Keycloak realm discovery (/realms/$KeycloakRealm)..."
        try {
            $kcRes = Invoke-WebRequest -Uri "$KeycloakUrl/realms/$KeycloakRealm" -Method Get -ErrorAction Stop
            if ([int]$kcRes.StatusCode -ne 200) {
                Log-Fail "Keycloak realm discovery failed with HTTP $($kcRes.StatusCode)"
                exit 1
            }
            Log-Pass "Keycloak realm '$KeycloakRealm' is accessible: HTTP 200"
        }
        catch {
            Log-Fail "Keycloak realm check failed. Is Keycloak running on $KeycloakUrl?"
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
    # Token Resolution for Test Actors
    # --------------------------------------------------------------------------
    Log-Info "Resolving authentication tokens for test actors..."

    Log-Substep "Resolving Employer token..."
    $EmployerToken = Resolve-Token -EnvToken $EmployerToken -Username $EmployerUsername -Password $EmployerPassword -Email $EmployerUsername -Role "employer" -Verified $true
    Log-Pass "Employer token acquired"

    Log-Substep "Resolving Verified Student token..."
    $VerifiedStudentToken = Resolve-Token -EnvToken $VerifiedStudentToken -Username $VerifiedStudentUsername -Password $VerifiedStudentPassword -Email $VerifiedStudentUsername -Role "student" -Verified $true
    Log-Pass "Verified Student token acquired (institutional email verified)"

    Log-Substep "Resolving Unverified Student token..."
    $UnverifiedStudentToken = Resolve-Token -EnvToken $UnverifiedStudentToken -Username $UnverifiedStudentUsername -Password $UnverifiedStudentPassword -Email $UnverifiedStudentUsername -Role "student" -Verified $false
    Log-Pass "Unverified Student token acquired (email_verified=false for gate enforcement)"

    # --------------------------------------------------------------------------
    # STEP 2: Auth Sync for Test Users (POST /api/v1/auth/sync)
    # --------------------------------------------------------------------------
    Log-Step "2" "User Auth Synchronization (POST /api/v1/auth/sync)"

    Log-Substep "Syncing Employer user..."
    $empSyncResp = Invoke-ApiRequest -Method "POST" -Endpoint "/api/v1/auth/sync" -Token $EmployerToken -Body '{"role":"employer"}' -ExpectedStatusCode 200
    $empSyncData = ($empSyncResp | ConvertFrom-Json).data
    $EmployerUserId = $empSyncData.id
    Log-Pass "Employer synced: UUID $EmployerUserId"

    Log-Substep "Syncing Verified Student user..."
    $stuSyncResp = Invoke-ApiRequest -Method "POST" -Endpoint "/api/v1/auth/sync" -Token $VerifiedStudentToken -Body '{"role":"student"}' -ExpectedStatusCode 200
    $stuSyncData = ($stuSyncResp | ConvertFrom-Json).data
    $VerifiedStudentUserId = $stuSyncData.id
    Log-Pass "Verified student synced: UUID $VerifiedStudentUserId"

    Log-Substep "Syncing Unverified Student user..."
    $unvSyncResp = Invoke-ApiRequest -Method "POST" -Endpoint "/api/v1/auth/sync" -Token $UnverifiedStudentToken -Body '{"role":"student"}' -ExpectedStatusCode 200
    $unvSyncData = ($unvSyncResp | ConvertFrom-Json).data
    $UnverifiedStudentUserId = $unvSyncData.id
    Log-Pass "Unverified student synced: UUID $UnverifiedStudentUserId"

    # --------------------------------------------------------------------------
    # STEP 3: Student & Employer Profile Updates
    # --------------------------------------------------------------------------
    Log-Step "3" "Profile Configuration (PUT /api/v1/profile/student & employer)"

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
    # STEP 4: Resume Upload & Presigned URL Fetch
    # --------------------------------------------------------------------------
    Log-Step "4" "Resume Upload & Presigned S3 Download URL Fetch"

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

    # --------------------------------------------------------------------------
    # STEP 5: Job Creation by Employer (POST /api/v1/jobs)
    # --------------------------------------------------------------------------
    Log-Step "5" "Job Creation by Employer (POST /api/v1/jobs)"

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
    # STEP 6: Job Filtering & Detail Retrieval
    # --------------------------------------------------------------------------
    Log-Step "6" "Job Search, Filtering & Detail Retrieval"

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
    # STEP 7: Institutional Email Gate Enforcement (Hard Invariant Verification)
    # --------------------------------------------------------------------------
    Log-Step "7" "Institutional Email Gate Enforcement (HTTP 403 EMAIL_NOT_VERIFIED)"

    Log-Substep "Assertion 7a: Unverified student attempts resume upload (Must return 403 EMAIL_NOT_VERIFIED)..."
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
    Log-Pass "Resume upload strictly blocked for unverified student: HTTP 403 (EMAIL_NOT_VERIFIED)"

    Log-Substep "Assertion 7b: Unverified student attempts job apply (Must return 403 EMAIL_NOT_VERIFIED)..."
    $unvApplyPayload = @{
        cover_letter = "Attempting application without verified university email."
    } | ConvertTo-Json

    $gateApplyResp = Invoke-ApiRequest -Method "POST" -Endpoint "/api/v1/jobs/$JobId/applications" -Token $UnverifiedStudentToken -Body $unvApplyPayload -ExpectedStatusCode 403
    $gateApplyJson = $gateApplyResp | ConvertFrom-Json
    if ($gateApplyJson.error.code -ne "EMAIL_NOT_VERIFIED") {
        Log-Fail "Email Gate returned wrong error code on job apply: expected EMAIL_NOT_VERIFIED, got '$($gateApplyJson.error.code)'"
        exit 1
    }
    Log-Pass "Job application strictly blocked for unverified student: HTTP 403 (EMAIL_NOT_VERIFIED)"

    # --------------------------------------------------------------------------
    # STEP 8: Verified Student Job Application (POST /api/v1/jobs/{id}/applications)
    # --------------------------------------------------------------------------
    Log-Step "8" "Verified Student Job Application Submission"

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

    # --------------------------------------------------------------------------
    # STEP 9: Employer Applicant Review & Acceptance (Atomic Contract Generation)
    # --------------------------------------------------------------------------
    Log-Step "9" "Employer Applicant Review & Acceptance"

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
    # STEP 10: Contract Status Progression (Active -> Completed)
    # --------------------------------------------------------------------------
    Log-Step "10" "Contract State Machine Progression (active -> completed)"

    Log-Substep "Inspecting active contract detail (GET /api/v1/contracts/$ContractId)..."
    $contractGetResp = Invoke-ApiRequest -Method "GET" -Endpoint "/api/v1/contracts/$ContractId" -Token $EmployerToken -ExpectedStatusCode 200
    $contractGetData = ($contractGetResp | ConvertFrom-Json).data

    if ($contractGetData.status -ne "active") {
        Log-Fail "Expected initial contract status 'active', got '$($contractGetData.status)'"
        exit 1
    }
    Log-Pass "Contract verified in active state (Agreed Budget: `$$($contractGetData.agreed_budget))"

    Log-Substep "Transitioning contract status to 'completed'..."
    $completeResp = Invoke-ApiRequest -Method "PATCH" -Endpoint "/api/v1/contracts/$ContractId/status" -Token $EmployerToken -Body '{"status":"completed"}' -ExpectedStatusCode 200
    $completeData = ($completeResp | ConvertFrom-Json).data

    if ($completeData.status -ne "completed") {
        Log-Fail "Contract transition to completed failed. Response: $completeResp"
        exit 1
    }
    Log-Pass "Contract successfully transitioned to terminal state: completed"

    # --------------------------------------------------------------------------
    # STEP 11: Peer Review Submission & Duplicate Conflict Assertion
    # --------------------------------------------------------------------------
    Log-Step "11" "Peer Review Submission & Duplicate Review Conflict Check"

    Log-Substep "Employer submits 5-star review for student on completed contract..."
    $empReviewPayload = @{
        rating = 5
        comment = "Exceptional work! Jordan delivered clean Go code, thorough unit tests, and excellent communication throughout the milestone."
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
        comment = "Outstanding employer to work with! Clear requirements, flexible milestones, and prompt feedback."
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

    # --------------------------------------------------------------------------
    # SUMMARY
    # --------------------------------------------------------------------------
    Write-Host ""
    Write-Host ("=" * 70) -ForegroundColor Green
    Write-Host "  ALL 11 END-TO-END SMOKE TEST PHASES PASSED SUCCESSFULLY!            " -ForegroundColor Green
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
