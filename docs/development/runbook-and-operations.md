# Operations, Runbook, & Local Developer Guide - Lynk

> **Authoritative Operational Runbook & Developer Guide**  
> **Environment:** Polyglot Monorepo (Next.js 14 Host + Docker Compose Backend)  
> **Databases:** PostgreSQL 16 (`lynk_db`, `supertokens_db`)  
> **Object Storage:** MinIO S3 (`resumes` bucket)  
> **Authentication:** Self-Hosted SuperTokens Core (`:3567`)

---

## 1. Prerequisites & Toolchain Specifications

Before setting up Lynk locally, ensure your developer workstation satisfies the following toolchain floors:

| Tool | Version Floor | Purpose | Installation Check |
| :--- | :--- | :--- | :--- |
| **Docker & Compose** | Docker v24+ / Compose v2.20+ | Backend containerization & service orchestration | `docker compose version` |
| **Node.js** | Node.js v22.0.0+ | Frontend development & native type-stripping ESM test runner | `node -v` |
| **npm** | npm v10.0.0+ | Package management for Next.js frontend | `npm -v` |
| **Go** | Go 1.22+ (Recommended: 1.25.x) | Direct Go compilation, testing, and toolchain checks | `go version` |
| **PostgreSQL Client** | `psql` v16+ | Direct database inspection and migration debugging | `psql --version` |
| **Git** | Git v2.40+ | Version control | `git --version` |

---

## 2. First-Time Developer Quickstart

Execute this exact sequence to bootstrap a fresh local development environment:

```bash
# 1. Clone the repository
git clone https://github.com/BIJJUDAMA/Lynk.git
cd Lynk

# 2. Configure environment variables from template
cp .env.example .env

# 3. Spin up backend container infrastructure
docker compose up -d

# 4. Verify all container services are healthy
docker compose ps

# Expected output:
# NAME                IMAGE                                   COMMAND                  SERVICE             STATUS              PORTS
# lynk-postgres       postgres:16-alpine                      "docker-entrypoint.s..."   postgres            Up (healthy)        0.0.0.0:5432->5432/tcp
# lynk-supertokens    registry.supertokens.io/...             "/bin/sh -c 'exec ./..."   supertokens         Up (healthy)        0.0.0.0:3567->3567/tcp
# lynk-minio          quay.io/minio/minio:RELEASE...          "/usr/bin/docker-ent..."   minio               Up (healthy)        0.0.0.0:9000->9000/tcp, 0.0.0.0:9001->9001/tcp
# lynk-api            lynk-backend                            "/app/api"               api                 Up (healthy)        0.0.0.0:8080->8080/tcp

# 5. Bootstrap and launch the Next.js frontend on host
cd frontend
npm install
npm run dev

# Frontend is live at: http://localhost:3000
# Backend REST API is live at: http://localhost:8080
# MinIO Web Console is live at: http://localhost:9001 (User: minio_admin / Pass: minio_password)
```

---

## 3. Environment Variable Configuration Matrix

The application relies on `.env` loaded at root. The table below details every configuration parameter:

| Variable Name | Default Value | Service | Description & Invariant |
| :--- | :--- | :--- | :--- |
| `DATABASE_URL` | `postgres://lynk_user:lynk_password@localhost:5432/lynk_db?sslmode=disable` | Go API | Connection string for application database (`pgxpool`) |
| `POSTGRES_USER` | `lynk_user` | PostgreSQL | Root database administrative user |
| `POSTGRES_PASSWORD` | `lynk_password` | PostgreSQL | Database password |
| `POSTGRES_DB` | `lynk_db` | PostgreSQL | Primary application database |
| `POSTGRES_MULTIPLE_DATABASES` | `lynk_db,supertokens_db` | Postgres Init | Comma-separated list auto-created on initial database boot |
| `SUPERTOKENS_CONNECTION_URI` | `http://localhost:3567` (or `http://supertokens:3567` in Docker) | Go API | HTTP connection address for SuperTokens Core engine |
| `SUPERTOKENS_API_KEY` | `lynk-supertokens-secret-api-key-2026` | SuperTokens / API | Shared secret key for administrative SuperTokens RPC calls |
| `API_DOMAIN` | `http://localhost:8080` | SuperTokens | Public root URL for the backend API |
| `WEBSITE_DOMAIN` | `http://localhost:3000` | SuperTokens | Public root URL for the Next.js frontend |
| `MINIO_ROOT_USER` | `minio_admin` | MinIO Server | MinIO server root administrative user |
| `MINIO_ROOT_PASSWORD` | `minio_password` | MinIO Server | MinIO server root password |
| `MINIO_ACCESS_KEY` | `minio_admin` | Go API | S3 access key ID (must match MinIO root user locally) |
| `MINIO_SECRET_KEY` | `minio_password` | Go API | S3 secret access key (must match MinIO root password locally) |
| `MINIO_ENDPOINT` | `localhost:9000` (or `minio:9000` in Docker) | Go API | Internal S3 API host and port |
| `MINIO_PUBLIC_ENDPOINT` | `http://localhost:9000` | Go API | Public endpoint used for presigned download URL generation |
| `MINIO_BUCKET` | `resumes` | Go API | Dedicated resume storage bucket |
| `MINIO_USE_SSL` | `false` | Go API | Toggles HTTPS TLS for MinIO communication |
| `APP_ENV` | `development` | All | Environment mode (`development`, `staging`, `production`) |
| `PORT` | `8080` | Go API | Local port the Go HTTP server binds to |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Go API | Whitelisted origin header allowed for CORS handshakes |
| `MIGRATIONS_DIR` | `backend/migrations` | Go API | Filesystem path to raw SQL migrations directory |
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | Next.js | Public API base URL exposed to browser clients |
| `NEXT_PUBLIC_WEBSITE_URL` | `http://localhost:3000` | Next.js | Public website URL |

### 3.1 Secret & API Key Rotation
In `docker-compose.yml`, all credentials utilize bash variable expansion with development fallbacks:
```yaml
API_KEYS: "${SUPERTOKENS_API_KEY:-lynk-supertokens-secret-api-key-2026}"
MINIO_ROOT_USER: "${MINIO_ROOT_USER:-minio_admin}"
MINIO_ROOT_PASSWORD: "${MINIO_ROOT_PASSWORD:-minio_password}"
```

To rotate any API key or secret in any environment:
1. Provide the new values in a `.env` file in the project root or in the host environment:
   ```env
   SUPERTOKENS_API_KEY=your-new-secret-key-here
   MINIO_ROOT_PASSWORD=your-new-minio-password-here
   ```
2. Re-create the affected containers:
   ```bash
   docker compose up -d
   ```
Docker Compose detects updated environment variables and cleanly restarts affected containers without requiring Docker Secrets file mounts or code changes.

---

## 4. Database Migration Management

Lynk utilizes pure sequential SQL migrations stored in `backend/migrations/`.

### 4.1 Automatic Startup Migrations
When `lynk-api` starts, `database.RunMigrations(ctx, pool, cfg.MigrationsDir)` executes in `cmd/api/main.go`. It checks the `schema_migrations` table and applies any pending forward `.up.sql` files in ascending lexicographical order inside an isolated transaction.

### 4.2 Creating New Migrations
Always create an incremental forward and rollback pair with a zero-padded 6-digit sequence:
```bash
# Template: backend/migrations/000011_<name>.up.sql and .down.sql
touch backend/migrations/000011_add_contract_milestones.up.sql
touch backend/migrations/000011_add_contract_milestones.down.sql
```

### 4.3 Migration Rules & Invariants
1. **Never mutate an applied migration:** Once a migration has been committed or executed, it is strictly immutable.
2. **Always write down migrations:** Every `.up.sql` must have a corresponding `.down.sql` reversing the schema changes.
3. **No file blobs in database:** Do not add `BYTEA` columns for user documents. All documents belong in MinIO S3.

### 4.4 Pre-Seeded Test Campus Member Accounts
Migration `000010_seed_test_campus_members` pre-seeds three standard test member accounts across both `lynk_db` and `supertokens_db`:

| Account | Email | Password | Role | Verification Status | Purpose |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Poster** | `poster@campus.edu` | `password123` | `member` | **Verified** (`true`) | Creates peer jobs & reviews applications |
| **Applicant** | `applicant@campus.edu` | `password123` | `member` | **Verified** (`true`) | Browses jobs & submits peer applications |
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

---

## 6. Disaster Recovery & Database Reset

If local database state becomes corrupt or dirty during development:

```bash
# 1. Stop all containers and delete attached volumes (destroys data in postgres & minio)
docker compose down -v

# 2. Prune orphan container artifacts
docker compose rm -f -v

# 3. Spin up fresh stack from zero
docker compose up -d

# 4. Verify clean schema creation
docker compose logs -f api
```
