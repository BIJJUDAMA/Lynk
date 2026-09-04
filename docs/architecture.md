# System Architecture Document — Lynk

> **Project:** Lynk — High-Trust Student Freelance & Campus Gig Marketplace  
> **Status:** MVP Architecture Specification  
> **Identity & Access Management (IAM):** Keycloak (OIDC / OAuth2)  
> **Evolution Phase:** Phase 1 (MVP — "Nothing Else Initially")

---

## 1. Executive Summary & Core Thesis

**Lynk** is a university-centric freelance and campus gig platform designed to bridge the trust gap between verified university students and campus/local employers. Unlike generic freelance platforms, Lynk enforces identity integrity via institutional email verification (`.edu` / recognized university domains) and structured milestone contracts with transparent peer reviews.

### Primary Architectural Invariants
1. **Strict MVP Scope ("Nothing Else Initially"):** Zero message queues, zero distributed caches, zero external cloud auth SaaS vendors (Auth0/Firebase/Supabase), and zero microservices during MVP. Self-hosted **Keycloak** in Docker handles OIDC/OAuth2, user credentials, role assignments, and email verification. Synchronous HTTP + PostgreSQL + MinIO + Keycloak handles 100% of MVP requirements.
2. **Containerization Boundary:**
   - **Backend & Core Services (`backend/`, Keycloak, Postgres, MinIO):** Dockerised and orchestrated using `docker-compose.yml`.
   - **Frontend (`frontend/`):** Co-located in the monorepo but **NOT** dockerised. Runs directly on the host (`npm run dev`) for instant Hot Module Replacement (HMR).
3. **Authentication & Identity Provider (Keycloak):** All user credentials, password policies, session tokens (JWT access & refresh tokens), and email verification workflows are managed centrally by Keycloak. The Go API acts as an OAuth2 Resource Server validating JWTs using Keycloak's JWKS public keys.
4. **Storage Boundary:** Resumes are stored exclusively in **MinIO** (S3-compatible object storage). PostgreSQL stores only metadata and S3 object keys.
5. **Institutional Email Gate:** Students cannot submit applications, accept contracts, or upload resumes until their institutional email address has been verified in Keycloak (`email_verified: true` claim in JWT).

---

## 2. High-Level System Architecture

```text
┌────────────────────────────────────────────────────────────────────────┐
│                        HOST ENVIRONMENT (DEVELOPMENT)                  │
│                                                                        │
│   Next.js 14+ (App Router + TypeScript + Tailwind CSS)                 │
│   Runs on host: http://localhost:3000                                  │
│   Auth Client: OIDC / OAuth2 Authorization Code Flow with PKCE         │
└───────────────────┬────────────────────────────────┬───────────────────┘
                    │                                │
                    │ 1. OIDC Login / Token Exchange │ 2. REST API Calls
                    │    Redirect / PKCE             │    Authorization: Bearer <JWT>
                    │    http://localhost:8081       │    http://localhost:8080
                    ▼                                ▼
┌────────────────────────────────────────────────────────────────────────┐
│                     DOCKER COMPOSE NETWORK (lynk-net)                  │
│                                                                        │
│   ┌───────────────────────────────┐ ┌──────────────────────────────┐   │
│   │  Keycloak IAM                 │ │  Go REST API                 │   │
│   │  (Container: lynk-keycloak)   │ │  (Container: lynk-api)       │   │
│   │  Port: 8081:8080              │ │  Port: 8080:8080             │   │
│   │                               │ │                              │   │
│   │  - Realm: "lynk"              │ │  - JWT Auth Middleware       │   │
│   │  - Roles: student, employer   │ │  - Validates JWKS from       │   │
│   │  - Email Verification (.edu)  │ │    Keycloak public endpoint  │   │
│   │  - Issues RS256 JWTs          │ │  - Layered Architecture      │   │
│   └───────────────┬───────────────┘ └───────┬──────────────┬───────┘   │
│                   │                         │              │           │
│                   │                         │              │ S3 API    │
│                   │ SQL (keycloak_db)       │ SQL(lynk_db) │ Port 9000 │
│                   ▼                         ▼              ▼           │
│   ┌─────────────────────────────────────────┐ ┌────────────────────┐   │
│   │  PostgreSQL 16+                         │ │  MinIO Storage     │   │
│   │  (Container: lynk-postgres)             │ │  (lynk-minio)      │   │
│   │  Port: 5432                             │ │  Port: 9000 / 9001 │   │
│   │                                         │ │                    │   │
│   │  - keycloak_db: Keycloak IAM state      │ │  Bucket: "resumes" │   │
│   │  - lynk_db: Profiles, Jobs,             │ │  Stores: Resumes   │   │
│   │    Applications, Contracts, Reviews     │ │                    │   │
│   └─────────────────────────────────────────┘ └────────────────────┘   │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Evolutionary Architecture Roadmap

Lynk starts with a synchronous Go backend and Keycloak IAM. Later phases introduce distributed performance tools strictly after the MVP baseline is verified.

```mermaid
flowchart TD
    subgraph Phase1["Phase 1: MVP (Active Baseline)"]
        FE[Next.js App on Host] -->|OIDC PKCE| KC[Keycloak IAM in Docker]
        FE -->|REST with Bearer JWT| API[Dockerised Go REST API]
        API -->|Validate Token via JWKS| KC
        API -->|Raw SQL Migrations| PG[(PostgreSQL 16+)]
        API -->|S3 API| S3[(MinIO: Resumes Bucket)]
    end

    subgraph Phase2["Phase 2: Performance & Protection"]
        REDIS[(Redis)]
        API -.->|JWKS Caching & Rate Limiting| REDIS
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

## 4. Keycloak IAM Integration & Auth Flow

### OpenID Connect (OIDC) Topology
* **Realm:** `lynk`
* **Clients:**
  - `lynk-frontend`: Public client (Next.js), PKCE enabled, standard authorization code flow. Redirect URIs: `http://localhost:3000/*`.
  - `lynk-api`: Bearer-only resource server, verifying incoming JWT claims.
* **Realm Roles:**
  - `student`: Assigned to student accounts.
  - `employer`: Assigned to campus/local employer accounts.
  - `admin`: Platform administrative oversight.
* **Token Standard:** Signed RS256 JSON Web Tokens (JWT).

### End-to-End Authentication Sequence

```mermaid
sequenceDiagram
    autonumber
    actor User as Student / Employer
    participant Next as Next.js Frontend (Host)
    participant KC as Keycloak (Docker :8081)
    participant API as Go REST API (Docker :8080)
    participant PG as PostgreSQL (lynk_db)

    User->>Next: Click Login / Register
    Next->>KC: Redirect to Keycloak OIDC Auth Endpoint (PKCE)
    KC->>User: Render Login / Registration UI (.edu validation)
    User->>KC: Submit Credentials
    KC->>KC: Verify Password / Trigger Email Verification
    KC-->>Next: Redirect to Callback with Auth Code
    Next->>KC: Exchange Code + Code Verifier for Tokens
    KC-->>Next: Return Access Token (JWT), ID Token, Refresh Token
    
    Next->>API: HTTP Request + Authorization: Bearer <access_token>
    API->>API: Keycloak Middleware validates JWT signature via cached JWKS
    API->>API: Check claims (sub, email_verified, realm_access.roles)
    API->>PG: Query / Mutate application data using sub (user UUID)
    PG-->>API: Return DB rows
    API-->>Next: JSON Response
```

### Go API Token Verification
The Go API does **not** make a network call to Keycloak on every incoming request. Instead:
1. On startup (and periodically refreshed with TTL), the Go API fetches and caches Keycloak's public JSON Web Key Set (JWKS) from:
   `http://keycloak:8080/realms/lynk/protocol/openid-connect/certs`
2. Incoming requests with `Authorization: Bearer <token>` are verified locally in-memory:
   - Signature verified against Keycloak RSA public key.
   - `iss` (issuer) matches `http://localhost:8081/realms/lynk` (or internal docker alias).
   - `exp` (expiration) has not elapsed.
3. Injected Context Values:
   - `user_id` (`sub` UUID)
   - `email`
   - `email_verified` (boolean)
   - `roles` (`[]string`: e.g. `["student"]`)

---

## 5. Backend Clean Architecture (Go)

The Go backend enforces strict separation of concerns:

```text
HTTP Request (with Bearer Token)
     │
     ▼
┌───────────────────────────────────────────────────────────────┐
│ 1. Middleware Layer                                           │
│    - CORS Middleware (origins: http://localhost:3000)         │
│    - Keycloak JWT Auth Middleware: validates signature,       │
│      extracts user_id (sub), email, email_verified, roles     │
│    - Injects claims into request context.Context              │
└──────────────────────────────┬────────────────────────────────┘
                               │
                               ▼
┌───────────────────────────────────────────────────────────────┐
│ 2. Handler Layer (Transport / HTTP)                           │
│    - Decodes & validates request JSON bodies / query params   │
│    - Reads authenticated user context from context.Context    │
│    - Maps domain errors to standard HTTP status codes         │
└──────────────────────────────┬────────────────────────────────┘
                               │
                               ▼
┌───────────────────────────────────────────────────────────────┐
│ 3. Service Layer (Business Logic & Invariants)                │
│    - Enforces RBAC & email verification invariants            │
│    - Manages contract state machine transitions               │
│    - Orchestrates database repositories and MinIO storage     │
└──────────────────────────────┬────────────────────────────────┘
                               │
                               ▼
┌───────────────────────────────────────────────────────────────┐
│ 4. Repository & Storage Layer (Persistence)                   │
│    - Repository: Raw SQL queries against PostgreSQL (lynk_db) │
│    - Storage: MinIO client for resume streaming / presigning  │
└───────────────────────────────────────────────────────────────┘
```

### Module Breakdown (`backend/internal/`)

| Package | Responsibility |
| :--- | :--- |
| `auth` | Keycloak JWKS client, JWT signature verification middleware, token claims extractor, user sync handler. |
| `user` | Student and employer profile creation, updates, and portfolio links (keyed by Keycloak `user_id`). |
| `job` | Job posting CRUD, search, filtering by department, budget, and required skills. |
| `application` | Application submission, cover letter, resume linking, and employer review (accept/reject). |
| `contract` | Milestone/contract state machine (`Draft` → `Active` → `Completed` \| `Cancelled`). |
| `review` | Rating (1–5) and written feedback submission following contract completion. |
| `storage` | MinIO client abstraction for uploading, fetching, and generating pre-signed URLs for resumes. |
| `database` | PostgreSQL connection pooling (`pgxpool`), health checks, and raw SQL migration runner. |
| `middleware` | CORS, Keycloak JWT auth, request logging, and panic recovery. |

---

## 6. Database Schema & Relational Model

PostgreSQL 16+ is the source of truth for all application data in the `lynk_db` database.
> **Note:** Credentials and authentication tokens are managed exclusively by Keycloak in `keycloak_db`. Application tables link to users via the Keycloak user UUID (`sub`).

### Entity Relationship Diagram (ERD)

```mermaid
erDiagram
    USERS ||--o| STUDENT_PROFILES : "has"
    USERS ||--o| EMPLOYER_PROFILES : "has"
    USERS ||--o{ JOBS : "posts (employer)"
    USERS ||--o{ APPLICATIONS : "submits (student)"
    JOBS ||--o{ APPLICATIONS : "receives"
    APPLICATIONS ||--o| CONTRACTS : "generates upon acceptance"
    CONTRACTS ||--o{ REVIEWS : "receives"

    USERS {
        uuid id PK "Matches Keycloak sub UUID"
        string email UK
        string role "student | employer | admin"
        timestamptz created_at
        timestamptz updated_at
    }

    STUDENT_PROFILES {
        uuid id PK
        uuid user_id FK,UK "Matches USERS.id"
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
        timestamptz updated_at
    }

    EMPLOYER_PROFILES {
        uuid id PK
        uuid user_id FK,UK "Matches USERS.id"
        string company_or_org
        string contact_name
        text description
        string website
        timestamptz updated_at
    }

    JOBS {
        uuid id PK
        uuid employer_id FK
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
        uuid student_id FK
        text cover_letter
        string resume_key
        string status "pending | accepted | rejected"
        timestamptz created_at
        timestamptz updated_at
    }

    CONTRACTS {
        uuid id PK
        uuid job_id FK,UK
        uuid application_id FK,UK
        uuid employer_id FK
        uuid student_id FK
        numeric agreed_budget
        string status "draft | active | completed | cancelled"
        timestamptz started_at
        timestamptz completed_at
        timestamptz created_at
        timestamptz updated_at
    }

    REVIEWS {
        uuid id PK
        uuid contract_id FK
        uuid reviewer_id FK
        uuid reviewee_id FK
        int rating "1 to 5"
        text comment
        timestamptz created_at
    }
```

---

## 7. Object Storage Architecture (MinIO)

MinIO provides local S3-compatible storage dedicated to student resumes.

### Invariants & Bucket Policy
* **Bucket Name:** `resumes` (strictly isolated to PDF/DOCX student resumes).
* **Access Policy:** Private. Objects cannot be accessed anonymously.
* **Upload Flow:**
  1. Student sends `multipart/form-data` to Go API (`POST /api/v1/profile/student/resume`) with Keycloak Bearer JWT.
  2. Go backend confirms `role == "student"` and `email_verified == true`.
  3. Go backend validates MIME type (`application/pdf`, `application/vnd.openxmlformats-officedocument.wordprocessingml.document`) and max size (5MB).
  4. File is streamed to MinIO: key format `resumes/{student_user_id}/{uuid}-{sanitized_filename}`.
  5. S3 key and metadata saved to `student_profiles`.
* **Download Flow:**
  - Go API generates a 15-minute pre-signed download URL (`GetObjectPresigned`) returned to authorized students and prospective employers.

---

## 8. REST API Contract Specification

All application endpoints are prefixed with `/api/v1`. Authentication is passed via the standard HTTP header:  
`Authorization: Bearer <Keycloak_Access_Token>`

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
    "message": "University email must be verified in Keycloak before applying to jobs."
  }
}
```

### Endpoint Matrix

#### Identity & User State (`/api/v1/auth`)
| Method | Path | Auth | Role | Description |
| :--- | :--- | :---: | :---: | :--- |
| `GET`  | `/auth/me` | Bearer | Any | Return current authenticated user profile, role, and verification status from JWT claims & DB. |
| `POST` | `/auth/sync` | Bearer | Any | Idempotent first-login hook to ensure user record exists in `lynk_db` with selected role. |

*(Note: User registration, login, password resets, and email verification are handled directly by Keycloak at `http://localhost:8081/realms/lynk/account` or via OIDC client redirection).*

#### Student Profiles (`/api/v1/profile/student`)
| Method | Path | Auth | Role | Description |
| :--- | :--- | :---: | :---: | :--- |
| `GET`  | `/profile/student` | Bearer | Student | Get own student profile details. |
| `PUT`  | `/profile/student` | Bearer | Student | Update bio, skills, department, graduation year, portfolio links. |
| `POST` | `/profile/student/resume` | Bearer | Verified Student | Upload resume (PDF/DOCX, max 5MB). Streams to MinIO. |
| `GET`  | `/profile/student/resume` | Bearer | Student | Get pre-signed download URL for own resume. |
| `GET`  | `/profile/student/{id}` | Bearer | Any | View public profile of a student (for employers reviewing applicants). |

#### Employer Profiles (`/api/v1/profile/employer`)
| Method | Path | Auth | Role | Description |
| :--- | :--- | :---: | :---: | :--- |
| `GET`  | `/profile/employer` | Bearer | Employer | Get own employer profile. |
| `PUT`  | `/profile/employer` | Bearer | Employer | Update company/org name, contact info, website, bio. |

#### Jobs (`/api/v1/jobs`)
| Method | Path | Auth | Role | Description |
| :--- | :--- | :---: | :---: | :--- |
| `GET`  | `/jobs` | Public | Any | Browse and search jobs with filters (`department`, `skill`, `min_budget`, `max_budget`, `status`). |
| `POST` | `/jobs` | Bearer | Employer | Create a new job posting. |
| `GET`  | `/jobs/{id}` | Public | Any | Retrieve detailed job view. |
| `PUT`  | `/jobs/{id}` | Bearer | Employer | Update job posting (owner only). |
| `DELETE` | `/jobs/{id}` | Bearer | Employer | Cancel/close job posting (owner only). |

#### Applications (`/api/v1/applications`)
| Method | Path | Auth | Role | Description |
| :--- | :--- | :---: | :---: | :--- |
| `POST` | `/jobs/{id}/applications` | Bearer | Verified Student | Submit application with cover letter and attached resume key. |
| `GET`  | `/jobs/{id}/applications` | Bearer | Employer | List all submitted applications for an employer's job. |
| `GET`  | `/applications/mine` | Bearer | Student | List applications submitted by current student. |
| `PATCH`| `/applications/{id}/status` | Bearer | Employer | Accept or Reject application. Accepting automatically creates a `Contract`. |

#### Contracts (`/api/v1/contracts`)
| Method | Path | Auth | Role | Description |
| :--- | :--- | :---: | :---: | :--- |
| `GET`  | `/contracts` | Bearer | Any | List contracts involving the authenticated user (as student or employer). |
| `GET`  | `/contracts/{id}` | Bearer | Participant | Retrieve detailed contract terms, status, and milestone history. |
| `PATCH`| `/contracts/{id}/status` | Bearer | Participant | Update contract status: `Active` → `Completed` \| `Cancelled`. |

#### Reviews & Ratings (`/api/v1/contracts/{id}/reviews`)
| Method | Path | Auth | Role | Description |
| :--- | :--- | :---: | :---: | :--- |
| `POST` | `/contracts/{id}/reviews` | Bearer | Participant | Submit 1–5 star rating and written review (allowed ONLY when contract status is `Completed`). |
| `GET`  | `/contracts/{id}/reviews` | Bearer | Participant | Get reviews associated with the contract. |
| `GET`  | `/users/{id}/reviews` | Public | Any | Get aggregated ratings and reviews for a user. |

---

## 9. Contract State Machine & Institutional Email Verification Gate

### Contract Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Draft: Employer accepts Application
    Draft --> Active: Both parties confirm terms / Start work
    Active --> Completed: Deliverables verified by Employer
    Active --> Cancelled: Mutual cancellation / Dispute
    Completed --> [*]: Review & Rating unlocked
    Cancelled --> [*]
```

### Institutional Email Verification Gate
* Keycloak enforces email verification (`verifyEmail: true`) requiring students to confirm ownership of their `.edu` or institutional email address.
* The Go API reads the `email_verified` boolean claim from the validated Keycloak JWT.
* If `email_verified == false`:
  - `POST /jobs/{id}/applications` returns `403 Forbidden` (`EMAIL_NOT_VERIFIED`).
  - `POST /profile/student/resume` returns `403 Forbidden` (`EMAIL_NOT_VERIFIED`).

---

## 10. Local Infrastructure Topology (`docker-compose.yml`)

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:16-alpine
    container_name: lynk-postgres
    restart: unless-stopped
    environment:
      POSTGRES_DB: lynk_db
      POSTGRES_USER: lynk_user
      POSTGRES_PASSWORD: lynk_password
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

  keycloak:
    image: quay.io/keycloak/keycloak:24.0
    container_name: lynk-keycloak
    restart: unless-stopped
    command: start-dev --import-realm
    environment:
      KC_DB: postgres
      KC_DB_URL: jdbc:postgresql://postgres:5432/keycloak_db
      KC_DB_USERNAME: lynk_user
      KC_DB_PASSWORD: lynk_password
      KEYCLOAK_ADMIN: admin
      KEYCLOAK_ADMIN_PASSWORD: admin_password
      KC_HEALTH_ENABLED: "true"
    ports:
      - "8081:8080"
    volumes:
      - ./.docker/keycloak/realm-export.json:/opt/keycloak/data/import/realm.json:ro
    depends_on:
      postgres:
        condition: service_healthy
    networks:
      - lynk-net
    healthcheck:
      test: ["CMD-SHELL", "exec 3<>/dev/tcp/localhost/8080 && echo -e 'GET /health/ready HTTP/1.1\\r\\nHost: localhost\\r\\nConnection: close\\r\\n\\r\\n' >&3 && cat <&3 | grep '200 OK'"]
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
    depends_on:
      postgres:
        condition: service_healthy
      minio:
        condition: service_healthy
      keycloak:
        condition: service_healthy
    environment:
      PORT: 8080
      DATABASE_URL: "postgres://lynk_user:lynk_password@postgres:5432/lynk_db?sslmode=disable"
      KEYCLOAK_ISSUER_URL: "http://keycloak:8080/realms/lynk"
      KEYCLOAK_JWKS_URL: "http://keycloak:8080/realms/lynk/protocol/openid-connect/certs"
      S3_ENDPOINT: "http://minio:9000"
      S3_ACCESS_KEY: "minio_admin"
      S3_SECRET_KEY: "minio_password"
      S3_BUCKET: "resumes"
      S3_USE_SSL: "false"
      CORS_ALLOWED_ORIGINS: "http://localhost:3000"
    ports:
      - "8080:8080"
    networks:
      - lynk-net

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
| **Go Unit Tests** | Go `testing` package | Keycloak JWT claims validation, mock JWKS verification, contract state transitions. | `cd backend && go test -v -race ./...` |
| **Go Linting** | `golangci-lint` | Static analysis, idiomatic conventions, errcheck. | `cd backend && golangci-lint run` |
| **Go Integration Tests** | Real Docker Compose Stack | Repository queries in `lynk_db`, Keycloak token validation against live Keycloak JWKS, MinIO S3 streaming. | `cd backend && go test -tags=integration ./...` |
| **Frontend Typecheck** | TypeScript compiler | Ensure TypeScript types and OIDC session types match Go API JSON responses. | `cd frontend && tsc --noEmit` |
| **Frontend Lint & Build**| Next.js / ESLint | App Router layout, Keycloak OIDC provider context, Tailwind UI styling. | `cd frontend && npm run build` |
| **End-to-End Smoke** | `curl` / HTTP test scripts | Full user lifecycle: Keycloak Token Issuance → Sync Profile → Post Job → Apply → Accept → Contract → Review. | `./scripts/smoke-test.sh` |
