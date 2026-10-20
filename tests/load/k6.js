// Shared k6 runner for the Lynk load suite.
//
// Runs a k6 script through a native `k6` binary when one is on PATH, and
// otherwise falls back to the official `grafana/k6` Docker image with the
// repository mounted at /lynk and host networking. Every caller gets back a
// container/native-neutral { exitCode, summaryPath } so downstream parsing does
// not care which path ran.
//
// 100% pure ASCII

'use strict';

const path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');

// Official k6 image used for the Docker fallback path.
const K6_IMAGE = 'grafana/k6';

// Repository root: the directory mounted into the container at /lynk and the
// working directory every invocation runs from.
const ROOT = path.resolve(__dirname, '../..');

// Mount point inside the k6 container that maps to the repository root.
const CONTAINER_ROOT = '/lynk';

// Detect a usable native k6 binary. Status 0 from `k6 version` is the signal;
// a missing binary or any non-zero exit leaves the Docker path in play.
function hasNativeK6() {
  try {
    const out = spawnSync('k6', ['version'], { windowsHide: true });
    return !out.error && out.status === 0;
  } catch (e) {
    return false;
  }
}

// Translate a host path to its container-relative /lynk/... address. Paths
// outside the repository are passed through unchanged so an absolute summary
// path the caller already resolved still lands somewhere legible.
function toContainerPath(p) {
  const abs = path.resolve(p);
  const rel = path.relative(ROOT, abs);
  if (rel.startsWith('..') || path.isAbsolute(rel)) return abs;
  return path.posix.join(CONTAINER_ROOT, rel.split(path.sep).join('/'));
}

// Build the argv for one k6 run.
//
//   native: ['run', '--summary-export=<p>', ...extraArgs, script] on `k6`
//   docker: ['run', '--rm', '--network=host', '-v', <ROOT>:/lynk, '-w', /lynk,
//            'grafana/k6', 'run', '--summary-export=<rel>', ...extraArgs,
//            <relScript>] on `docker`
//
// `native` defaults to hasNativeK6() so callers can omit it in production and
// override it deterministically in tests.
function buildK6Args(opts) {
  const options = opts || {};
  const script = options.script;
  const summaryExport = options.summaryExport;
  const extraArgs = options.extraArgs || [];
  const native = options.native === undefined ? hasNativeK6() : !!options.native;

  if (native) {
    const args = ['run'];
    if (summaryExport) args.push(`--summary-export=${summaryExport}`);
    args.push(...extraArgs, script);
    return { cmd: 'k6', args };
  }

  const relScript = toContainerPath(script);
  const args = [
    'run',
    '--rm',
    '--network=host',
    '-v',
    `${ROOT}:${CONTAINER_ROOT}`,
    '-w',
    CONTAINER_ROOT,
    K6_IMAGE,
    'run',
  ];
  if (summaryExport) args.push(`--summary-export=${toContainerPath(summaryExport)}`);
  args.push(...extraArgs, relScript);
  return { cmd: 'docker', args };
}

// Spawn one k6 run asynchronously (never blocking the event loop) and resolve
// { exitCode, summaryPath } when the child exits. onChild(child) receives the
// ChildProcess so a supervisor can record its pid and kill it.
//
// `native` may be supplied to force a path; it otherwise falls back to
// hasNativeK6().
function spawnK6(opts) {
  const options = opts || {};
  const summaryPath = options.summaryExport;
  const { cmd, args } = buildK6Args({
    script: options.script,
    summaryExport: summaryPath,
    extraArgs: options.extraArgs,
    native: options.native,
  });
  const env = options.env || {};

  return new Promise((resolve) => {
    const child = spawn(cmd, args, {
      cwd: ROOT,
      stdio: 'inherit',
      env: { ...process.env, ...env },
    });
    if (typeof options.onChild === 'function') options.onChild(child);

    let settled = false;
    const done = (code) => {
      if (settled) return;
      settled = true;
      resolve({
        exitCode: code === null || code === undefined ? 1 : code,
        summaryPath,
      });
    };
    child.on('error', () => done(1));
    child.on('close', (code) => done(code));
  });
}

module.exports = {
  K6_IMAGE,
  ROOT,
  CONTAINER_ROOT,
  hasNativeK6,
  buildK6Args,
  spawnK6,
  toContainerPath,
};
