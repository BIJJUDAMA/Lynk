# Local Development & Testing Guide - Lynk

> Complete runbook for running, testing, and developing Lynk locally.
> Co-locates full-stack Docker orchestration, hybrid host development, pre-configured test personas, and verification procedures.

---

## 1. Quick Orientation & Prerequisites

Before starting, ensure your workstation has:
- **Docker & Docker Compose** (v24+ / Compose v2.20+)
- **Node.js 20+ / npm 10+** (only required if running frontend on host via Option 2)
- **PowerShell** (Windows) or **Bash** (macOS / Linux)

---

## 2. Option 1: Full-Stack Docker (Recommended - 1 Single Command)

This spins up the entire platform (PostgreSQL, SuperTokens Core, MinIO, Redis, Go API, AI Subsystem, and the Next.js Frontend) in optimized production-grade containers.

### Step 1: Start the Entire Stack

From the repository root (`Lynk/`), run:

```bash
docker compose up -d --build
```

### Step 2: Verify All Services Are Healthy

Run our synthetic health probe to check all 8 platform endpoints:

**PowerShell (Windows):**
```powershell
powershell -ExecutionPolicy Bypass -File scripts/health-probe.ps1
```

**Bash (Linux / macOS):**
```bash
chmod +x scripts/health-probe.sh
./scripts/health-probe.sh
```

All 8 services should report `[PASS]`:
1. PostgreSQL (`5432`)
2. PgBouncer (`6432`)
3. SuperTokens Core (`3567`)
4. MinIO S3 (`9000`)
5. Redis (`6379`)
6. Go REST API (`8080`)
7. FastAPI AI Subsystem (`8000`)
8. Next.js Frontend (`3000`)

### Step 3: Open in Your Browser

| Service | Local URL | Credentials / Notes |
| :--- | :--- | :--- |
| **Web App (Next.js)** | `http://localhost:3000` | Landing page, discovery, activity |
| **Project Workspaces** | `http://localhost:3000/projects` | Browse and filter campus projects and roles |
| **Auth & Registration** | `http://localhost:3000/register` | Requires valid institutional `.edu` email |
| **Login** | `http://localhost:3000/login` | Direct login with credentials |
| **MinIO S3 Console** | `http://localhost:9001` | Username: `minio_admin`<br/>Password: `minio_password` |
| **Gateway / Go API** | `http://localhost:8080/health` | High-performance Nginx reverse proxy & Go API health |
| **Prometheus Metrics** | `http://localhost:8080/metrics` | Prometheus scrape target |
| **FastAPI AI Docs** | `http://localhost:8000/docs` | Swagger documentation for AI endpoints |

### Step 4: Useful Operational Commands

```bash
# View live logs for any service:
docker compose logs -f api
docker compose logs -f frontend
docker compose logs -f ai-api

# View running container statuses:
docker compose ps

# Tear down the stack:
docker compose down
```

---

## 3. Option 2: Hybrid Local Dev Mode (Fast Hot Module Reloading)

If you plan on editing frontend code and want instant Next.js Hot Module Reloading (HMR) without rebuilding Docker images on each edit:

### Step 1: Start Backend & Infrastructure in Docker

From repository root:

```bash
docker compose up -d postgres supertokens minio redis api ai-api
```

### Step 2: Start Next.js on Your Host

Open a second terminal window:

```bash
cd frontend
npm install
npm run dev
```

Visit `http://localhost:3000` in your browser. All API requests from the frontend will proxy to `http://localhost:8080` (or your configured `NEXT_PUBLIC_API_URL`).

---

## 4. One-Command Onboarding with Root Makefile

The repository includes a top-level `Makefile` to automate environment setup, git hooks, and verification across Go, TypeScript, and Python:

```bash
# Run complete multi-service setup (hooks, Go modules, npm packages, Python venv):
make setup

# Or run individual setup targets:
make setup-hooks      # Installs pre-commit hook into .git/hooks/pre-commit
make setup-go         # Downloads and verifies Go modules in backend/
make setup-frontend   # Installs npm dependencies in frontend/
make setup-ai         # Creates ai/.venv and installs requirements.txt
```

---

## 5. Pre-Configured Test Personas & Accounts

The database seed migration (`000010_seed_test_campus_members.up.sql`) provisions three pre-configured campus member accounts in `supertokens_db` and `lynk_db`:

| Account / Email | Password | Email Verification | Primary Purpose & Profile Persona |
| :--- | :--- | :--- | :--- |
| **`poster@campus.edu`** | `password123` | **Verified** | **Project Lead / Organizer Persona**<br/>**Profile:** Campus Poster (Computer Science, Class of 2026, Lab Organizer). Use this account to create projects, review applications, and manage workspace rosters. |
| **`applicant@campus.edu`** | `password123` | **Verified** | **Student Contributor Persona**<br/>**Profile:** Campus Applicant (Software Engineering, Class of 2027, Skills: Go, TypeScript, React, PostgreSQL). Use this account to discover projects, submit applications, attach resumes, and collaborate in workspaces. |
| **`unverified@campus.edu`** | `password123` | **Unverified** | **Fresh / Gate-Testing Persona**<br/>**Profile:** New Student (Design, Class of 2028). Used to test trust gate enforcement -- verifies that unverified students cannot apply to projects or upload resumes until completing email verification. |

---

## 6. Creating Your Own Custom Test Account

You can also register any new account directly via the UI:

1. Navigate to `http://localhost:3000/register` (or click **Sign in with .edu** -> **Sign up**).
2. Enter any institutional email ending in `.edu` (e.g., `alice@berkeley.edu`, `john@mit.edu`, `dev@stanford.edu`).
3. Set any password meeting standard criteria (minimum 8 characters, at least 1 number).
4. Complete initial profile setup (name, department, graduation year, skills).

---

## 7. Running Test Suites & Code Quality Checks Locally

You can run the entire verification suite via `make` or standard CLI commands:

### Via Makefile (Unified)

```bash
make test           # Runs backend, frontend, and AI test suites
make lint           # Runs golangci-lint, oxlint, and ruff check
make format         # Formats Go (gofmt), frontend (prettier), and Python (ruff)
make format-check   # Checks formatting across all services (identical to CI)
make load-test      # Executes 5-stage k6 load test suite (up to 1,000 VUs)
make load-test-ultra # Executes 6-stage ultra-scale k6 suite (up to 2,500 VUs)
```

### Via Direct CLI Commands

```bash
# Backend (Go):
cd backend
go test ./...
go vet ./...

# Frontend (Next.js / TypeScript):
cd frontend
npm run format:check
npm run lint:ox
npm run typecheck
npm test

# AI Subsystem (Python):
python -m ruff check ai/
python -m mypy --config-file ai/pyproject.toml ai/
python -m pytest ai/tests/ -v
```
