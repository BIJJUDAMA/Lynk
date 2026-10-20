// Parameterized load test suite entrypoint for Lynk.
// Dispatches to one of several load-testing modes; each mode is wired in by
// its own module (Tasks 2-5). This file owns argument parsing and mode routing.
//
// The ram paths run k6 asynchronously under a live supervisor so the
// ResourceSampler can keep sampling and interrupt a run that breaches the host
// safety floor. Connections/throughput keep their existing behavior.
// 100% pure ASCII

'use strict';

const { spawnSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const k6 = require('./k6.js');
const connections = require('./connections.js');
const resources = require('./resources.js');
const stack = require('./stack.js');
const { writeReport, exitCodeFor } = require('./report.js');
const { ResourceSampler } = require('./ResourceSampler.js');

const MODES = ['throughput', 'connections', 'ram', 'smoke'];

const DEFAULT_API_URL = 'http://localhost:8080/api/v1';

// Repository root: the cwd every k6/docker invocation runs from.
const ROOT = path.resolve(__dirname, '../..');

const DEFAULTS = {
  mode: 'smoke',
  levels: null,
  target: null,
  max: null,
  dwell: null,
  safetyMb: 4096,
  tolerance: 0.05,
  iterations: 8,
  seed: 1,
  noUp: false,
};

// Windows default install paths for k6, mirroring run_all.js.
const extraPaths = [
  'C:\\Program Files\\k6',
  path.join(process.env.LOCALAPPDATA || '', 'Microsoft\\WinGet\\Links'),
  path.join(process.env.USERPROFILE || '', 'AppData\\Local\\Microsoft\\WinGet\\Links'),
];
process.env.PATH = `${extraPaths.join(path.delimiter)}${path.delimiter}${process.env.PATH}`;

function parseLevels(raw) {
  const parts = String(raw).split(',').map((s) => s.trim()).filter((s) => s.length > 0);
  return parts.map((p) => {
    const n = Number(p);
    if (!Number.isInteger(n)) {
      throw new Error(`invalid level "${p}": levels must be comma-separated integers`);
    }
    return n;
  });
}

// Coerce a flag to a positive integer, rejecting NaN, non-integers and <= 0.
function parsePositiveInt(name, raw) {
  const n = Number(raw);
  if (!Number.isInteger(n) || n <= 0) {
    throw new Error(`invalid ${name} "${raw}": must be a positive integer`);
  }
  return n;
}

// Coerce a flag to a positive finite number (fractions allowed), rejecting NaN
// and <= 0.
function parsePositiveNumber(name, raw) {
  const n = Number(raw);
  if (!Number.isFinite(n) || n <= 0) {
    throw new Error(`invalid ${name} "${raw}": must be a positive number`);
  }
  return n;
}

function parseArgs(argv) {
  const opts = { ...DEFAULTS };

  for (const arg of argv) {
    if (!arg.startsWith('--')) continue;
    const eq = arg.indexOf('=');
    const key = eq === -1 ? arg.slice(2) : arg.slice(2, eq);
    const value = eq === -1 ? '' : arg.slice(eq + 1);

    switch (key) {
      case 'mode':
        if (!MODES.includes(value)) {
          throw new Error(`unknown mode "${value}"; expected one of ${MODES.join(', ')}`);
        }
        opts.mode = value;
        break;
      case 'levels':
        opts.levels = parseLevels(value);
        break;
      case 'target':
        opts.target = parsePositiveInt('target', value);
        break;
      case 'max':
        opts.max = parsePositiveInt('max', value);
        break;
      case 'dwell':
        opts.dwell = parsePositiveInt('dwell', value);
        break;
      case 'safety-mb':
        opts.safetyMb = parsePositiveInt('safety-mb', value);
        break;
      case 'tolerance':
        opts.tolerance = parsePositiveNumber('tolerance', value);
        break;
      case 'iterations':
        opts.iterations = parsePositiveInt('iterations', value);
        break;
      case 'seed':
        opts.seed = Number(value);
        break;
      case 'no-up':
        opts.noUp = true;
        break;
      default:
        throw new Error(`unknown flag "${arg}"`);
    }
  }

  return opts;
}

// A short, self-contained smoke scenario: one VU for two seconds against the
// API health endpoint. Written to a temp file so the repo carries no extra
// script, then removed after the run.
const SMOKE_SCRIPT = `import http from 'k6/http';
import { check } from 'k6';

export const options = {
  vus: 1,
  duration: '2s',
  thresholds: {
    checks: ['rate>0.99'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  const res = http.get(\`\${__ENV.API_BASE_URL}/health\`);
  check(res, { 'health status is 200': (r) => r.status === 200 });
}
`;

// Default smoke runner: spawn k6 synchronously with the built argv and env.
// Injected in tests so a smoke verdict can be exercised without a real k6.
function defaultRunK6({ cmd, args, env }) {
  return spawnSync(cmd, args, { cwd: ROOT, stdio: 'inherit', env });
}

function runSmoke(opts, deps) {
  const d = deps || {};
  const dir = path.join(__dirname, 'results');
  fs.mkdirSync(dir, { recursive: true });
  const script = path.join(dir, '.smoke.js');
  fs.writeFileSync(script, SMOKE_SCRIPT, 'utf8');

  let status = 1;
  try {
    const apiUrl = process.env.API_BASE_URL || DEFAULT_API_URL;
    // Forward only API_BASE_URL through buildK6Args: the Docker form turns each
    // env entry into a `-e KEY=VAL`, so passing the whole host environment here
    // would leak secrets and override the container PATH with Windows paths. The
    // native form is unchanged (buildK6Args emits no flags) and still reads the
    // value from the child process environment.
    const { cmd, args } = k6.buildK6Args({ script, env: { API_BASE_URL: apiUrl } });
    const runK6 = d.runK6 || defaultRunK6;
    const proc = runK6({ cmd, args, env: { ...process.env, API_BASE_URL: apiUrl } });
    status = proc && proc.status !== null && proc.status !== undefined ? proc.status : 1;
  } finally {
    try {
      fs.unlinkSync(script);
    } catch (e) {
      // best-effort cleanup
    }
  }

  const result = { mode: 'smoke', exitCode: status, ok: status === 0 };
  return reportResult(result, d);
}

// Print one driver's result as JSON so a run is self-describing.
function emit(result) {
  process.stdout.write(`${JSON.stringify(result, null, 2)}\n`);
}

// Where every mode drops its report artifacts (git-ignored).
const resultsDir = path.join(__dirname, 'results');

// Persist one mode's result as JSON + Markdown and print where they landed.
// Returns the mode's exit code so callers can gate on it directly.
function reportResult(result, deps) {
  const d = deps || {};
  const write = d.report || writeReport;
  const { jsonPath, mdPath } = write(result, { dir: resultsDir, stamp: Date.now() });
  const out = d.out || process.stdout;
  out.write(`report json: ${jsonPath}\n`);
  out.write(`report md:   ${mdPath}\n`);
  return exitCodeFor(result);
}

// Return the first finite numeric field present in obj, else the fallback.
function pickNumber(obj, keys, fallback) {
  if (obj) {
    for (const key of keys) {
      const value = obj[key];
      if (typeof value === 'number' && Number.isFinite(value)) return value;
    }
  }
  return fallback;
}

// Parse a k6 --summary-export document into the fields the suite reports:
// failure rate, latency p95, and the achieved request rate (RPS).
function parseSummaryResult(summary) {
  const metrics = (summary && summary.metrics) || {};
  return {
    failedRate: pickNumber(metrics.http_req_failed, ['value', 'rate'], 0),
    p95: pickNumber(metrics.http_req_duration, ['p(95)'], null),
    rps: pickNumber(metrics.http_reqs, ['rate', 'value'], null),
  };
}

// Read and parse a summary-export file; a missing/invalid file yields 0/null.
function parseSummaryFile(outFile) {
  try {
    return parseSummaryResult(JSON.parse(fs.readFileSync(outFile, 'utf8')));
  } catch (e) {
    return { failedRate: 0, p95: null, rps: null };
  }
}

// Arrival job: drive k6's arrival executor by reusing the Stage 7 saturation
// script, then parse its exported summary. When opts.arrivalTarget is set the
// script is told to hold that constant arrival rate (TARGET_RPS); otherwise the
// script falls back to its built-in ramp. Best-effort parsing: a missing summary
// leaves the exit code as the signal.
async function runArrivalJob(opts, onChild) {
  const options = opts || {};
  const resultsDir = path.join(__dirname, 'results');
  fs.mkdirSync(resultsDir, { recursive: true });
  const outFile = path.join(resultsDir, '.arrival.summary.json');
  try {
    fs.unlinkSync(outFile);
  } catch (e) {
    // no previous summary
  }

  const script = path.join(__dirname, 'scripts', '07_saturation_ceiling.js');
  const apiUrl = process.env.API_BASE_URL || DEFAULT_API_URL;
  const env = { API_BASE_URL: apiUrl };
  if (options.arrivalTarget != null) env.TARGET_RPS = String(options.arrivalTarget);
  if (options.dwell != null) env.DWELL = String(options.dwell);

  const proc = await k6.spawnK6({
    script,
    summaryExport: outFile,
    env,
    onChild,
  });
  return { exitCode: proc.exitCode, ...parseSummaryFile(outFile) };
}

// Read the effective compose config so the mem-limit guard can reject a capped
// stack. Uses docker-compose.ramcap.yml as an override when it is present.
function readComposeConfig() {
  const override = path.join(ROOT, 'docker-compose.ramcap.yml');
  const args = ['compose', '-f', path.join(ROOT, 'docker-compose.yml')];
  if (fs.existsSync(override)) args.push('-f', override);
  args.push('config');
  const out = spawnSync('docker', args, {
    cwd: ROOT,
    encoding: 'utf8',
    windowsHide: true,
    maxBuffer: 32 * 1024 * 1024,
  });
  if (out.error || out.status !== 0) {
    throw new Error('docker compose config failed; cannot verify mem-limit guard');
  }
  return out.stdout || '';
}

// Resolve a promise after ms, without holding the event loop open.
function delay(ms) {
  return new Promise((resolve) => {
    const t = setTimeout(resolve, ms);
    if (t && typeof t.unref === 'function') t.unref();
  });
}

// Terminate a k6 child: SIGTERM, then SIGKILL if it is still alive after the
// grace period. Idempotent per child.
function killChild(child, graceMs) {
  if (!child || typeof child.kill !== 'function') return Promise.resolve();
  if (child._supervisorKilled) return Promise.resolve();
  child._supervisorKilled = true;
  try {
    child.kill('SIGTERM');
  } catch (e) {
    // already gone
  }
  return delay(graceMs).then(() => {
    if (child.exitCode === null && child.signalCode === null) {
      try {
        child.kill('SIGKILL');
      } catch (e) {
        // already gone
      }
    }
  });
}

// Run an async load job under a live ResourceSampler.
//
//   run(onChild)  -> Promise; onChild(child) records the child's pid on the
//                    sampler (for client RSS) and lets the supervisor kill it.
//
// The sampler is started before the run and stopped after. If the sampler trips
// during the run the child is terminated (SIGTERM, then SIGKILL) and the run is
// reported as aborted.
async function runWithSupervisor(run, opts) {
  const options = opts || {};
  const sampler = options.sampler || new ResourceSampler({ safetyMb: options.safetyMb });
  const pollMs = options.pollMs === undefined ? 100 : options.pollMs;
  const graceMs = options.graceMs === undefined ? 2000 : options.graceMs;

  let child = null;
  let aborted = false;
  const onChild = (c) => {
    if (!c) return;
    child = c;
    if (c.pid) sampler.clientPid = c.pid;
  };

  sampler.start();
  const monitor = setInterval(() => {
    if (sampler.tripped && child && child.exitCode === null) {
      aborted = true;
      killChild(child, graceMs);
    }
  }, pollMs);
  if (monitor && typeof monitor.unref === 'function') monitor.unref();

  let result;
  try {
    result = await Promise.resolve().then(() => run(onChild));
  } finally {
    clearInterval(monitor);
    sampler.stop();
  }

  if (sampler.tripped && child && child.exitCode === null) {
    aborted = true;
    await killChild(child, graceMs);
  }

  return { result, aborted, tripped: !!sampler.tripped, sampler };
}

// connections mode: self-searching concurrent-connection ceiling.
function runConnections(opts, deps) {
  const d = deps || {};
  const connectionDeps =
    d.connectionDeps ||
    connections.defaultDeps(new ResourceSampler({ safetyMb: opts.safetyMb }));
  const searchResult = connections.searchCeiling(
    {
      max: opts.max === null ? undefined : opts.max,
      dwell: opts.dwell === null ? undefined : opts.dwell,
      tolerance: opts.tolerance,
      iterations: opts.iterations,
      start: opts.target === null ? undefined : opts.target,
    },
    connectionDeps
  );
  const result = { mode: 'connections', ...searchResult };
  (d.emit || emit)(result);
  return reportResult(result, d);
}

// ram mode (default): ramping arrival load with continuous resource sampling.
async function runRamRamp(opts, deps) {
  const d = deps || {};
  const readConfig = d.readComposeConfig || readComposeConfig;
  resources.assertNoMemLimit(readConfig());

  const sampler = d.sampler || new ResourceSampler({ safetyMb: opts.safetyMb });
  const job = d.runArrivalJob || runArrivalJob;
  const sup = await runWithSupervisor((onChild) => job(opts, onChild), { sampler });

  const samples = sampler.samples();
  const base = {
    mode: 'ramp',
    curve: samples,
    stackPeakMb: resources.peakOf(samples, 'totalContainerMb'),
    clientPeakMb: resources.peakOf(samples, 'clientMb'),
  };
  const result = sup.tripped
    ? { ...base, verdict: 'SAFETY_STOP' }
    : { ...base, verdict: 'FAILURE_KNEE' };

  (d.emit || emit)(result);
  return reportResult(result, d);
}

// ram mode (--levels given): one row per fixed arrival level. Each level is
// driven at its own arrival rate so the applied load matches the row's rps.
async function runRamLevels(opts, deps) {
  const d = deps || {};
  const readConfig = d.readComposeConfig || readComposeConfig;
  resources.assertNoMemLimit(readConfig());

  const sampler = d.sampler || new ResourceSampler({ safetyMb: opts.safetyMb });
  const job = d.runArrivalJob || runArrivalJob;
  const levels = opts.levels || [];

  const sup = await (d.runWithSupervisor || runWithSupervisor)(async (onChild) => {
    const table = [];
    for (const rps of levels) {
      const before = sampler.samples().length;
      const res = (await job({ arrivalTarget: rps }, onChild)) || {};
      sampler.tick();
      const window = sampler.samples().slice(before);
      table.push({
        rps,
        p95: typeof res.p95 === 'number' ? res.p95 : null,
        stackMb: resources.peakOf(window, 'totalContainerMb'),
        clientMb: resources.peakOf(window, 'clientMb'),
      });
      if (sampler.tripped) break;
    }
    return table;
  }, { sampler });

  // A tripped supervisor means the host safety floor aborted the run; that is a
  // SAFETY_STOP, not a healthy result, so it must exit non-zero via exitCodeFor.
  const result = {
    mode: 'levels',
    table: sup.result,
    verdict: sup.tripped ? 'SAFETY_STOP' : undefined,
  };
  (d.emit || emit)(result);
  return reportResult(result, d);
}

// throughput mode: the arrival-ramp saturation ceiling on its own.
async function runThroughput(opts, deps) {
  const d = deps || {};
  const job = d.runArrivalJob || runArrivalJob;
  const jobResult = await job(opts);
  const out = { mode: 'throughput', ...jobResult };
  (d.emit || emit)(out);
  return reportResult(out, d);
}

// The real driver set. Injecting an alternative object into runSuite() lets a
// test assert mode routing without spawning k6 or touching a live stack.
function defaultDeps() {
  return {
    runSmoke,
    searchCeiling: runConnections,
    runRamp: runRamRamp,
    runLevels: runRamLevels,
    runThroughput,
  };
}

function runSuite(opts, deps) {
  const d = deps || defaultDeps();
  switch (opts.mode) {
    case 'smoke':
      return d.runSmoke(opts);
    case 'connections':
      return d.searchCeiling(opts);
    case 'ram':
      return opts.levels ? d.runLevels(opts) : d.runRamp(opts);
    case 'throughput':
      return d.runThroughput(opts);
    default:
      throw new Error(`unknown mode "${opts.mode}"`);
  }
}

// The bring-up -> readiness -> guard -> dispatch flow, as a value-returning
// seam. Every external touchpoint (stack bring-up, readiness polling, the
// compose config read, the guard, the mode drivers and the error sink) is
// injectable so tests can assert the ordering without a live stack or k6.
//
// The mem-limit guard runs AFTER bring-up and only on the ram paths, so it
// inspects the compose config the running stack actually produced rather than
// the pre-bring-up tree. Resolves to the process exit code.
async function runFlow(opts, deps) {
  const d = deps || {};
  const error = d.error || ((msg) => console.error(msg));
  const healthUrl = d.healthUrl || stack.HEALTH_URL;

  if (!opts.noUp) {
    const up = (d.upStack || stack.upStack)({ build: true });
    if (!up || !up.ok) {
      error('[ERROR] stack failed to come up');
      if (up && up.log) error(up.log);
      return 2;
    }
    const ready = await (d.waitForReady || stack.waitForReady)({
      url: healthUrl,
      timeoutMs: 180000,
      intervalMs: 3000,
    });
    if (!ready) {
      error(`[ERROR] stack not ready at ${healthUrl}`);
      return 2;
    }
  }

  let code;
  try {
    // Guard placement is deliberate: after bring-up, ram modes only.
    if (opts.mode === 'ram') {
      const readConfig = d.readComposeConfig || readComposeConfig;
      const assertNoMemLimit = d.assertNoMemLimit || resources.assertNoMemLimit;
      assertNoMemLimit(readConfig());
    }
    code = runSuite(opts, d.suiteDeps);
  } catch (err) {
    error(`[ERROR] ${err.message}`);
    return 2;
  }

  if (code && typeof code.then === 'function') {
    try {
      return await code;
    } catch (err) {
      error(`[ERROR] ${err.message}`);
      return 2;
    }
  }
  return code;
}

function main() {
  let opts;
  try {
    opts = parseArgs(process.argv.slice(2));
  } catch (err) {
    console.error(`[ERROR] ${err.message}`);
    process.exit(2);
  }

  // runFlow always resolves to an exit code and never rejects; the catch is a
  // backstop so an unexpected throw is still a clean 2 rather than an
  // unhandled rejection.
  return runFlow(opts).then(
    (code) => process.exit(code),
    (err) => {
      console.error(`[ERROR] ${err && err.message ? err.message : err}`);
      process.exit(2);
    }
  );
}

if (require.main === module) {
  main();
}

module.exports = {
  parseArgs,
  MODES,
  runSuite,
  runFlow,
  runWithSupervisor,
  runSmoke,
  runRamRamp,
  runRamLevels,
  runThroughput,
  runArrivalJob,
  parseSummaryResult,
  readComposeConfig,
  killChild,
  parsePositiveInt,
  parsePositiveNumber,
  SMOKE_SCRIPT,
};
