<div align="center">

# Lynk

**Campus collaboration, project workspace, and peer gig platform.**

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go" />
  <img src="https://img.shields.io/badge/TypeScript-5.6+-3178C6?style=flat-square&logo=typescript&logoColor=white" alt="TypeScript" />
  <img src="https://img.shields.io/badge/Next.js-16+-black?style=flat-square&logo=next.js&logoColor=white" alt="Next.js" />
  <img src="https://img.shields.io/badge/React-18.3-61DAFB?style=flat-square&logo=react&logoColor=black" alt="React" />
  <img src="https://img.shields.io/badge/Tailwind-CSS_3.4-38B2AC?style=flat-square&logo=tailwind-css&logoColor=white" alt="Tailwind CSS" />
  <img src="https://img.shields.io/badge/Python-3.11+-3776AB?style=flat-square&logo=python&logoColor=white" alt="Python" />
  <img src="https://img.shields.io/badge/PostgreSQL-16-336791?style=flat-square&logo=postgresql&logoColor=white" alt="PostgreSQL" />
  <img src="https://img.shields.io/badge/pgvector-Enabled-336791?style=flat-square" alt="pgvector" />
  <img src="https://img.shields.io/badge/Docker-Compose-2496ED?style=flat-square&logo=docker&logoColor=white" alt="Docker" />
</p>

</div>

---

Lynk is a campus collaboration and work platform connecting verified university students with projects, modular workspaces, team roles, and peer reviews. Built around a Unified Campus Member model, any verified student can create project workspaces, invite teammates, and submit direct applications under a single academic identity without role silos or commercial fees.

---

## Core Invariants

- **Unified Campus Member Model:** Single identity across the platform. Any member can publish project workspaces and submit applications under one verified account.
- **Institutional (.edu) Verification Gate:** Registration mandates an institutional university `.edu` email address. Sensitive marketplace actions (creating projects, sending invitations, applying, uploading resumes) require email confirmation.
- **Authoritative Go Backend Boundary:** The Go REST API (`:8080`) is the sole authority for authentication, authorization, business rules, transactional state machines, and public APIs.
- **Defense-in-Depth AI Subsystem:** A containerized FastAPI AI service (`ai-api:8000`) on an internal network executes ML, vector search, and ranking pipelines, secured via `X-Internal-AI-Secret`.
- **Graceful Degradation Everywhere:** All AI operations (semantic search, ranking, draft generation, recommendations, moderation, analytics) automatically fall back to deterministic SQL logic if the AI service is unreachable.
- **Authoritative Non-Mutation:** AI endpoints return advisory draft payloads and never mutate authoritative database tables directly.
- **Self-Hosted IAM (SuperTokens Core):** Session management, credential handling, and email verification run on-premise in Docker on port `3567`.
- **Dense Semantic Vectors (pgvector):** PostgreSQL 16 stores 384-dimensional dense vectors (`all-MiniLM-L6-v2`) indexed with HNSW cosine distance for natural language discovery.
- **Dedicated Object Storage (MinIO S3):** Student resumes are stored exclusively in MinIO S3 (`resumes` bucket); PostgreSQL stores only object keys and file metadata.
- **Containerization Boundary:** Go API, AI API, AI Worker, SuperTokens Core, PostgreSQL, and MinIO run in Docker Compose. The Next.js frontend runs on the host machine for instant Hot Module Replacement (HMR).
- **Unified Control Plane:** A root `Makefile` standardizes onboarding, dependency installation, and quality gates across Go, TypeScript, and Python.

---

## System Topology

```text
+------------------------------------------------------------------------+
|                        HOST ENVIRONMENT (DEVELOPMENT)                  |
|                                                                        |
|   Next.js 16 (App Router + React 18.3 + TypeScript 5.6 + Tailwind 3.4) |
|   URL: http://localhost:3000                                           |
|   Auth: supertokens-web-js (Session, EmailPassword, EmailVerification) |
+------------------------------------------------------------------------+
                    |                                |
                    | 1. Authentication & Sessions   | 2. REST API Requests
                    |    http://localhost:8080/api/v1|    http://localhost:8080
                    v                                v
+------------------------------------------------------------------------+
|                     DOCKER COMPOSE NETWORK (lynk-net)                  |
|                                                                        |
|   +-------------------------------+ +------------------------------+   |
|   |  SuperTokens Core             | |  Go REST API (Authoritative) |   |
|   |  (Container: lynk-supertokens)| |  (Container: lynk-api)       |   |
|   |  Port: 3567                   | |  Port: 8080                  |   |
|   +-------------------------------+ +------------------------------+   |
|                   |                       |             |      |       |
|                   |                       | X-Secret    | SQL  | S3    |
|                   |                       v             v      v       |
|                   |         +-------------------------+ |  +---------+ |
|                   |         | FastAPI AI Backend      | |  | MinIO   | |
|                   |         | (Container: ai-api)     | |  | (resumes| |
|                   |         | Port: 8000 (Internal)   | |  | :9000)  | |
|                   |         +-------------------------+ |  +---------+ |
|                   |                       |             |              |
|                   |                       v             |              |
|                   |         +-------------------------+ |              |
|                   |         | AI Background Worker    | |              |
|                   |         | (Container: ai-worker)  | |              |
|                   |         | SKIP LOCKED Queue Daemon| |              |
|                   |         +-------------------------+ |              |
|                   |                       |             |              |
|                   | SQL                   v SQL         |              |
|                   v                       v             v              |
|   +----------------------------------------------------------------+   |
|   |  PostgreSQL 16 + pgvector (Container: lynk-postgres:5432)      |   |
|   |  Databases: lynk_db (Tables + vector(384)), supertokens_db     |   |
|   +----------------------------------------------------------------+   |
+------------------------------------------------------------------------+
```

---

## Technology Stack

| Layer | Component | Description |
| :--- | :--- | :--- |
| **Control Plane** | Root `Makefile` | Multi-language onboarding, Git hook installation, and unified test/lint gates |
| **Gateway & Reverse Proxy** | Nginx (`lynk-gateway`:8080) | Port 8080 entrypoint, `least_conn` load balancing, keep-alive pooling (64), SSE bypass |
| **Frontend** | Next.js 16 (App Router) | React 18.3, TypeScript 5.6, Tailwind CSS 3.4, TanStack Query 5 (host-executed) |
| **Authoritative Backend** | Go 1.25+ | Chi router v5.3, pgx/v5, SuperTokens SDK, MinIO Go SDK (Dockerised, scalable) |
| **Caching & Rate Limiting** | Redis 7 (`lynk-redis`:6379) | 120 req/min sliding-window limiter & cache-aside store for project feeds |
| **Connection Pooling** | PgBouncer (`lynk-pgbouncer`:6432) | Transaction mode multiplexing 2,000 client connections onto 25-40 PostgreSQL backends |
| **Intelligent Autoscaling** | Compose Controller (`lynk-autoscaler`) | Single-host dynamic scaling (2-6 API replicas) with connection-aware fast-path triggers |
| **AI Backend** | Python 3.11 + FastAPI | PyTorch, sentence-transformers, scikit-learn, vLLM client (Dockerised) |
| **Background Worker** | Python 3.11 (`ai-worker`) | Concurrency-safe queue worker (PostgreSQL SKIP LOCKED + exponential backoff) |
| **Database** | PostgreSQL 16 + pgvector | Relational schema + 384-dim vector storage with HNSW cosine distance indexing |
| **IAM & Auth** | SuperTokens Core 9.3 | Self-hosted session management, EmailPassword, EmailVerification recipes |
| **Object Storage** | MinIO | S3-compatible object storage dedicated exclusively to student resumes |
| **Orchestration** | Docker Compose | Multi-container environment (Gateway, API, AI API, AI Worker, DB, IAM, MinIO) |

---

## Repository Structure

```text
Lynk/
+-- Makefile                    # Universal developer control plane (setup, test, lint, format)
+-- docker-compose.yml          # Postgres (pgvector), MinIO, SuperTokens, Go API, AI API, AI Worker
+-- .env.example                # Canonical environment variable template
+-- .pre-commit-config.yaml     # Universal Git hook suite (Prettier, Ruff, Go fmt, whitespace)
+-- ai/                         # FastAPI AI Backend & Async Workers
|   +-- Dockerfile              # Multi-stage Python 3.11 container build
|   +-- requirements.txt        # PyTorch, sentence-transformers, asyncpg, scikit-learn
|   +-- app/                    # FastAPI entrypoint, config, health probes, routers
|   |   +-- api/                # Internal routes (search, ranking, skills, jobs, analytics)
|   |   +-- middleware/         # X-Internal-AI-Secret, correlation tracing, run_tracker
|   +-- models/embeddings/      # Singleton EmbeddingProvider & content hashing
|   +-- pipelines/              # ML pipelines (search, ranking, moderation, reviews, forecasting)
|   +-- llm/                    # vLLM client abstraction, prompts, schemas
|   +-- workers/                # SKIP LOCKED queue daemon & background handlers
|   +-- evaluation/             # Offline IR metrics (NDCG, MRR, F1) & benchmark harness
|   +-- tests/                  # 115 pytest test cases across all AI subsystems
+-- backend/                    # Authoritative Go REST API
|   +-- Dockerfile              # Multi-stage production container build
|   +-- cmd/api/main.go         # Dependency injection, router setup, server entrypoint
|   +-- internal/
|   |   +-- ai/                 # Go AI client, orchestrator, public handlers, project embeddings
|   |   +-- auth/               # SuperTokens SDK initialization & session claims
|   |   +-- user/               # Campus member profiles and resume handling
|   |   +-- project/            # Project workspace management & visibility
|   |   +-- team/               # Team members, role transitions, and invitations
|   |   +-- recruitment/        # Project applications and applicant review
|   |   +-- workspace/          # Modular section registries and external resources
|   |   +-- job/                # Job postings, filtering, and search (legacy)
|   |   +-- application/        # Proposal applications (legacy)
|   |   +-- contract/           # Deliverable contract state machine (legacy)
|   |   +-- review/             # Post-completion ratings and reviews
|   |   +-- storage/            # MinIO S3 client wrapper
|   |   +-- database/           # Connection pooling (pgxpool) and raw SQL migrator
|   |   +-- middleware/         # SuperTokens session auth, CORS, logging, recovery
|   +-- migrations/             # Incremental SQL migration files (000001 - 000022)
+-- frontend/                   # Next.js App Router frontend (runs on host)
|   +-- src/
|   |   +-- app/                # App Router pages ((auth), projects, activity, profile)
|   |   +-- components/         # Reusable UI components, ProjectCard, TeamManagementModal
|   |   +-- lib/                # API client, SuperTokens helpers, project-api
|   |   +-- types/              # TypeScript interfaces matching backend models
+-- docs/                       # Comprehensive documentation suite, API specs, and ERDs
+-- scripts/                    # Automated smoke test suites (PowerShell and Bash)
```

---

## Local Development Setup

You can set up the Lynk development environment using either the **Root Makefile** (recommended for automated multi-language bootstrapping) or manually via **Individual CLI Commands**.

### Prerequisites

- **Docker and Docker Compose** (v24+)
- **Go** (v1.22+, recommended: 1.25.x)
- **Node.js** (v20+) and **npm**
- **Python** (v3.11+)
- **Git** (v2.40+)

---

### Option A: Quickstart via Root Makefile (Recommended)

The top-level `Makefile` automates environment variable copying, Git hook installation, Go module verification, frontend dependencies, and Python virtual environment creation in a single command.

#### 1. One-Command Setup

```bash
# Clone the repository
git clone https://github.com/BIJJUDAMA/Lynk.git
cd Lynk

# Initialize all dependencies and Git hooks
make setup
```

This single command:
1. Creates `.env` from `.env.example` if not already present.
2. Installs and activates Git pre-commit hooks via `pre-commit install`.
3. Downloads and verifies backend Go modules in `backend/`.
4. Installs frontend npm packages (`npm install`) in `frontend/`.
5. Creates a Python virtual environment (`ai/.venv`) and installs `ai/requirements.txt`.

#### 2. Start Backend Infrastructure

```bash
docker compose up -d --build
```

#### 3. Launch Frontend on Host

```bash
cd frontend
npm run dev
```

Visit `http://localhost:3000` in your browser.

#### Targeted Makefile Setup Targets

If you only need to bootstrap a specific subsystem:

```bash
make setup-hooks      # Link pre-commit hooks into .git/hooks/pre-commit
make setup-go         # Download and verify Go modules
make setup-frontend   # Install npm packages in frontend/
make setup-ai         # Create Python venv in ai/.venv and install dependencies
```

---

### Option B: Manual Setup Step-by-Step (Without Makefile)

Follow these manual steps if you prefer direct command execution without `make`:

#### Step 1: Environment Configuration

```bash
cp .env.example .env
```

#### Step 2: Install Git Pre-Commit Hooks

Activate local Git hooks so code formatting (Prettier, Ruff, Go fmt) runs automatically before each commit:

```bash
pre-commit install
```

#### Step 3: Authoritative Backend (Go) Dependencies

Download and verify backend Go module dependencies:

```bash
cd backend
go mod download
go mod verify
cd ..
```

#### Step 4: Python AI Subsystem Dependencies

Create a local virtual environment for the AI service and install requirements:

```bash
# Linux / macOS:
python3 -m venv ai/.venv
source ai/.venv/bin/activate
pip install -r ai/requirements.txt
deactivate

# Windows (PowerShell):
python -m venv ai\.venv
.\ai\.venv\Scripts\Activate.ps1
pip install -r ai\requirements.txt
deactivate
```

#### Step 5: Start Container Infrastructure

Launch PostgreSQL (`pgvector`), SuperTokens Core, MinIO, the Go REST API, FastAPI AI Backend, and the AI Worker:

```bash
docker compose up -d --build
```

Verify that all containers are healthy:

```bash
docker compose ps
```

Service access registry:
- **Go REST API (Authoritative):** `http://localhost:8080`
- **FastAPI AI Backend (Internal):** `http://localhost:8000`
- **SuperTokens Core IAM:** `http://localhost:3567`
- **MinIO S3 API:** `http://localhost:9000`
- **MinIO Web Console:** `http://localhost:9001` (User: `minio_admin`, Password: `minio_password`)
- **PostgreSQL 16 + pgvector:** `localhost:5432` (Database: `lynk_db`, User: `lynk_user`)

#### Step 6: Start Frontend Development Server

Install dependencies and start the Next.js server on the host machine:

```bash
cd frontend
npm install
npm run dev
```

Open `http://localhost:3000` in your browser.

---

## Unified Developer Control Plane (Makefile)

The root `Makefile` provides unified targets for cross-stack verification, linting, and formatting:

| Target | Description | Subsystems Executed |
| :--- | :--- | :--- |
| `make setup` | Complete onboarding (hooks, Go modules, npm dependencies, Python venv) | All |
| `make setup-hooks` | Installs Git pre-commit hook into `.git/hooks/` | Git |
| `make setup-go` | Downloads and verifies backend Go modules | Backend (`backend/`) |
| `make setup-frontend` | Installs npm dependencies | Frontend (`frontend/`) |
| `make setup-ai` | Creates Python venv and installs requirements | AI (`ai/`) |
| `make test` | Runs unit test suites across Go, frontend, and AI | All |
| `make lint` | Runs `golangci-lint`, `oxlint`, and `ruff check` | All |
| `make format` | Auto-formats Go (`gofmt`), frontend (`prettier`), and Python (`ruff`) | All |
| `make format-check` | Enforces formatting compliance (identical to CI) | All |
| `make load-test` | Runs complete 8-stage sequential k6 backend load test suite | Backend & Database |
| `make load-test-stress` | Runs Stage 5 high-concurrency stress test (1,000 VUs) | Backend & Database |

---

## API Endpoints Overview

All public REST API endpoints are served by the authoritative Go API on port `8080` and prefixed with `/api/v1`.

| Domain | Method | Path | Auth Required | Description |
| :--- | :--- | :--- | :---: | :--- |
| **Auth** | POST | `/auth/*` | Public | Handled by SuperTokens Core via mounted middleware |
| **Profile** | GET | `/profile/me` | Session | Retrieve current campus member profile |
| **Profile** | PUT | `/profile/me` | Session | Update bio, skills, department, graduation year |
| **Profile** | GET | `/profile/recommendations` | Verified .edu | Intelligent profile completeness and skill advice |
| **Resume** | POST | `/profile/resume` | Verified .edu | Upload resume PDF/DOCX to MinIO (5MB limit) |
| **Resume** | GET | `/profile/resume` | Session | Download member resume via presigned S3 URL |
| **Projects** | GET | `/projects` | Public | List and filter projects by category, skills, recruitment |
| **Projects** | POST | `/projects` | Verified .edu | Create a new project workspace |
| **Projects** | GET | `/projects/{slugOrId}` | Public | Detailed view of project workspace and sections |
| **Projects** | PATCH | `/projects/{id}` | Lead / Maintainer | Update project title, description, category |
| **Projects** | PATCH | `/projects/{id}/visibility` | Lead / Maintainer | Update visibility (`public`, `unlisted`, `private`) |
| **Team** | GET | `/projects/{id}/members` | Public | List team members and assigned roles |
| **Team** | PATCH | `/projects/{id}/members/{mId}` | Lead / Maintainer | Adjust member role (Lead, Maintainer, Member) |
| **Team** | DELETE | `/projects/{id}/members/{mId}` | Lead / Maintainer | Remove member from project team roster |
| **Team** | POST | `/projects/{id}/invitations` | Lead / Maintainer | Invite university student via institutional `.edu` email |
| **Team** | POST | `/invitations/{id}/accept` | Verified .edu | Accept team invitation and join workspace |
| **Sections** | GET | `/projects/{id}/sections` | Public | Retrieve ordered modular workspace sections |
| **Sections** | PUT | `/projects/{id}/sections` | Lead / Maintainer | Update and re-order modular workspace sections |
| **Resources** | GET | `/projects/{id}/resources` | Public | List external project resources and documentation links |
| **Resources** | POST | `/projects/{id}/resources` | Lead / Maintainer | Add project documentation link or external resource |
| **Resources** | DELETE | `/projects/{id}/resources/{rId}` | Lead / Maintainer | Delete external resource link |
| **Recruitment** | POST | `/projects/{id}/applications` | Verified .edu | Submit project recruitment application with resume |
| **Recruitment** | GET | `/projects/{id}/applications` | Lead / Maintainer | Review received applications and applicant notes |
| **Recruitment** | GET | `/applications/me` | Session | List personal applications submitted by current user |
| **Recruitment** | PATCH | `/applications/{id}/status` | Lead / Maintainer | Accept (adds to team roster), shortlist, or reject |
| **Search** | GET | `/search/projects?q=...` | Public | Hybrid semantic + keyword project discovery |
| **Search** | GET | `/search/people?q=...` | Verified .edu | Hybrid semantic + keyword student directory discovery |
| **Reviews** | GET | `/users/{id}/reviews` | Public | View aggregated bilateral peer ratings and reviews |
| **AI Insights** | GET | `/users/{id}/ai-insights` | Public | Aspect breakdown (technical, timeliness, communication) |
| **AI Analytics** | GET | `/analytics/skills` | Public | Campus skill demand metrics and statistical growth forecasts |

---

## Verification & Quality Assurance

You can verify the entire monorepo using the root `Makefile` or through individual toolchain commands.

### Via Root Makefile (Recommended)

```bash
# Run all test suites across Go, Frontend, and Python AI
make test

# Run all linters (golangci-lint, oxlint, ruff check, typecheck)
make lint

# Check code formatting compliance (Go fmt, Prettier, Ruff)
make format-check

# Auto-format all code
make format
```

### Via Direct Toolchain Commands

#### Backend Verification (Go)
```bash
cd backend
go test -v -race ./...
go vet ./...
```

#### Frontend Verification (Next.js & TypeScript)
```bash
cd frontend
npm test
npm run typecheck
npm run lint:ox
npm run format:check
npm run build
```

#### Python AI Subsystem Verification
```bash
python -m pytest ai/tests/ -v
python -m ai.evaluation.harness
```

#### End-to-End Multi-Container Smoke Tests
```bash
# Linux / macOS:
bash scripts/smoke-test.sh

# Windows (PowerShell):
pwsh scripts/smoke-test.ps1
```

---

## Performance & Scale Benchmarks

Lynk includes an automated, protocol-level load testing suite implemented in [k6](https://k6.io/) to continuously validate system performance under ultra-concurrency campus traffic. For detailed architectural diagrams, capacity formulas, and complete latency distributions, see:

> [**Performance, Concurrency & Scale Specification**](docs/architecture/performance-and-scale.md)

### Key Performance Indicators (Audited Single-Server Baseline)

| Metric | Measured Value | Operational Context |
| :--- | :---: | :--- |
| **Sustainable Throughput (Zone 1)** | **2,042.3 req/s** | Verified Green-Zone SLA with **11.4ms p95 latency** and **0.00% errors** |
| **Saturation Ingestion Ceiling (Zone 2)** | **3,193.4 req/s** | Maximum server ingestion throughput before queueing latency inflection |
| **Concurrent Connection Hold (Zone 3)** | **20,000 VUs** | Persistent keep-alive sockets sustained with **0.00% socket drops** |
| **DDoS Rate Limiting** | **1.80ms drop** | 120 req/min single-IP burst strictly blocked with **HTTP 429 Too Many Requests** |
| **Real-World Campus Scale** | **~20,400 to 30,600 users** | Active campus students browsing with realistic 10s-15s browse/think time |
| **Dynamic Autoscaling** | **2 -> 6 Replicas** | Proactive connection-aware scale-up with 5s sustained response window |

### 8-Stage Load Benchmark Results

The consolidated benchmark suite (`node tests/load/run_all.js` or `make load-test`) executes 8 sequential load stages evaluating rate limiting, connection pooling, user breakpoint, saturation ceiling, and connection hold:

| Stage | Total Reqs | Throughput | p95 Latency | p99 Latency | Error Rate | Peak CPU | Peak RAM | Status |
| :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :--- |
| **01. Rate Limiting (DDoS Burst)** | 300 | 30.0 req/s | **1.8ms** | 3.3ms | 100.00% | 30.8% | 662 MB | **PASS (Limiter Enforced)** |
| **02. Cache & Read Benchmark** | 7,131 | 177.1 req/s | **1.2ms** | 2.2ms | 0.00% | 72.7% | 1,184 MB | **PASS** |
| **03. DB Pool Saturation** | 6,351 | 157.9 req/s | **675us** | 785us | 0.00% | 15.6% | 1,165 MB | **PASS** |
| **04. Max User Breakpoint** | 4,866 | 48.5 req/s | **2.5ms** | 4.0ms | 0.00% | 41.6% | 1,177 MB | **PASS** |
| **05. High-Concurrency Stress** | 24,688 | 295.3 req/s | **2.0ms** | 2.7ms | 0.00% | 87.4% | 1,217 MB | **PASS (1,000 VUs)** |
| **06. Ultra-Concurrency Stress** | 119,744 | 1,464.1 req/s | **2.2ms** | 3.5ms | 0.00% | 261.2% | 1,242 MB | **PASS (2,500 VUs)** |
| **07. Constant-Arrival Saturation** | 335,340 | 3,193.4 req/s | **755.8ms** | 1.01s | 0.00% | 611.8% | 1,908 MB | **WARN (Latency High)** |
| **08. Concurrent Connection Hold** | 254,558 | 2,042.3 req/s | **11.4ms** | 55.3ms | 0.00% | 368.2% | 1,648 MB | **PASS (20,000 VUs)** |

### Certified Capacity Operating Zones

1. **Zone 1: Certified Sustainable Operating Capacity (Green Zone)**
   - **Throughput:** `2,042.3 req/sec` | **Latency SLA:** `p95 = 11.4ms` | **Error Rate:** `0.00%`
   - **Supported Campus Population:** **~20,400 to 30,600 simultaneous active students** (assuming 10s to 15s pause between clicks).
2. **Zone 2: Saturation Ingestion Ceiling (Yellow Zone - Stress Limit)**
   - **Ingestion Peak:** `3,193.4 req/sec` | **Latency Profile:** `p95 = 755.8ms` (Request queueing backpressure).
   - **Operational Limit:** Production workloads must remain below 50% of this ceiling (~1,600 req/s).
3. **Zone 3: Connection Concurrency Limit**
   - **Simultaneous Keep-Alive Sockets:** **20,000 concurrent sockets** sustained with zero drops.

### Production Hardware Sizing

| Tier | vCPU | RAM | Storage (NVMe SSD) | Campus Scale Supported |
| :--- | :---: | :---: | :---: | :--- |
| **Recommended (Sweet Spot)** | **8 vCPU** | **16 GB - 32 GB** | **250 GB - 500 GB** | 20,000 - 40,000 students (sub-10ms latency, 50% CPU headroom) |
| **Minimum Viable** | **4 vCPU** | **8 GB** | **50 GB** | Up to 15,000 students (~1,000 RPS sustained; stack uses ~3.8 GB RAM) |

#### Executing Load Tests

```bash
# Run the complete 8-stage sequential load testing suite
make load-test

# Inspect real-time controller scaling logs during load
docker compose logs -f autoscaler

# Filter execution to an individual stage
node tests/load/run_all.js --stage=01_rate_limit
node tests/load/run_all.js --stage=06_ultra_concurrency_stress
node tests/load/run_all.js --stage=08_connection_hold
```

---

## Security & Operational Invariants

1. **Strict .edu Validation:** Registrations with non-.edu email domains are rejected immediately with HTTP 400.
2. **Email Verification Gate:** Creating projects, inviting members, applying, uploading resumes, and generating AI drafts require confirmed institutional email ownership. Unverified attempts return HTTP 403 (`EMAIL_NOT_VERIFIED`).
3. **Authoritative Go Boundary:** The Go backend is the sole authority for authentication, authorization, domain validation, and transactional state. FastAPI is strictly an internal compute engine.
4. **Defense-in-Depth AI Secret:** The FastAPI AI backend requires `X-Internal-AI-Secret` on all internal routes. It is never exposed directly to public web traffic.
5. **Authoritative Non-Mutation:** Generative AI endpoints return draft payloads and never mutate authoritative database rows directly.
6. **Graceful Degradation Everywhere:** If the AI backend or GPU is unreachable, all Go endpoints seamlessly fall back to deterministic SQL logic with HTTP 200.
7. **Circuit Breaker Protection:** The Go AI orchestrator trips after 5 consecutive failures for 30s, fast-bypassing to heuristics without blocking requests.
8. **Deterministic Database Migrations:** All migrations in `backend/migrations/` (`000001` through `000022`) are immutable and sequential.
9. **Resumes in MinIO Exclusively:** Resumes are never stored on local disk or inside PostgreSQL. All resume transfers stream directly through MinIO with short-lived presigned URLs for client downloads.
10. **Concurrency & Fairness:** Candidate ranking excludes protected demographic attributes (gender, race, age, graduation year, ethnicity) for algorithmic fairness.
