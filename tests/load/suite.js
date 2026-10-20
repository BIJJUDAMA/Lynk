// Parameterized load test suite entrypoint for Lynk.
// Dispatches to one of several load-testing modes; each mode is wired in by
// its own module (Tasks 2-5). This file owns argument parsing and mode routing.
// 100% pure ASCII

'use strict';

const { spawnSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const MODES = ['throughput', 'connections', 'ram', 'smoke'];

const DEFAULT_API_URL = 'http://localhost:8080/api/v1';

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

// Coerce a flag to a positive integer, rejecting non-integers and <= 0.
function parsePositiveInt(name, raw) {
  const n = Number(raw);
  if (!Number.isInteger(n) || n <= 0) {
    throw new Error(`invalid ${name} "${raw}": must be a positive integer`);
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
        opts.safetyMb = Number(value);
        break;
      case 'tolerance':
        opts.tolerance = Number(value);
        break;
      case 'iterations':
        opts.iterations = Number(value);
        break;
      case 'seed':
        opts.seed = Number(value);
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

export const options = { vus: 1, duration: '2s' };

export default function () {
  const res = http.get(\`\${__ENV.API_BASE_URL}/health\`);
  check(res, { 'health status is 200': (r) => r.status === 200 });
}
`;

function runSmoke(opts) {
  const resultsDir = path.join(__dirname, 'results');
  fs.mkdirSync(resultsDir, { recursive: true });
  const script = path.join(resultsDir, '.smoke.js');
  fs.writeFileSync(script, SMOKE_SCRIPT, 'utf8');

  try {
    const apiUrl = process.env.API_BASE_URL || DEFAULT_API_URL;
    const args = ['run', script];
    const proc = spawnSync('k6', args, {
      cwd: path.resolve(__dirname, '../..'),
      stdio: 'inherit',
      env: { ...process.env, API_BASE_URL: apiUrl },
    });
    return proc.status === null ? 1 : proc.status;
  } finally {
    try {
      fs.unlinkSync(script);
    } catch (e) {
      // best-effort cleanup
    }
  }
}

function runSuite(opts) {
  switch (opts.mode) {
    case 'smoke':
      return runSmoke(opts);
    case 'throughput':
    case 'connections':
    case 'ram':
      throw new Error('not implemented');
    default:
      throw new Error(`unknown mode "${opts.mode}"`);
  }
}

function main() {
  let opts;
  try {
    opts = parseArgs(process.argv.slice(2));
  } catch (err) {
    console.error(`[ERROR] ${err.message}`);
    process.exit(2);
  }
  const code = runSuite(opts);
  process.exit(code);
}

if (require.main === module) {
  main();
}

module.exports = { parseArgs, MODES, runSuite };
