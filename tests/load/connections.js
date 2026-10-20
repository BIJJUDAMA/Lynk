// Self-searching concurrent-connection ceiling driver for Lynk.
//
// Answers "how many concurrent keep-alive connections can the stack hold?"
// without a hand-maintained tier list. The driver probes upward (doubling) until
// the first failure, then binary-searches the gap between the last good target
// and the first bad target until it is within a relative tolerance.
//
// Three independent failure modes are distinguished so the reported ceiling is
// attributed correctly:
//   - SERVER_CEILING: the server rejected/failed requests while the client was healthy.
//   - CLIENT_BOUND:   the load generator itself died (exit code, dropped
//                     iterations, or local ephemeral-port TIME_WAIT saturation).
//   - SAFETY_STOP:    the host safety sampler tripped (free memory floor).
//
// Controller ruling: client-death signals are evaluated BEFORE server failure,
// so a client-bound run is never mislabeled as a server ceiling. A k6 threshold
// exit (code 99) is explicitly NOT a client death: the server-saturation probe
// trips the failure threshold, and treating that exit as a crash would invert
// the very ceiling this driver exists to report.
//
// Every external effect is injectable through a `deps` seam so the search logic
// is unit-testable without k6 or a live stack. The real defaults are best-effort
// and never throw; a failed probe degrades to null/0.
//
// 100% pure ASCII

'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const hostmetrics = require('./hostmetrics.js');
const k6 = require('./k6.js');

// Server request-failure rate above which the server is considered saturated.
const FAILURE_RATE_THRESHOLD = 0.05;

// Client-side TIME_WAIT sockets above which the load generator is considered
// port-exhausted rather than the server being saturated.
const TIME_WAIT_HIGH = 20000;

// Fraction of the ephemeral port range at which the client is considered
// port-bound when TIME_WAIT is not readable on this host.
const PORT_RANGE_NEAR_FRACTION = 0.8;

// k6 exits with this code when a threshold is breached. That is a metric
// verdict, not a client crash, so it must never be read as a client death.
const K6_THRESHOLD_EXIT = 99;

const DEFAULT_STUB_STATUS_URL =
  process.env.NGINX_STUB_STATUS_URL || 'http://localhost:8080/nginx_status';

// Classify a single probe's signals into a verdict.
//
// Precedence (client-death before server-failure):
//   1. tripped                          -> SAFETY_STOP
//   2. genuine client crash (exit != 0 and != k6 threshold exit), or dropped
//      iterations, or client port exhaustion -> CLIENT_BOUND
//   3. failedRate > threshold           -> SERVER_CEILING
//   4. otherwise                        -> healthy (verdict null)
//
// A k6 threshold exit (99) is deliberately NOT a client-death signal: without
// this, a server-saturation probe (which trips the threshold) would be inverted
// into a client bound -- exactly the misattribution this driver prevents.
function classify(signals) {
  const s = signals || {};

  if (s.tripped) {
    return { verdict: 'SAFETY_STOP', reason: 'host safety floor tripped' };
  }

  const exited =
    s.exitCode !== undefined && s.exitCode !== null && s.exitCode !== 0;
  if (exited && s.exitCode !== K6_THRESHOLD_EXIT) {
    return {
      verdict: 'CLIENT_BOUND',
      reason: `load client exited with code ${s.exitCode}`,
    };
  }

  if (Number(s.dropped) > 0) {
    return {
      verdict: 'CLIENT_BOUND',
      reason: `load client dropped ${s.dropped} iterations`,
    };
  }

  if (s.clientPortsExhausted) {
    return {
      verdict: 'CLIENT_BOUND',
      reason: 'client ephemeral port range exhausted',
    };
  }

  if (s.timeWaitHigh) {
    return {
      verdict: 'CLIENT_BOUND',
      reason: 'client ephemeral-port TIME_WAIT saturation',
    };
  }

  if (Number(s.failedRate) > FAILURE_RATE_THRESHOLD) {
    return {
      verdict: 'SERVER_CEILING',
      reason: `server failed ${(Number(s.failedRate) * 100).toFixed(1)}% of requests`,
    };
  }

  return { verdict: null, reason: 'healthy' };
}

// Build the classify() input for one probe run, folding in live sampler state.
function readSignals(run, deps) {
  const r = run || {};
  return {
    exitCode: r.exitCode,
    failedRate: Number(r.failedRate) || 0,
    dropped: Number(r.dropped) || 0,
    tripped: !!(deps.sampler && deps.sampler.tripped),
    timeWaitHigh: !!r.timeWaitHigh,
    clientPortsExhausted: !!r.clientPortsExhausted,
  };
}

// Search for the concurrent-connection ceiling.
//
// opts: { max, dwell, tolerance = 0.05, iterations = 8, start = 500 }
// deps: { runK6(target, dwellSec), serverConns(), sampler }
//
// Returns { verdict, n, bracket, reason, serverConns } where n is the highest
// target sustained without failure and bracket is [lastGood, firstBad].
function searchCeiling(opts, deps) {
  const options = opts || {};
  const max = options.max === undefined ? 64000 : options.max;
  const dwell = options.dwell === undefined ? 20 : options.dwell;
  const tolerance = options.tolerance === undefined ? 0.05 : options.tolerance;
  const iterations = options.iterations === undefined ? 8 : options.iterations;
  const start = options.start === undefined ? 500 : options.start;

  const probe = (target) => {
    if (deps.sampler && typeof deps.sampler.tick === 'function') {
      deps.sampler.tick();
    }
    const run = deps.runK6(target, dwell) || {};
    const signals = readSignals(run, deps);
    return { run, signals, cls: classify(signals) };
  };

  let lo = 0;
  let loConns = null;
  let hi = null;
  let verdict = null;
  let reason = 'no failure observed within max';

  // Phase 1: exponential probe from a small start until the first failure.
  let target = start;
  while (target <= max) {
    const p = probe(target);
    if (p.cls.verdict !== null) {
      hi = target;
      verdict = p.cls.verdict;
      reason = p.cls.reason;
      break;
    }
    lo = target;
    loConns = deps.serverConns();
    target *= 2;
  }

  if (hi === null) {
    return { verdict, n: lo, bracket: [lo, null], reason, serverConns: loConns };
  }

  // A safety trip is authoritative: stop immediately, do not push further.
  if (verdict === 'SAFETY_STOP') {
    return { verdict, n: lo, bracket: [lo, hi], reason, serverConns: loConns };
  }

  // Phase 2: binary search the [lastGood, firstBad] gap.
  let iters = 0;
  while (
    iters < iterations &&
    hi - lo > 1 &&
    hi - lo > tolerance * Math.max(lo, 1)
  ) {
    iters += 1;
    const mid = Math.floor((lo + hi) / 2);
    if (mid <= lo || mid >= hi) break;

    const p = probe(mid);

    if (p.cls.verdict === 'SAFETY_STOP') {
      verdict = 'SAFETY_STOP';
      reason = p.cls.reason;
      hi = mid;
      break;
    }

    if (p.cls.verdict === null) {
      lo = mid;
      loConns = deps.serverConns();
    } else {
      hi = mid;
      verdict = p.cls.verdict;
      reason = p.cls.reason;
    }
  }

  return { verdict, n: lo, bracket: [lo, hi], reason, serverConns: loConns };
}

// --- Real (non-injected) dependency implementations -----------------------

// Normalize a k6 summary metric node to its values object.
function metricValues(node) {
  if (!node) return null;
  if (node.values && typeof node.values === 'object') return node.values;
  return node;
}

// Parse a k6 --summary-export JSON document into the signals the driver needs.
function parseSummary(summary) {
  const metrics = (summary && summary.metrics) || {};

  const failed = metricValues(metrics.http_req_failed);
  let failedRate = 0;
  if (failed) {
    if (typeof failed.value === 'number') failedRate = failed.value;
    else if (typeof failed.rate === 'number') failedRate = failed.rate;
  }

  const dropped = metricValues(metrics.dropped_iterations);
  let droppedCount = 0;
  if (dropped) {
    if (typeof dropped.count === 'number') droppedCount = dropped.count;
    else if (typeof dropped.value === 'number') droppedCount = dropped.value;
  }

  const vusMax = metricValues(metrics.vus_max);
  let vus = null;
  if (vusMax) {
    if (typeof vusMax.value === 'number') vus = vusMax.value;
    else if (typeof vusMax.max === 'number') vus = vusMax.max;
  }

  return { failedRate, dropped: droppedCount, vusMax: vus };
}

// Parse Nginx stub_status body into the active connection count.
function parseStubStatus(body) {
  if (!body) return null;
  const m = String(body).match(/Active connections:\s*(\d+)/i);
  if (!m) return null;
  const n = Number(m[1]);
  return Number.isFinite(n) ? n : null;
}

// Best-effort client-side port-pressure signal.
//
// Preferred probe: the host TIME_WAIT socket count. When the host cannot report
// it (timeWaitCount() returns null -- unsupported platform), fall back to
// comparing live non-loopback egress connections against the ephemeral port
// range, so a port-bound client is still classified CLIENT_BOUND and never
// mistaken for the server's ceiling.
function clientPortSignal() {
  const tw = hostmetrics.timeWaitCount();
  if (typeof tw === 'number') {
    return { timeWaitHigh: tw >= TIME_WAIT_HIGH, clientPortsExhausted: false };
  }

  const range = hostmetrics.ephemeralPortRange();
  const conns = hostmetrics.primaryEgressConnCount();
  let exhausted = false;
  if (range && typeof conns === 'number') {
    const size = range.end - range.start + 1;
    if (size > 0) {
      exhausted = conns >= Math.floor(size * PORT_RANGE_NEAR_FRACTION);
    }
  }
  return { timeWaitHigh: false, clientPortsExhausted: exhausted };
}

// Synchronously read Nginx stub_status (dep seam requires number|null).
// Best-effort: any failure returns null.
function serverConns(url) {
  const target = url || DEFAULT_STUB_STATUS_URL;
  const out = spawnSync('curl', ['-s', '--max-time', '3', target], {
    encoding: 'utf8',
    timeout: 5000,
    windowsHide: true,
  });
  if (out.error || out.status !== 0) return null;
  return parseStubStatus(out.stdout);
}

// Prepare the shared run inputs that are path/env only: a fresh summary path
// and the k6 script/env. Deliberately free of cmd/args so an async caller does
// not build (and detect for) a native invocation it never uses.
function prepareEnv(target, dwellSec) {
  const resultsDir = path.join(__dirname, 'results');
  fs.mkdirSync(resultsDir, { recursive: true });
  const outFile = path.join(resultsDir, '.connections.summary.json');
  try {
    fs.unlinkSync(outFile);
  } catch (e) {
    // no previous summary
  }

  const script = path.join(__dirname, 'scripts', 'hold.js');
  const apiUrl = process.env.API_BASE_URL || 'http://localhost:8080/api/v1';
  // Forward ONLY the variables hold.js reads. buildK6Args turns each entry into
  // a `-e KEY=VAL` on the Docker path, so spreading the whole host environment
  // here would leak secrets and clobber the container PATH with host paths.
  return {
    outFile,
    script,
    env: {
      API_BASE_URL: apiUrl,
      TARGET_VUS: String(target),
      DWELL: String(dwellSec),
    },
  };
}

// Prepare the synchronous k6 invocation: the shared inputs plus the resolved
// cmd/args for the one spawnSync call.
function prepareRun(target, dwellSec) {
  const run = prepareEnv(target, dwellSec);
  const { cmd, args } = k6.buildK6Args({ script: run.script, summaryExport: run.outFile, env: run.env });
  return { outFile: run.outFile, script: run.script, cmd, args, env: run.env };
}

// Fold a finished k6 run into the driver's signal shape. Shared by the sync and
// async runners so both classify identically.
function buildRunResult(outFile, target, status) {
  let failedRate = 0;
  let dropped = 0;
  let vusMax = target;
  let summaryPresent = false;
  try {
    const raw = fs.readFileSync(outFile, 'utf8');
    const parsed = parseSummary(JSON.parse(raw));
    summaryPresent = true;
    failedRate = parsed.failedRate;
    dropped = parsed.dropped;
    vusMax = parsed.vusMax != null ? parsed.vusMax : target;
  } catch (e) {
    // summary unavailable; fall back to exit-code-only signal
  }

  // k6 exits 99 when a threshold is breached. When the summary exists (or the
  // code is the threshold code) the run completed and must be classified from
  // its parsed metrics, never read as a client death. Only a genuine crash --
  // no summary JSON produced -- keeps a non-zero exit as the client-death signal.
  let exitCode = status === null ? 1 : status;
  if (exitCode === K6_THRESHOLD_EXIT || summaryPresent) {
    exitCode = 0;
  }

  const ports = clientPortSignal();
  return {
    exitCode,
    failedRate,
    dropped,
    vusMax,
    timeWaitHigh: ports.timeWaitHigh,
    clientPortsExhausted: ports.clientPortsExhausted,
  };
}

// Real synchronous runner: spawn k6 on scripts/hold.js with the target/dwell
// from env and parse the exported summary. Best-effort parsing; a missing
// summary yields 0s. NOTE: spawnSync blocks the event loop, so a live sampler
// cannot tick during this run. Prefer runK6Async when a supervisor is present.
function runK6(target, dwellSec) {
  const run = prepareRun(target, dwellSec);
  const proc = spawnSync(run.cmd, run.args, {
    cwd: path.resolve(__dirname, '../..'),
    stdio: 'inherit',
    env: run.env,
  });
  return buildRunResult(run.outFile, target, proc.status);
}

// Asynchronous runner: spawn k6 without blocking the event loop so a live
// supervisor can keep sampling and interrupt the run. Resolves to the same
// shape as runK6. opts.onChild(child) receives the ChildProcess so the caller
// can read child.pid and call child.kill().
function runK6Async(target, dwellSec, opts) {
  const options = opts || {};
  const run = prepareEnv(target, dwellSec);
  return k6
    .spawnK6({
      script: run.script,
      summaryExport: run.outFile,
      env: run.env,
      onChild: options.onChild,
    })
    .then((proc) => buildRunResult(run.outFile, target, proc.exitCode));
}

// The real dependency set used when the driver runs against a live stack.
// `sampler` must be supplied by the caller so its safety latch is shared.
function defaultDeps(sampler) {
  return {
    runK6,
    serverConns,
    sampler: sampler || { tripped: false, tick() {} },
  };
}

module.exports = {
  searchCeiling,
  classify,
  parseSummary,
  parseStubStatus,
  clientPortSignal,
  serverConns,
  runK6,
  runK6Async,
  defaultDeps,
  prepareEnv,
  prepareRun,
  FAILURE_RATE_THRESHOLD,
  TIME_WAIT_HIGH,
  PORT_RANGE_NEAR_FRACTION,
  K6_THRESHOLD_EXIT,
};
