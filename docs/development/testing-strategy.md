# Testing, Quality Assurance, & Verification Strategy - Lynk

> **Authoritative Testing Handbook & Verification Standard**  
> **Backend Testing:** Go 1.25 Standard Library + Race Detector (`-race -count=1`) across 16 packages  
> **AI Subsystem Testing:** Python 3.11+ Pytest Suite (100 tests) + Offline IR Benchmarks (`ai/evaluation/`)  
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
        IT4["Raw SQL migration execution (000001 -> 000012)"]
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

## 4. AI Subsystem Testing Architecture (Python & Offline Benchmarks)

The AI subsystem maintains a dedicated Python testing suite and offline evaluation harness under `ai/tests/` and `ai/evaluation/`:

```mermaid
flowchart TD
    subgraph PytestSuite["1. Fast Isolated Pytest Suite (ai/tests/)"]
        PT1["Skill Normalization & Fuzzy Aliasing (test_normalizer.py)"]
        PT2["Dense Vector Cosine Similarity (test_embeddings.py)"]
        PT3["Hybrid Search Fusion (test_search.py)"]
        PT4["Demographic-Free Candidate Ranking (test_ranking.py)"]
        PT5["Member Recommendations Engine (test_recommendations.py)"]
        PT6["Duplicate & Velocity Moderation (test_moderation.py)"]
        PT7["Aspect-Based Review Sentiment (test_reviews.py)"]
        PT8["Statistical Skill Demand Forecasting (test_analytics.py)"]
        PT9["vLLM Generative Drafts & Schema Validation (test_drafts.py)"]
        PT10["Asynchronous Queue Worker with SKIP LOCKED (test_worker.py)"]
    end

    subgraph EvalHarness["2. Offline Evaluation & IR Benchmarks (ai/evaluation/)"]
        EH1["Precision@K & Recall@K metrics"]
        EH2["Mean Reciprocal Rank (MRR)"]
        EH3["Normalized Discounted Cumulative Gain (NDCG@K)"]
        EH4["F1 Score with zero-division guards"]
        EH5["Curated Benchmark Ground Truth (datasets/sample_eval.json)"]
    end

    PytestSuite --> EvalHarness
```

### 4.1 Test Execution Commands

```bash
# Run all Python AI unit and pipeline tests (100 passing tests)
python -m pytest ai/tests/ -v

# Run targeted test suites
python -m pytest ai/tests/test_search.py -v
python -m pytest ai/tests/test_ranking.py -v
python -m pytest ai/tests/test_worker.py -v

# Run offline evaluation benchmark harness
python -m ai.evaluation.harness
```

### 4.2 Offline Evaluation Metrics Standard
The benchmark harness (`ai/evaluation/metrics.py`) enforces strict information retrieval metrics for quantifying pipeline accuracy across model and prompt revisions:
- **Precision@K & Recall@K:** Measures relevant candidate recall within the top-K recommendations.
- **Mean Reciprocal Rank (MRR):** Quantifies where the first relevant profile or job appears in search results.
- **NDCG@K (Normalized Discounted Cumulative Gain):** Evaluates non-binary relevance gradations against theoretical ideal rankings (`IDCG`).
- **Mathematical Invariant:** All evaluation functions include zero-division and empty dataset guards, ensuring reliable test automation without NaN exceptions.

---

## 5. Continuous Integration Pipeline (.github/workflows/ci.yml)

The platform is guarded by a multi-job GitHub Actions workflow triggered on every push and pull request to `main`:

```mermaid
flowchart TB
    subgraph GitHubActions["GitHub Actions CI Pipeline (.github/workflows/ci.yml)"]
        subgraph BackendJob["Job 1: Backend CI (Ubuntu Latest)"]
            B1["PostgreSQL 16 Service Container with pgvector (port 5432)"]
            B2["Setup Go 1.25.x with Module Cache"]
            B3["golangci-lint v2.4.0 (Go 1.25 compatible)"]
            B4["Verify SQL migrations on throwaway DB (golang-migrate)"]
            B5["Apply application migrations on lynk_db (000001 -> 000012)"]
            B6["Test with Go Race Detector across 16 packages"]
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

        subgraph AIJob["Job 3: AI Subsystem CI (Ubuntu Latest)"]
            A1["Setup Python 3.11 with pip cache"]
            A2["Install dependencies (FastAPI, PyTorch, pgvector, pydantic)"]
            A3["Execute 100-test Pytest Suite (pytest ai/tests/ -v)"]
            A4["Run Offline IR Evaluation Harness (python -m ai.evaluation.harness)"]
            A1 --> A2 --> A3 --> A4
        end
    end
```

### 5.1 Backend CI Verification Sequence
1. **Container Health:** PostgreSQL 16 Alpine container spins up with `pgvector` extension and health checks (`pg_isready`).
2. **Linting:** `golangci-lint` (v2.4.0) performs static analysis across all Go packages.
3. **Migration Verification:** `golang-migrate` tests raw SQL migration files (`000001` through `000012`) against a throwaway database (`lynk_migrate_check`), testing forward `up`, rollback `down -all`, and re-application `up`.
4. **App Migrations:** The application migrator applies migrations to `lynk_db` via `TestRunMigrations_AppliesInitOnEmptyDatabase`.
5. **Race Detector Suite:** `go test -race -count=1 ./...` executes across all 16 Go packages under thread-safety scrutiny.

### 5.2 Frontend CI Verification Sequence
1. **Clean Installation:** `npm ci` restores exact locked dependencies from `package-lock.json`.
2. **Linter:** `npm run lint` (`next lint`) validates ESLint rules and React hook constraints.
3. **Unit Tests:** `npm test` executes all unit tests using Node 22 native type-stripping.
4. **Typecheck:** `tsc --noEmit` validates all TypeScript interfaces and props.
5. **Production Build:** `next build` compiles all 13 static and dynamic routes.

### 5.3 Container Build Workflow (.github/workflows/docker-build.yml)
Validates that multi-stage Go and Python container images build cleanly via Buildx on any changes to `backend/**`, `ai/**`, or `docker-compose.yml`.

### 5.4 End-to-End Smoke Test Workflow (.github/workflows/smoke-test.yml)
Spins up the complete Docker Compose multi-container stack (`lynk-postgres`, `lynk-supertokens`, `lynk-minio`, `lynk-api`, `ai-api`, and `ai-worker`), waits for all service healthchecks to pass, and executes `./scripts/smoke-test.sh` exercising the complete 14-phase marketplace lifecycle against live network sockets.

---

## 6. End-to-End Smoke Test Automation

For full-stack verification of live running containers and endpoints, automated smoke test scripts are provided:

```bash
# PowerShell Smoke Test Suite (Windows):
.\scripts\smoke-test.ps1

# Bash Smoke Test Suite (Linux / macOS):
./scripts/smoke-test.sh
```

### 6.1 Automated 14-Phase Smoke Test Coverage
1. **Phase 1: Deep Health Probes:** Evaluates Go backend health (`GET /health`), database connectivity, and MinIO S3 bucket availability.
2. **Phase 2: SuperTokens IAM Ping:** Validates SuperTokens Core connectivity (`GET /hello`).
3. **Phase 3: Member Registration & Verification:** Provisions student accounts with institutional `.edu` email validation and token activation.
4. **Phase 4: Profile Management:** Updates profile bio, major, skills, and portfolio URLs.
5. **Phase 5: Multipart Resume Upload:** Streams binary resume upload to MinIO S3 and validates download pre-signed URLs.
6. **Phase 6: Campus Gig Publication:** Posts campus opportunity with title, task description, and required skill tags.
7. **Phase 7: Application Submission:** Submits proposal pitch with attached resume key.
8. **Phase 8: Application Acceptance & Contract State Machine:** Accepts application, verifies atomic state transition (`pending` -> `accepted`), closes job, and spawns `active` contract.
9. **Phase 9: Contract Cancellation & Re-Hire Restoration:** Tests employer cancellation, restoring job to `open` and peer applications to `pending`.
10. **Phase 10: Contract Re-Formation & Completion:** Re-hires candidate and marks contract `completed`.
11. **Phase 11: Peer Review & 1-5 Star Rating:** Submits qualitative review and numerical rating.
12. **Phase 12: Activity Feed Stream Verification:** Verifies unified audit events across contract and job actions.
13. **Phase 13: Security Boundaries & Edge Cases:** Tests unverified email blocks (403), 1MB JSON clamping (413), and protocol injection filters.
14. **Phase 14: First-Class AI Subsystem End-to-End Verification:**
    - **14.1 AI Health Probe:** Verifies internal FastAPI health (`:8000/health`) and Go orchestrator connectivity.
    - **14.2 Generative Job Drafts:** Invokes `POST /api/v1/jobs/generate` to produce structured job drafts from brief raw ideas.
    - **14.3 Database Non-Mutation:** Proves generative drafts are strictly in-memory suggestions and do NOT insert uncommitted rows into `jobs`.
    - **14.4 Applicant Advisory Ranking & RBAC:** Verifies `GET /api/v1/jobs/{id}/applicants/ranking` returns 200 with non-empty scoring for the job creator, and strictly rejects non-owners with 403 Forbidden.
    - **14.5 Hybrid Semantic Search:** Validates `GET /api/v1/search/jobs?q=...` returning semantic similarity and keyword scores, with deterministic SQL fallback when AI is disabled.
    - **14.6 Member Recommendations:** Tests `GET /api/v1/profile/recommendations` returning personalized skill acquisition suggestions.
    - **14.7 Review Insights:** Tests `GET /api/v1/users/{id}/ai-insights` returning aggregated aspect scores for completed work.
    - **14.8 Skill Demand Analytics:** Verifies `GET /api/v1/analytics/skills` returning campus velocity and growth snapshots.
    - **14.9 Graceful Degradation:** Simulates AI service outage and proves Go backend returns HTTP 200 with deterministic SQL fallback data.
