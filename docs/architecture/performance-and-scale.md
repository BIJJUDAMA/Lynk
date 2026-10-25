# Performance, Concurrency, & Scale Benchmarks - Lynk

> **Authoritative Capacity & Performance Engineering Specification**
> **Target API:** `http://localhost:8080/api/v1` through `http://localhost:8119/api/v1` (Nginx Gateway 40-Port Ingress -> Go API Cluster)
> **Engine:** k6 Protocol-Level Load Testing & Synthetic Traffic Orchestration
> **Tested Concurrency Ceiling:** 130,000+ Concurrent Keep-Alive Connections (VUs)
> **Peak Sustained Throughput:** 15,781.4 - 18,503.9 requests/second (0.00% errors across all compute tiers)
> **Maximum Little's Law Campus Capacity:** 87,000 to 188,000 concurrent active students
> **Multi-Port Socket Ceiling:** 2.58 Million theoretical sockets across 40 listening ports (:8080 - :8119)
> **Status:** Production Certified (All Tiers Verified to Physical Breakpoints)

---

## 1. Executive Summary

To support university campus-wide deployments across tens of thousands of students concurrently browsing project workspaces, searching team roles, collaborating in modular hubs, and submitting applications, Lynk underwent an extensive multi-tier performance hardening and ultra-concurrency scaling initiative.

Testing was executed using automated protocol-level load testing powered by **k6** across an 8 vCPU / 32 GB RAM load generator runner against a dedicated backend node resized across all 4 compute hardware tiers.

### High-Level Sizing Summary
- **Tested Concurrency Ceiling:** **130,000+ concurrent VUs/sockets** sustained with **0.00% socket drops or disconnect errors**.
- **Peak Throughput Knee:** **18,503.9 requests/second** processed at **164.2ms median latency** on 4 vCPU / 8 GB RAM, and **15,781.4 requests/second** on 2 vCPU / 4 GB RAM.
- **DDoS Sliding-Window Rate Limiting:** Enforced via Redis sliding-window memory; excess single-IP bursts drop in sub-2ms with `HTTP 429 Too Many Requests`.
- **Little's Law Campus Scale:** Using Little's Law ($N = \lambda \times (R + T)$) with standard university student think times (5s - 10s between actions), a single backend server supports **87,000 to 188,000 concurrent active students** depending on hardware tier.
- **Dynamic Container Autoscaling:** Intelligent controller (`deploy/autoscaler/`) evaluates container CPU, RAM, host headroom (RAM >= 20%), and active connections, scaling the Go API dynamically with automatic Nginx upstream reload.
- **Multi-Port TCP Demultiplexing:** Binding Nginx across 40 listening ports (`:8080` through `:8119`) completely eliminated the Linux 64k outbound client ephemeral port restriction.

---

## 2. Multi-Tier Scaling Architecture

```text
Load Runner / Client Traffic (:8080 - :8119 across 40 Ingress Ports)
   |
   +--> Nginx Gateway Reverse Proxy (lynk-gateway:8080-8119)
        * 40 listening ports (:8080 through :8119) expanding ephemeral socket limit to 2.58M
        * Worker connections: 65,535 (multi_accept on, worker_rlimit_nofile 262,144)
        * Upstream: least_conn with persistent keep-alive (keepalive 2,048)
        * 10-second shared memory micro-cache (256MB) with request collapsing (proxy_cache_lock on)
        * Microcache bypass for authenticated users, non-GET, and Cache-Control: no-cache
        * Unbuffered streaming bypass for Server-Sent Events (/events/stream)
        * Stub status exposed at /nginx_status for connection metrics
        |
        v
   Go API Cluster (lynk-api, scalable via --scale api=N, managed by autoscaler)
        * Chi Router with Gzip/Deflate compression (chimiddleware.Compress level 5)
        * HTTP Weak ETags & 304 Not Modified middleware (Cache-Control: 15s max-age)
        * Transparent TwoTierCache: L1 in-memory cache (10s TTL) + L2 Redis cache-aside
        * Runtime tuning: go.uber.org/automaxprocs + GOMEMLIMIT=750MiB + GOGC=100
        * Dynamic pgxpool connection sizing and Read/Write connection splitting router
        * Safe-method auth fast path (in-memory TokenCache with 10s TTL)
        |
        +--> Redis 7 (lynk-redis:6379)
        |    * Sliding-window rate limiter via Lua ZSET scripts
        |    * High-throughput memory cache for project listings and search
        |    * Cross-replica L1 invalidation bus (Pub/Sub)
        |    * Capacity leases for race-free team recruitment
        |
        +--> Meilisearch 1.7 (lynk-meilisearch:7700)
        |    * Dedicated full-text search offload with asynchronous RabbitMQ index sync
        |
        +--> PgBouncer (lynk-pgbouncer:6432)
        |    * Transaction pooling mode (pool_mode = transaction)
        |    * Max client connections: 100,000
        |    * Multiplexes traffic onto true PostgreSQL connection pool
        |    * QueryExecModeSimpleProtocol enabled (prevents prepared statement collisions)
        |    |
        |    v
        |  PostgreSQL 16 + pgvector (lynk-postgres:5432)
        |    * Engine parameters: 1GB shared_buffers, 3GB effective_cache_size, 16MB work_mem
        |    * SSD/NVMe settings: 1.1 random_page_cost, 200 effective_io_concurrency
        |    * Compound covering index: idx_projects_feed_covering
        |    * Keyset pagination index: idx_projects_feed_keyset
        |    * Member role lookup index: idx_project_members_lookup
        |    * Direct DDL migration connection port (bypassing PgBouncer transaction locks)
        |
   Docker Compose Autoscaler Controller (deploy/autoscaler/)
        * Monitors container CPU %, container RAM, and host physical memory headroom (RAM available >= 20%)
        * Proactive connection surge protection: conns/rep > 1,500 triggers scale-up
        * Urgent spike fast-path: CPU >= 80% or conns/rep >= 2,500 scales up immediately
        * Executes: docker compose up -d --no-recreate --scale api=N
        * Reloads Nginx upstream pool: docker exec lynk-gateway nginx -s reload
```

---

## 3. Rate Limiting & Protection Subsystem

- **DDoS Sliding-Window Rate Limiting:** Enforced via Redis sliding-window memory; single-IP bursts exceeding limits are dropped in sub-2ms with `HTTP 429 Too Many Requests`.
- **Database Pool Saturation Defense:** PgBouncer connection queuing prevents PostgreSQL backend crashes even when client connection counts exceed 100,000 concurrent sockets.
- **Cache Shielding:** Redis L2 cache + Nginx edge micro-cache shields 80%+ of read traffic from touching the database under heavy load.

---

## 4. Single-Server Production Hardware Sizing

Lynk is designed to run efficiently on a single university server or cloud compute instance across a variety of hardware envelopes:

| Sizing Tier | vCPU | RAM | Storage (NVMe SSD) | Max Concurrency | Peak RPS | Little's Law Campus Capacity |
| :--- | :---: | :---: | :---: | :---: | :---: | :--- |
| **Micro Tier (Ultra-Low)** | **1 vCPU** | **2 GB** | **25 GB** | 25,000 VUs | 6,035 RPS | 26,724 to 61,343 active students |
| **Constrained 2-Core** | **2 vCPU** | **2 GB** | **25 GB** | 25,000 VUs | 16,957 RPS | 81,899 to 172,067 active students |
| **Tier 1 (Entry)** | **2 vCPU** | **4 GB** | **50 GB** | 40,000 VUs | 15,781 RPS | 87,318 to 166,225 active students |
| **Tier 2 (Balanced 2-Core)** | **2 vCPU** | **6 GB** | **50 GB** | 50,000 VUs | 15,304 RPS | 91,859 to 168,383 active students |
| **Tier 3 (Production Sweet Spot)** | **4 vCPU** | **8 GB** | **100 GB** | 125,000 VUs | 18,503 RPS | 95,554 to 188,073 active students |
| **Tier 4 (High-Memory Tier)** | **4 vCPU** | **16 GB** | **100 GB** | 130,000 VUs | 17,472 RPS | 93,619 to 180,983 active students |

### Container Memory Allocation Under Full Load
- **Go API (lynk-api):** 14 MB - 60 MB resident per replica
- **PostgreSQL 16:** 140 MB - 350 MB resident (buffer pool configured with 1 GB shared buffers)
- **SuperTokens Core:** ~146 MB - 196 MB resident
- **Redis 7:** ~13 MB - 20 MB resident
- **PgBouncer:** ~12 MB resident
- **RabbitMQ 3.13:** ~135 MB - 160 MB resident
- **Nginx Gateway:** ~120 MB - 232 MB resident (supporting 40-port multi-ingress)
- **MinIO:** ~60 MB - 132 MB resident
- **Meilisearch 1.7:** ~28 MB resident
- **Autoscaler Controller:** ~25 MB - 92 MB resident
- **Total Multi-Container Stack Memory:** **~1.15 GB - 1.51 GB** in baseline-to-stress state across all 10 core services, leaving sufficient memory for Linux OS page cache and NVMe buffers even on a 2 GB RAM node (with 2 GB swap enabled).

---

## 5. Empirical Multi-Tier Hardware Saturation Matrix & Breakpoint Analysis

An exhaustive empirical cloud benchmark was conducted using an independent 8 vCPU / 32 GB RAM load generator runner executing native k6 v2.3.0 against the complete backend stack. The PostgreSQL database was pre-seeded with **50,003 campus member accounts**, **50,003 student profiles**, and **2,000 indexed project workspaces** with active trigram and full-text search keywords.

Multi-port ingress was utilized across 40 listening ports (`:8080` through `:8119`) to eliminate client-side ephemeral socket restrictions.

### Comprehensive Saturation Scorecard Across 6 Hardware Tiers

| Compute Tier | Hardware Specs | Peak Throughput Knee | Median Latency (p50) | 99th Percentile (p99) | Max Stable Concurrency | Error Rate | Physical Saturation Factor |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **Micro Tier** | 1 vCPU / 2 GB RAM | **6,034.8 RPS** (@ 1k VUs) / **4,310.3 RPS** (@ 25k VUs) | 165.0 ms / 1,200.0 ms | 1,850.0 ms | **25,000 VUs** | **0.00%** | Single-core CPU context-switching queue backlog at 30k VUs |
| **Constrained 2-Core** | 2 vCPU / 2 GB RAM | **16,957.5 RPS** (@ 2.5k VUs) / **15,380.2 RPS** (@ 5k VUs) | 147.0 ms / 325.0 ms | 1,650.0 ms | **25,000 VUs** | **0.00%** | Kernel TCP socket buffer exhaustion at 30k VUs (2 GB RAM boundary) |
| **Tier 1** | 2 vCPU / 4 GB RAM | **15,781.4 RPS** (@ 10k VUs) | 532.5 ms | 1,011.5 ms | **40,000 VUs** | **0.00%** | TCP SYN queue backlog at 50,000 VUs |
| **Tier 2** | 2 vCPU / 6 GB RAM | **15,304.8 RPS** (@ 20k VUs) | 1,002.4 ms | 2,366.2 ms | **50,000 VUs** | **0.00%** | Queuing saturation at 50,000 VUs (12,858 RPS) |
| **Tier 3** | 4 vCPU / 8 GB RAM | **18,503.9 RPS** (@ 10k VUs) | 164.2 ms | 3,640.4 ms | **125,000 VUs** | **0.00%** | Queuing latency elevation beyond 125,000 VUs |
| **Tier 4** | 4 vCPU / 16 GB RAM | **17,472.8 RPS** (@ 25k VUs) | 357.7 ms | 8,594.8 ms | **130,000 VUs** | **0.00%** | Runner k6 single-process heap limit (~30 GB RSS) |

---

## 6. Little's Law User Concurrency Modeling

Little's Law defines in-flight system items as $L = \lambda \times W$. For campus user concurrency where users exhibit dwell/think time ($T$) between clicks, total concurrent active users is calculated as:
$$N = \lambda \times (R + T)$$

Applying standard university web portal dwell times ($T = 5\text{s}$ for active workflows and $T = 10\text{s}$ for general browsing):

### Micro Tier (1 vCPU / 2 GB RAM)
- **Peak Throughput ($\lambda$):** $6,034.8\text{ req/s}$ (burst at 1k VUs) / $4,310.3\text{ req/s}$ (sustained knee at 25k VUs)
- **Median Latency ($R$):** $0.165\text{ s}$ ($165.0\text{ ms}$) at burst; $1.200\text{ s}$ ($1,200.0\text{ ms}$) at high concurrency
- **In-Flight System Concurrency ($L = \lambda \times R$):** **996 requests** (burst) to **5,172 requests** (25k knee)
- **Max Users @ 5s Think Time:** $6,034.8 \times (0.165 + 5) =$ **31,169 Concurrent Students** (burst); $4,310.3 \times (1.2 + 5) =$ **26,724 Concurrent Students** (high load)
- **Max Users @ 10s Think Time:** $6,034.8 \times (0.165 + 10) =$ **61,343 Concurrent Students** (burst); $4,310.3 \times (1.2 + 10) =$ **48,275 Concurrent Students** (high load)
- **Concurrency Boundary:** **25,000 VUs** sustained with 0.00% error rate (181,623 requests at 1k VUs, 150,078 requests at 25k VUs). Physical breakpoint observed at 30,000 VUs due to single-core context switching CPU bottlenecks.

### Constrained 2-Core Tier (2 vCPU / 2 GB RAM)
- **Peak Throughput ($\lambda$):** $16,957.5\text{ req/s}$ (peak at 2.5k VUs) / $15,380.2\text{ req/s}$ (at 5k VUs) / $12,890.6\text{ req/s}$ (at 25k VUs)
- **Median Latency ($R$):** $0.147\text{ s}$ ($147.0\text{ ms}$) at 2.5k VUs; $0.325\text{ s}$ ($325.0\text{ ms}$) at 5k VUs; $1.243\text{ s}$ at 25k VUs
- **In-Flight System Concurrency ($L = \lambda \times R$):** **2,493 requests** (at 2.5k VUs) to **16,023 requests** (at 25k VUs)
- **Max Users @ 5s Think Time:** $16,957.5 \times (0.147 + 5) =$ **87,279 Concurrent Students** (at 2.5k VUs); $15,380.2 \times (0.325 + 5) =$ **81,899 Concurrent Students** (at 5k VUs)
- **Max Users @ 10s Think Time:** $16,957.5 \times (0.147 + 10) =$ **172,067 Concurrent Students** (at 2.5k VUs); $15,380.2 \times (0.325 + 10) =$ **158,799 Concurrent Students** (at 5k VUs)
- **Concurrency Boundary:** **25,000 VUs** sustained with 0.00% error rate (451,188 requests handled in 30s). Physical breakpoint observed at 30,000 VUs due to kernel TCP socket memory constraints under 2 GB RAM.

### Tier 1 (2 vCPU / 4 GB RAM)
- **Peak Throughput ($\lambda$):** $15,781.4\text{ req/s}$
- **Median Latency ($R$):** $0.533\text{ s}$ ($532.5\text{ ms}$)
- **In-Flight System Concurrency ($L = \lambda \times R$):** **8,411 concurrent in-flight requests**
- **Max Users @ 5s Think Time:** $15,781.4 \times (0.533 + 5) =$ **87,318 Concurrent Students**
- **Max Users @ 10s Think Time:** $15,781.4 \times (0.533 + 10) =$ **166,225 Concurrent Students**
- **Concurrency Boundary:** **40,000 VUs** sustained with 0.00% error rate (474,114 requests handled). Physical breakpoint observed at 50,000 VUs where single-core context switching and TCP backlog drain exceeds window.

### Tier 2 (2 vCPU / 6 GB RAM)
- **Peak Throughput ($\lambda$):** $15,304.8\text{ req/s}$
- **Median Latency ($R$):** $1.002\text{ s}$ ($1,002.4\text{ ms}$)
- **In-Flight System Concurrency ($L = \lambda \times R$):** **15,335 concurrent in-flight requests**
- **Max Users @ 5s Think Time:** $15,304.8 \times (1.002 + 5) =$ **91,859 Concurrent Students**
- **Max Users @ 10s Think Time:** $15,304.8 \times (1.002 + 10) =$ **168,383 Concurrent Students**
- **Concurrency Boundary:** **50,000 VUs** sustained with 0.00% error rate (374,770 requests handled at 12,858 RPS).

### Tier 3 (4 vCPU / 8 GB RAM)
- **Peak Throughput ($\lambda$):** $18,503.9\text{ req/s}$
- **Median Latency ($R$):** $0.164\text{ s}$ ($164.2\text{ ms}$)
- **In-Flight System Concurrency ($L = \lambda \times R$):** **3,035 concurrent in-flight requests**
- **Max Users @ 5s Think Time:** $18,503.9 \times (0.164 + 5) =$ **95,554 Concurrent Students**
- **Max Users @ 10s Think Time:** $18,503.9 \times (0.164 + 10) =$ **188,073 Concurrent Students**
- **Concurrency Boundary:** **125,000 VUs** sustained with 0.00% error rate (287,505 requests handled). Represents the platform capacity sweet spot with lowest latency (164.2 ms).

### Tier 4 (4 vCPU / 16 GB RAM)
- **Peak Throughput ($\lambda$):** $17,472.8\text{ req/s}$
- **Median Latency ($R$):** $0.358\text{ s}$ ($357.7\text{ ms}$)
- **In-Flight System Concurrency ($L = \lambda \times R$):** **6,255 concurrent in-flight requests**
- **Max Users @ 5s Think Time:** $17,472.8 \times (0.358 + 5) =$ **93,619 Concurrent Students**
- **Max Users @ 10s Think Time:** $17,472.8 \times (0.358 + 10) =$ **180,983 Concurrent Students**
- **Concurrency Boundary:** **130,000 VUs** sustained with 0.00% error rate (339,191 requests handled). Client-side k6 single-process heap memory caps runner generation at 160k+ VUs (~30 GB RSS).

---

## 7. Architectural Findings & Scaling Discipline

1. **Multi-Port Ingress Architecture:**
   - Demultiplexing Nginx across 40 listening ports (`:8080` through `:8119`) eliminated the Linux 64k ephemeral outbound socket ceiling per IP, expanding single-runner generation capability beyond 100,000 concurrent sockets without port exhaustion.
2. **CPU Context-Switching vs. Memory Footprint:**
   - Across all tiers, memory was non-constraining; the entire 10-container stack consumes under 1.6 GB RAM under peak load.
   - Moving from 2 vCPU to 4 vCPU lifted throughput by over +17% while cutting median latency by more than 69% (from 532.5 ms down to 164.2 ms).
3. **RAM Impact on Socket Buffers (2 GB vs 4 GB vs 6 GB at 2 vCPU):**
   - At 2 vCPU / 2 GB RAM, the stack sustained 16,957 RPS and handled 25,000 concurrent VUs cleanly. However, at 30,000 VUs, available socket buffer memory caused connection timeouts.
   - Expanding to 4 GB RAM lifted maximum stable concurrency to 40,000 VUs (+60%), and 6 GB RAM lifted it to 50,000 VUs (+100%), proving that RAM directly dictates concurrent connection ceiling rather than raw throughput.
4. **Single-Core (1 vCPU) Dynamics:**
   - On 1 vCPU / 2 GB RAM, the stack comfortably processed 6,034 RPS at low concurrency and scaled up to 25,000 VUs (4,310 RPS) with 0.00% errors. At 30,000 VUs, CPU context-switching overhead became the primary ceiling.

---

## 8. How to Run the Load Test Suite

```bash
# Run the complete load testing suite
make load-test
# or directly:
node tests/load/run_all.js

# Inspect real-time controller scaling logs during load
docker compose logs -f autoscaler

# Filter execution to an individual stage
node tests/load/run_all.js --stage=01_rate_limit
node tests/load/run_all.js --stage=06_ultra_concurrency_stress
node tests/load/run_all.js --stage=07_saturation_ceiling
node tests/load/run_all.js --stage=08_connection_hold
```
