# Operations, Runbook, & Local Developer Guide - Lynk

> **Authoritative Operational Runbook & Developer Guide**
> **Environment:** Polyglot Monorepo (Next.js 16 Host + Docker Compose Backend + AI Subsystem)
> **Control Plane:** Root `Makefile` for unified onboarding, testing, linting, and formatting
> **Databases:** PostgreSQL 16 (`lynk_db`, `supertokens_db`) with `pgvector`
> **AI Subsystem:** FastAPI (`:8000`), `ai-worker` queue daemon, and optional profile-gated `vllm`
> **Object Storage:** MinIO S3 (`resumes` bucket)
> **Authentication:** Self-Hosted SuperTokens Core (`:3567`)

---

## 1. Prerequisites & Toolchain Specifications

Before setting up Lynk locally, ensure your developer workstation satisfies the following toolchain floors:

| Tool | Version Floor | Purpose | Installation Check |
| :--- | :--- | :--- | :--- |
| **Docker & Compose** | Docker v24+ / Compose v2.20+ | Backend containerization & service orchestration | `docker compose version` |
| **Node.js** | Node.js v20.0.0+ | Frontend development & native type-stripping ESM test runner | `node -v` |
| **npm** | npm v10.0.0+ | Package management for Next.js frontend | `npm -v` |
| **Go** | Go 1.22+ (Recommended: 1.25.x) | Direct Go compilation, testing, and toolchain checks | `go version` |
| **Python** | Python v3.11+ | AI subsystem service, queue worker daemon, and pytest suite | `python --version` |
| **PostgreSQL Client** | `psql` v16+ | Direct database inspection and migration debugging | `psql --version` |
| **Git** | Git v2.40+ | Version control | `git --version` |

---

## 2. First-Time Developer Quickstart

You can bootstrap a fresh local development environment using either the **Root Makefile** (recommended) or manual CLI commands.

### Option A: Quickstart via Root Makefile (Recommended)

The top-level `Makefile` automates environment variable copying, Git hook installation, Go module verification, frontend dependencies, and Python virtual environment creation in a single command:

```bash
# 1. Clone the repository
git clone https://github.com/BIJJUDAMA/Lynk.git
cd Lynk

# 2. Run universal onboarding (copies .env, installs hooks, Go modules, npm packages, Python venv)
make setup

# 3. Spin up backend container infrastructure
docker compose up -d --build

# 4. Verify all container services are healthy
docker compose ps

# 5. Launch frontend development server on host
cd frontend
npm run dev

# Frontend is live at: http://localhost:3000
# Authoritative Go REST API is live at: http://localhost:8080
# SuperTokens Core IAM is live at: http://localhost:3567
# MinIO Web Console is live at: http://localhost:9001 (User: minio_admin / Pass: minio_password)
```

Subsystem-specific setup targets:
```bash
make setup-hooks      # Link pre-commit hooks into .git/hooks/pre-commit
make setup-go         # Download and verify Go modules
make setup-frontend   # Install npm packages in frontend/
make setup-ai         # Create Python venv in ai/.venv and install dependencies
```

---

### Option B: Manual Setup Step-by-Step (Without Makefile)

If you prefer direct command execution without `make`:

```bash
# 1. Clone the repository
git clone https://github.com/BIJJUDAMA/Lynk.git
cd Lynk

# 2. Configure environment variables from template
cp .env.example .env

# 3. Install Git pre-commit hooks
pre-commit install

# 4. Download and verify backend Go modules
cd backend
go mod download
go mod verify
cd ..

# 5. Setup Python AI virtual environment
python -m venv ai/.venv
# Windows: .\ai\.venv\Scripts\Activate.ps1
# Linux/macOS: source ai/.venv/bin/activate
pip install -r ai/requirements.txt
deactivate

# 6. Spin up backend container infrastructure
docker compose up -d --build

# 7. Verify all container services are healthy
docker compose ps

# Expected output:
# NAME                IMAGE                                   COMMAND                  SERVICE             STATUS              PORTS
# lynk-gateway        nginx:1.25-alpine                       "/docker-entrypoint...." gateway             Up (healthy)        0.0.0.0:8080->8080/tcp
# lynk-api            lynk-backend                            "/app/api"               api                 Up (healthy)        8080/tcp
# lynk-postgres       pgvector/pgvector:pg16                  "docker-entrypoint.s..." postgres            Up (healthy)        0.0.0.0:5432->5432/tcp
# lynk-pgbouncer      edoburu/pgbouncer:latest                "/entrypoint.sh /etc..." pgbouncer           Up (healthy)        0.0.0.0:6432->6432/tcp
# lynk-redis          redis:7-alpine                          "docker-entrypoint.s..." redis               Up (healthy)        0.0.0.0:6379->6379/tcp
# lynk-supertokens    registry.supertokens.io/...             "/bin/sh -c 'exec ./..." supertokens         Up (healthy)        0.0.0.0:3567->3567/tcp
# lynk-minio          cgr.dev/chainguard/minio:latest         "server /data --cons..." minio               Up (healthy)        0.0.0.0:9000->9000/tcp, 0.0.0.0:9001->9001/tcp
# lynk-ai-api         lynk-ai-api                             "uvicorn ai.app.main..." ai-api              Up (healthy)        0.0.0.0:8000->8000/tcp
# lynk-ai-worker      lynk-ai-worker                          "python -m ai.worker..." ai-worker           Up

# 8. Bootstrap and launch the Next.js frontend on host
cd frontend
npm install
npm run dev

# Frontend is live at: http://localhost:3000
# High-Performance Gateway (Reverse Proxy) is live at: http://localhost:8080
# AI Subsystem API is live at: http://localhost:8000
# MinIO Web Console is live at: http://localhost:9001 (User: minio_admin / Pass: minio_password)
```

---

## 3. Environment Variable Configuration Matrix

The application relies on `.env` loaded at root. The table below details every configuration parameter:

| Variable Name | Default Value | Service | Description & Invariant |
| :--- | :--- | :--- | :--- |
| `DATABASE_URL` | `postgres://lynk_user:lynk_password@pgbouncer:6432/lynk_db?sslmode=disable` | Go API | Connection string for application database through PgBouncer |
| `MIGRATION_DATABASE_URL` | `postgres://lynk_user:lynk_password@postgres:5432/lynk_db?sslmode=disable` | Go API | Direct connection string for DDL migrations bypassing pooler |
| `POSTGRES_USER` | `lynk_user` | PostgreSQL | Root database administrative user |
| `POSTGRES_PASSWORD` | `lynk_password` | PostgreSQL | Database password |
| `POSTGRES_DB` | `lynk_db` | PostgreSQL | Primary application database |
| `POSTGRES_MULTIPLE_DATABASES` | `lynk_db,supertokens_db` | Postgres Init | Comma-separated list auto-created on initial database boot |
| `REDIS_URL` | `redis://redis:6379` | Go API / AI | Redis connection URI for L2 caching and sliding-window rate limiting |
| `SUPERTOKENS_CONNECTION_URI` | `http://supertokens:3567` | Go API | HTTP connection address for SuperTokens Core engine |
| `SUPERTOKENS_API_KEY` | `lynksupertokenssecretapikey2026dev` | SuperTokens / API | Shared secret key for administrative SuperTokens RPC calls |
| `API_DOMAIN` | `http://localhost:8080` | SuperTokens | Public root URL for the backend API |
| `WEBSITE_DOMAIN` | `http://localhost:3000` | SuperTokens | Public root URL for the Next.js frontend |
| `MINIO_ROOT_USER` | `minio_admin` | MinIO Server | MinIO server root administrative user |
| `MINIO_ROOT_PASSWORD` | `minio_password` | MinIO Server | MinIO server root password |
| `MINIO_ACCESS_KEY` | `minio_admin` | Go API | S3 access key ID (must match MinIO root user locally) |
| `MINIO_SECRET_KEY` | `minio_password` | Go API | S3 secret access key (must match MinIO root password locally) |
| `MINIO_ENDPOINT` | `minio:9000` | Go API | Internal S3 API host and port |
| `MINIO_PUBLIC_ENDPOINT` | `http://localhost:9000` | Go API | Public endpoint used for presigned download URL generation |
| `MINIO_BUCKET` | `resumes` | Go API | Dedicated resume storage bucket |
| `MINIO_USE_SSL` | `false` | Go API | Toggles HTTPS TLS for MinIO communication |
| `APP_ENV` | `development` | All | Environment mode (`development`, `staging`, `production`) |
| `PORT` | `8080` | Go API | Local port the Go HTTP server binds to |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Go API | Whitelisted origin header allowed for CORS handshakes |
| `MIGRATIONS_DIR` | `backend/migrations` | Go API | Filesystem path to raw SQL migrations directory |
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | Next.js | Public API base URL exposed to browser clients |
| `NEXT_PUBLIC_WEBSITE_URL` | `http://localhost:3000` | Next.js | Public website URL |
| `AI_API_URL` | `http://ai-api:8000` | Go API | Internal base URL for the FastAPI AI subsystem |
| `INTERNAL_AI_SECRET` | `lynk-ai-internal-secret-key-2026` | Go API / AI | Shared defense-in-depth secret for internal AI routes |
| `RATE_LIMIT_PER_MINUTE` | `120` | Go API | Rate limit per minute per client IP |
| `TRUST_FORWARDED_FOR` | `true` | Go API | Enables extracting client IP from X-Forwarded-For behind Nginx |
| `GOMEMLIMIT` | `750MiB` | Go API | Go runtime memory ceiling to prevent OOM in container |
| `GOGC` | `100` | Go API | Garbage collector target percentage |
| `AI_ENABLED` | `true` | Go API | Master toggle for AI orchestrator pipelines |
| `AI_SKILL_NORMALIZATION` | `true` | Go API | Enables canonical skill normalization |

---

## 4. Database Migration Management

Lynk utilizes pure sequential SQL migrations stored in `backend/migrations/` (`000001` through `000020`).

### 4.1 Automatic Startup Migrations
When `lynk-api` starts, `database.RunMigrations(ctx, pool, cfg.MigrationsDir)` executes in `cmd/api/main.go`. It checks the `schema_migrations` table and applies any pending forward `.up.sql` files in ascending lexicographical order inside an isolated transaction.

### 4.2 Creating New Migrations
Always create an incremental forward and rollback pair with a zero-padded 6-digit sequence:
```bash
# Template: backend/migrations/000021_<name>.up.sql and .down.sql
touch backend/migrations/000021_add_feature.up.sql
touch backend/migrations/000021_add_feature.down.sql
```

### 4.3 Migration Rules & Invariants
1. **Never mutate an applied migration:** Once a migration has been committed or executed, it is strictly immutable.
2. **Always write down migrations:** Every `.up.sql` must have a corresponding `.down.sql` reversing the schema changes.
3. **No file blobs in database:** Do not add `BYTEA` columns for user documents. All documents belong in MinIO S3.

### 4.4 Pre-Seeded Test Campus Member Accounts
Migration `000010_seed_test_campus_members` pre-seeds standard test member accounts across both `lynk_db` and `supertokens_db`:

| Account | Email | Password | Role | Verification Status | Purpose |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Project Lead** | `poster@campus.edu` | `password123` | `member` | **Verified** (`true`) | Creates projects, manages workspaces & team rosters |
| **Contributor** | `applicant@campus.edu` | `password123` | `member` | **Verified** (`true`) | Discovers projects & submits recruitment applications |
| **Unverified** | `unverified@campus.edu` | `password123` | `member` | **Pending** (`false`) | Tests HTTP 403 `EMAIL_NOT_VERIFIED` gate |

For manual standalone execution directly into `supertokens_db`, run:
```bash
docker exec -i lynk-postgres psql -U lynk_user -d supertokens_db < .docker/postgres/seed-supertokens.sql
```

---

## 5. Operational Troubleshooting & Recovery Matrix

| Symptom / Error | Root Cause | Immediate Diagnostic Command | Remediation Action |
| :--- | :--- | :--- | :--- |
| **Go API: `connection refused` on port 5432** | PostgreSQL container is starting or port 5432 is occupied | `docker compose ps postgres` | Run `docker compose logs postgres`. Ensure local host PostgreSQL daemon is stopped if port 5432 collides. |
| **MinIO: `NoSuchBucket: resumes`** | `resumes` bucket was not initialized | `curl -i http://localhost:9000/minio/health/live` | Restart API container (`docker compose restart api`). Go API auto-probes and creates bucket on boot. |
| **CORS error in browser console** | Next.js making requests to non-whitelisted origin | Check browser Network tab for `Origin` header | Verify `CORS_ALLOWED_ORIGINS=http://localhost:3000` in `.env` and `internal/middleware/cors.go`. |
| **Node ESM: `ERR_UNSUPPORTED_DIR_IMPORT`** | SuperTokens web-js import missing `/index.js` subpath | Run `npm test` in `frontend/` | Ensure imports in `frontend/src/lib/supertokens.ts` specify `/index.js` (e.g. `supertokens-web-js/recipe/session/index.js`). |
| **PostgreSQL: `relation "users" does not exist` in CI** | Integration tests running before database migration step | Run `go test -v ./internal/database` | Ensure CI workflow runs `Apply application migrations` before parallel race detector test suite. |
| **Resume upload TCP socket drop mid-transfer** | Go HTTP server write timeout lower than handler deadline | Inspect `cmd/api/main.go` `ServerWriteTimeout` | Confirm `ServerReadTimeout=95s` and `ServerWriteTimeout=100s` exceed the 90s dynamic upload timeout. |
| **SuperTokens: `Invalid API key`** | `SUPERTOKENS_API_KEY` mismatch between compose and API | Inspect `docker compose logs supertokens` | Synchronize `SUPERTOKENS_API_KEY` in `.env` across `docker-compose.yml` and Go API config. |
| **AI API: `401 Unauthorized` on internal routes** | `INTERNAL_AI_SECRET` header mismatch | Inspect `docker compose logs ai-api` | Ensure `INTERNAL_AI_SECRET` matches across `.env`, `docker-compose.yml`, and Go API configuration. |
| **AI Worker: connection refused to postgres** | PostgreSQL not ready when worker booted | `docker compose logs ai-worker` | Restart worker via `docker compose restart ai-worker`. In production, worker retries with backoff. |
| **Search fallback warning in Go API logs** | AI backend offline, degraded, or timed out | Inspect `docker compose ps ai-api` | Normal graceful degradation. Go automatically serves standard SQL results without breaking. |
| **PostgreSQL: `type "vector" does not exist`** | `pgvector` extension not loaded on database | Check `docker compose exec postgres psql -U lynk_user -d lynk_db -c '\dx'` | Run migration `000012_ai_subsystem_init.up.sql` or `000019_project_vector_search.up.sql` to install `vector` extension. |

---

## 6. Disaster Recovery & Database Reset

If local database state becomes corrupt or dirty during development:

```bash
# 1. Stop all containers and delete attached volumes (destroys data in postgres & minio)
docker compose down -v

# 2. Prune orphan container artifacts
docker compose rm -f -v

# 3. Spin up fresh stack from zero
docker compose up -d --build

# 4. Verify clean schema creation
docker compose logs -f api
```

---

## 7. Universal Quality Verification Suite (Makefile)

Run verification across all three languages without manual path changes:

```bash
# Execute unit tests across Go backend, frontend, and Python AI:
make test

# Run multi-language linters (golangci-lint, oxlint, ruff check, typecheck):
make lint

# Check code formatting compliance (Go fmt, Prettier, Ruff):
make format-check

# Auto-format all code in monorepo:
make format

# Run comprehensive k6 load and concurrency stress testing:
make load-test         # 5-stage benchmark suite (up to 1,000 VUs)
make load-test-ultra   # Full 6-stage ultra-scale suite (up to 2,500 VUs)
```
