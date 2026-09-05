# Lynk: High-Trust Student Freelance and Campus Gig Marketplace

Lynk is a university-centric freelance and campus gig platform designed to connect verified students, faculty, and campus organizations. Built around a Unified Campus Member architecture, Lynk eliminates rigid role silos: every verified member can publish opportunities, apply to existing gigs, upload resumes, and manage contracts and reviews.

---

## Architectural Invariants

- Unified Campus Member Model: Single user identity for all participants. Any member can act as both an opportunity organizer and a contributor without role switching or multiple accounts.
- Strict Institutional (.edu) Verification Gate: Registration requires a valid institutional .edu email address. Sensitive marketplace operations (posting jobs, submitting applications, uploading resumes) require verified email status.
- Self-Hosted IAM (SuperTokens Core): Session tokens, credential authentication, and email verification are managed on-premise using SuperTokens Core backed by PostgreSQL.
- Dedicated Object Storage (MinIO S3): Student resumes are stored exclusively in an S3-compatible MinIO bucket ("resumes"), keeping application database records lightweight.
- Layered Backend Architecture: The Go backend strictly follows Handler -> Service -> Repository separation using standard library primitives and lightweight routing.
- Containerization Boundary: The Go API, SuperTokens Core, PostgreSQL, and MinIO run in Docker Compose. The Next.js frontend runs directly on the host machine for rapid development and instant Hot Module Replacement.

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
|   |  SuperTokens Core             | |  Go REST API                 |   |
|   |  (Container: lynk-supertokens)| |  (Container: lynk-api)       |   |
|   |  Port: 3567                   | |  Port: 8080                  |   |
|   +-------------------------------+ +------------------------------+   |
|                   |                         |              |           |
|                   |                         |              | S3 API    |
|                   | SQL (supertokens_db)    | SQL(lynk_db) | Port 9000 |
|                   v                         v              v           |
|   +-----------------------------------------+ +--------------------+   |
|   |  PostgreSQL 16+                         | |  MinIO Storage     |   |
|   |  (Container: lynk-postgres)             | |  (lynk-minio)      |   |
|   |  Port: 5432                             | |  Port: 9000 / 9001 |   |
|   |  Databases: lynk_db, supertokens_db     | |  Bucket: resumes   |   |
|   +-----------------------------------------+ +--------------------+   |
+------------------------------------------------------------------------+
```

---

## Technology Stack

| Layer | Component | Description |
| :--- | :--- | :--- |
| Frontend | Next.js 14+ (App Router) | TypeScript, Tailwind CSS, supertokens-web-js (host-executed) |
| Backend | Go 1.22+ | Chi router, pgx/v5, SuperTokens Go SDK, MinIO Go SDK (Dockerised) |
| Database | PostgreSQL 16+ | Relational persistence with raw SQL migrations (lynk_db, supertokens_db) |
| IAM & Auth | SuperTokens Core 9.2 | Self-hosted session management, EmailPassword, EmailVerification |
| Object Storage | MinIO | S3-compatible object storage dedicated to student resumes |
| Orchestration | Docker Compose | Local multi-container development environment |

---

## Repository Structure

```text
Lynk/
+-- docker-compose.yml          # Postgres, MinIO, SuperTokens Core, Go API
+-- .env.example                # Canonical environment variable template
+-- backend/                    # Go REST API
|   +-- Dockerfile              # Multi-stage production container build
|   +-- cmd/api/main.go         # Dependency injection, router setup, server entrypoint
|   +-- internal/
|   |   +-- auth/               # SuperTokens SDK initialization & session claims
|   |   +-- user/               # Campus member profiles and resume handling
|   |   +-- job/                # Job postings, filtering, and search
|   |   +-- application/        # Job applications and proposal management
|   |   +-- contract/           # Milestone/contract state machine
|   |   +-- review/             # Post-completion ratings and reviews
|   |   +-- storage/            # MinIO S3 client wrapper
|   |   +-- database/           # Connection pooling (pgxpool) and raw SQL migrator
|   |   +-- middleware/         # SuperTokens session auth, CORS, logging, recovery
|   +-- migrations/             # Incremental SQL migration files
+-- frontend/                   # Next.js App Router frontend (runs on host)
|   +-- src/
|   |   +-- app/                # App Router pages ((auth), jobs, profile, activity)
|   |   +-- components/         # Reusable UI components & AuthProvider
|   |   +-- lib/                # API client, SuperTokens helpers, domain validation
|   |   +-- types/              # TypeScript interfaces matching backend models
+-- docs/                       # Architectural documentation and ERDs
+-- scripts/                    # Automated smoke test suites (PowerShell and Bash)
```

---

## Local Development Setup

### Prerequisites

- Docker and Docker Compose (v20+)
- Go (v1.22+)
- Node.js (v18+) and npm

### 1. Environment Configuration

Copy the example environment configuration:

```bash
cp .env.example .env
```

### 2. Start Core Infrastructure

Launch PostgreSQL, SuperTokens Core, MinIO, and the Go API in Docker:

```bash
docker compose up -d --build
```

Confirm that all services are healthy:

```bash
docker compose ps
```

Service access points:
- Go REST API: http://localhost:8080
- SuperTokens Core: http://localhost:3567
- MinIO S3 API: http://localhost:9000
- MinIO Web Console: http://localhost:9001 (User: `minio_admin`, Password: `minio_password`)
- PostgreSQL: localhost:5432 (Database: `lynk_db`, User: `lynk_user`)

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

All REST API endpoints are prefixed with `/api/v1`.

| Domain | Method | Path | Auth Required | Description |
| :--- | :--- | :--- | :---: | :--- |
| Auth | POST | /auth/* | Public | Handled by SuperTokens Core via mounted middleware |
| Profile | GET | /profile/me | Session | Retrieve current campus member profile |
| Profile | PUT | /profile/me | Session | Update bio, skills, department, graduation year |
| Resume | POST | /profile/resume | Verified .edu | Upload resume PDF to MinIO |
| Resume | GET | /profile/resume | Session | Download current member resume via presigned URL |
| Jobs | GET | /jobs | Public | Browse and search campus opportunities |
| Jobs | POST | /jobs | Verified .edu | Create a new opportunity posting |
| Jobs | GET | /jobs/{id} | Public | Get detailed view of an opportunity |
| Jobs | PUT | /jobs/{id} | Owner | Update an active job posting |
| Jobs | DELETE | /jobs/{id} | Owner | Cancel or close a job posting |
| Applications | POST | /jobs/{id}/applications | Verified .edu | Submit proposal with attached resume |
| Applications | GET | /jobs/{id}/applications | Owner | Review submitted applications for posting |
| Applications | GET | /applications/mine | Session | List applications submitted by current user |
| Applications | PATCH | /applications/{id}/status | Owner | Accept or reject application (acceptance creates contract) |
| Contracts | GET | /contracts | Session | List contracts for authenticated user |
| Contracts | GET | /contracts/{id} | Participant | View contract details and status |
| Contracts | PATCH | /contracts/{id}/status | Participant | Transition contract state (Active -> Completed or Cancelled) |
| Reviews | POST | /contracts/{id}/reviews | Participant | Submit 1-5 star rating (unlocked upon contract completion) |
| Reviews | GET | /users/{id}/reviews | Public | View aggregated ratings and reviews for a member |

---

## Running Tests and Verification

### Backend Verification

Run the complete Go test suite with race detector and code vet:

```bash
cd backend
go test -v -race ./...
go vet ./...
```

### Frontend Verification

Execute frontend unit tests, TypeScript type checking, and production build:

```bash
cd frontend
npm test
npm run typecheck
npm run build
```

### End-to-End Smoke Test

Run the full lifecycle smoke test (validates .edu email rejection, account creation, email verification gate, job posting, application, contract transition, and review):

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
2. Email Verification Gate: Posting opportunities, applying to jobs, and uploading resumes require confirmed institutional email ownership. Unverified attempts return HTTP 403 (EMAIL_NOT_VERIFIED).
3. Contract State Machine: Contracts strictly follow Draft -> Active -> Completed or Cancelled transitions. Reviews can only be submitted for completed contracts.
4. Resumes in MinIO: Resumes are never stored on local disk or inside PostgreSQL. All resume transfers stream directly through MinIO with short-lived presigned URLs for client downloads.
