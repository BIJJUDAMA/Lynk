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

- Nginx gateway with `least_conn` load balancing.
- Go circuit breakers around SuperTokens, the AI service, and MinIO.
- Redis sliding-window rate limiting (120 req/min per IP by default) with an in-memory fallback.
- Prometheus metrics (`/metrics`) with latency histograms and pool gauges; structured JSON logs with request correlation IDs.
- Graceful shutdown with bounded contexts on background flushes.
- Kubernetes production manifests (`deploy/k8s/`) with HPA v2 (5-100 replicas), a PodDisruptionBudget, TLS ingress, and a shallow `tcpSocket` liveness probe alongside a DB-only `/readyz`.

---

## 7. Measured Behavior

Measured on a single developer host (Intel i7-13620H, 10 cores / 16 threads, 15.6 GB RAM; Docker Desktop, 16 logical CPUs, ~11.7 GB to the VM), single `lynk-api` container, single PostgreSQL, PgBouncer, Redis, Nginx gateway:

| Concurrent VUs | Throughput | p95 | p99 | Errors |
| :--- | :--- | :--- | :--- | :--- |
| 40 | 173 req/s | 1.8 ms | 4.9 ms | 0.00% |
| 350 | 48 req/s | 4.7 ms | 11.0 ms | 0.00% |
| 1,000 | 289 req/s | 2.3 ms | 3.0 s | 0.00% |
| **2,500** | **1,458 req/s** | **2.1 ms** | **4.8 ms** | **0.00%** |

At peak the Go API used under 51 MiB RSS and never exceeded ~74% of one container CPU; PostgreSQL peaked near 52% CPU. The service is comfortable on a laptop, which is the point: the horizontal story (many replicas behind the gateway, HPA to 100 pods) is what turns this into campus scale, and nothing in the per-request path is a global lock.

That table is VU-bound (each stage has a think time). A **constant-arrival saturation stage** (`07_saturation_ceiling.js`, no think time) removes the client's VU ceiling and measures the server directly. Run twice under hard whole-stack RAM caps (swap disabled), it shows RAM is not the limiting resource:

| Budget (whole stack) | Throughput | p95 | p99 | Errors | Peak stack RSS |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **4 GiB** | **3,462 req/s** | 13.0 ms | 3.0 s | 0.00% | ~0.85 GiB |
| **2 GiB** | **3,618 req/s** | 5.2 ms | 55.4 ms | 0.28% | ~0.53 GiB |

The stack's working set fits in under 1 GiB; per-pod CPU (~3 cores at ~3.5k req/s) is the lever, which is what the HPA scales on. The RAM cap is parameterized by any integer MiB: `node tests/load/ramcap.js <MiB> > docker-compose.ramcap.yml` or `make stack-ram RAM=<MiB>`.

Full numbers and methodology: `tests/load/results/REPORT.md` (and the auto-generated `SUMMARY.md`).

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
| Autoscaling and manifests | `deploy/k8s/base/` |
| Load tests | `tests/load/` |
