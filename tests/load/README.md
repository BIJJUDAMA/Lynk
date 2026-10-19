# Lynk Backend Load Testing & Capacity Benchmark

This directory contains the automated protocol-level load testing suite for the Lynk Go REST API using [k6](https://k6.io/).

---

## 1. Test Architecture & Stages

The test suite is orchestrated sequentially by `tests/load/run_all.js` across 4 progressive stages:

| Stage | Script | Objective | Target Invariant |
| :--- | :--- | :--- | :--- |
| **Stage 1** | `scripts/01_rate_limit.js` | Single-IP burst & DDoS rejection | Blocks at >120 req/min with sub-10ms response time |
| **Stage 2** | `scripts/02_cache_benchmark.js` | Read throughput and search performance | Evaluates latency across discovery feed and search |
| **Stage 3** | `scripts/03_pool_saturation.js` | PostgreSQL pool contention | Verifies Go worker queue stability above 25 connections |
| **Stage 4** | `scripts/04_user_concurrency.js` | Max user breakpoint test | Progressive ramp up to 350 VUs with realistic think times |

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

### Execute Complete Load Suite (All 4 Stages Sequentially):
```bash
make load-test
```
or directly with Node.js:
```bash
node tests/load/run_all.js
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

Every run generates individual stage outputs in `tests/load/results/*.json` and produces a consolidated summary report at `tests/load/results/SUMMARY.md`.
