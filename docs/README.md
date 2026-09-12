# Lynk Documentation Suite & Architecture Hub

> **Authoritative Technical Documentation Hub**  
> **Platform:** Lynk - High-Trust Campus Opportunity Discovery & Student Networking Platform  
> **Repository:** Monorepo (`frontend/`, `backend/`, `docs/`, `docker-compose.yml`)  
> **Status:** MVP Implementation Baseline (Production-Ready Architecture)  
> **Identity & Access Management (IAM):** Self-Hosted SuperTokens Core 9.3 (`:3567`)  
> **Object Storage:** MinIO S3 (`:9000` API / `:9001` Web Console) - Dedicated exclusively to resumes

---

## 1. Executive Summary & Core Thesis

**Lynk** is an institutional campus opportunity discovery and student networking platform engineered to connect verified university students, student founders, faculty, campus laboratories, and student organizations. Operating as a pure discovery-first network with zero payment handling (similar to a specialized LinkedIn or Facebook directory for university campuses), Lynk facilitates peer collaboration, deliverable agreements, and verified reputation building.

Traditional platforms fail campus ecosystems because they:
1. Impose financial payment processing overhead and commercial transaction friction on academic and peer collaborations.
2. Segregate participants into rigid, adversarial "employer" and "freelancer" account silos.
3. Lack institutional identity verification, exposing students to identity fraud and unverified external actors.
4. Fail to capture academic context (departments, majors, graduation cohorts, and campus reputation).

### The Unified Campus Member Model
Lynk completely replaces the fragmented dual-account paradigm with the **Unified Campus Member** model:
- **Single Identity, Dual Capabilities:** Every authenticated member in the platform possesses a single account backed by a mandatory institutional `.edu` email address.
- **Fluid Role Switching:** Any member can simultaneously post opportunities (as an organizer/poster) and apply to opportunities (as a contributor/student).
- **Academic Context:** Profiles feature major, department, graduation year, verified skills, and campus club affiliations alongside professional portfolios.
- **Trust Through Invariants:** Opportunities and proposals are backed by deliverable collaboration contracts, strict relational constraints, and peer reviews tied to completed contracts.

```
       +-------------------------------------------------------------+
       |               Unified Campus Member Identity                |
       |             Verified institutional .edu Account             |
       +-------------------------------------------------------------+
                       /                             \
                      /                               \
                     v                                 v
        [ Organizer / Poster ]              [ Contributor / Student ]
        - Create opportunity postings       - Browse campus opportunities
        - Review proposals                  - Submit applications
        - Coordinate & execute contracts    - Upload verified resume
        - Review student deliverable        - Receive peer rating & review
```

---

## 2. 30-Second Technical Orientation

| Dimension | Specification | Notes & Invariants |
| :--- | :--- | :--- |
| **Monorepo Topology** | Polyglot monorepo (`frontend/`, `backend/`, `docs/`) | Co-located frontend and backend with decoupled deployment boundaries. |
| **Frontend Stack** | Next.js 14.2 (App Router), React 18.3, TypeScript 5.6, Tailwind CSS 3.4 | **Runs on host (`npm run dev`) at `http://localhost:3000`**. NOT dockerised for instant HMR. |
| **Frontend Auth Client** | `supertokens-web-js` 0.16.0 | Handles session tokens (`sAccessToken`, `sRefreshToken`), direct login, and email verification. |
| **Backend Stack** | Go 1.25, Chi Router v5.3, `pgx/v5` 5.5, AWS SDK Go v2 (S3) | **Dockerised (`lynk-api`) at `http://localhost:8080`**. Layered architecture: Handler -> Service -> Repository. |
| **Database** | PostgreSQL 16 Alpine (`lynk-postgres` on port `5432`) | Two logical databases: `lynk_db` (application schema) and `supertokens_db` (auth engine state). |
| **Migrations** | Raw sequential SQL (`backend/migrations/000001` - `000011`) | Immutable forward migrations applied automatically on startup or via `make migrate-up`. |
| **Identity & Access (IAM)** | Self-hosted SuperTokens Core 9.3 (`lynk-supertokens` on port `3567`) | Recipes: `EmailPassword`, `Session`, `EmailVerification`. Direct HTTP integration with Go API. |
| **Object Storage** | MinIO S3 (`lynk-minio` on port `9000` API, `9001` Web Console) | Dedicated exclusively to the `resumes` bucket. Stores PDF/DOCX resumes with pre-signed retrieval URLs. |
| **Local Network** | Docker Compose bridge network (`lynk-net`) | Internal container DNS: `postgres:5432`, `supertokens:3567`, `minio:9000`, `api:8080`. |
| **Scope Boundary** | Strict MVP ("Nothing Else Initially") | Zero Redis, zero RabbitMQ, zero Kafka, zero cloud auth SaaS vendors. All operations are synchronous and direct. |

---

## 3. High-Level Monorepo Topology & Container Boundary

The platform strictly isolates containerized backend infrastructure from the host development environment to preserve rapid local UI iteration while guaranteeing production-grade container parity for services.

```mermaid
flowchart TB
    subgraph HostEnvironment["HOST ENVIRONMENT (Local Developer Workstation)"]
        FE["Next.js 14 App Router<br/>(Port: 3000)<br/>Host Process (npm run dev)"]
        Browser["Developer Web Browser<br/>(http://localhost:3000)"]
        Browser <-->|HMR & UI Navigation| FE
    end

    subgraph DockerBridge["DOCKER COMPOSE BRIDGE NETWORK (lynk-net)"]
        subgraph APIContainer["API Container (lynk-api)"]
            Router["Chi Router v5.3<br/>(Port: 8080)"]
            Middlewares["Middlewares: CORS, RequestID, Logger, 1MB Body Limit,<br/>Supertokens Middleware, Verified Email Gate"]
            Handlers["Layered Core:<br/>Handler -> Service -> Repository"]
            Router --> Middlewares --> Handlers
        end

        subgraph IAMContainer["SuperTokens Container (lynk-supertokens)"]
            STCore["SuperTokens Core 9.3<br/>(Port: 3567)<br/>Recipes: EmailPassword, Session, EmailVerification"]
        end

        subgraph DBContainer["PostgreSQL Container (lynk-postgres)"]
            Postgres["PostgreSQL 16 Alpine<br/>(Port: 5432)"]
            AppDB[("lynk_db<br/>(Profiles, Jobs, Applications, Contracts, Reviews)")]
            AuthDB[("supertokens_db<br/>(Users, Sessions, Verification Tokens)")]
            Postgres --> AppDB
            Postgres --> AuthDB
        end

        subgraph S3Container["MinIO Object Storage (lynk-minio)"]
            MinIO["MinIO S3 Engine<br/>(Port: 9000 API / 9001 Console)"]
            ResumeBucket[("Bucket: resumes<br/>(Encrypted PDF/DOCX Resumes)")]
            MinIO --> ResumeBucket
        end
    end

    %% Network Connections
    FE -->|HTTP / Credentials & Session Handshake| Router
    FE -->|Direct SuperTokens SDK Handshake| Router
    Handlers -->|Go SDK Session Verification| STCore
    STCore -->|Internal SQL Connection| AuthDB
    Handlers -->|pgx Connection Pool| AppDB
    Handlers -->|AWS SDK S3 PutObject / Presigned URLs| MinIO
```

---

## 4. Complete Documentation Sitemap

The documentation suite is structured into dedicated modular domains under [`architecture/`](architecture/) and [`development/`](development/). Every specification is authoritative, fully typed, and verified against the production codebase.

| Document | Relative Path | Core Subject Matter | Primary Target Audience |
| :--- | :--- | :--- | :--- |
| **System Architecture & Monorepo Topology** | [`architecture/system-architecture.md`](architecture/system-architecture.md) | High-level system topology, container boundaries, layered Go architecture, dynamic socket timeouts, evolutionary roadmap (Phases 1-4). | All Engineers, Architects, Technical Leads |
| **Database Schema, Relational Constraints & ERD** | [`architecture/database-schema-erd.md`](architecture/database-schema-erd.md) | Mermaid ERD, table-by-table column specs, foreign keys (`ON DELETE RESTRICT`), indexes, state machines, lock ordering discipline, migration registry (000001-000011). | Backend Engineers, Database Admins, System Architects |
| **Complete REST API Specifications** | [`architecture/api-specifications.md`](architecture/api-specifications.md) | Exhaustive documentation of all 24 HTTP endpoints across 6 domains: Auth, Profile, Jobs, Applications, Contracts, Reviews. Status codes, request/response JSON schemas, and error shapes. | Frontend Engineers, Backend Engineers, Integrators |
| **OpenAPI 3.1.0 Contract** | [`architecture/openapi.yaml`](architecture/openapi.yaml) | Machine-readable OpenAPI 3.1.0 specification defining components, schemas, parameters, security schemes (`sessionCookie`, `bearerAuth`), and response envelopes. | API Tooling, Frontend Codegen, QA Engineers |
| **Security, Threat Model & IAM Architecture** | [`architecture/security-and-iam.md`](architecture/security-and-iam.md) | Self-hosted SuperTokens Core configuration, institutional `.edu` email gate, CORS/CSRF defenses, 1MB body limit DoS prevention, dynamic socket timeouts (F-05), password hashing, and threat mitigation. | Security Auditors, DevOps, Backend Engineers |
| **Object Storage Architecture & MinIO Policies** | [`architecture/object-storage.md`](architecture/object-storage.md) | MinIO S3 bucket configuration (`resumes`), bucket isolation, streaming uploads, MIME type validation (`application/pdf`, `msword`), pre-signed download URLs, and storage security. | Cloud Engineers, Backend Engineers, DevOps |
| **Operations, Runbook & Local Developer Guide** | [`development/runbook-and-operations.md`](development/runbook-and-operations.md) | First-time setup, Docker Compose lifecycle, environment variable matrices, database seed scripts, troubleshooting runbook, disaster recovery, and operational procedures. | All Developers, DevOps, Site Reliability Engineers |
| **Testing, Quality Assurance & Verification Strategy** | [`development/testing-strategy.md`](development/testing-strategy.md) | Testing pyramid (unit, integration, HTTP handler, frontend), test execution commands, coverage targets, CI/CD pipeline specifications, and pre-release gates. | QA Engineers, Backend Engineers, Frontend Engineers |

---

## 5. Persona-Based Reading Guides

To accelerate onboarding, follow the tailored reading paths based on your primary engineering focus.

### 5.1 For Backend Engineers
*Stack: Go 1.25, Chi v5.3, PostgreSQL 16 (`pgx/v5`), SuperTokens Core Go SDK, MinIO AWS S3 SDK v2.*

1. **System Foundation:** Begin with [`architecture/system-architecture.md`](architecture/system-architecture.md) to understand the layered execution model (`Handler` -> `Service` -> `Repository`), dependency injection in `backend/cmd/api/main.go`, and internal networking.
2. **Data & Migrations:** Read [`architecture/database-schema-erd.md`](architecture/database-schema-erd.md) thoroughly. Pay careful attention to:
   - Migration `000003`: The transition of `id` and `user_id` to `VARCHAR(64)` for SuperTokens compatibility.
   - Migration `000009`: Enforcement of `ON DELETE RESTRICT` on `applications.job_id` preventing dangling application states.
   - State transition invariants: Contract status transitions (`Draft` -> `Active` -> `Completed` | `Cancelled`) and the lock ordering discipline (`jobs -> applications -> contracts`) to prevent deadlocks.
3. **API Implementation:** Inspect [`architecture/api-specifications.md`](architecture/api-specifications.md) to verify expected route handlers, input validation constraints, and standardized JSON error structures (`{"error": "string"}`).
4. **Storage & Resumes:** Review [`architecture/object-storage.md`](architecture/object-storage.md) for MinIO S3 integration patterns, streaming multipart uploads, and bucket readiness probes.
5. **Local Workflow & Testing:** Use [`development/runbook-and-operations.md`](development/runbook-and-operations.md) for environment configuration and [`development/testing-strategy.md`](development/testing-strategy.md) for running `go test -v -race ./...`.

### 5.2 For Frontend Engineers
*Stack: Next.js 14 (App Router), React 18, TypeScript 5, Tailwind CSS, SuperTokens Web JS.*

1. **Orientation & Architecture:** Start with [`architecture/system-architecture.md`](architecture/system-architecture.md) to review runtime boundaries (Next.js executes directly on the host machine at `http://localhost:3000` while proxying API calls to `http://localhost:8080`).
2. **Authentication Flow:** Read the IAM and Session section in [`architecture/security-and-iam.md`](architecture/security-and-iam.md):
   - SuperTokens Web JS session initialization (`supertokens-web-js`).
   - Session cookies (`sAccessToken`, `sRefreshToken`) transmitted with `credentials: "include"`.
   - The Institutional Email Gate: Handling `403 Forbidden` responses when an unverified student attempts restricted marketplace actions.
3. **API Contract & Typing:** Consult [`architecture/api-specifications.md`](architecture/api-specifications.md) and [`architecture/openapi.yaml`](architecture/openapi.yaml). Ensure TypeScript types in `frontend/src/types/` exactly match Go struct JSON tags.
4. **Resume Uploads:** Understand the multipart form data requirements (`multipart/form-data`, file key `resume`, 5MB limit, PDF/DOCX only) defined in [`architecture/object-storage.md`](architecture/object-storage.md).
5. **Testing & Validation:** Review frontend test procedures (`tsc --noEmit`, `npm run lint`, node test runners) in [`development/testing-strategy.md`](development/testing-strategy.md).

### 5.3 For Security Auditors & DevOps Engineers
*Stack: Docker Compose, Network Security, IAM, Content Protection, Linux Hardening.*

1. **Security & Threat Model:** Prioritize [`architecture/security-and-iam.md`](architecture/security-and-iam.md). Key inspection areas:
   - Self-hosted SuperTokens Core credential handling and token rotation.
   - Institutional `.edu` domain verification enforcement in Go middleware (`RequireVerifiedEmail()`).
   - Request body limit protection: 1MB limit on JSON payloads preventing OOM exhaustion attacks (F-09).
   - TCP Socket Timeout configuration (F-05): Read/Write socket timeouts (95s/100s) comfortably exceeding application context deadlines (90s dynamic timeout for resume uploads) to eliminate connection drops.
   - CORS isolation: Strict origin whitelisting (`http://localhost:3000`).
2. **Object Storage Hardening:** Inspect [`architecture/object-storage.md`](architecture/object-storage.md) for MinIO private bucket policies, S3 signature v4 validation, and pre-signed URL expiration.
3. **Relational Data Integrity:** Review [`architecture/database-schema-erd.md`](architecture/database-schema-erd.md) for cascade policies, check constraints, and migration immutability.
4. **Operations & Runbook:** Review [`development/runbook-and-operations.md`](development/runbook-and-operations.md) for container health checks, secret hygiene (`SUPERTOKENS_API_KEY`, `MINIO_ROOT_PASSWORD`), and database backup/restore procedures.

---

## 6. Core Architectural Invariants & Non-Negotiables

Every engineer and agent working within this repository must uphold these non-negotiable invariants:

### Invariant 1: Strict MVP Scope ("Nothing Else Initially")
The MVP strictly consists of Next.js, Go API, PostgreSQL 16, SuperTokens Core, and MinIO S3.
- **NO** Redis or distributed caching.
- **NO** RabbitMQ, Kafka, or background message queues.
- **NO** Third-party commercial auth SaaS vendors (Auth0, Supabase, Firebase).
- **NO** Microservices. Data flow remains synchronous and direct. Asynchronous processing is reserved for post-MVP phases.

### Invariant 2: The Containerization Boundary
- **Backend Infrastructure:** The Go REST API (`backend/Dockerfile`), PostgreSQL, SuperTokens Core, and MinIO must run inside Docker containers orchestrated via `docker-compose.yml`.
- **Frontend Development:** The Next.js application (`frontend/`) must **NOT** be dockerised during development. It runs on the host machine (`npm run dev`) to leverage native operating system file-system events for instant Hot Module Replacement (HMR).

### Invariant 3: The Institutional Email Gate
Students and campus members cannot post opportunities, submit applications, accept contracts, or upload resumes until their institutional `.edu` email address has been verified. The Go API enforces this check via `middleware.RequireVerifiedEmail()`.

### Invariant 4: Resume Object Storage Exclusivity
Resumes are stored exclusively in **MinIO** (`resumes` bucket). The PostgreSQL database stores only metadata (object key, file name, MIME type, byte size, and upload timestamp). Binary blobs are strictly forbidden in PostgreSQL.

### Invariant 5: Database Migration Immutability
Never modify an existing raw SQL migration in `backend/migrations/` that has already been executed. Always write an incremental forward migration (`NNNNNN_name.up.sql`) paired with a rollback migration (`NNNNNN_name.down.sql`).

---

## 7. Operational Quick Reference & Command Cheat Sheet

### 7.1 Infrastructure Management
```bash
# Start all backend containers (PostgreSQL, SuperTokens, MinIO, Go API)
docker compose up -d

# Verify container health status (all containers should show healthy)
docker compose ps

# View unified container logs
docker compose logs -f

# Stop container infrastructure
docker compose down
```

### 7.2 Running Application Processes
```bash
# Terminal 1 (Host Frontend):
cd frontend
npm install
npm run dev
# Frontend runs at: http://localhost:3000

# Terminal 2 (Optional: Direct Host Go API for debugging):
cd backend
go run cmd/api/main.go
# Go API runs at: http://localhost:8080
```

### 7.3 Health Check Verification Probes
```bash
# Deep health check probe (evaluates PostgreSQL connection pool + MinIO bucket status)
curl -i http://localhost:8080/health

# Expected response (HTTP 200 OK):
# {"status":"ok","time":"2026-09-11T18:00:00Z"}

# SuperTokens Core health check
curl -i http://localhost:3567/hello

# MinIO S3 live probe
curl -i http://localhost:9000/minio/health/live
```

### 7.4 Verification & Quality Assurance Suite
```bash
# Backend unit & integration test suite (with race detector)
cd backend
go test -v -race ./...

# Frontend typecheck, linting & test suite
cd frontend
npm run lint
npm test
npm run typecheck
npm run build

# End-to-end multi-container smoke test suite (against live local Docker stack)
./scripts/smoke-test.sh          # Linux / macOS
.\scripts\smoke-test.ps1         # Windows PowerShell
```

### 7.5 Automated GitHub Actions CI/CD Pipelines
The repository enforces code quality and deployment safety across three decoupled workflows:
- **Core CI (`.github/workflows/ci.yml`):** Runs `golangci-lint`, forward/rollback migration checks (`up`, `down -all`, `up`), Go race detector tests, ESLint, frontend unit tests, TypeScript typechecks, and Next.js production builds.
- **Container Build (`.github/workflows/docker-build.yml`):** Validates that `backend/Dockerfile` builds cleanly via Buildx with layer caching on any backend or compose change.
- **E2E Smoke Test (`.github/workflows/smoke-test.yml`):** Orchestrates a clean Docker Compose environment, polls healthchecks for all four services, and executes `./scripts/smoke-test.sh` with automated container log dumps on failure.

---

## 8. Documentation Maintenance & Evolutionary Roadmap

This documentation suite is synchronized with the repository lifecycle. When modifying schema, endpoints, or policies:
1. Update the authoritative domain document in `docs/architecture/` or `docs/development/`.
2. Update the OpenAPI 3.1.0 contract in `docs/architecture/openapi.yaml`.
3. Verify all relative links from this navigation hub remain intact.
4. Keep all instructions aligned with the phased evolutionary roadmap:

```mermaid
timeline
    title Lynk Platform Evolutionary Roadmap
    Phase 1 (Active MVP) : Direct Go REST API
                         : Self-Hosted SuperTokens Core IAM
                         : PostgreSQL 16 Application Storage
                         : MinIO S3 Resume Storage
                         : Next.js 14 Host Development
    Phase 2 (Protection) : Redis Session Caching
                         : Redis Sliding-Window Rate Limiting
                         : Edge CDN Caching for Static Assets
    Phase 3 (Async Scale) : RabbitMQ Message Broker
                          : Asynchronous Worker Pool
                          : Transactional Email Queue
                          : Audit Logging & Event Sourcing
    Phase 4 (Enterprise) : k6 Stress & Load Testing Suite
                         : Prometheus Metrics & Grafana Telemetry
                         : OpenTelemetry Distributed Tracing
                         : Multi-Campus Multi-Tenant Partitioning
```
