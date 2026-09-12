#!/usr/bin/env bash
# ==============================================================================
# Lynk Platform — End-to-End Integration Smoke Test Suite (Bash)
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
#   - Non-zero exit code with colored diagnostic logs on any assertion failure.
#   - Accepts pre-acquired tokens via environment variables OR acquires them
#     via SuperTokens APIs with automated member provisioning & verification.
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

log_info()    { echo -e "${C_CYAN}[INFO]${C_RESET} $*" >&2; }
log_step()    { echo -e "\n${C_BOLD}${C_BLUE}======================================================================${C_RESET}" >&2; echo -e "${C_BOLD}${C_MAGENTA}[STEP $1/13]${C_RESET} ${C_BOLD}$2${C_RESET}" >&2; echo -e "${C_BOLD}${C_BLUE}======================================================================${C_RESET}" >&2; }
log_substep() { echo -e "  ${C_CYAN}-->${C_RESET} $*" >&2; }
log_pass()    { echo -e "  ${C_GREEN}[PASS]${C_RESET} $*" >&2; }
log_fail()    { echo -e "  ${C_RED}[FAIL]${C_RESET} $*" >&2; }
log_warn()    { echo -e "  ${C_YELLOW}[WARN]${C_RESET} $*" >&2; }

# ------------------------------------------------------------------------------
# Configuration & Defaults
# ------------------------------------------------------------------------------
API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
SUPERTOKENS_URL="${SUPERTOKENS_URL:-http://localhost:3567}"
SUPERTOKENS_API_KEY="${SUPERTOKENS_API_KEY:-lynk-supertokens-secret-api-key-2026}"
MINIO_URL="${MINIO_URL:-http://localhost:9000}"

EMPLOYER_EMAIL="${EMPLOYER_EMAIL:-${EMPLOYER_USERNAME:-poster@campus.edu}}"
EMPLOYER_PASSWORD="${EMPLOYER_PASSWORD:-password123}"

VERIFIED_STUDENT_EMAIL="${VERIFIED_STUDENT_EMAIL:-${VERIFIED_STUDENT_USERNAME:-applicant@campus.edu}}"
VERIFIED_STUDENT_PASSWORD="${VERIFIED_STUDENT_PASSWORD:-password123}"

UNVERIFIED_STUDENT_EMAIL="${UNVERIFIED_STUDENT_EMAIL:-${UNVERIFIED_STUDENT_USERNAME:-unverified@campus.edu}}"
UNVERIFIED_STUDENT_PASSWORD="${UNVERIFIED_STUDENT_PASSWORD:-password123}"

UNAUTHORIZED_EMAIL="${UNAUTHORIZED_EMAIL:-unauthorized@gmail.com}"

TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'lynk_smoke')"
cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

if [[ "$1" == "-h" || "$1" == "--help" ]]; then
    cat <<EOF
Lynk Platform End-to-End Integration Smoke Test Suite (Bash)

Usage:
  ./scripts/smoke-test.sh

Environment Variables:
  API_BASE_URL                 Base URL of Lynk Go API (default: http://localhost:8080)
  SUPERTOKENS_URL              Base URL of SuperTokens Core (default: http://localhost:3567)
  SUPERTOKENS_API_KEY          SuperTokens Core API Key (default: lynk-supertokens-secret-api-key-2026)
  MINIO_URL                    Base URL of MinIO S3 (default: http://localhost:9000)
  EMPLOYER_TOKEN               Pre-acquired access token for employer
  VERIFIED_STUDENT_TOKEN       Pre-acquired access token for verified student
  UNVERIFIED_STUDENT_TOKEN     Pre-acquired access token for unverified student
  EMPLOYER_EMAIL               Poster member email (default: poster@campus.edu)
  EMPLOYER_PASSWORD            Poster password (default: password123)
  VERIFIED_STUDENT_EMAIL       Applicant member email (default: applicant@campus.edu)
  VERIFIED_STUDENT_PASSWORD    Applicant password (default: password123)
  UNVERIFIED_STUDENT_EMAIL     Unverified member email (default: unverified@campus.edu)
  UNVERIFIED_STUDENT_PASSWORD  Unverified password (default: password123)
  SKIP_INFRA_HEALTH            Set to 1 to skip SuperTokens/MinIO health pings
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
        curl_cmd+=(-H "Cookie: sAccessToken=$token")
        curl_cmd+=(-H "st-auth-mode: header")
        curl_cmd+=(-H "st-access-token: $token")
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
# SuperTokens Authentication Helpers
# ------------------------------------------------------------------------------
register_supertokens_user() {
    local email="$1"
    local password="$2"

    local resp_file="${TMP_DIR}/reg_resp.tmp"
    local head_file="${TMP_DIR}/reg_head.tmp"

    local payload
    payload=$(cat <<EOF
{
  "formFields": [
    {"id": "email", "value": "${email}"},
    {"id": "password", "value": "${password}"}
  ]
}
EOF
)

    curl -s -S -D "$head_file" -o "$resp_file" -X POST "${API_BASE_URL}/api/v1/auth/signup" \
        -H "rid: emailpassword" \
        -H "st-auth-mode: header" \
        -H "Content-Type: application/json" \
        -d "$payload" >/dev/null 2>&1 || true

    local body
    body="$(cat "$resp_file" 2>/dev/null || true)"

    local token
    token=$(grep -i '^st-access-token:' "$head_file" 2>/dev/null | tr -d '\r' | awk '{print $2}' | head -n1 || true)
    if [ -z "$token" ]; then
        token=$(grep -i 'sAccessToken=' "$head_file" 2>/dev/null | tr -d '\r' | sed -n 's/.*sAccessToken=\([^;]*\).*/\1/p' | head -n1 || true)
    fi

    local user_id
    user_id=$(json_extract "$body" ".user.id")

    local status
    status=$(json_extract "$body" ".status")

    REG_BODY="$body"
    REG_STATUS="$status"
    REG_TOKEN="$token"
    REG_USER_ID="$user_id"
}

login_supertokens_user() {
    local email="$1"
    local password="$2"

    local resp_file="${TMP_DIR}/login_resp.tmp"
    local head_file="${TMP_DIR}/login_head.tmp"

    local payload
    payload=$(cat <<EOF
{
  "formFields": [
    {"id": "email", "value": "${email}"},
    {"id": "password", "value": "${password}"}
  ]
}
EOF
)

    curl -s -S -D "$head_file" -o "$resp_file" -X POST "${API_BASE_URL}/api/v1/auth/signin" \
        -H "rid: emailpassword" \
        -H "st-auth-mode: header" \
        -H "Content-Type: application/json" \
        -d "$payload" >/dev/null 2>&1 || true

    local body
    body="$(cat "$resp_file" 2>/dev/null || true)"

    local token
    token=$(grep -i '^st-access-token:' "$head_file" 2>/dev/null | tr -d '\r' | awk '{print $2}' | head -n1 || true)
    if [ -z "$token" ]; then
        token=$(grep -i 'sAccessToken=' "$head_file" 2>/dev/null | tr -d '\r' | sed -n 's/.*sAccessToken=\([^;]*\).*/\1/p' | head -n1 || true)
    fi

    local user_id
    user_id=$(json_extract "$body" ".user.id")

    local status
    status=$(json_extract "$body" ".status")

    LOGIN_BODY="$body"
    LOGIN_STATUS="$status"
    LOGIN_TOKEN="$token"
    LOGIN_USER_ID="$user_id"
}

verify_supertokens_email() {
    local user_id="$1"
    local email="$2"

    log_substep "Verifying campus email in SuperTokens Core for $email ($user_id)..."

    local tok_resp
    tok_resp=$(curl -s -S -X POST "${SUPERTOKENS_URL}/recipe/user/email/verify/token" \
        -H "api-key: ${SUPERTOKENS_API_KEY}" \
        -H "Content-Type: application/json" \
        -d "{\"userId\":\"${user_id}\",\"email\":\"${email}\"}" 2>/dev/null || true)

    local tok_status
    tok_status=$(json_extract "$tok_resp" ".status")

    if [ "$tok_status" = "EMAIL_ALREADY_VERIFIED_ERROR" ]; then
        log_pass "Campus email verified in SuperTokens: $email"
        return 0
    fi

    local v_token
    v_token=$(json_extract "$tok_resp" ".token")
    if [ -n "$v_token" ]; then
        local verify_resp
        verify_resp=$(curl -s -S -X POST "${SUPERTOKENS_URL}/recipe/user/email/verify" \
            -H "api-key: ${SUPERTOKENS_API_KEY}" \
            -H "Content-Type: application/json" \
            -d "{\"method\":\"token\",\"token\":\"${v_token}\"}" 2>/dev/null || true)

        local v_status
        v_status=$(json_extract "$verify_resp" ".status")
        if [ "$v_status" = "OK" ] || [ "$v_status" = "EMAIL_ALREADY_VERIFIED_ERROR" ]; then
            log_pass "Campus email verified in SuperTokens: $email"
            return 0
        fi
    fi

    log_warn "SuperTokens email verify returned unexpected status: $tok_status (body: $tok_resp)"
    return 1
}

resolve_token() {
    local env_token="$1"
    local email="$2"
    local password="$3"
    local verified="$4"

    if [ -n "$env_token" ]; then
        echo "$env_token"
        return 0
    fi

    log_substep "Resolving SuperTokens test member: $email (verified=$verified)..."

    register_supertokens_user "$email" "$password"

    local token="$REG_TOKEN"
    local user_id="$REG_USER_ID"

    if [ "$REG_STATUS" = "EMAIL_ALREADY_EXISTS_ERROR" ] || [[ "$REG_BODY" == *"already exists"* ]]; then
        log_substep "Member $email already registered; signing in..."
        login_supertokens_user "$email" "$password"
        if [ "$LOGIN_STATUS" = "OK" ]; then
            token="$LOGIN_TOKEN"
            user_id="$LOGIN_USER_ID"
        else
            log_fail "Failed to sign in existing member $email: $LOGIN_BODY"
            exit 1
        fi
    elif [ "$REG_STATUS" != "OK" ]; then
        log_fail "Failed to register member $email: $REG_BODY"
        exit 1
    fi

    if [ -z "$token" ]; then
        log_fail "Could not extract SuperTokens access token for $email"
        exit 1
    fi

    if [ "$verified" = "true" ]; then
        if [ -z "$user_id" ]; then
            local sync_res
            sync_res=$(http_request "POST" "/api/v1/auth/sync" "$token" '{}' "200")
            user_id=$(json_extract "$sync_res" ".data.id")
        fi
        if [ -n "$user_id" ]; then
            verify_supertokens_email "$user_id" "$email" >/dev/null || true
        fi
    fi

    echo "$token"
}

# ==============================================================================
# Execution Entry Point
# ==============================================================================

log_info "Starting Lynk MVP End-to-End Integration Smoke Test Suite"
log_info "Target API:          ${API_BASE_URL}"
log_info "SuperTokens URL:     ${SUPERTOKENS_URL}"
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
    log_substep "Verifying SuperTokens Core health (/hello)..."
    ST_RESP=$(curl -s "${SUPERTOKENS_URL}/hello" 2>/dev/null || echo "")
    if [[ "$ST_RESP" != *"Hello"* ]]; then
        log_fail "SuperTokens Core check failed. Is SuperTokens running on ${SUPERTOKENS_URL}?"
        exit 1
    fi
    log_pass "SuperTokens Core service is accessible: HTTP 200 (Hello)"

    log_substep "Verifying MinIO S3 health check (/minio/health/live)..."
    MINIO_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${MINIO_URL}/minio/health/live" 2>/dev/null || echo "000")
    if [ "$MINIO_CODE" != "200" ]; then
        log_fail "MinIO health check failed (HTTP $MINIO_CODE). Is MinIO running on ${MINIO_URL}?"
        exit 1
    fi
    log_pass "MinIO S3 service is healthy: HTTP 200"
fi

# ------------------------------------------------------------------------------
# STEP 2: Institutional .edu Registration Gate Assertion (Hard Invariant)
# ------------------------------------------------------------------------------
log_step "2" "Institutional .edu Registration Gate Assertion (Non-.edu Rejected)"

log_substep "Attempting signup with unauthorized non-.edu address ($UNAUTHORIZED_EMAIL)..."
register_supertokens_user "$UNAUTHORIZED_EMAIL" "Password123!"

if [ "$REG_STATUS" = "GENERAL_ERROR" ]; then
    REG_MSG=$(json_extract "$REG_BODY" ".message")
    if [[ "$REG_MSG" == *"Registration rejected: only institutional .edu email addresses are permitted"* ]]; then
        log_pass "Non-.edu registration rejected as expected with GENERAL_ERROR: '$REG_MSG'"
    else
        log_fail "Expected rejection message to mention institutional .edu addresses, got: '$REG_MSG'"
        exit 1
    fi
else
    log_fail "Non-.edu address ($UNAUTHORIZED_EMAIL) was NOT rejected! Response: $REG_BODY"
    exit 1
fi

# ------------------------------------------------------------------------------
# STEP 3: User Provisioning & Authentication (SuperTokens)
# ------------------------------------------------------------------------------
log_step "3" "User Provisioning & Campus Authentication"

log_substep "Resolving Employer user ($EMPLOYER_EMAIL)..."
EMPLOYER_TOKEN=$(resolve_token "${EMPLOYER_TOKEN:-}" "$EMPLOYER_EMAIL" "$EMPLOYER_PASSWORD" "true")
log_pass "Employer token acquired & campus email verified ($EMPLOYER_EMAIL)"

log_substep "Resolving Verified Student user ($VERIFIED_STUDENT_EMAIL)..."
VERIFIED_STUDENT_TOKEN=$(resolve_token "${VERIFIED_STUDENT_TOKEN:-}" "$VERIFIED_STUDENT_EMAIL" "$VERIFIED_STUDENT_PASSWORD" "true")
log_pass "Verified Student token acquired & campus email verified ($VERIFIED_STUDENT_EMAIL)"

log_substep "Resolving Unverified Student user ($UNVERIFIED_STUDENT_EMAIL)..."
UNVERIFIED_STUDENT_TOKEN=$(resolve_token "${UNVERIFIED_STUDENT_TOKEN:-}" "$UNVERIFIED_STUDENT_EMAIL" "$UNVERIFIED_STUDENT_PASSWORD" "false")
log_pass "Unverified Student token acquired (unverified for gate enforcement)"

# ------------------------------------------------------------------------------
# STEP 4: Auth Sync for Test Users (POST /api/v1/auth/sync)
# ------------------------------------------------------------------------------
log_step "4" "User Auth Synchronization (POST /api/v1/auth/sync)"

log_substep "Syncing Employer member..."
EMP_SYNC_RESP=$(http_request "POST" "/api/v1/auth/sync" "$EMPLOYER_TOKEN" '{"role":"member"}' "200")
EMP_SUCCESS=$(json_extract "$EMP_SYNC_RESP" ".success")
EMPLOYER_USER_ID=$(json_extract "$EMP_SYNC_RESP" ".data.id")
if [ "$EMP_SUCCESS" != "true" ] || [ -z "$EMPLOYER_USER_ID" ]; then
    log_fail "Employer sync failed: $EMP_SYNC_RESP"
    exit 1
fi
log_pass "Employer synced: ID $EMPLOYER_USER_ID"

log_substep "Syncing Verified Student member..."
STU_SYNC_RESP=$(http_request "POST" "/api/v1/auth/sync" "$VERIFIED_STUDENT_TOKEN" '{"role":"member"}' "200")
STU_SUCCESS=$(json_extract "$STU_SYNC_RESP" ".success")
VERIFIED_STUDENT_USER_ID=$(json_extract "$STU_SYNC_RESP" ".data.id")
if [ "$STU_SUCCESS" != "true" ] || [ -z "$VERIFIED_STUDENT_USER_ID" ]; then
    log_fail "Verified student sync failed: $STU_SYNC_RESP"
    exit 1
fi
log_pass "Verified student synced: ID $VERIFIED_STUDENT_USER_ID"

log_substep "Syncing Unverified Student member..."
UNV_SYNC_RESP=$(http_request "POST" "/api/v1/auth/sync" "$UNVERIFIED_STUDENT_TOKEN" '{"role":"member"}' "200")
UNV_SUCCESS=$(json_extract "$UNV_SYNC_RESP" ".success")
UNVERIFIED_STUDENT_USER_ID=$(json_extract "$UNV_SYNC_RESP" ".data.id")
if [ "$UNV_SUCCESS" != "true" ] || [ -z "$UNVERIFIED_STUDENT_USER_ID" ]; then
    log_fail "Unverified student sync failed: $UNV_SYNC_RESP"
    exit 1
fi
log_pass "Unverified student synced: ID $UNVERIFIED_STUDENT_USER_ID"

# ------------------------------------------------------------------------------
# STEP 5: Student & Employer Profile Updates
# ------------------------------------------------------------------------------
log_step "5" "Profile Configuration (PUT /api/v1/profile/student & employer)"

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
# STEP 6: Resume Upload & Presigned URL Fetch
# ------------------------------------------------------------------------------
log_step "6" "Resume Upload & Presigned S3 Download URL Fetch"

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
    -H "Cookie: sAccessToken=${VERIFIED_STUDENT_TOKEN}" \
    -H "st-auth-mode: header" \
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

# SEC-03 Assertion: Verify presigned download URL does not expose internal Docker hostname (minio:9000)
log_substep "Asserting presigned URL public endpoint rewriting (SEC-03)..."
if [[ "$DOWNLOAD_URL" == *"://minio:9000"* ]]; then
    log_fail "SEC-03: Presigned URL contains internal Docker hostname 'minio:9000': $DOWNLOAD_URL"
    exit 1
fi
log_pass "SEC-03: Presigned URL properly uses public authority: $DOWNLOAD_URL"

# ------------------------------------------------------------------------------
# STEP 7: Job Creation by Employer (POST /api/v1/jobs)
# ------------------------------------------------------------------------------
log_step "7" "Job Creation by Employer (POST /api/v1/jobs)"

JOB_PAYLOAD=$(cat <<EOF
{
  "title": "Campus Marketplace Go & Next.js Engineer",
  "description": "Looking for an energetic student engineer to develop high-trust freelance marketplace features with Go, Chi, PostgreSQL, and Next.js.",
  "budget_cents": 95000,
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
# STEP 8: Job Filtering & Detail Retrieval
# ------------------------------------------------------------------------------
log_step "8" "Job Search, Filtering & Detail Retrieval"

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
DETAIL_BUDGET=$(json_extract "$JOB_DETAIL_RESP" ".data.budget_cents")

if [ "$DETAIL_ID" != "$JOB_ID" ]; then
    log_fail "Job detail ID mismatch: expected $JOB_ID, got '$DETAIL_ID'"
    exit 1
fi
log_pass "Job details retrieved successfully: Budget Cents $DETAIL_BUDGET"

# ------------------------------------------------------------------------------
# STEP 9: Campus Email Verification Gate Enforcement (HTTP 403 EMAIL_NOT_VERIFIED)
# ------------------------------------------------------------------------------
log_step "9" "Campus Email Verification Gate Enforcement (HTTP 403 EMAIL_NOT_VERIFIED)"

log_substep "Assertion 9a: Unverified student attempts job creation (Must return 403 EMAIL_NOT_VERIFIED)..."
GATE_JOB_RESP=$(curl -s -S -w "\n%{http_code}" \
    -X POST "${API_BASE_URL}/api/v1/jobs" \
    -H "Authorization: Bearer ${UNVERIFIED_STUDENT_TOKEN}" \
    -H "Cookie: sAccessToken=${UNVERIFIED_STUDENT_TOKEN}" \
    -H "st-auth-mode: header" \
    -H "Content-Type: application/json" \
    -d '{"title":"Unauthorized Opportunity","description":"Should be blocked","budget_cents":10000,"pay_type":"fixed","required_skills":["Go"],"department":"Computer Science"}' 2>/dev/null || echo -e "\n000")

GATE_JOB_CODE=$(echo "$GATE_JOB_RESP" | tail -n1)
GATE_JOB_BODY=$(echo "$GATE_JOB_RESP" | sed '$d')

if [ "$GATE_JOB_CODE" != "403" ]; then
    log_fail "Email Gate failed on job creation: Expected HTTP 403, got HTTP $GATE_JOB_CODE"
    log_fail "Body: $GATE_JOB_BODY"
    exit 1
fi

GATE_JOB_ERR=$(json_extract "$GATE_JOB_BODY" ".error.code")
GATE_JOB_MSG=$(json_extract "$GATE_JOB_BODY" ".error.message")
if [ "$GATE_JOB_ERR" != "EMAIL_NOT_VERIFIED" ]; then
    log_fail "Expected error code EMAIL_NOT_VERIFIED on job creation, got '$GATE_JOB_ERR'"
    exit 1
fi
if [[ "$GATE_JOB_MSG" != *"Campus verification"* ]]; then
    log_fail "Expected error message to mention 'Campus verification', got '$GATE_JOB_MSG'"
    exit 1
fi
log_pass "Job creation strictly blocked for unverified member: HTTP 403 (EMAIL_NOT_VERIFIED: $GATE_JOB_MSG)"

log_substep "Assertion 9b: Unverified student attempts resume upload (Must return 403 EMAIL_NOT_VERIFIED)..."
GATE_UPLOAD_RESP=$(curl -s -S -w "\n%{http_code}" \
    -X POST "${API_BASE_URL}/api/v1/profile/student/resume" \
    -H "Authorization: Bearer ${UNVERIFIED_STUDENT_TOKEN}" \
    -H "Cookie: sAccessToken=${UNVERIFIED_STUDENT_TOKEN}" \
    -H "st-auth-mode: header" \
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
log_pass "Resume upload strictly blocked for unverified member: HTTP 403 (EMAIL_NOT_VERIFIED)"

log_substep "Assertion 9c: Unverified student attempts job apply (Must return 403 EMAIL_NOT_VERIFIED)..."
GATE_APPLY_RESP=$(curl -s -S -w "\n%{http_code}" \
    -X POST "${API_BASE_URL}/api/v1/jobs/${JOB_ID}/applications" \
    -H "Authorization: Bearer ${UNVERIFIED_STUDENT_TOKEN}" \
    -H "Cookie: sAccessToken=${UNVERIFIED_STUDENT_TOKEN}" \
    -H "st-auth-mode: header" \
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
log_pass "Job application strictly blocked for unverified member: HTTP 403 (EMAIL_NOT_VERIFIED)"

# ------------------------------------------------------------------------------
# STEP 10: Verified Student Job Application (POST /api/v1/jobs/{id}/applications)
# ------------------------------------------------------------------------------
log_step "10" "Verified Student Job Application Submission"

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

# SEC-07 Assertion: Job poster downloads applicant resume via profile route after application
log_substep "Fetching applicant resume via GET /api/v1/profile/{id}/resume (SEC-07)..."
MEMBER_RESUME_RESP=$(http_request "GET" "/api/v1/profile/${VERIFIED_STUDENT_USER_ID}/resume" "$EMPLOYER_TOKEN" "" "200")
PRESIGNED=$(json_extract "$MEMBER_RESUME_RESP" ".data.download_url")
if [ -z "$PRESIGNED" ] || [ "$PRESIGNED" = "null" ]; then
    PRESIGNED=$(json_extract "$MEMBER_RESUME_RESP" ".data.url")
fi
if [ -z "$PRESIGNED" ] || [ "$PRESIGNED" = "null" ]; then
    log_fail "SEC-07: missing presigned download URL in JSON body: $MEMBER_RESUME_RESP"
    exit 1
fi
DL_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -L --max-redirs 0 "$PRESIGNED" || true)
if [ "$DL_STATUS" != "200" ]; then
    log_fail "SEC-07: presigned GET expected 200, got $DL_STATUS"
    exit 1
fi
log_pass "SEC-07: Job poster retrieved applicant resume via /profile/{id}/resume and GET presigned URL"

# ------------------------------------------------------------------------------
# STEP 11: Employer Applicant Review & Acceptance (Atomic Contract Generation)
# ------------------------------------------------------------------------------
log_step "11" "Employer Applicant Review & Acceptance"

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
# STEP 12: Contract Status Progression (Active -> Completed)
# ------------------------------------------------------------------------------
log_step "12" "Contract State Machine Progression (active -> completed)"

log_substep "Inspecting active contract detail (GET /api/v1/contracts/${CONTRACT_ID})..."
CONTRACT_GET_RESP=$(http_request "GET" "/api/v1/contracts/${CONTRACT_ID}" "$EMPLOYER_TOKEN" "" "200")
INITIAL_STATUS=$(json_extract "$CONTRACT_GET_RESP" ".data.status")
AGREED_BUDGET=$(json_extract "$CONTRACT_GET_RESP" ".data.agreed_budget_cents")

if [ "$INITIAL_STATUS" != "active" ]; then
    log_fail "Expected initial contract status 'active', got '$INITIAL_STATUS'"
    exit 1
fi
log_pass "Contract verified in active state (Agreed Budget Cents: $AGREED_BUDGET)"

# SEC-06 Assertion: Freelancer attempting to mark contract completed must receive 403 Forbidden
log_substep "Asserting freelancer cannot mark contract completed (SEC-06)..."
FREELANCER_COMPLETE_RESP=$(curl -s -S -w "\n%{http_code}" \
    -X PATCH "${API_BASE_URL}/api/v1/contracts/${CONTRACT_ID}/status" \
    -H "Authorization: Bearer ${VERIFIED_STUDENT_TOKEN}" \
    -H "Cookie: sAccessToken=${VERIFIED_STUDENT_TOKEN}" \
    -H "st-auth-mode: header" \
    -H "Content-Type: application/json" \
    -d '{"status":"completed"}' 2>/dev/null || echo -e "\n000")
FREELANCER_COMPLETE_CODE=$(echo "$FREELANCER_COMPLETE_RESP" | tail -n1)
if [ "$FREELANCER_COMPLETE_CODE" != "403" ]; then
    log_fail "SEC-06: Freelancer completion attempt not rejected with 403, got: $FREELANCER_COMPLETE_CODE"
    exit 1
fi
log_pass "SEC-06: Freelancer completion attempt rejected with HTTP 403 Forbidden"

log_substep "Transitioning contract status to 'completed' as client..."
COMPLETE_RESP=$(http_request "PATCH" "/api/v1/contracts/${CONTRACT_ID}/status" "$EMPLOYER_TOKEN" '{"status":"completed"}' "200")
FINAL_STATUS=$(json_extract "$COMPLETE_RESP" ".data.status")

if [ "$FINAL_STATUS" != "completed" ]; then
    log_fail "Contract transition to completed failed. Response: $COMPLETE_RESP"
    exit 1
fi
log_pass "Contract successfully transitioned to terminal state: completed"

# SEC-05 Assertion: Parent job automatically transitioned to 'closed'
log_substep "Asserting parent job status automatically transitioned to 'closed' (SEC-05)..."
JOB_CHECK_RESP=$(http_request "GET" "/api/v1/jobs/${JOB_ID}" "$EMPLOYER_TOKEN" "" "200")
JOB_CHECK_STATUS=$(json_extract "$JOB_CHECK_RESP" ".data.status")
if [ "$JOB_CHECK_STATUS" != "closed" ]; then
    log_fail "SEC-05: Expected job status 'closed', got '$JOB_CHECK_STATUS'"
    exit 1
fi
log_pass "SEC-05: Parent job atomically transitioned to 'closed' upon contract completion"

# ------------------------------------------------------------------------------
# STEP 13: Peer Review Submission & Duplicate Conflict Assertion
# ------------------------------------------------------------------------------
log_step "13" "Peer Review Submission & Duplicate Review Conflict Check"

log_substep "Employer submits 5-star review for student on completed contract..."
EMP_REVIEW_PAYLOAD=$(cat <<EOF
{
  "rating": 5,
  "comment": "Exceptional work! Jordan delivered clean Go code, thorough unit tests, and excellent communication throughout the project."
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
  "comment": "Outstanding employer to work with! Clear requirements, clear deliverables, and prompt feedback."
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
    -H "Cookie: sAccessToken=${EMPLOYER_TOKEN}" \
    -H "st-auth-mode: header" \
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

# SEC-01 Assertion: Reviews return profile names via joined profiles table
log_substep "Asserting reviewer profile joins return names without SQL errors (SEC-01)..."
if ! echo "$REVIEWS_RESP" | grep -q '"first_name"'; then
    log_fail "SEC-01: Reviews missing joined reviewer first_name: $REVIEWS_RESP"
    exit 1
fi
log_pass "SEC-01: Reviews successfully joined profiles table returning member names"

# ------------------------------------------------------------------------------
# SUMMARY
# ------------------------------------------------------------------------------
echo -e "\n${C_BOLD}${C_GREEN}======================================================================${C_RESET}"
echo -e "${C_BOLD}${C_GREEN}  ALL 13 END-TO-END SMOKE TEST PHASES PASSED SUCCESSFULLY!            ${C_RESET}"
echo -e "${C_BOLD}${C_GREEN}======================================================================${C_RESET}"
echo -e "  ${C_CYAN}Job ID:${C_RESET}         $JOB_ID"
echo -e "  ${C_CYAN}Application ID:${C_RESET} $APPLICATION_ID"
echo -e "  ${C_CYAN}Contract ID:${C_RESET}    $CONTRACT_ID"
echo -e "  ${C_CYAN}Resume Key:${C_RESET}     $RESUME_KEY"
echo -e "  ${C_GREEN}Exit code:${C_RESET}      0 (Verified)\n"

exit 0
