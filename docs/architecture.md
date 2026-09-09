# System Architecture Document - Lynk

> **Project:** Lynk - High-Trust Campus Freelance & Gig Marketplace  
> **Status:** MVP Architecture Specification  
> **Identity & Access Management (IAM):** SuperTokens Core (Session + EmailPassword + EmailVerification)  
> **Evolution Phase:** Phase 1 (MVP - "Nothing Else Initially")

---

## 1. Executive Summary & Core Thesis

**Lynk** is a university-centric freelance and campus gig platform designed to bridge the trust gap between students, campus departments, and student founders. Unlike generic freelance platforms that segregate users into rigid "student" vs "employer" silos, Lynk enforces a **Unified Campus Member** architecture. Any verified student or faculty member can both offer services (as a contributor) and publish opportunities (as an organizer), powered by institutional `.edu` email verification, direct deliverable contracts, and transparent peer reviews.

### Primary Architectural Invariants
1. **Strict MVP Scope ("Nothing Else Initially"):** Zero message queues, zero distributed caches, and zero commercial cloud auth SaaS vendors (Auth0/Firebase/Supabase) during MVP. Self-hosted **SuperTokens Core** in Docker handles session management, user credentials, and email verification. Synchronous HTTP + PostgreSQL + MinIO + SuperTokens handles 100% of MVP requirements.
2. **Containerization Boundary:**
   - **Backend & Core Services (`backend/`, SuperTokens, Postgres, MinIO):** Dockerised and orchestrated using `docker-compose.yml`.
   - **Frontend (`frontend/`):** Co-located in the monorepo but **NOT** dockerised. Runs directly on the host (`npm run dev`) for instant Hot Module Replacement (HMR).
3. **Authentication & Identity Provider (SuperTokens Core):** User credentials, session tokens (via `sAccessToken` and `sRefreshToken`), and email verification workflows are managed by self-hosted SuperTokens Core on port 3567. The Go API mounts SuperTokens middleware verifying sessions directly.
4. **Storage Boundary:** Resumes are stored exclusively in **MinIO** (S3-compatible object storage). PostgreSQL stores only metadata and S3 object keys.
5. **Institutional Email Gate:** Campus members cannot post opportunities, submit applications, accept contracts, or upload resumes until their institutional `.edu` email address has been verified. Non-.edu registrations are strictly rejected at sign-up.

---

## 2. High-Level System Architecture

```text
+------------------------------------------------------------------------+
|                        HOST ENVIRONMENT (DEVELOPMENT)                  |
|                                                                        |
|   Next.js 14+ (App Router + TypeScript + Tailwind CSS)                 |
|   Runs on host: http://localhost:3000                                  |
|   Auth Client: supertokens-web-js (Session, EmailPassword, Verify)     |
+------------------------------------------------------------------------+
                    |                                |
                    | 1. Sign In / Verify Email      | 2. REST API Calls
                    |    Credentials & Session       |    Cookie / Bearer Token
                    |    http://localhost:8080/api/v1|    http://localhost:8080
                    v                                v
+------------------------------------------------------------------------+
|                     DOCKER COMPOSE NETWORK (lynk-net)                  |
|                                                                        |
|   +-------------------------------+ +------------------------------+   |
|   |  SuperTokens Core             | |  Go REST API                 |   |
|   |  (Container: lynk-supertokens)| |  (Container: lynk-api)       |   |
|   |  Port: 3567:3567              | |  Port: 8080:8080             |   |
|   |                               | |                              |   |
|   |  - Recipe: EmailPassword      | |  - SuperTokens Go SDK        |   |
|   |  - Recipe: Session            | |  - Session & Role Middleware |   |
|   |  - Recipe: EmailVerification  | |  - Strict .edu Gate          |   |
|   |  - Port 3567 Internal Core    | |  - Layered Architecture      |   |
|   +-------------------------------+ +------------------------------+   |
|                   |                         |              |           |
|                   |                         |              | S3 API    |
|                   | SQL (supertokens_db)    | SQL(lynk_db) | Port 9000 |
|                   v                         v              v           |
|   +-----------------------------------------+ +--------------------+   |
|   |  PostgreSQL 16+                         | |  MinIO Storage     |   |
|   |  (Container: lynk-postgres)             | |  (lynk-minio)      |   |
|   |  Port: 5432                             | |  Port: 9000 / 9001 |   |
|   |                                         | |                    |   |
|   |  - supertokens_db: Auth & Session state | |  Bucket: "resumes" |   |
|   |  - lynk_db: Profiles, Jobs,             | |  Stores: Resumes   |   |
|   |    Applications, Contracts, Reviews     | |                    |   |
|   +-----------------------------------------+ +--------------------+   |
+------------------------------------------------------------------------+
```

---

## 3. Evolutionary Architecture Roadmap

Lynk starts with a synchronous Go backend and SuperTokens Core IAM. Later phases introduce distributed performance tools strictly after the MVP baseline is verified.

```mermaid
flowchart TD
    subgraph Phase1["Phase 1: MVP (Active Baseline)"]
        FE[Next.js App on Host] -->|SuperTokens Web JS| API[Dockerised Go REST API]
        API -->|SuperTokens Core API| ST[SuperTokens Core in Docker]
        API -->|Raw SQL Migrations| PG[(PostgreSQL 16+)]
        API -->|S3 API| S3[(MinIO: Resumes Bucket)]
    end

    subgraph Phase2["Phase 2: Performance & Protection"]
        REDIS[(Redis)]
        API -.->|Session Caching & Rate Limiting| REDIS
    end

    subgraph Phase3["Phase 3: Async Processing & Workers"]
        RMQ{{RabbitMQ Message Broker}}
        WORKER[Go Background Worker]
        API -.->|Publish Email & Event Tasks| RMQ
        RMQ -.->|Consume Tasks| WORKER
        WORKER -.->|Notifications / Audit Logs| EXT[External Services]
    end

    subgraph Phase4["Phase 4: Reliability & Scale"]
        K6[k6 Load Testing Suite]
        OBS[Prometheus & Grafana Telemetry]
        K6 -.->|Stress & Benchmark| API
        API -.->|Metrics| OBS
    end

    Phase1 --> Phase2
    Phase2 --> Phase3
    Phase3 --> Phase4
```

---

## 4. SuperTokens Core IAM Integration & Auth Flow

### Architecture & Recipes
* **Recipes:**
  - `emailpassword`: Email & password credentials authentication. Non-.edu email signups are rejected immediately via SignUp override.
  - `session`: Cookie and header-based session tokens with automatic refresh handling.
  - `emailverification`: ModeRequired email verification enforcing institutional email ownership before unlocking marketplace actions.
  - `userroles`: Role assignment (`member` by default, with `admin` for platform operators).

### End-to-End Authentication Sequence

```mermaid
sequenceDiagram
    autonumber
    actor User as Campus Member
    participant Next as Next.js Frontend (Host)
    participant API as Go REST API (Docker :8080)
    participant ST as SuperTokens Core (:3567)
    participant PG as PostgreSQL (lynk_db)

    User->>Next: Submit Credentials (email + password)
    Next->>API: POST /api/v1/auth/signup or /signin
    API->>API: Validate institutional .edu domain
    API->>ST: Create / Verify User in SuperTokens
    ST-->>API: Return Session Tokens & Claims
    API-->>Next: Return Session Cookie (sAccessToken)
    
    Next->>API: Authenticated Request (Cookie: sAccessToken or Bearer)
    API->>API: SessionMiddleware validates session
    API->>API: Check EmailVerification status
    API->>PG: Query / Mutate application data using user_id
    PG-->>API: Return DB rows
    API-->>Next: JSON Response
```

---

## 5. Backend Clean Architecture (Go)

The Go backend enforces strict separation of concerns:

```text
HTTP Request (with Session Cookie / Bearer Token)
     |
     v
+---------------------------------------------------------------+
| 1. Middleware Layer                                           |
|    - CORS Middleware (origins: http://localhost:3000)         |
|    - SuperTokens Session Middleware: verifies session,        |
|      extracts user_id, email, email_verified, roles           |
|    - Injects claims into request context.Context              |
+---------------------------------------------------------------+
                                |
                                v
+---------------------------------------------------------------+
| 2. Handler Layer (Transport / HTTP)                           |
|    - Decodes & validates request JSON bodies / query params   |
|    - Reads authenticated user context from context.Context    |
|    - Maps domain errors to standard HTTP status codes         |
+---------------------------------------------------------------+
                                |
                                v
+---------------------------------------------------------------+
| 3. Service Layer (Business Logic & Invariants)                |
|    - Enforces Campus Member invariants & email verification   |
|    - Manages contract state machine transitions               |
|    - Orchestrates database repositories and MinIO storage     |
+---------------------------------------------------------------+
                                |
                                v
+---------------------------------------------------------------+
| 4. Repository & Storage Layer (Persistence)                   |
|    - Repository: Raw SQL queries against PostgreSQL (lynk_db) |
|    - Storage: MinIO client for resume streaming / presigning  |
+---------------------------------------------------------------+
```

### Module Breakdown (`backend/internal/`)

| Package | Responsibility |
| :--- | :--- |
| `auth` | SuperTokens Go SDK initialization, email verification overrides, and session user claims extraction. |
| `user` | Unified campus member profile creation, updates, and portfolio links (keyed by SuperTokens `user_id`). |
| `job` | Job posting CRUD, search, filtering by department, budget, and required skills. |
| `application` | Application submission, cover letter, resume linking, and member proposal review (accept/reject). |
| `contract` | Deliverable contract state machine (`Draft` -> `Active` -> `Completed` | `Cancelled`). |
| `review` | Rating (1-5) and written feedback submission following contract completion. |
| `storage` | MinIO client abstraction for uploading, fetching, and generating pre-signed URLs for resumes. |
| `database` | PostgreSQL connection pooling (`pgxpool`), health checks, and raw SQL migration runner. |
| `middleware` | CORS, SuperTokens session auth, request logging, and panic recovery. |

---

## 6. Database Schema & Relational Model

PostgreSQL 16+ is the source of truth for all application data in the `lynk_db` database.
> **Note:** User identity and authentication tokens are managed by SuperTokens in `supertokens_db`. Application tables link to users via the SuperTokens user identifier (`VARCHAR(64)`).

### Entity Relationship Diagram (ERD)

```mermaid
erDiagram
    USERS ||--o| PROFILES : "has"
    USERS ||--o{ JOBS : "posts"
    USERS ||--o{ APPLICATIONS : "submits"
    JOBS ||--o{ APPLICATIONS : "receives"
    APPLICATIONS ||--o| CONTRACTS : "generates upon acceptance"
    CONTRACTS ||--o{ REVIEWS : "receives"

    USERS {
        varchar id PK "Matches SuperTokens user ID"
        string email UK
        string role "member | admin"
        timestamptz created_at
        timestamptz updated_at
    }

    PROFILES {
        uuid id PK
        varchar user_id FK,UK "Matches USERS.id"
        string first_name
        string last_name
        text bio
        string department
        int graduation_year
        text_array skills
        jsonb portfolio_links
        string resume_key
        string resume_filename
        bigint resume_byte_size
        string organization
        string organization_website
        timestamptz updated_at
    }

    JOBS {
        uuid id PK
        varchar created_by FK "Organizer member ID"
        string title
        text description
        numeric budget
        string pay_type "fixed | hourly"
        text_array required_skills
        string department
        date deadline
        string status "open | in_progress | closed | cancelled"
        timestamptz created_at
        timestamptz updated_at
    }

    APPLICATIONS {
        uuid id PK
        uuid job_id FK
        varchar applicant_id FK "Applicant member ID"
        text cover_letter
        string resume_key
        string status "pending | accepted | rejected"
        timestamptz created_at
        timestamptz updated_at
    }

    CONTRACTS {
        uuid id PK
        uuid job_id FK
        uuid application_id FK
        varchar client_id FK
        varchar freelancer_id FK
        numeric agreed_budget
        string status "active | completed | cancelled"
        timestamptz started_at
        timestamptz completed_at
        timestamptz created_at
        timestamptz updated_at
    }

    REVIEWS {
        uuid id PK
        uuid contract_id FK
        varchar reviewer_id FK
        varchar reviewee_id FK
        int rating "1 to 5"
        text comment
        timestamptz created_at
    }
```

### Partial Unique Indexes (Contracts)
* `uq_contracts_active_job` — at most one non-cancelled contract per `job_id`.
* `uq_contracts_active_application` — at most one non-cancelled contract per `application_id`.

---

## 7. Object Storage Architecture (MinIO)

MinIO provides local S3-compatible storage dedicated to student resumes.

### Invariants & Bucket Policy
* **Bucket Name:** `resumes` (strictly isolated to PDF/DOCX resumes).
* **Access Policy:** Private. Objects cannot be accessed anonymously.
* **Upload Flow:**
  1. Campus member sends `multipart/form-data` to Go API (`POST /api/v1/profile/resume`) with active session.
  2. Go backend confirms institutional email is verified (`EmailVerified == true`).
  3. Go backend validates MIME type (`application/pdf`, `application/vnd.openxmlformats-officedocument.wordprocessingml.document`) and max size (5MB).
  4. File is streamed to MinIO: key format `resumes/{user_id}/{uuid}-{sanitized_filename}`.
  5. S3 key and metadata saved to `profiles`.
* **Download Flow:**
  - Go API generates a 15-minute pre-signed download URL (`GetObjectPresigned`) returned to authorized members.

---

## 8. REST API Contract Specification

All application endpoints are prefixed with `/api/v1`. Authentication is passed via session cookies or Bearer token header.

### Standard Response Envelope
```json
{
  "success": true,
  "data": {},
  "error": null
}
```

### Standard Error Envelope
```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "EMAIL_NOT_VERIFIED",
    "message": "Campus verification pending: Please verify your institutional .edu email before accessing opportunities."
  }
}
```

### Endpoint Matrix

#### Authentication & Session State (`/api/v1/auth`)
| Method | Path | Auth | Role | Description |
| :--- | :--- | :---: | :---: | :--- |
| `POST` | `/auth/signup` | Public | Any | Register with university .edu email and password. Non-.edu domains rejected. |
| `POST` | `/auth/signin` | Public | Any | Authenticate with credentials and initialize session cookie. |
| `POST` | `/auth/signout` | Session | Any | Revoke active SuperTokens session. |
| `GET`  | `/auth/me` | Session | Any | Return current authenticated user profile, role, and verification status. |
| `POST` | `/auth/sync` | Session | Any | Idempotent first-login hook to ensure user and profile exist in `lynk_db`. |

#### Unified Member Profiles (`/api/v1/profile`)
| Method | Path | Auth | Role | Description |
| :--- | :--- | :---: | :---: | :--- |
| `GET`  | `/profile/me` | Session | Member | Get own unified campus member profile. |
| `PUT`  | `/profile/me` | Session | Member | Update bio, skills, department, graduation year, organization, and links. |
| `POST` | `/profile/resume` | Session | Verified Member | Upload resume (PDF/DOCX, max 5MB). Streams to MinIO. |
| `GET`  | `/profile/resume` | Session | Member | Get pre-signed download URL for own resume. |
| `GET`  | `/profile/{id}` | Session | Any | View public profile of a campus member. |

#### Jobs (`/api/v1/jobs`)
| Method | Path | Auth | Role | Description |
| :--- | :--- | :---: | :---: | :--- |
| `GET`  | `/jobs` | Public | Any | Browse and search jobs with filters (`department`, `skill`, `min_budget`, `max_budget`, `status`). |
| `POST` | `/jobs` | Session | Verified Member | Create a new campus job posting. |
| `GET`  | `/jobs/{id}` | Public | Any | Retrieve detailed job view. |
| `PUT`  | `/jobs/{id}` | Session | Owner | Update job posting. |
| `DELETE` | `/jobs/{id}` | Session | Owner | Cancel/close job posting. |

#### Applications (`/api/v1/applications`)
| Method | Path | Auth | Role | Description |
| :--- | :--- | :---: | :---: | :--- |
| `POST` | `/jobs/{id}/applications` | Session | Verified Member | Submit application with cover letter and attached resume key. |
| `GET`  | `/jobs/{id}/applications` | Session | Owner | List all submitted applications for a job posting. |
| `GET`  | `/applications/mine` | Session | Member | List applications submitted by current member. |
| `PATCH`| `/applications/{id}/status` | Session | Owner | Accept or Reject application. Accepting automatically creates a `Contract`. |

#### Contracts (`/api/v1/contracts`)
| Method | Path | Auth | Role | Description |
| :--- | :--- | :---: | :---: | :--- |
| `GET`  | `/contracts` | Session | Any | List contracts involving the authenticated member. |
| `GET`  | `/contracts/{id}` | Session | Participant | Retrieve detailed contract terms, status, and contract history. |
| `PATCH`| `/contracts/{id}/status` | Session | Participant | Update contract status: `Active` -> `Completed` | `Cancelled`. |

#### Reviews & Ratings (`/api/v1/contracts/{id}/reviews`)
| Method | Path | Auth | Role | Description |
| :--- | :--- | :---: | :---: | :--- |
| `POST` | `/contracts/{id}/reviews` | Session | Participant | Submit 1-5 star rating and written review (allowed ONLY when contract is `Completed`). |
| `GET`  | `/contracts/{id}/reviews` | Session | Participant | Get reviews associated with the contract. |
| `GET`  | `/users/{id}/reviews` | Public | Any | Get aggregated ratings and reviews for a user. |

---

## 9. Contract State Machine & Institutional Email Verification Gate

### Contract Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Draft: Organizer accepts Application
    Draft --> Active: Both parties confirm terms / Start work
    Active --> Completed: Deliverables verified by Organizer
    Active --> Cancelled: Mutual cancellation / Dispute
    Completed --> [*]: Review & Rating unlocked
    Cancelled --> [*]
```

### Institutional Email Verification Gate
* SuperTokens enforces email verification requiring users to confirm ownership of their `.edu` email address.
* The Go API checks `EmailVerified` boolean status on every sensitive domain action.
* If `EmailVerified == false`:
  - `POST /jobs` returns `403 Forbidden` (`EMAIL_NOT_VERIFIED`).
  - `POST /jobs/{id}/applications` returns `403 Forbidden` (`EMAIL_NOT_VERIFIED`).
  - `POST /profile/resume` returns `403 Forbidden` (`EMAIL_NOT_VERIFIED`).

---

## 10. Local Infrastructure Topology (`docker-compose.yml`)

```yaml
services:
  postgres:
    image: postgres:16-alpine
    container_name: lynk-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: lynk_user
      POSTGRES_PASSWORD: lynk_password
      POSTGRES_DB: lynk_db
      POSTGRES_MULTIPLE_DATABASES: "lynk_db,supertokens_db"
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./.docker/postgres/init-databases.sh:/docker-entrypoint-initdb.d/init-databases.sh
    networks:
      - lynk-net
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U lynk_user -d lynk_db"]
      interval: 5s
      timeout: 5s
      retries: 5

  supertokens:
    image: registry.supertokens.io/supertokens/supertokens-postgresql:9.2
    container_name: lynk-supertokens
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      POSTGRESQL_CONNECTION_URI: "postgresql://lynk_user:lynk_password@postgres:5432/supertokens_db"
      API_KEYS: "lynk_supertokens_secret_api_key_2026"
    ports:
      - "3567:3567"
    networks:
      - lynk-net
    healthcheck:
      test: ["CMD-SHELL", "exec 3<>/dev/tcp/localhost/3567 && echo -e 'GET /hello HTTP/1.1\\r\\nHost: localhost\\r\\nConnection: close\\r\\n\\r\\n' >&3 && cat <&3 | grep -q 'Hello'"]
      interval: 10s
      timeout: 5s
      retries: 5

  minio:
    image: minio/minio:RELEASE.2024-01-18T22-51-28Z
    container_name: lynk-minio
    restart: unless-stopped
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minio_admin
      MINIO_ROOT_PASSWORD: minio_password
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio_data:/data
    networks:
      - lynk-net
    healthcheck:
      test: ["CMD", "mc", "ready", "local"]
      interval: 5s
      timeout: 5s
      retries: 5

  minio-init:
    image: minio/mc:latest
    container_name: lynk-minio-init
    depends_on:
      minio:
        condition: service_healthy
    networks:
      - lynk-net
    entrypoint: >
      /bin/sh -c "
      mc alias set myminio http://minio:9000 minio_admin minio_password;
      mc mb --ignore-existing myminio/resumes;
      exit 0;
      "

  api:
    build:
      context: ./backend
      dockerfile: Dockerfile
    container_name: lynk-api
    restart: unless-stopped
    environment:
      PORT: "8080"
      DATABASE_URL: "postgres://lynk_user:lynk_password@postgres:5432/lynk_db?sslmode=disable"
      MIGRATIONS_DIR: "/app/migrations"
      SUPERTOKENS_CONNECTION_URI: "http://supertokens:3567"
      SUPERTOKENS_API_KEY: "lynk_supertokens_secret_api_key_2026"
      API_DOMAIN: "http://localhost:8080"
      WEBSITE_DOMAIN: "http://localhost:3000"
      MINIO_ENDPOINT: "minio:9000"
      MINIO_PUBLIC_ENDPOINT: "http://localhost:9000"
      MINIO_ACCESS_KEY: "minio_admin"
      MINIO_SECRET_KEY: "minio_password"
      MINIO_BUCKET: "resumes"
      MINIO_USE_SSL: "false"
      CORS_ALLOWED_ORIGINS: "http://localhost:3000"
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
      supertokens:
        condition: service_healthy
      minio:
        condition: service_healthy
      minio-init:
        condition: service_completed_successfully
    networks:
      - lynk-net
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://localhost:8080/health || exit 1"]
      interval: 10s
      timeout: 5s
      retries: 3
      start_period: 15s

networks:
  lynk-net:
    driver: bridge

volumes:
  postgres_data:
  minio_data:
```

---

## 11. Testing & Verification Strategy

| Layer | Tooling | Scope | Command |
| :--- | :--- | :--- | :--- |
| **Go Unit Tests** | Go `testing` package | SuperTokens claims validation, email domain checks, contract transitions. | `cd backend && go test -v -race ./...` |
| **Go Vet & Static Check** | Go standard tooling | Static analysis, idiomatic conventions. | `cd backend && go vet ./...` |
| **Go Integration Tests** | Real Docker Compose Stack | Repository queries in `lynk_db`, SuperTokens session validation, MinIO S3 streaming. | `cd backend && go test -count=1 ./...` |
| **Frontend Typecheck** | TypeScript compiler | Ensure TypeScript types and SuperTokens session types match Go API JSON responses. | `cd frontend && npm run typecheck` |
| **Frontend Unit Tests**| Node Test Runner | Email validation, JWT decode, formatters, and API client tests. | `cd frontend && npm test` |
| **Frontend Production Build**| Next.js / ESLint | App Router static & dynamic route compilation. | `cd frontend && npm run build` |
| **End-to-End Smoke** | PowerShell / Bash test scripts | Full user lifecycle: .edu rejection check, SuperTokens registration, verification gate, post job, apply, contract, and review. | `pwsh scripts/smoke-test.ps1` or `bash scripts/smoke-test.sh` |
