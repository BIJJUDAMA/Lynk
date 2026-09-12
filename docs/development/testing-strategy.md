# Testing, Quality Assurance, & Verification Strategy - Lynk

> **Authoritative Testing Handbook & Verification Standard**  
> **Backend Testing:** Go 1.25 Standard Library + Race Detector (`-race -count=1`)  
> **Frontend Testing:** Node 22+ Native ESM Runner (`--experimental-strip-types --test`)  
> **Continuous Integration:** GitHub Actions Multi-Job Pipeline (`.github/workflows/ci.yml`)

---

## 1. Core Verification Disciplines

All contributors and agentic systems operating in the Lynk codebase must adhere to the three non-negotiable verification disciplines:

### Discipline 1: The Iron Law of Debugging
```text
NO FIXES WITHOUT ROOT CAUSE INVESTIGATION FIRST
```
Before proposing or applying any code change:
1. **Read Traces Completely:** Inspect HTTP status codes, Go stack traces, database query error logs, and browser network responses.
2. **Reproduce with Minimal Case:** Replicate bugs using a targeted unit test, SQL query, or curl command before modifying application code.
3. **Inspect Boundaries:** Determine whether the fault originated at the HTTP handler level, service domain invariants, database constraints, or MinIO S3 I/O.

### Discipline 2: The Iron Law of Verification
```text
NO COMPLETION CLAIMS WITHOUT FRESH VERIFICATION EVIDENCE
```
- A task is never marked complete based on assumption. Every claim must include raw command output demonstrating 0 errors, 0 lint warnings, and 0 failing tests.
- Prohibited phrases prior to raw command evidence: *"should work"*, *"looks fixed"*, *"probably passing"*.

### Discipline 3: Behavioral Testing Over Mocks
- Prioritize real system behavior over brittle, mock-heavy assertions.
- Verify real database constraints (`ON DELETE RESTRICT`, unique indexes) and real HTTP response shapes (`httptest.ResponseRecorder`) instead of mocking SQL drivers or HTTP interfaces.

---

## 2. Backend Testing Architecture (Go)

The Go test suite is structured into three distinct layers:

```mermaid
flowchart TD
    subgraph UnitTests["1. Domain Unit Tests (In-Memory)"]
        UT1["Email domain regex validator (IsEduEmail)"]
        UT2["Contract state transitions"]
        UT3["Calendar-day deadline UTC normalization"]
        UT4["Claims role checks (slices.Contains)"]
        UT5["Job description bounds & skill truncation"]
    end

    subgraph HandlerTests["2. HTTP Handler Tests (httptest)"]
        HT1["httptest.NewRecorder() verification"]
        HT2["JSON error envelopes (400, 403, 404, 409)"]
        HT3["Multipart resume boundary validation"]
        HT4["Header & Cookie assertions"]
    end

    subgraph IntegrationTests["3. Database Integration Tests (pgxpool)"]
        IT1["Atomic contract creation (AcceptApplicationTx)"]
        IT2["Lock ordering deadlock tests (jobs -> apps -> contracts)"]
        IT3["Contract cancellation & re-hire restoration"]
        IT4["Raw SQL migration execution (000001 -> 000009)"]
    end
```

### 2.1 Test Execution Commands

```bash
# Run all backend unit and integration tests (with cached results)
cd backend
go test ./...

# Run clean test execution with race detector (zero cache)
cd backend
go test -race -count=1 ./...

# Run static analysis and vet checks
cd backend
go vet ./...
golangci-lint run
```

### 2.2 Integration Test Database Isolation
Integration tests connect via the `TEST_DATABASE_URL` environment variable:
```go
dbURL := os.Getenv("TEST_DATABASE_URL")
if dbURL == "" {
    t.Skip("TEST_DATABASE_URL not set; skipping integration test")
}
pool, err := pgxpool.New(ctx, dbURL)
```
- In CI, `TEST_DATABASE_URL` targets the containerized PostgreSQL service (`postgres://lynk_user:lynk_password@localhost:5432/lynk_db?sslmode=disable`).
- Migrations are applied to `lynk_db` prior to test execution via `TestRunMigrations_AppliesInitOnEmptyDatabase`.

---

## 3. Frontend Testing Architecture (Next.js / TypeScript)

Lynk leverages modern Node.js 22+ native TypeScript type-stripping for lightning-fast test execution without bloated Jest/Babel configurations:

```mermaid
flowchart LR
    TSFiles["TypeScript Source (*.test.ts)"] --> NodeRunner["Node.js 22 Native Runner<br/>node --experimental-strip-types --test"]
    NodeRunner --> Results["43/43 Passing Tests<br/>(Sub-second Execution)"]
```

### 3.1 Test Execution Commands

```bash
cd frontend

# 1. Run unit test suite (Node 22 native runner)
npm test

# 2. Enforce strict TypeScript compilation (zero any, zero type errors)
npm run typecheck
# Equivalently: npx tsc --noEmit

# 3. Enforce Next.js ESLint rules
npm run lint

# 4. Compile production Next.js build (verifies static & dynamic route trees)
npm run build
```

### 3.2 Key Testing Suites
- `api.test.ts`: Verifies HTTP client header attachment, static SuperTokens `Session` import resolution, error normalization, and cleared field preservation.
- `verify-session.test.ts`: Validates session verification helpers, redirect logic, and email verification status evaluation.
- `formatters.test.ts`: Verifies monetary formatting, hourly contract rate displays, and date formatting.

---

## 4. Continuous Integration Pipeline (.github/workflows/ci.yml)

The platform is guarded by a multi-job GitHub Actions workflow triggered on every push and pull request to `main`:

```mermaid
flowchart TB
    subgraph GitHubActions["GitHub Actions CI Pipeline (.github/workflows/ci.yml)"]
        subgraph BackendJob["Job 1: Backend CI (Ubuntu Latest)"]
            B1["PostgreSQL 16 Service Container (port 5432)"]
            B2["Setup Go 1.25.x with Module Cache"]
            B3["golangci-lint v2.4.0 (Go 1.25 compatible)"]
            B4["Verify SQL migrations on throwaway DB (golang-migrate)"]
            B5["Apply application migrations on lynk_db"]
            B6["Test with Go Race Detector (go test -race -count=1 ./...)"]
            B1 --> B2 --> B3 --> B4 --> B5 --> B6
        end

        subgraph FrontendJob["Job 2: Frontend CI (Ubuntu Latest)"]
            F1["Setup Node.js 22 with npm cache"]
            F2["npm ci (clean dependency installation)"]
            F3["npm test (native type-stripping runner)"]
            F4["npm run typecheck (tsc --noEmit)"]
            F5["npm run build (Next.js 14 production compilation)"]
            F1 --> F2 --> F3 --> F4 --> F5
        end
    end
```

### 4.1 Backend CI Verification Sequence
1. **Container Health:** PostgreSQL 16 Alpine container spins up with health checks (`pg_isready`).
2. **Linting:** `golangci-lint` (v2.4.0) performs static analysis across all Go packages.
3. **Migration Verification:** `golang-migrate` tests raw SQL migration files (`000001` through `000010`) against a throwaway database (`lynk_migrate_check`), testing forward `up`, rollback `down -all`, and re-application `up`.
4. **App Migrations:** The application migrator applies migrations to `lynk_db` via `TestRunMigrations_AppliesInitOnEmptyDatabase`.
5. **Race Detector Suite:** `go test -race -count=1 ./...` executes across all 13 Go packages under thread-safety scrutiny.

### 4.2 Frontend CI Verification Sequence
1. **Clean Installation:** `npm ci` restores exact locked dependencies from `package-lock.json`.
2. **Linter:** `npm run lint` (`next lint`) validates ESLint rules and React hook constraints.
3. **Unit Tests:** `npm test` executes all unit tests using Node 22 native type-stripping.
4. **Typecheck:** `tsc --noEmit` validates all TypeScript interfaces and props.
5. **Production Build:** `next build` compiles all 13 static and dynamic routes.

### 4.3 Container Build Workflow (.github/workflows/docker-build.yml)
Validates that the multi-stage Go container image builds cleanly via Buildx on any changes to `backend/**` or `docker-compose.yml`.

### 4.4 End-to-End Smoke Test Workflow (.github/workflows/smoke-test.yml)
Spins up the complete Docker Compose multi-container stack (`lynk-postgres`, `lynk-supertokens`, `lynk-minio`, and `lynk-api`), waits for all four healthchecks to pass, and executes `./scripts/smoke-test.sh` exercising the complete marketplace lifecycle against live network sockets.

---

## 5. End-to-End Smoke Test Automation

For full-stack verification of live running containers and endpoints, automated smoke test scripts are provided:

```bash
# PowerShell Smoke Test Suite (Windows):
.\scripts\smoke-test.ps1

# Bash Smoke Test Suite (Linux / macOS):
./scripts/smoke-test.sh
```

### 5.1 Automated Smoke Test Coverage
1. Deep health check evaluation (`GET /health`).
2. SuperTokens Core ping (`GET /hello`).
3. Member registration with verified `.edu` email address.
4. Profile retrieval and metadata updates.
5. Resume multipart streaming upload to MinIO S3.
6. Job opportunity publication.
7. Application submission against the open job.
8. Application acceptance and contract generation.
9. Contract completion and peer review submission.
