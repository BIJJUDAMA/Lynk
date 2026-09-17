# Lynk: High-Trust Student Freelance and Campus Gig Marketplace

Lynk is a university-centric freelance and campus gig platform designed to connect verified students, faculty, and campus organizations. Built around a Unified Campus Member architecture, Lynk eliminates rigid role silos: every verified member can publish opportunities, apply to existing gigs, upload resumes, and manage contracts and reviews.

---

## Architectural Invariants

- Unified Campus Member Model: Single user identity for all participants. Any member can act as both an opportunity organizer and a contributor without role switching or multiple accounts.
- Strict Institutional (.edu) Verification Gate: Registration requires a valid institutional .edu email address. Sensitive marketplace operations (posting jobs, submitting applications, uploading resumes) require verified email status.
- Authoritative Go Backend Boundary: The Go backend is the sole authority for authentication, authorization, business rules, transactional domain state machines, and public APIs.
- Defense-in-Depth AI Subsystem: An internal FastAPI AI backend (`ai-api:8000`) on `lynk-net` handles ML/embedding/LLM pipelines, protected by `X-Internal-AI-Secret`. It is never exposed directly to public browsers.
- Graceful Degradation: All AI capabilities (search, ranking, draft generation, recommendations, moderation, analytics) gracefully fall back to deterministic SQL/heuristic logic if the AI backend is unreachable.
- Authoritative Non-Mutation: Generative AI endpoints return draft payloads and never mutate authoritative database rows directly.
- Self-Hosted IAM (SuperTokens Core): Session tokens, credential authentication, and email verification are managed on-premise using SuperTokens Core backed by PostgreSQL.
- Dense Semantic Vectors (pgvector): PostgreSQL 16 is equipped with `pgvector` storing 384-dimensional dense vectors (`all-MiniLM-L6-v2`) indexed with HNSW cosine distance.
- Dedicated Object Storage (MinIO S3): Student resumes are stored exclusively in an S3-compatible MinIO bucket ("resumes"), keeping application database records lightweight.
- Layered Backend Architecture: The Go backend strictly follows Handler -> Service -> Repository separation using standard library primitives and lightweight routing.
- Containerization Boundary: The Go API, FastAPI AI Backend, AI Worker, SuperTokens Core, PostgreSQL, and MinIO run in Docker Compose. The Next.js frontend runs directly on the host machine for rapid development and instant Hot Module Replacement.

---

## System Topology

```text
+------------------------------------------------------------------------+
|                        HOST ENVIRONMENT (DEVELOPMENT)                  |
|                                                                        |
|   Next.js 14+ (App Router + TypeScript + Tailwind CSS)                 |
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
| Frontend | Next.js 14+ (App Router) | TypeScript, Tailwind CSS, supertokens-web-js (host-executed) |
| Authoritative Backend | Go 1.25 | Chi router, pgx/v5, SuperTokens SDK, MinIO Go SDK (Dockerised) |
| AI Backend | Python 3.11 + FastAPI | PyTorch, sentence-transformers, scikit-learn, vLLM client (Dockerised) |
| Background Worker | Python 3.11 (ai-worker) | Concurrency-safe queue worker (PostgreSQL SKIP LOCKED + backoff) |
| Database | PostgreSQL 16 + pgvector | Relational & 384-dim vector storage with HNSW indexing |
| IAM & Auth | SuperTokens Core 9.3 | Self-hosted session management, EmailPassword, EmailVerification |
| Object Storage | MinIO | S3-compatible object storage dedicated to student resumes |
| Orchestration | Docker Compose | Multi-container environment (API, AI API, AI Worker, DB, IAM, MinIO) |

---

## Repository Structure

```text
Lynk/
+-- docker-compose.yml          # Postgres (pgvector), MinIO, SuperTokens, Go API, AI API, AI Worker
+-- .env.example                # Canonical environment variable template
+-- ai/                         # FastAPI AI Backend & Async Workers
|   +-- Dockerfile              # Multi-stage Python 3.11 container build
|   +-- requirements.txt        # PyTorch, sentence-transformers, asyncpg, scikit-learn
|   +-- app/                    # FastAPI entrypoint, config, health probes, routers
|   |   +-- api/                # Internal routes (search, ranking, skills, jobs, analytics)
|   |   +-- middleware/         # X-Internal-AI-Secret, correlation tracing, run_tracker
|   +-- models/embeddings/      # Centralized EmbeddingProvider & content hashing
|   +-- pipelines/              # ML pipelines (search, ranking, moderation, reviews, forecasting)
|   +-- llm/                    # vLLM client abstraction, prompts, schemas
|   +-- workers/                # SKIP LOCKED queue daemon & background handlers
|   +-- evaluation/             # Offline IR metrics (NDCG, MRR, F1) & benchmark harness
|   +-- tests/                  # 100 pytest test cases across all AI subsystems
+-- backend/                    # Authoritative Go REST API
|   +-- Dockerfile              # Multi-stage production container build
|   +-- cmd/api/main.go         # Dependency injection, router setup, server entrypoint
|   +-- internal/
|   |   +-- ai/                 # Go AI client, orchestrator, public handlers, models
|   |   +-- auth/               # SuperTokens SDK initialization & session claims
|   |   +-- user/               # Campus member profiles and resume handling
|   |   +-- job/                # Job postings, filtering, and search
|   |   +-- application/        # Job applications and proposal management
|   |   +-- contract/           # Deliverable contract state machine
|   |   +-- review/             # Post-completion ratings and reviews
|   |   +-- storage/            # MinIO S3 client wrapper
|   |   +-- database/           # Connection pooling (pgxpool) and raw SQL migrator
|   |   +-- middleware/         # SuperTokens session auth, CORS, logging, recovery
|   +-- migrations/             # Incremental SQL migration files (000001 - 000012)
+-- frontend/                   # Next.js App Router frontend (runs on host)
|   +-- src/
|   |   +-- app/                # App Router pages ((auth), jobs, profile, activity)
|   |   +-- components/         # Reusable UI components & AuthProvider
|   |   +-- lib/                # API client, SuperTokens helpers, domain validation
|   |   +-- types/              # TypeScript interfaces matching backend models
+-- docs/                       # Architectural documentation, API specs, and ERDs
+-- scripts/                    # Automated smoke test suites (PowerShell and Bash)
```

---

## Local Development Setup

### Prerequisites

- Docker and Docker Compose (v20+)
- Go (v1.22+)
- Python (v3.11+)
- Node.js (v18+) and npm

### 1. Environment Configuration

Copy the example environment configuration:

```bash
cp .env.example .env
```

### 2. Start Core Infrastructure

Launch PostgreSQL (pgvector), SuperTokens Core, MinIO, the Go API, FastAPI AI Backend, and AI Worker in Docker:

```bash
docker compose up -d --build
```

Confirm that all services are healthy:

```bash
docker compose ps
```

Service access points:
- Go REST API (Authoritative): http://localhost:8080
- FastAPI AI Backend (Internal): http://localhost:8000
- SuperTokens Core: http://localhost:3567
- MinIO S3 API: http://localhost:9000
- MinIO Web Console: http://localhost:9001 (User: `minio_admin`, Password: `minio_password`)
- PostgreSQL + pgvector: localhost:5432 (Database: `lynk_db`, User: `lynk_user`)

### 3. Start Frontend Development Server

Install dependencies and start the Next.js dev server on the host:

```bash
cd frontend
npm install
npm run dev
```

Open http://localhost:3000 in your browser.

---

## API Endpoints Overview

All public REST API endpoints are prefixed with `/api/v1`.

| Domain | Method | Path | Auth Required | Description |
| :--- | :--- | :--- | :---: | :--- |
| Auth | POST | /auth/* | Public | Handled by SuperTokens Core via mounted middleware |
| Profile | GET | /profile/me | Session | Retrieve current campus member profile |
| Profile | PUT | /profile/me | Session | Update bio, skills, department, graduation year |
| Profile | GET | /profile/recommendations | Verified .edu | Intelligent profile completeness and skill advice |
| Resume | POST | /profile/resume | Verified .edu | Upload resume PDF to MinIO |
| Resume | GET | /profile/resume | Session | Download current member resume via presigned URL |
| Jobs | GET | /jobs | Public | Browse and search campus opportunities |
| Jobs | POST | /jobs | Verified .edu | Create a new opportunity posting |
| Jobs | GET | /jobs/{id} | Public | Get detailed view of an opportunity |
| Jobs | PUT | /jobs/{id} | Owner | Update an active job posting |
| Jobs | DELETE | /jobs/{id} | Owner | Cancel or close a job posting |
| AI Jobs | POST | /jobs/generate | Verified .edu | Generate structured job description draft via vLLM |
| AI Ranking | GET | /jobs/{id}/applicants/ranking | Job Creator | Advisory candidate ranking based on skills & semantics |
| Search | GET | /search/jobs?q=... | Public | Hybrid semantic + keyword job search |
| Search | GET | /search/people?q=... | Verified .edu | Hybrid semantic + keyword student member discovery |
| Applications | POST | /jobs/{id}/applications | Verified .edu | Submit proposal with attached resume |
| Applications | GET | /jobs/{id}/applications | Owner | Review submitted applications for posting |
| Applications | GET | /applications/mine | Session | List applications submitted by current user |
| Applications | PATCH | /applications/{id}/status | Owner | Accept or reject application (acceptance creates contract) |
| Contracts | GET | /contracts | Session | List contracts for authenticated user |
| Contracts | GET | /contracts/{id} | Participant | View contract details and status |
| Contracts | PATCH | /contracts/{id}/status | Participant | Transition contract state (Active -> Completed or Cancelled) |
| Reviews | POST | /contracts/{id}/reviews | Participant | Submit 1-5 star rating (unlocked upon contract completion) |
| Reviews | GET | /users/{id}/reviews | Public | View aggregated ratings and reviews for a member |
| AI Insights | GET | /users/{id}/ai-insights | Public | Aspect breakdown (technical, timeliness, communication) |
| AI Analytics| GET | /analytics/skills | Public | Campus skill demand metrics and statistical growth forecasts |

---

## Running Tests and Verification

### Backend Verification

Run the complete Go test suite with race detector and code vet:

```bash
cd backend
go test -v -race ./...
go vet ./...
```

### Python AI Subsystem Verification

Run all 100 unit, pipeline, worker, and evaluation tests:

```bash
python -m pytest ai/tests/ -v
```

### Offline Evaluation Suite & Benchmarks

Execute offline IR metrics (NDCG@K, MRR, Precision@K, F1):

```bash
python -c "from ai.evaluation.harness import EvaluationHarness; h = EvaluationHarness(); print(h.run_full_evaluation())"
```

### Frontend Verification

Execute frontend unit tests, TypeScript type checking, and production build:

```bash
cd frontend
npm test
npx tsc --noEmit
npm run lint
npm run build
```

### End-to-End Smoke Test

Run the full 14-phase lifecycle smoke test (validates .edu email rejection, account creation, email verification gate, job posting, application, contract transition, review, AI health, draft generation, candidate ranking, semantic search, and fallback degradation):

PowerShell (Windows):
```powershell
pwsh scripts/smoke-test.ps1
```

Bash (Linux / macOS):
```bash
bash scripts/smoke-test.sh
```

---

## Security and Operational Invariants

1. Strict .edu Validation: Registrations with non-.edu email domains are rejected immediately with HTTP 400.
2. Email Verification Gate: Posting opportunities, applying to jobs, uploading resumes, and generating AI drafts require confirmed institutional email ownership. Unverified attempts return HTTP 403 (EMAIL_NOT_VERIFIED).
3. Authoritative Go Boundary: The Go backend is the sole authority for authentication, authorization, domain validation, and transactional state. FastAPI is strictly an internal compute engine.
4. Defense-in-Depth AI Secret: The FastAPI AI backend requires `X-Internal-AI-Secret` on all internal routes. It is never exposed directly to public web traffic.
5. Authoritative Non-Mutation: Generative AI endpoints return draft payloads and never mutate authoritative database rows directly.
6. Graceful Degradation Everywhere: If the AI backend or GPU crashes or times out, all Go endpoints seamlessly fall back to deterministic SQL logic with HTTP 200.
7. Contract State Machine: Contracts strictly follow Draft -> Active -> Completed or Cancelled transitions. Reviews can only be submitted for completed contracts.
8. Resumes in MinIO: Resumes are never stored on local disk or inside PostgreSQL. All resume transfers stream directly through MinIO with short-lived presigned URLs for client downloads.
9. Concurrency & Fairness: Candidate ranking excludes protected demographic attributes (gender, race, age, graduation year, ethnicity) for algorithmic fairness.

