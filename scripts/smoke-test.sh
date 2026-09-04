#!/usr/bin/env bash
# ==============================================================================
# Lynk Platform — End-to-End Integration Smoke Test Suite
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
#   - Non-zero exit code with colored diagnostic logs on any assertion failure.
#   - Accepts pre-acquired tokens via environment variables OR acquires them
#     via Keycloak direct grant (with automated user provisioning fallback).
# ==============================================================================

set -eo pipefail

# ------------------------------------------------------------------------------
# ANSI Color Formatting
# ------------------------------------------------------------------------------
if [ -t 1 ]; then
    C_RESET="\033[0m"
    C_BOLD="\033[1m"
    C_GREEN="\033[32m"
    C_RED="\033[31m"
    C_YELLOW="\033[33m"
    C_BLUE="\033[34m"
    C_CYAN="\033[36m"
    C_MAGENTA="\033[35m"
else
    C_RESET=""
    C_BOLD=""
    C_GREEN=""
    C_RED=""
    C_YELLOW=""
    C_BLUE=""
    C_CYAN=""
    C_MAGENTA=""
fi

log_info()    { echo -e "${C_CYAN}[INFO]${C_RESET} $*"; }
log_step()    { echo -e "\n${C_BOLD}${C_BLUE}======================================================================${C_RESET}"; echo -e "${C_BOLD}${C_MAGENTA}[STEP $1/11]${C_RESET} ${C_BOLD}$2${C_RESET}"; echo -e "${C_BOLD}${C_BLUE}======================================================================${C_RESET}"; }
log_substep() { echo -e "  ${C_CYAN}-->${C_RESET} $*"; }
log_pass()    { echo -e "  ${C_GREEN}[PASS]${C_RESET} $*"; }
log_fail()    { echo -e "  ${C_RED}[FAIL]${C_RESET} $*" >&2; }
log_warn()    { echo -e "  ${C_YELLOW}[WARN]${C_RESET} $*"; }

# ------------------------------------------------------------------------------
# Configuration & Defaults
# ------------------------------------------------------------------------------
API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
KEYCLOAK_URL="${KEYCLOAK_URL:-http://localhost:8081}"
KEYCLOAK_REALM="${KEYCLOAK_REALM:-lynk}"
KEYCLOAK_CLIENT_ID="${KEYCLOAK_CLIENT_ID:-lynk-frontend}"
MINIO_URL="${MINIO_URL:-http://localhost:9000}"

KEYCLOAK_ADMIN="${KEYCLOAK_ADMIN:-admin}"
KEYCLOAK_ADMIN_PASSWORD="${KEYCLOAK_ADMIN_PASSWORD:-admin_password}"

EMPLOYER_USERNAME="${EMPLOYER_USERNAME:-employer@lynk.test}"
EMPLOYER_PASSWORD="${EMPLOYER_PASSWORD:-password123}"

VERIFIED_STUDENT_USERNAME="${VERIFIED_STUDENT_USERNAME:-student.verified@stanford.edu}"
VERIFIED_STUDENT_PASSWORD="${VERIFIED_STUDENT_PASSWORD:-password123}"

UNVERIFIED_STUDENT_USERNAME="${UNVERIFIED_STUDENT_USERNAME:-student.unverified@stanford.edu}"
UNVERIFIED_STUDENT_PASSWORD="${UNVERIFIED_STUDENT_PASSWORD:-password123}"

TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'lynk_smoke')"
cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

if [[ "$1" == "-h" || "$1" == "--help" ]]; then
    cat <<EOF
Lynk Platform End-to-End Integration Smoke Test Suite

Usage:
  ./scripts/smoke-test.sh

Environment Variables:
  API_BASE_URL                 Base URL of Lynk Go API (default: http://localhost:8080)
  KEYCLOAK_URL                 Base URL of Keycloak (default: http://localhost:8081)
  KEYCLOAK_REALM               Keycloak realm name (default: lynk)
  KEYCLOAK_CLIENT_ID           Keycloak client ID (default: lynk-frontend)
  MINIO_URL                    Base URL of MinIO S3 (default: http://localhost:9000)
  EMPLOYER_TOKEN               Pre-acquired JWT for employer (skips Keycloak login)
  VERIFIED_STUDENT_TOKEN       Pre-acquired JWT for verified student (skips Keycloak login)
  UNVERIFIED_STUDENT_TOKEN     Pre-acquired JWT for unverified student (skips Keycloak login)
  KEYCLOAK_ADMIN               Keycloak admin username (default: admin)
  KEYCLOAK_ADMIN_PASSWORD      Keycloak admin password (default: admin_password)
  SKIP_INFRA_HEALTH            Set to 1 to skip Keycloak/MinIO health pings
EOF
    exit 0
fi

# ------------------------------------------------------------------------------
# JSON Parsing Helper (jq -> node -> python -> awk/sed fallback)
# ------------------------------------------------------------------------------
json_extract() {
    local json_input="$1"
    local json_path="$2"

    if command -v jq >/dev/null 2>&1; then
        echo "$json_input" | jq -r "$json_path // empty" 2>/dev/null
        return 0
    fi

    if command -v node >/dev/null 2>&1; then
        node -e '
            try {
                const data = JSON.parse(process.argv[1]);
                const rawPath = process.argv[2];
                // Support .data.id, .[0].id, data.id etc.
                const parts = rawPath.replace(/^\./, "").replace(/\[(\d+)\]/g, ".$1").split(".").filter(Boolean);
                let cur = data;
                for (const p of parts) {
                    if (cur === null || cur === undefined) { cur = ""; break; }
                    cur = cur[p];
                }
                if (cur !== null && cur !== undefined) {
                    if (typeof cur === "object") process.stdout.write(JSON.stringify(cur));
                    else process.stdout.write(String(cur));
                }
            } catch (e) {
                process.exit(0);
            }
        ' "$json_input" "$json_path"
        return 0
    fi

    if command -v python >/dev/null 2>&1; then
        python -c '
import sys, json, re
try:
    data = json.loads(sys.argv[1])
    raw_path = sys.argv[2].lstrip(".")
    tokens = [t for t in re.split(r"\.|\[(\d+)\]", raw_path) if t]
    cur = data
    for t in tokens:
        if cur is None:
            break
        if isinstance(cur, list) and t.isdigit():
            cur = cur[int(t)]
        elif isinstance(cur, dict):
            cur = cur.get(t)
        else:
            cur = None
            break
    if cur is not None:
        if isinstance(cur, (dict, list)):
            sys.stdout.write(json.dumps(cur))
        else:
            sys.stdout.write(str(cur))
except Exception:
    pass
' "$json_input" "$json_path"
        return 0
    fi

    # Fallback sed/grep for key extract
    local key="${json_path##*.}"
    echo "$json_input" | grep -o "\"$key\"[[:space:]]*:[[:space:]]*[^,}]*" | head -n1 | sed -e "s/\"$key\"[[:space:]]*:[[:space:]]*//; s/^\"//; s/\"$//"
}

# ------------------------------------------------------------------------------
# HTTP Request Dispatcher & Assertion Helper
# ------------------------------------------------------------------------------
# Usage:
#   http_request METHOD ENDPOINT TOKEN DATA EXPECTED_HTTP_CODE [CONTENT_TYPE]
# Returns response body via stdout and sets global LAST_HTTP_CODE.
http_request() {
    local method="$1"
    local endpoint="$2"
    local token="$3"
    local data="$4"
    local expected_status="$5"
    local content_type="${6:-application/json}"

    local url="${API_BASE_URL}${endpoint}"
    local resp_file="${TMP_DIR}/resp.tmp"
    local head_file="${TMP_DIR}/head.tmp"

    local curl_cmd=(curl -s -S -w "%{http_code}" -o "$resp_file" -D "$head_file" -X "$method")

    if [ -n "$token" ]; then
        curl_cmd+=(-H "Authorization: Bearer $token")
    fi

    if [ -n "$content_type" ]; then
        curl_cmd+=(-H "Content-Type: $content_type")
    fi

    if [ -n "$data" ]; then
        curl_cmd+=(-d "$data")
    fi

    curl_cmd+=("$url")

    local status_code
    status_code=$("${curl_cmd[@]}" 2>/dev/null || echo "000")

    local body
    body="$(cat "$resp_file" 2>/dev/null || true)"

    if [ "$status_code" != "$expected_status" ]; then
        log_fail "Request: $method $endpoint"
        log_fail "Expected HTTP $expected_status, got HTTP $status_code"
        log_fail "Response body: $body"
        exit 1
    fi

    LAST_HTTP_CODE="$status_code"
    echo "$body"
}

# ------------------------------------------------------------------------------
# Keycloak Token Acquisition & User Provisioning Helper
# ------------------------------------------------------------------------------
get_direct_grant_token() {
    local username="$1"
    local password="$2"

    local token_url="${KEYCLOAK_URL}/realms/${KEYCLOAK_REALM}/protocol/openid-connect/token"
    local token_resp
    token_resp=$(curl -s -X POST "$token_url" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "grant_type=password&client_id=${KEYCLOAK_CLIENT_ID}&username=${username}&password=${password}" 2>/dev/null || true)

    local token
    token=$(json_extract "$token_resp" ".access_token")
    if [ -n "$token" ] && [ "$token" != "null" ]; then
        echo "$token"
        return 0
    fi
    return 1
}

provision_keycloak_user() {
    local username="$1"
    local password="$2"
    local email="$3"
    local role="$4"
    local verified="$5" # "true" or "false"

    log_substep "Provisioning Keycloak test user: $username ($role, verified=$verified)..."

    local admin_token_resp
    admin_token_resp=$(curl -s -X POST "${KEYCLOAK_URL}/realms/master/protocol/openid-connect/token" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "grant_type=password&client_id=admin-cli&username=${KEYCLOAK_ADMIN}&password=${KEYCLOAK_ADMIN_PASSWORD}" 2>/dev/null || true)

    local admin_token
    admin_token=$(json_extract "$admin_token_resp" ".access_token")
    if [ -z "$admin_token" ] || [ "$admin_token" = "null" ]; then
        log_warn "Unable to acquire Keycloak admin token. Skipping automated provisioning."
        return 1
    fi

    # 1. Check if user already exists
    local existing_user_resp
    existing_user_resp=$(curl -s -X GET "${KEYCLOAK_URL}/admin/realms/${KEYCLOAK_REALM}/users?username=${username}" \
        -H "Authorization: Bearer ${admin_token}")
    local user_id
    user_id=$(json_extract "$existing_user_resp" ".[0].id")

    # 2. Create user if not existing
    if [ -z "$user_id" ] || [ "$user_id" = "null" ]; then
        local user_payload
        user_payload=$(cat <<EOF
{
  "username": "${username}",
  "email": "${email}",
  "emailVerified": ${verified},
  "enabled": true,
  "credentials": [
    {
      "type": "password",
      "value": "${password}",
      "temporary": false
    }
  ]
}
EOF
)
        curl -s -X POST "${KEYCLOAK_URL}/admin/realms/${KEYCLOAK_REALM}/users" \
            -H "Authorization: Bearer ${admin_token}" \
            -H "Content-Type: application/json" \
            -d "$user_payload" >/dev/null

        existing_user_resp=$(curl -s -X GET "${KEYCLOAK_URL}/admin/realms/${KEYCLOAK_REALM}/users?username=${username}" \
            -H "Authorization: Bearer ${admin_token}")
        user_id=$(json_extract "$existing_user_resp" ".[0].id")
    fi

    if [ -z "$user_id" ] || [ "$user_id" = "null" ]; then
        log_warn "Failed to resolve user ID for $username after creation attempt."
        return 1
    fi

    # 3. Map realm role
    local role_resp
    role_resp=$(curl -s -X GET "${KEYCLOAK_URL}/admin/realms/${KEYCLOAK_REALM}/roles/${role}" \
        -H "Authorization: Bearer ${admin_token}")
    local role_id
    role_id=$(json_extract "$role_resp" ".id")

    if [ -n "$role_id" ] && [ "$role_id" != "null" ]; then
        curl -s -X POST "${KEYCLOAK_URL}/admin/realms/${KEYCLOAK_REALM}/users/${user_id}/role-mappings/realm" \
            -H "Authorization: Bearer ${admin_token}" \
            -H "Content-Type: application/json" \
            -d "[{\"id\":\"${role_id}\",\"name\":\"${role}\"}]" >/dev/null
    fi

    return 0
}

resolve_token() {
    local env_token="$1"
    local username="$2"
    local password="$3"
    local email="$4"
    local role="$5"
    local verified="$6"

    # Priority 1: Already passed in environment variable
    if [ -n "$env_token" ]; then
        echo "$env_token"
        return 0
    fi

    # Priority 2: Direct grant request against Keycloak
    local token
    if token=$(get_direct_grant_token "$username" "$password"); then
        echo "$token"
        return 0
    fi

    # Priority 3: Auto-provision Keycloak user, then retry direct grant
    if provision_keycloak_user "$username" "$password" "$email" "$role" "$verified"; then
        if token=$(get_direct_grant_token "$username" "$password"); then
            echo "$token"
            return 0
        fi
    fi

    log_fail "Could not acquire JWT for $username ($role)."
    log_fail "Please ensure Keycloak is running at $KEYCLOAK_URL or provide token via environment variable."
    exit 1
}

# ==============================================================================
# Execution Entry Point
# ==============================================================================

log_info "Starting Lynk MVP End-to-End Integration Smoke Test Suite"
log_info "Target API:          ${API_BASE_URL}"
log_info "Keycloak URL:        ${KEYCLOAK_URL} (realm: ${KEYCLOAK_REALM})"
log_info "MinIO URL:           ${MINIO_URL}"

# ------------------------------------------------------------------------------
# STEP 1: Health Check Verification
# ------------------------------------------------------------------------------
log_step "1" "Infrastructure & API Health Checks"

log_substep "Verifying Lynk Go REST API (/health)..."
API_HEALTH_RESP=$(curl -s -w "\n%{http_code}" "${API_BASE_URL}/health" 2>/dev/null || echo -e "\n000")
API_HEALTH_CODE=$(echo "$API_HEALTH_RESP" | tail -n1)
API_HEALTH_BODY=$(echo "$API_HEALTH_RESP" | sed '$d')

if [ "$API_HEALTH_CODE" != "200" ]; then
    log_fail "Go API health check failed (HTTP $API_HEALTH_CODE). Is Lynk API running on ${API_BASE_URL}?"
    exit 1
fi
API_STATUS=$(json_extract "$API_HEALTH_BODY" ".status")
if [ "$API_STATUS" != "ok" ]; then
    log_fail "Go API health check did not return status: ok. Body: $API_HEALTH_BODY"
    exit 1
fi
log_pass "Go API is healthy: HTTP 200 (status: ok)"

if [ "${SKIP_INFRA_HEALTH:-0}" != "1" ]; then
    log_substep "Verifying Keycloak realm discovery (/realms/${KEYCLOAK_REALM})..."
    KC_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${KEYCLOAK_URL}/realms/${KEYCLOAK_REALM}" 2>/dev/null || echo "000")
    if [ "$KC_CODE" != "200" ]; then
        log_fail "Keycloak realm check failed (HTTP $KC_CODE). Is Keycloak running on ${KEYCLOAK_URL}?"
        exit 1
    fi
    log_pass "Keycloak realm '${KEYCLOAK_REALM}' is accessible: HTTP 200"

    log_substep "Verifying MinIO S3 health check (/minio/health/live)..."
    MINIO_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${MINIO_URL}/minio/health/live" 2>/dev/null || echo "000")
    if [ "$MINIO_CODE" != "200" ]; then
        log_fail "MinIO health check failed (HTTP $MINIO_CODE). Is MinIO running on ${MINIO_URL}?"
        exit 1
    fi
    log_pass "MinIO S3 service is healthy: HTTP 200"
fi

# ------------------------------------------------------------------------------
# Token Resolution for Test Actors
# ------------------------------------------------------------------------------
log_info "Resolving authentication tokens for test actors..."

log_substep "Resolving Employer token..."
EMPLOYER_TOKEN=$(resolve_token "${EMPLOYER_TOKEN:-}" "$EMPLOYER_USERNAME" "$EMPLOYER_PASSWORD" "$EMPLOYER_USERNAME" "employer" "true")
log_pass "Employer token acquired"

log_substep "Resolving Verified Student token..."
VERIFIED_STUDENT_TOKEN=$(resolve_token "${VERIFIED_STUDENT_TOKEN:-}" "$VERIFIED_STUDENT_USERNAME" "$VERIFIED_STUDENT_PASSWORD" "$VERIFIED_STUDENT_USERNAME" "student" "true")
log_pass "Verified Student token acquired (institutional email verified)"

log_substep "Resolving Unverified Student token..."
UNVERIFIED_STUDENT_TOKEN=$(resolve_token "${UNVERIFIED_STUDENT_TOKEN:-}" "$UNVERIFIED_STUDENT_USERNAME" "$UNVERIFIED_STUDENT_PASSWORD" "$UNVERIFIED_STUDENT_USERNAME" "student" "false")
log_pass "Unverified Student token acquired (email_verified=false for gate enforcement)"

# ------------------------------------------------------------------------------
# STEP 2: Auth Sync for Test Users (POST /api/v1/auth/sync)
# ------------------------------------------------------------------------------
log_step "2" "User Auth Synchronization (POST /api/v1/auth/sync)"

log_substep "Syncing Employer user..."
EMP_SYNC_RESP=$(http_request "POST" "/api/v1/auth/sync" "$EMPLOYER_TOKEN" '{"role":"employer"}' "200")
EMP_SUCCESS=$(json_extract "$EMP_SYNC_RESP" ".success")
EMPLOYER_USER_ID=$(json_extract "$EMP_SYNC_RESP" ".data.id")
if [ "$EMP_SUCCESS" != "true" ] || [ -z "$EMPLOYER_USER_ID" ]; then
    log_fail "Employer sync failed: $EMP_SYNC_RESP"
    exit 1
fi
log_pass "Employer synced: UUID $EMPLOYER_USER_ID"

log_substep "Syncing Verified Student user..."
STU_SYNC_RESP=$(http_request "POST" "/api/v1/auth/sync" "$VERIFIED_STUDENT_TOKEN" '{"role":"student"}' "200")
STU_SUCCESS=$(json_extract "$STU_SYNC_RESP" ".success")
VERIFIED_STUDENT_USER_ID=$(json_extract "$STU_SYNC_RESP" ".data.id")
if [ "$STU_SUCCESS" != "true" ] || [ -z "$VERIFIED_STUDENT_USER_ID" ]; then
    log_fail "Verified student sync failed: $STU_SYNC_RESP"
    exit 1
fi
log_pass "Verified student synced: UUID $VERIFIED_STUDENT_USER_ID"

log_substep "Syncing Unverified Student user..."
UNV_SYNC_RESP=$(http_request "POST" "/api/v1/auth/sync" "$UNVERIFIED_STUDENT_TOKEN" '{"role":"student"}' "200")
UNV_SUCCESS=$(json_extract "$UNV_SYNC_RESP" ".success")
UNVERIFIED_STUDENT_USER_ID=$(json_extract "$UNV_SYNC_RESP" ".data.id")
if [ "$UNV_SUCCESS" != "true" ] || [ -z "$UNVERIFIED_STUDENT_USER_ID" ]; then
    log_fail "Unverified student sync failed: $UNV_SYNC_RESP"
    exit 1
fi
log_pass "Unverified student synced: UUID $UNVERIFIED_STUDENT_USER_ID"

# ------------------------------------------------------------------------------
# STEP 3: Student & Employer Profile Updates
# ------------------------------------------------------------------------------
log_step "3" "Profile Configuration (PUT /api/v1/profile/student & employer)"

log_substep "Updating student profile for verified student..."
STUDENT_PROFILE_PAYLOAD=$(cat <<EOF
{
  "first_name": "Jordan",
  "last_name": "Lee",
  "bio": "CS undergraduate specializing in cloud infrastructure and distributed Go systems.",
  "department": "Computer Science",
  "graduation_year": 2026,
  "skills": ["Go", "Docker", "PostgreSQL", "Next.js", "Kubernetes"],
  "portfolio_links": ["https://github.com/jordanlee", "https://jordanlee.dev"]
}
EOF
)
STU_PROF_RESP=$(http_request "PUT" "/api/v1/profile/student" "$VERIFIED_STUDENT_TOKEN" "$STUDENT_PROFILE_PAYLOAD" "200")
FIRST_NAME=$(json_extract "$STU_PROF_RESP" ".data.first_name")
if [ "$FIRST_NAME" != "Jordan" ]; then
    log_fail "Student profile first_name mismatch (expected Jordan, got '$FIRST_NAME')"
    exit 1
fi
log_pass "Student profile updated successfully (First Name: $FIRST_NAME)"

log_substep "Updating employer profile..."
EMPLOYER_PROFILE_PAYLOAD=$(cat <<EOF
{
  "company_or_org": "Lynk Campus Innovations",
  "contact_name": "Dr. Aris Thorne",
  "description": "University research lab building student freelance marketplace platforms.",
  "website": "https://lynk.campus.edu"
}
EOF
)
EMP_PROF_RESP=$(http_request "PUT" "/api/v1/profile/employer" "$EMPLOYER_TOKEN" "$EMPLOYER_PROFILE_PAYLOAD" "200")
COMPANY_NAME=$(json_extract "$EMP_PROF_RESP" ".data.company_or_org")
if [ "$COMPANY_NAME" != "Lynk Campus Innovations" ]; then
    log_fail "Employer profile company mismatch (expected Lynk Campus Innovations, got '$COMPANY_NAME')"
    exit 1
fi
log_pass "Employer profile updated successfully (Company: $COMPANY_NAME)"

# ------------------------------------------------------------------------------
# STEP 4: Resume Upload & Presigned URL Fetch
# ------------------------------------------------------------------------------
log_step "4" "Resume Upload & Presigned S3 Download URL Fetch"

SAMPLE_PDF="${TMP_DIR}/sample_resume.pdf"
cat <<'EOF' > "$SAMPLE_PDF"
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
EOF

log_substep "Uploading PDF resume for verified student..."
RESUME_UPLOAD_RESP=$(curl -s -S -w "\n%{http_code}" \
    -X POST "${API_BASE_URL}/api/v1/profile/student/resume" \
    -H "Authorization: Bearer ${VERIFIED_STUDENT_TOKEN}" \
    -F "resume=@${SAMPLE_PDF};type=application/pdf" 2>/dev/null || echo -e "\n000")

RESUME_UPLOAD_CODE=$(echo "$RESUME_UPLOAD_RESP" | tail -n1)
RESUME_UPLOAD_BODY=$(echo "$RESUME_UPLOAD_RESP" | sed '$d')

if [ "$RESUME_UPLOAD_CODE" != "200" ]; then
    log_fail "Resume upload failed: HTTP $RESUME_UPLOAD_CODE. Body: $RESUME_UPLOAD_BODY"
    exit 1
fi

RESUME_KEY=$(json_extract "$RESUME_UPLOAD_BODY" ".data.resume_key")
if [ -z "$RESUME_KEY" ] || [ "$RESUME_KEY" = "null" ]; then
    log_fail "Resume upload did not return data.resume_key: $RESUME_UPLOAD_BODY"
    exit 1
fi
log_pass "Resume uploaded to MinIO bucket 'resumes'. Key: $RESUME_KEY"

log_substep "Fetching presigned download URL for uploaded resume..."
RESUME_URL_RESP=$(http_request "GET" "/api/v1/profile/student/resume" "$VERIFIED_STUDENT_TOKEN" "" "200")
DOWNLOAD_URL=$(json_extract "$RESUME_URL_RESP" ".data.download_url")
if [ -z "$DOWNLOAD_URL" ] || [ "$DOWNLOAD_URL" = "null" ]; then
    DOWNLOAD_URL=$(json_extract "$RESUME_URL_RESP" ".data.url")
fi

if [[ "$DOWNLOAD_URL" != http* ]]; then
    log_fail "Presigned download URL invalid or missing: $RESUME_URL_RESP"
    exit 1
fi
log_pass "Presigned S3 download URL generated successfully (Valid 15m)"

# ------------------------------------------------------------------------------
# STEP 5: Job Creation by Employer (POST /api/v1/jobs)
# ------------------------------------------------------------------------------
log_step "5" "Job Creation by Employer (POST /api/v1/jobs)"

JOB_PAYLOAD=$(cat <<EOF
{
  "title": "Campus Marketplace Go & Next.js Engineer",
  "description": "Looking for an energetic student engineer to develop high-trust freelance marketplace features with Go, Chi, PostgreSQL, and Next.js.",
  "budget": 950.00,
  "pay_type": "fixed",
  "required_skills": ["Go", "PostgreSQL", "Docker", "Next.js"],
  "department": "Computer Science"
}
EOF
)

JOB_CREATE_RESP=$(http_request "POST" "/api/v1/jobs" "$EMPLOYER_TOKEN" "$JOB_PAYLOAD" "201")
JOB_ID=$(json_extract "$JOB_CREATE_RESP" ".data.id")
JOB_STATUS=$(json_extract "$JOB_CREATE_RESP" ".data.status")

if [ -z "$JOB_ID" ] || [ "$JOB_ID" = "null" ] || [ "$JOB_STATUS" != "open" ]; then
    log_fail "Job creation failed. Response: $JOB_CREATE_RESP"
    exit 1
fi
log_pass "Job created successfully: UUID $JOB_ID (Status: $JOB_STATUS)"

# ------------------------------------------------------------------------------
# STEP 6: Job Filtering & Detail Retrieval
# ------------------------------------------------------------------------------
log_step "6" "Job Search, Filtering & Detail Retrieval"

log_substep "Filtering jobs with query params (search=Marketplace&department=Computer+Science)..."
JOB_FILTER_RESP=$(http_request "GET" "/api/v1/jobs?search=Marketplace&department=Computer+Science&skill=Go" "" "" "200")
FOUND_MATCH=$(echo "$JOB_FILTER_RESP" | grep -q "$JOB_ID" && echo "yes" || echo "no")
if [ "$FOUND_MATCH" != "yes" ]; then
    log_fail "Created job $JOB_ID was not found in filtered listings response: $JOB_FILTER_RESP"
    exit 1
fi
log_pass "Job discovered via public search & filter query"

log_substep "Retrieving single job detail (GET /api/v1/jobs/${JOB_ID})..."
JOB_DETAIL_RESP=$(http_request "GET" "/api/v1/jobs/${JOB_ID}" "" "" "200")
DETAIL_ID=$(json_extract "$JOB_DETAIL_RESP" ".data.id")
DETAIL_BUDGET=$(json_extract "$JOB_DETAIL_RESP" ".data.budget")

if [ "$DETAIL_ID" != "$JOB_ID" ]; then
    log_fail "Job detail ID mismatch: expected $JOB_ID, got '$DETAIL_ID'"
    exit 1
fi
log_pass "Job details retrieved successfully: Budget \$$DETAIL_BUDGET"

# ------------------------------------------------------------------------------
# STEP 7: Institutional Email Gate Enforcement (Hard Invariant Verification)
# ------------------------------------------------------------------------------
log_step "7" "Institutional Email Gate Enforcement (HTTP 403 EMAIL_NOT_VERIFIED)"

log_substep "Assertion 7a: Unverified student attempts resume upload (Must return 403 EMAIL_NOT_VERIFIED)..."
GATE_UPLOAD_RESP=$(curl -s -S -w "\n%{http_code}" \
    -X POST "${API_BASE_URL}/api/v1/profile/student/resume" \
    -H "Authorization: Bearer ${UNVERIFIED_STUDENT_TOKEN}" \
    -F "resume=@${SAMPLE_PDF};type=application/pdf" 2>/dev/null || echo -e "\n000")

GATE_UPLOAD_CODE=$(echo "$GATE_UPLOAD_RESP" | tail -n1)
GATE_UPLOAD_BODY=$(echo "$GATE_UPLOAD_RESP" | sed '$d')

if [ "$GATE_UPLOAD_CODE" != "403" ]; then
    log_fail "Email Gate failed on resume upload: Expected HTTP 403, got HTTP $GATE_UPLOAD_CODE"
    log_fail "Body: $GATE_UPLOAD_BODY"
    exit 1
fi

GATE_UPLOAD_ERR=$(json_extract "$GATE_UPLOAD_BODY" ".error.code")
if [ "$GATE_UPLOAD_ERR" != "EMAIL_NOT_VERIFIED" ]; then
    log_fail "Email Gate returned wrong error code on resume upload: expected EMAIL_NOT_VERIFIED, got '$GATE_UPLOAD_ERR'"
    exit 1
fi
log_pass "Resume upload strictly blocked for unverified student: HTTP 403 (EMAIL_NOT_VERIFIED)"

log_substep "Assertion 7b: Unverified student attempts job apply (Must return 403 EMAIL_NOT_VERIFIED)..."
GATE_APPLY_RESP=$(curl -s -S -w "\n%{http_code}" \
    -X POST "${API_BASE_URL}/api/v1/jobs/${JOB_ID}/applications" \
    -H "Authorization: Bearer ${UNVERIFIED_STUDENT_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"cover_letter":"Attempting application without verified university email."}' 2>/dev/null || echo -e "\n000")

GATE_APPLY_CODE=$(echo "$GATE_APPLY_RESP" | tail -n1)
GATE_APPLY_BODY=$(echo "$GATE_APPLY_RESP" | sed '$d')

if [ "$GATE_APPLY_CODE" != "403" ]; then
    log_fail "Email Gate failed on job application: Expected HTTP 403, got HTTP $GATE_APPLY_CODE"
    log_fail "Body: $GATE_APPLY_BODY"
    exit 1
fi

GATE_APPLY_ERR=$(json_extract "$GATE_APPLY_BODY" ".error.code")
if [ "$GATE_APPLY_ERR" != "EMAIL_NOT_VERIFIED" ]; then
    log_fail "Email Gate returned wrong error code on job apply: expected EMAIL_NOT_VERIFIED, got '$GATE_APPLY_ERR'"
    exit 1
fi
log_pass "Job application strictly blocked for unverified student: HTTP 403 (EMAIL_NOT_VERIFIED)"

# ------------------------------------------------------------------------------
# STEP 8: Verified Student Job Application (POST /api/v1/jobs/{id}/applications)
# ------------------------------------------------------------------------------
log_step "8" "Verified Student Job Application Submission"

APPLY_PAYLOAD=$(cat <<EOF
{
  "cover_letter": "I have extensive experience building scalable microservices in Go and full-stack Next.js web applications.",
  "resume_key": "${RESUME_KEY}"
}
EOF
)

APPLY_RESP=$(http_request "POST" "/api/v1/jobs/${JOB_ID}/applications" "$VERIFIED_STUDENT_TOKEN" "$APPLY_PAYLOAD" "201")
APPLICATION_ID=$(json_extract "$APPLY_RESP" ".data.id")
APPLICATION_STATUS=$(json_extract "$APPLY_RESP" ".data.status")

if [ -z "$APPLICATION_ID" ] || [ "$APPLICATION_ID" = "null" ] || [ "$APPLICATION_STATUS" != "pending" ]; then
    log_fail "Application submission failed: $APPLY_RESP"
    exit 1
fi
log_pass "Application submitted successfully: UUID $APPLICATION_ID (Status: $APPLICATION_STATUS)"

# ------------------------------------------------------------------------------
# STEP 9: Employer Applicant Review & Acceptance (Atomic Contract Generation)
# ------------------------------------------------------------------------------
log_step "9" "Employer Applicant Review & Acceptance"

log_substep "Employer lists applications for job ${JOB_ID}..."
APP_LIST_RESP=$(http_request "GET" "/api/v1/jobs/${JOB_ID}/applications" "$EMPLOYER_TOKEN" "" "200")
FOUND_APP=$(echo "$APP_LIST_RESP" | grep -q "$APPLICATION_ID" && echo "yes" || echo "no")
if [ "$FOUND_APP" != "yes" ]; then
    log_fail "Submitted application $APPLICATION_ID not found in employer job applications: $APP_LIST_RESP"
    exit 1
fi
log_pass "Application visible in employer candidate list"

log_substep "Employer accepts application $APPLICATION_ID (triggers atomic contract creation)..."
ACCEPT_RESP=$(http_request "PATCH" "/api/v1/applications/${APPLICATION_ID}/status" "$EMPLOYER_TOKEN" '{"status":"accepted"}' "200")
APP_FINAL_STATUS=$(json_extract "$ACCEPT_RESP" ".data.status")
CONTRACT_ID=$(json_extract "$ACCEPT_RESP" ".data.contract.id")
CONTRACT_STATUS=$(json_extract "$ACCEPT_RESP" ".data.contract.status")

if [ "$APP_FINAL_STATUS" != "accepted" ]; then
    log_fail "Expected application status 'accepted', got '$APP_FINAL_STATUS'"
    exit 1
fi

if [ -z "$CONTRACT_ID" ] || [ "$CONTRACT_ID" = "null" ] || [ "$CONTRACT_STATUS" != "active" ]; then
    log_fail "Atomic contract creation failed. Response: $ACCEPT_RESP"
    exit 1
fi
log_pass "Application accepted and atomic Contract created: UUID $CONTRACT_ID (Status: $CONTRACT_STATUS)"

# ------------------------------------------------------------------------------
# STEP 10: Contract Status Progression (Active -> Completed)
# ------------------------------------------------------------------------------
log_step "10" "Contract State Machine Progression (active -> completed)"

log_substep "Inspecting active contract detail (GET /api/v1/contracts/${CONTRACT_ID})..."
CONTRACT_GET_RESP=$(http_request "GET" "/api/v1/contracts/${CONTRACT_ID}" "$EMPLOYER_TOKEN" "" "200")
INITIAL_STATUS=$(json_extract "$CONTRACT_GET_RESP" ".data.status")
AGREED_BUDGET=$(json_extract "$CONTRACT_GET_RESP" ".data.agreed_budget")

if [ "$INITIAL_STATUS" != "active" ]; then
    log_fail "Expected initial contract status 'active', got '$INITIAL_STATUS'"
    exit 1
fi
log_pass "Contract verified in active state (Agreed Budget: \$$AGREED_BUDGET)"

log_substep "Transitioning contract status to 'completed'..."
COMPLETE_RESP=$(http_request "PATCH" "/api/v1/contracts/${CONTRACT_ID}/status" "$EMPLOYER_TOKEN" '{"status":"completed"}' "200")
FINAL_STATUS=$(json_extract "$COMPLETE_RESP" ".data.status")

if [ "$FINAL_STATUS" != "completed" ]; then
    log_fail "Contract transition to completed failed. Response: $COMPLETE_RESP"
    exit 1
fi
log_pass "Contract successfully transitioned to terminal state: completed"

# ------------------------------------------------------------------------------
# STEP 11: Peer Review Submission & Duplicate Conflict Assertion
# ------------------------------------------------------------------------------
log_step "11" "Peer Review Submission & Duplicate Review Conflict Check"

log_substep "Employer submits 5-star review for student on completed contract..."
EMP_REVIEW_PAYLOAD=$(cat <<EOF
{
  "rating": 5,
  "comment": "Exceptional work! Jordan delivered clean Go code, thorough unit tests, and excellent communication throughout the milestone."
}
EOF
)
EMP_REV_RESP=$(http_request "POST" "/api/v1/contracts/${CONTRACT_ID}/reviews" "$EMPLOYER_TOKEN" "$EMP_REVIEW_PAYLOAD" "201")
EMP_REV_RATING=$(json_extract "$EMP_REV_RESP" ".data.rating")
if [ "$EMP_REV_RATING" != "5" ]; then
    log_fail "Employer review rating mismatch: expected 5, got '$EMP_REV_RATING'"
    exit 1
fi
log_pass "Employer review submitted successfully: Rating 5/5"

log_substep "Student submits 5-star review for employer on completed contract..."
STU_REVIEW_PAYLOAD=$(cat <<EOF
{
  "rating": 5,
  "comment": "Outstanding employer to work with! Clear requirements, flexible milestones, and prompt feedback."
}
EOF
)
STU_REV_RESP=$(http_request "POST" "/api/v1/contracts/${CONTRACT_ID}/reviews" "$VERIFIED_STUDENT_TOKEN" "$STU_REVIEW_PAYLOAD" "201")
STU_REV_RATING=$(json_extract "$STU_REV_RESP" ".data.rating")
if [ "$STU_REV_RATING" != "5" ]; then
    log_fail "Student review rating mismatch: expected 5, got '$STU_REV_RATING'"
    exit 1
fi
log_pass "Student review submitted successfully: Rating 5/5"

log_substep "Duplicate Conflict Assertion: Employer attempts duplicate review on same contract (Must return 409)..."
DUP_RESP=$(curl -s -S -w "\n%{http_code}" \
    -X POST "${API_BASE_URL}/api/v1/contracts/${CONTRACT_ID}/reviews" \
    -H "Authorization: Bearer ${EMPLOYER_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"rating":4,"comment":"Attempting duplicate review submission."}' 2>/dev/null || echo -e "\n000")

DUP_CODE=$(echo "$DUP_RESP" | tail -n1)
DUP_BODY=$(echo "$DUP_RESP" | sed '$d')

if [ "$DUP_CODE" != "409" ]; then
    log_fail "Duplicate review assertion failed: Expected HTTP 409, got HTTP $DUP_CODE"
    log_fail "Body: $DUP_BODY"
    exit 1
fi

DUP_ERR=$(json_extract "$DUP_BODY" ".error.code")
if [ "$DUP_ERR" != "DUPLICATE_REVIEW" ]; then
    log_fail "Expected error code DUPLICATE_REVIEW, got '$DUP_ERR'"
    exit 1
fi
log_pass "Duplicate review blocked with HTTP 409 Conflict (DUPLICATE_REVIEW)"

log_substep "Verifying contract reviews list (GET /api/v1/contracts/${CONTRACT_ID}/reviews)..."
REVIEWS_RESP=$(http_request "GET" "/api/v1/contracts/${CONTRACT_ID}/reviews" "" "" "200")
REVIEWS_COUNT=$(json_extract "$REVIEWS_RESP" ".data | length")
if [ -z "$REVIEWS_COUNT" ] || [ "$REVIEWS_COUNT" -lt 2 ]; then
    # Fallback count if jq expression not evaluated
    if ! echo "$REVIEWS_RESP" | grep -q "Exceptional work" || ! echo "$REVIEWS_RESP" | grep -q "Outstanding employer"; then
        log_fail "Contract reviews list incomplete: $REVIEWS_RESP"
        exit 1
    fi
fi
log_pass "Contract reviews verified: Both counterparties recorded"

# ------------------------------------------------------------------------------
# SUMMARY
# ------------------------------------------------------------------------------
echo -e "\n${C_BOLD}${C_GREEN}======================================================================${C_RESET}"
echo -e "${C_BOLD}${C_GREEN}  ALL 11 END-TO-END SMOKE TEST PHASES PASSED SUCCESSFULLY!            ${C_RESET}"
echo -e "${C_BOLD}${C_GREEN}======================================================================${C_RESET}"
echo -e "  ${C_CYAN}Job ID:${C_RESET}         $JOB_ID"
echo -e "  ${C_CYAN}Application ID:${C_RESET} $APPLICATION_ID"
echo -e "  ${C_CYAN}Contract ID:${C_RESET}    $CONTRACT_ID"
echo -e "  ${C_CYAN}Resume Key:${C_RESET}     $RESUME_KEY"
echo -e "  ${C_GREEN}Exit code:${C_RESET}      0 (Verified)\n"

exit 0
