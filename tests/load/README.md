# Lynk Backend Load Testing & Capacity Benchmark

This directory contains the automated protocol-level load testing suite for the Lynk Go REST API using [k6](https://k6.io/).

---

## 1. Test Architecture & Stages

The test suite is orchestrated sequentially by `tests/load/run_all.js` across 6 progressive stages:

| Stage | Script | Objective | Target Invariant |
| :--- | :--- | :--- | :--- |
| **Stage 1** | `scripts/01_rate_limit.js` | Single-IP burst & DDoS rejection | Blocks at >120 req/min with sub-10ms response time |
| **Stage 2** | `scripts/02_cache_benchmark.js` | Read throughput and search performance | Evaluates latency across discovery feed and search |
| **Stage 3** | `scripts/03_pool_saturation.js` | PostgreSQL pool contention | Verifies Go worker queue stability above 25 connections |
| **Stage 4** | `scripts/04_user_concurrency.js` | Max user breakpoint test | Progressive ramp up to 350 VUs with realistic think times |
| **Stage 5** | `scripts/05_high_concurrency_stress.js` | High-concurrency stress | 1,000 VUs exercising PgBouncer, Redis cache, covering indexes |
| **Stage 6** | `scripts/06_ultra_concurrency_stress.js` | Ultra-concurrency stress | 2,500 VUs exercising Nginx micro-caching, sharded L1 cache, DB buffers |

The two heavy stages (05, 06) cover the merged high-throughput integration: the sharded L1 cache, SingleFlight request coalescing, Redis L2 with the invalidation bus, keyset pagination, capacity leases, and the safe-method session fast path. Measured results for the current build are recorded in `tests/load/results/REPORT.md`; the architecture behind them is in `docs/architecture/scalability.md`.

Stage 7 (`07_saturation_ceiling.js`) is different: it uses k6's constant-arrival-rate executor with **no think time**, ramping a fixed arrival rate upward so the *server* is the limit rather than the VU count. Pair it with a RAM cap to see what the platform serves within a fixed server budget:

```powershell
node tests/load/ramcap.js 6144 > docker-compose.ramcap.yml   # any integer MiB
docker compose -f docker-compose.yml -f docker-compose.ramcap.yml up -d --force-recreate
node tests/load/run_all.js --stage=07_saturation_ceiling
docker compose -f docker-compose.yml -f docker-compose.ramcap.yml down
```

Or with make: `make stack-ram RAM=6144`, `make load-test-saturation`, `make stack-ram-down`.

Stage 8 (`08_connection_hold.js`) measures concurrent connection capacity: each VU opens a keep-alive connection and holds it open (sleeping ~20s between heartbeats) while the ladder ramps 2,000 -> 64,000 VUs. Run it with `make load-test-connections`. On a single laptop the stack held 32,000+ concurrent connections cleanly; past ~36,000 the k6 client itself is OOM-killed, so a distributed generator is required to find the server's true ceiling.

---

## 2. Prerequisites

You need **either** the native `k6` CLI **or** Docker:

### Option A: Native k6 (Recommended for Windows)
```powershell
winget install k6 --source winget
```

### Option B: Docker (Zero Installation)
If `k6` is not installed on PATH, the master orchestrator automatically detects Docker and runs `grafana/k6` in a temporary container.

---

## 3. Running the Tests

Ensure the Go backend and PostgreSQL database are running:

```bash
docker compose up -d
```

### Execute Complete Load Suite (All 6 Stages Sequentially):
```bash
make load-test
```
or directly with Node.js:
```bash
node tests/load/run_all.js
```

To run a single stage (for example the peak stress stage):
```bash
node tests/load/run_all.js --stage=06_ultra_concurrency_stress
```

### Run Individual Stages:
```bash
# Test rate limiter enforcement only
make load-test-rate-limit

# Test cache & read paths only
make load-test-cache

# Test database pool saturation only
make load-test-pool

# Test maximum concurrent users only
make load-test-concurrency
```

---

## 4. Understanding Output Metrics

* **`http_reqs (rate)`**: Processed requests per second (RPS).
* **`http_req_duration (p95)`**: 95th percentile latency. The target is `< 500ms` under peak load.
* **`http_req_failed`**: Request failure rate. Target is `< 1%` during regular traffic. Note that Stage 1 deliberately triggers HTTP 429 to verify rate limiter rejection.
* **Concurrent Users Formula**:
  $$\text{Real-World Supported Students} = \text{Peak RPS} \times \text{Average Think Time (15s)}$$

---

## 5. Artifacts & Reports

Every run generates individual stage outputs in `tests/load/results/*.json` and produces a consolidated summary report at `tests/load/results/SUMMARY.md`. The full write-up for the current build, including the compute profile and measured RPS/VU/latency table, lives at `tests/load/results/REPORT.md`, and the architecture it validates is documented in `docs/architecture/scalability.md`.
