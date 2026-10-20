// Stack bring-up and readiness helpers for the Lynk load suite.
//
// Brings the local Docker Compose stack up (building images so the running
// gateway reflects the current tree, not a stale layer) and then polls the
// gateway health endpoint until it is actually ready. `upStack` and
// `waitForReady` are the two seams the suite uses before applying any load.
//
// 100% pure ASCII

'use strict';

const path = require('node:path');
const http = require('node:http');
const { spawnSync } = require('node:child_process');

// Repository root: the cwd every docker invocation runs from.
const ROOT = path.resolve(__dirname, '../..');

// The gateway's liveness/readiness endpoint, served once the stack is ready.
const HEALTH_URL = 'http://localhost:8080/api/v1/health';

// Bring the compose stack up, building images when `build` is set.
//
// Runs from ROOT and captures combined output so a failed bring-up can be
// reported without interleaving partial container logs into the suite's stdout.
// Returns { ok, log } where `ok` is true only on a clean exit (status 0); a
// spawn error or signal termination leaves `ok` false.
function upStack(opts) {
  const options = opts || {};
  const build = options.build === undefined ? true : options.build;
  const args = ['compose', 'up', '-d'];
  if (build) args.push('--build');

  let out;
  try {
    out = spawnSync('docker', args, {
      cwd: ROOT,
      encoding: 'utf8',
      windowsHide: true,
      maxBuffer: 64 * 1024 * 1024,
    });
  } catch (e) {
    return { ok: false, log: e.message };
  }

  const log = `${out.stdout || ''}${out.stderr || ''}`;
  if (out.error) return { ok: false, log: `${log}${out.error.message}` };
  return { ok: out.status === 0, log };
}

// Default readiness probe: an HTTP GET to `url`, resolving true only on 200.
// Every other outcome (non-200, error, timeout) resolves false so the caller
// keeps polling instead of treating a transient failure as fatal.
function httpProbe(url) {
  return new Promise((resolve) => {
    let settled = false;
    const done = (value) => {
      if (settled) return;
      settled = true;
      resolve(value);
    };

    let req;
    try {
      req = http.get(url, (res) => {
        const status = res.statusCode;
        res.resume();
        done(status === 200);
      });
    } catch (e) {
      done(false);
      return;
    }

    req.on('error', () => done(false));
    req.setTimeout(2000, () => {
      req.destroy();
      done(false);
    });
  });
}

// Poll `probe` until it reports healthy or the deadline passes.
//
//   waitForReady({ url, timeoutMs, intervalMs, probe }) -> Promise<boolean>
//
// `probe` is injectable so tests can drive the loop deterministically; it
// defaults to an http.get to `url`. Polls immediately, then every `intervalMs`,
// resolving true on the first healthy result and false once `timeoutMs` has
// elapsed. Never throws: a rejected probe is treated as not-yet-healthy.
async function waitForReady(opts) {
  const options = opts || {};
  const url = options.url;
  const timeoutMs = options.timeoutMs === undefined ? 60000 : options.timeoutMs;
  const intervalMs = options.intervalMs === undefined ? 3000 : options.intervalMs;
  const probe = options.probe || (() => httpProbe(url));

  const deadline = Date.now() + timeoutMs;

  for (;;) {
    let healthy = false;
    try {
      healthy = !!(await probe());
    } catch (e) {
      healthy = false;
    }
    if (healthy) return true;
    if (Date.now() >= deadline) return false;
    // Deliberately NOT unref'd: this timer is the only handle keeping the event
    // loop alive between polls. Unref'ing it lets Node exit (code 0) mid-wait,
    // silently bypassing the readiness gate.
    await new Promise((resolve) => {
      setTimeout(resolve, intervalMs);
    });
  }
}

module.exports = {
  ROOT,
  HEALTH_URL,
  upStack,
  waitForReady,
  httpProbe,
};
