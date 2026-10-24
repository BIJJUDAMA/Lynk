# Lynk Scalability Architecture

**Last updated:** 2026-10-20
**Scope:** Single-campus deployment (`@university.edu`), Go REST API behind an Nginx gateway.
**Read with:** `docs/architecture/scalability.md` (this file), `PLAN.md`, `docs/superpowers/specs/2026-10-19-1m-rps-integration-design.md`, `tests/load/results/REPORT.md`.

---

## 1. What "Scalability" Means Here

Lynk is one campus. All students share a single project/gig namespace, one PostgreSQL 16 database, one Redis 7 instance, and one object store. Scaling is therefore not about multi-tenant partitioning; it is about serving a very high read:write ratio (students browsing feeds, opening projects, searching) while keeping writes (applications, acceptances, view counters) correct under bursts.

The architecture is a **defense-in-depth cascade**: each layer absorbs load before the layer behind it sees it.

```
Browser
  |
  v
Nginx gateway (least_conn, 5s micro-cache for public feeds, proxy_cache_lock)
  |
  v
Go API (N replicas)
  |-- Sharded in-memory L1 cache (64 shards, xxhash-partitioned, probed per request)
  |-- SingleFlight request coalescing (collapses concurrent identical cache misses)
  |-- HTTP ETag + Gzip
  |
  +--> Redis L2 cache (cache-aside, Pub/Sub L1 invalidation bus)
  +--> SuperTokens Core (session verification; safe-method fast path in Go memory)
  |
  +--> PgBouncer (transaction pooling, 25 backend connections)
         |
         +--> PostgreSQL 16 (1 GB shared_buffers, pgvector + covering/keyset indexes)
```

---

## 2. The Read Path (the hot path)

Reads dominate and are handled without touching PostgreSQL in the common case:

1. **Nginx micro-cache** (`proxy_cache_lock on`, 5s TTL) collapses duplicate feed requests at the edge.
2. **Sharded L1 cache** (`internal/cache/sharded_l1.go`): 64 xxhash-partitioned shards, each with its own `RWMutex`. Reads take a read lock and bump an atomic hit counter, so a hot key does not serialize the whole cache. Values are defensive copies.
3. **SingleFlight** (`internal/project/service.go`): concurrent requests for the same uncached slug/feed collapse into one database query, preventing cache stampedes.
4. **Redis L2 cache**: cache-aside. A cross-replica invalidation bus (Redis Pub/Sub, loopback-suppressed via node IDs) evicts L1 on writes so replicas do not serve stale data. `ShardedL1Cache.DeletePattern` reuses `MemoryCache`'s glob/prefix semantics.
5. **ETag / Gzip**: list and health responses carry `ETag` and `Cache-Control: public, max-age=15, stale-while-revalidate=45`, so unchanged responses return `304`.

Because the L1 is a `cache.Cacher` (`ShardedL1Cacher` adapter), it slots into the existing two-tier cache used by the user, job, project, and idempotency layers.

---

## 3. The Authentication Path

SuperTokens remains the sole session provider. Every request still goes through `session.GetSession`, with one bounded optimization:

- **Safe-method fast path** (`internal/middleware/auth.go`): `GET`/`HEAD`/`OPTIONS` requests whose access token is already in a short-lived (10s) in-memory `auth.TokenCache` skip the repeated crypto/RPC work.
- **Mutating methods never fast-path**, so SuperTokens anti-CSRF validation is always performed for `POST`/`PUT`/`PATCH`/`DELETE`.
- **The cache key is the token SuperTokens actually verified** (`sessionContainer.GetAccessToken()`), never the request-parsed header, so a client cannot poison another user's cache entry.
- **Token extraction mirrors the SDK exactly** (case-sensitive `Bearer `, `url.QueryUnescape`d single `sAccessToken` cookie, duplicate cookies force a miss), so the fast path and the full path always agree on which token is in play.

Trade-off: there is no server-side logout hook yet, so a revoked token can still read for up to the 10s TTL. This is an accepted window, not a bypass.

---

## 4. The Write Path

Writes are kept few and are made safe against concurrency and network retries:

- **Idempotency-Key middleware**: duplicate network retries of a mutating request replay the stored response (`Idempotent-Replayed: true`) for 24h; an in-flight duplicate gets `409 Conflict`.
- **Capacity leases** (`internal/recruitment/lease.go`): flash recruitment is front-lined by an atomic Redis Lua reservation (with a 300s TTL so a crash cannot pin a project). The database remains authoritative: `AcceptApplicationTx` re-checks capacity under `SELECT ... FOR UPDATE`. The lease is released on any failure via a `defer` guarded by a success flag, using a detached bounded context so a canceled request cannot leak a slot.
- **View counts** (`internal/project/view_batcher.go`): high-frequency page views accumulate in memory and flush in periodic micro-batches (`view_count = view_count + delta`). Flushes use a bounded context and retry-merge on failure, so a stalled database cannot drop views or hang shutdown. Private projects never record views.
- **Sticky primary reads**: a successful mutation sets a 2s cookie/header so that client's next reads go to the primary, eliminating read-after-write replication lag.
- **Transactional outbox**: domain events are written in the same transaction as the state change and dispatched by a worker with retry backoff.

---

## 5. The Data Layer

- **PgBouncer transaction pooling** (port 6432) lets 2,000+ client connections share 25 PostgreSQL backends. Migrations bypass PgBouncer (`MIGRATION_DATABASE_URL`, port 5432) because transaction pooling forbids session-level prepared statements; migrations additionally serialize across replicas with a `pg_advisory_lock` held on a dedicated connection.
- **Indexes**: covering indexes for feed scans (`INCLUDE` payload columns) and a keyset index `(visibility, recruitment_status, category, created_at DESC, id DESC)` so deep pagination is an index range scan, not an offset scan.
- **Keyset pagination**: `GET /api/v1/projects?cursor=<opaque>` uses `(created_at, id) < (...)` with an `id DESC` tiebreaker; the `COUNT(*)` for totals stays cursor-free, and a malformed cursor is rejected with `400 INVALID_CURSOR`.
- **pgvector HNSW** for semantic search.

---

## 6. The Apex Layer (existing foundation)

The controls above sit on top of hardening already in the repo:

- Nginx gateway with `least_conn` load balancing and `worker_connections 65535` (`worker_rlimit_nofile 262144`).
- Single-host dynamic autoscaler (`deploy/autoscaler/controller.py`) in Docker Compose:
  - Continuously samples container CPU %, RAM, host memory headroom, and active connections.
  - Proactively triggers scale-up when `conns_per_replica > 1,500` or `cpu_percent > 65%`.
  - Fast-path urgent scale-up when CPU >= 80% or connections/rep >= 2,500, bypassing sustained timers.
  - Dynamically scales Go API replicas (2 to 6) and executes non-disruptive Nginx upstream reloads.
- Go circuit breakers around SuperTokens, the AI service, and MinIO.
- Redis sliding-window rate limiting (120 req/min per IP by default) with an in-memory fallback.
- Prometheus metrics (`/metrics`) with latency histograms and pool gauges; structured JSON logs with request correlation IDs.
- Graceful shutdown with bounded contexts on background flushes.

---

## 7. Measured Behavior

Audited on a dedicated Linux host running the full Docker Compose stack (PostgreSQL 16 with 1 GB `shared_buffers`, PgBouncer with 25 backends, Redis 7, Nginx gateway, SuperTokens Core, MinIO, and autoscaled Go API replicas):

### 7.1 Consolidated 8-Stage Load Results

| Stage Index & Name | Total Requests | Throughput | p95 Latency | p99 Latency | Error Rate | Peak CPU | Peak RAM | Status |
| :--- | :---: | :---: | :---: | :---: | :---: | :---: | :---: | :--- |
| **01. Rate Limiting (DDoS Burst)** | 300 | 30.0 req/s | **1.8 ms** | 3.3 ms | 100.00% | 30.8% | 662 MB | **PASS (Limiter Enforced)** |
| **02. Cache & Read Benchmark** | 7,131 | 177.1 req/s | **1.2 ms** | 2.2 ms | 0.00% | 72.7% | 1,184 MB | **PASS** |
| **03. DB Pool Saturation** | 6,351 | 157.9 req/s | **675 us** | 785 us | 0.00% | 15.6% | 1,165 MB | **PASS** |
| **04. Max User Breakpoint** | 4,866 | 48.5 req/s | **2.5 ms** | 4.0 ms | 0.00% | 41.6% | 1,177 MB | **PASS** |
| **05. High-Concurrency Stress** | 24,688 | 295.3 req/s | **2.0 ms** | 2.7 ms | 0.00% | 87.4% | 1,217 MB | **PASS (1,000 VUs)** |
| **06. Ultra-Concurrency Stress** | 119,744 | 1,464.1 req/s | **2.2 ms** | 3.5 ms | 0.00% | 261.2% | 1,242 MB | **PASS (2,500 VUs)** |
| **07. Constant-Arrival Saturation** | 335,340 | 3,193.4 req/s | **755.8 ms** | 1.01 s | 0.00% | 611.8% | 1,908 MB | **WARN (Latency High)** |
| **08. Concurrent Connection Hold** | 254,558 | 2,042.3 req/s | **11.4 ms** | 55.3 ms | 0.00% | 368.2% | 1,648 MB | **PASS (20,000 VUs)** |

### 7.2 Certified Operating Zones

To prevent ungrounded capacity assertions, capacity is divided into three distinct operational regimes:

1. **Zone 1: Certified Sustainable Operating Capacity (Green Zone)**
   - **Throughput:** `2,042.3 req/sec` | **Latency SLA:** `p95 = 11.4 ms` | **Error Rate:** `0.00%`
   - **Campus Scale:** **~20,400 to 30,600 concurrent active students** (Little's Law assuming 10s to 15s pause between clicks).
   - Under this regime, the system operates with sub-15ms response times and zero queueing backpressure.

2. **Zone 2: Saturation Ingestion Ceiling (Yellow Zone - Stress Limit)**
   - **Ingestion Peak:** `3,193.4 req/sec` | **Latency Profile:** `p95 = 755.8 ms` (queueing delay under constant arrival).
   - **Operational Limit:** Production workloads must remain below 50% of this saturation cliff (~1,600 req/s) to guarantee Green-Zone SLAs.

3. **Zone 3: Connection Concurrency Limit**
   - **Simultaneous Keep-Alive Sockets:** **20,000 live connections** held simultaneously with periodic heartbeats and 0.00% socket drops.
   - Proves Nginx worker limits and OS socket backlogs comfortably sustain a full campus body connected simultaneously.

### 7.3 Single-Server Production Sizing

| Sizing Tier | vCPU | RAM | Storage (NVMe SSD) | Supported Campus Scale |
| :--- | :---: | :---: | :---: | :--- |
| **Recommended (Sweet Spot)** | **8 vCPU** | **16 GB - 32 GB** | **250 GB - 500 GB** | 20,000 - 40,000 students (50%+ CPU headroom at 2,000 RPS) |
| **Minimum Viable** | **4 vCPU** | **8 GB** | **50 GB** | Up to 15,000 students (~1,000 RPS sustained; stack uses ~3.8 GB RAM) |

Full numbers and methodology: `docs/architecture/performance-and-scale.md` and `tests/load/results/SUMMARY.md`.

---

## 8. Where to Change What

| Concern | File |
| :--- | :--- |
| L1 shards, TTLs, `DeletePattern` | `backend/internal/cache/sharded_l1.go`, `sharded_l1_adapter.go` |
| Two-tier wiring and cleanup | `backend/cmd/api/app_cache.go` |
| Session fast path, token extraction | `backend/internal/middleware/auth.go`, `backend/internal/auth/token_extract.go` |
| Token cache | `backend/internal/auth/token_cache.go` |
| Cache stampede control | `backend/internal/project/service.go` (SingleFlight) |
| Capacity leases | `backend/internal/recruitment/lease.go`, `service.go` |
| View batching | `backend/internal/project/view_batcher.go` |
| Keyset pagination | `backend/internal/project/cursor.go`, `repository.go` |
| Feed/keyset indexes | `backend/migrations/000020_*.sql`, `000021_*.sql` |
| Autoscaling controller | `deploy/autoscaler/` |
| Load tests | `tests/load/` |
