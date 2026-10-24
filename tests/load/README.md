# Lynk Backend Load Testing & Capacity Benchmark

This directory contains the automated protocol-level load testing suite for the Lynk Go REST API using [k6](https://k6.io/).

---

## 0. Clone and Run

A fresh clone of the repository can run a load test end to end with three commands. The suite brings the Docker Compose stack up itself, waits for the gateway to report healthy, then runs and writes a report.

```bash
git clone <repo-url> && cd Lynk
make setup                # installs git hooks, deps, and k6 (apt on Linux, winget on Windows)
make load-connections     # builds + waits for the stack, runs the connections mode, writes a report
```

What each step does:

- **`make setup`** runs `setup-hooks`, `setup-go`, `setup-frontend`, `setup-ai`, and `setup-k6`. The `setup-k6` target checks for an existing `k6` binary and installs it from the official apt repository on Linux or via `winget` (`k6.k6`) on Windows if it is missing.
- **`make load-connections`** calls `node tests/load/suite.js --mode=connections`. The suite runs `docker compose up -d --build` (`tests/load/stack.js` `upStack`), polls `http://localhost:8080/api/v1/health` until it returns 200 (`waitForReady`, up to 180s), then runs the mode and writes its report.

### Docker k6 fallback

A native `k6` on `PATH` is preferred. When `k6` is absent, `tests/load/k6.js` automatically falls back to the official `grafana/k6` Docker image, mounting the repository at `/lynk` with host networking and forwarding k6 environment variables (`API_BASE_URL`, `TARGET_RPS`, ...). This means the suite works even when `make setup-k6` was skipped, as long as Docker is available.

### Skipping bring-up with `--no-up`

Every suite mode accepts `--no-up`, which skips both the stack bring-up and the readiness wait. Use it when the stack is already running (for example right after `make load-up`):

```bash
node tests/load/suite.js --mode=connections --no-up
```

### Exit codes

The suite gates on its verdict; inspect `$LASTEXITCODE` (PowerShell) or `$?` (bash) after a run:

| Code | Meaning |
| :---: | :--- |
| `0` | OK: a usable result (verdict `SERVER_CEILING`, `FAILURE_KNEE`, or `HEALTHY`; or no failure signal). |
| `1` | Failed verdict: the run is not usable (`CLIENT_BOUND`, `SAFETY_STOP`, `FAILURE`, a non-zero k6 exit, or `ok:false`). |
| `2` | Usage/setup error: bad flags, a failed stack bring-up, the stack not becoming ready, or an unknown mode. |

---

## 1. Test Architecture & Stages (legacy fixed scripts)

> **New in this directory:** the parameterized suite (`tests/load/suite.js`) supersedes the
> fixed 01-08 stages below for capacity work; see Section 2. Stages 01-08 are retained as
> **legacy** fixed scripts and still run via `make load-test`.

The consolidated suite is orchestrated sequentially by `tests/load/run_all.js` across 8 stages:

| Stage | Script | Objective | Target Invariant |
| :--- | :--- | :--- | :--- |
| **Stage 1** | `scripts/01_rate_limit.js` | Single-IP burst & DDoS rejection | Blocks at >120 req/min with HTTP 429 in sub-10ms |
| **Stage 2** | `scripts/02_cache_benchmark.js` | Read throughput and search performance | Evaluates latency across discovery feed and search |
| **Stage 3** | `scripts/03_pool_saturation.js` | PostgreSQL pool contention | Verifies Go worker queue stability above 25 connections |
| **Stage 4** | `scripts/04_user_concurrency.js` | Max user breakpoint test | Progressive ramp up to 350 VUs with realistic think times |
| **Stage 5** | `scripts/05_high_concurrency_stress.js` | High-concurrency stress | 1,000 VUs exercising PgBouncer, Redis cache, covering indexes |
| **Stage 6** | `scripts/06_ultra_concurrency_stress.js` | Ultra-concurrency stress | 2,500 VUs exercising Nginx micro-caching, sharded L1 cache, DB buffers |
| **Stage 7** | `scripts/07_saturation_ceiling.js` | Constant-arrival saturation | Pure arrival rate (no think time) identifying the queueing latency cliff |
| **Stage 8** | `scripts/08_connection_hold.js` | Concurrent connection hold | 2,000 -> 20,000 live keep-alive sockets sustained with 5s heartbeats |

The heavy stages (05, 06, 07, 08) cover the high-throughput architecture: the sharded L1 cache, SingleFlight request coalescing, Redis L2 with the invalidation bus, keyset pagination, capacity leases, the safe-method session fast path, and the dynamic Docker Compose autoscaler. Measured results are recorded in `tests/load/results/SUMMARY.md` and `docs/architecture/performance-and-scale.md`.

Stage 7 (`07_saturation_ceiling.js`) uses k6's constant-arrival-rate executor with **no think time**, ramping a fixed arrival rate upward so the *server* is the limit rather than the VU count. Pair it with a RAM cap to see what the platform serves within a fixed server budget:

```powershell
node tests/load/ramcap.js 6144 > docker-compose.ramcap.yml   # any integer MiB
docker compose -f docker-compose.yml -f docker-compose.ramcap.yml up -d --force-recreate
node tests/load/run_all.js --stage=07_saturation_ceiling
docker compose -f docker-compose.yml -f docker-compose.ramcap.yml down
```

Or with make: `make stack-ram RAM=6144`, `make load-test-saturation`, `make stack-ram-down`.

Stage 8 (`08_connection_hold.js`) measures concurrent connection capacity: each VU opens a keep-alive connection and holds it open with periodic 5s heartbeats while the ladder ramps 2,000 -> 20,000 VUs. Run it with `node tests/load/run_all.js --stage=08_connection_hold`. On a single server the stack sustained 20,000 concurrent connections cleanly with 11.4ms p95 latency and 0.00% socket drops.

### Real-World Multi-Host Cloud Benchmark Results (BookMyHost Dedicated Instances)
When evaluated across two separate 4 vCPU / 5.8 GiB RAM cloud instances (App Server vs Load Generator, 0.33ms inter-host latency):
- **Stage 02 (Cache Read Baseline, 40 VUs):** 189.57 req/s, 0.62ms p50, 1.73ms p95, 0.00% error rate.
- **Stage 04 (Progressive Concurrency, 350 VUs):** 48.34 req/s (with 1-2s think times), 1.21ms p50, 2.49ms p95, 3.65ms p99.
- **Stage 05 (High Concurrency, 1,000 VUs):** 296.32 req/s, 1.09ms p50, 2.17ms p95, 2.98ms p99, 0.00% error rate.
- **Stage 06 (Ultra Concurrency, 2,500 VUs):** 1,461.23 req/s, 1.09ms p50, 2.63ms p95, 4.93ms p99, 0.00% error rate across 119,564 requests.
- **Stage 07 (Constant-Arrival Saturation Ceiling, 4,000 VUs):** 2,300.96 req/s peak throughput (~138,057 req/min), 0.00% HTTP errors across 241,618 requests, 729.46ms p50, 1,652.56ms p95.
- **Host Footprint under 2,500 VUs:** Entire multi-container stack (`api`, `postgres`, `redis`, `pgbouncer`, `gateway`, `supertokens`, `minio`, `ai-api`) used ~1.15 GiB RAM out of 5.79 GiB available (~20% memory utilization).

### Empirical 4-Tier Hardware Scaling Matrix (50,000 Users Seeded)
Tested sequentially using a dedicated 8 vCPU / 32 GB RAM load generator host against an active backend stack seeded with 50,003 users and 2,000 project workspaces:
- **Tier 1 (2 vCPU / 4 GB RAM / 50 GB SSD):** 1,140.5 RPS | p50: 0.48ms | p95: 1,566.9ms | 0.00% failure rate (39,934 requests)
- **Tier 2 (2 vCPU / 6 GB RAM / 50 GB SSD):** 1,145.8 RPS | p50: 0.46ms | p95: 1,608.1ms | 0.00% failure rate (40,328 requests)
- **Tier 3 (4 vCPU / 8 GB RAM / 50 GB SSD):** 2,353.4 RPS | p50: 0.46ms | p95: 732.2ms | 0.00% failure rate (82,372 requests, +106% RPS)
- **Tier 4 (4 vCPU / 16 GB RAM / 50 GB SSD):** 2,291.4 RPS | p50: 0.47ms | p95: 716.9ms | 0.00% failure rate (80,258 requests)

**Conclusion:** Memory is non-constraining (<600 MB resident across 8 containers). Concurrency scales linearly with vCPU count, establishing 4 vCPU / 8 GB RAM as the optimal university production tier.

---

## 2. The Parameterized Suite (`suite.js`)

`node tests/load/suite.js --mode=<mode>` is a single entrypoint that dispatches to one
of four modes. It supersedes the fixed 01-08 stages for capacity measurement; 01-08 remain
as legacy fixed scripts. Each mode is implemented by its own module (`connections.js`,
`resources.js`, `ResourceSampler.js`, `hostmetrics.js`) and the driver reads the Nginx
`stub_status` connection count for server-side context.

| Mode | Driver | What it answers |
| :--- | :--- | :--- |
| `smoke` | inline k6 health script | One VU for 2s against `/health`; a fast liveness check. |
| `throughput` | `scripts/07_saturation_ceiling.js` | Constant-arrival (no think time) saturation ceiling: the server's true RPS limit. |
| `connections` | `connections.searchCeiling` | Concurrent keep-alive connection ceiling, self-searched (doubling then binary search), attributing the wall to server, client, or the safety stop. |
| `ram` | `resources.runRamp` / `runLevels` | RAM/CPU curve while resource-sampling; a ramping run stops at the safety floor or failure knee (`runRamp`, the default). `--levels=500,2000,8000` drives a constant-arrival run at each listed rate and samples one row per level. |

Flags: `--target=N`/`--max=N`/`--dwell=SEC` (connections mode), `--levels=500,2000,8000` (ram mode),
`--safety-mb=N`, `--tolerance=F`, `--iterations=N`, and `--seed=N` (parsed; currently unused).

```bash
make load-suite MODE=smoke
make load-connections        # suite --mode=connections
make load-resources          # suite --mode=ram
make load-throughput         # suite --mode=throughput
```

The gateway exposes server-side connection counts at `GET /nginx_status` (an Nginx
`stub_status` location restricted to the Docker network and localhost). The connections
driver already defaults to `http://localhost:8080/nginx_status`; set
`NGINX_STUB_STATUS_URL` only to override it with a different host or port.

---

## 3. Prerequisites

You need **either** the native `k6` CLI **or** Docker:

### Option A: Native k6 (Recommended for Windows)
```powershell
winget install k6 --source winget
```

### Option B: Docker (Zero Installation)
If `k6` is not installed on PATH, the master orchestrator automatically detects Docker and runs `grafana/k6` in a temporary container.

---

## 4. Running the Tests

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

## 5. Understanding Output Metrics

* **`http_reqs (rate)`**: Processed requests per second (RPS).
* **`http_req_duration (p95)`**: 95th percentile latency. The target is `< 500ms` under peak load.
* **`http_req_failed`**: Request failure rate. Target is `< 1%` during regular traffic. Note that Stage 1 deliberately triggers HTTP 429 to verify rate limiter rejection.
* **Concurrent Users Formula**:
  $$\text{Real-World Supported Students} = \text{Peak RPS} \times \text{Average Think Time (15s)}$$

---

## 6. Artifacts & Reports

Every run generates individual stage outputs in `tests/load/results/*.json` and produces a consolidated summary report at `tests/load/results/SUMMARY.md`. The full write-up for the current build, including the compute profile and measured RPS/VU/latency table, lives at `tests/load/results/REPORT.md`, and the architecture it validates is documented in `docs/architecture/scalability.md`.
