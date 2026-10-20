// Resource sampler for the load-test suite.
//
// Periodically samples three independent memory signals so the suite can
// attribute pressure to the right culprit:
//   - freeMb:            host free physical memory (safety floor source)
//   - totalContainerMb:  sum of the lynk-* docker container RSS
//   - clientMb:          RSS of the load-generator process (native k6)
//   - perContainer:      per-container MiB breakdown
//
// When host free memory reaches the safety floor the sampler latches
// tripped = true so callers can stop the run before the machine thrashes.
//
// Every reader is injectable so tests run without a real stack. Defaults are
// best-effort and never throw: a failed probe yields 0/null rather than
// crashing the sampling interval.
//
// 100% pure ASCII

'use strict';

const { execFileSync } = require('node:child_process');
const hostmetrics = require('./hostmetrics.js');

// Run a command and return stdout as text, or null on any failure.
function tryExec(cmd, args, opts) {
  try {
    return execFileSync(cmd, args, {
      encoding: 'utf8',
      timeout: 5000,
      windowsHide: true,
      maxBuffer: 32 * 1024 * 1024,
      ...(opts || {}),
    });
  } catch (err) {
    return null;
  }
}

// Parse a docker MemUsage cell ("45.83MiB", "1.5GiB", "0B") into MiB.
// Returns null when the token cannot be understood.
function parseMemToMb(token) {
  if (token == null) return null;
  const m = String(token).trim().match(/^([\d.]+)\s*([KMGTP]i?B)?$/i);
  if (!m) return null;
  const value = Number(m[1]);
  if (!Number.isFinite(value)) return null;
  const unit = (m[2] || 'B').toUpperCase();
  const factor = {
    B: 1 / (1024 * 1024),
    KIB: 1 / 1024,
    KB: 1 / 1024,
    MIB: 1,
    MB: 1,
    GIB: 1024,
    GB: 1024,
    TIB: 1024 * 1024,
    TB: 1024 * 1024,
    PIB: 1024 * 1024 * 1024,
    PB: 1024 * 1024 * 1024,
  }[unit];
  if (factor == null) return null;
  return value * factor;
}

// Parse `docker stats --no-stream` output of the form:
//   lynk-api-1|45.83MiB / 512MiB
// into { total, perContainer }. A container's usage is the left-hand side of
// the slash; failures are skipped. Returns { total: 0, perContainer: {} } when
// nothing parses.
function parseDockerStats(text) {
  const perContainer = {};
  let total = 0;
  if (!text) return { total, perContainer };
  for (const line of String(text).split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const bar = trimmed.indexOf('|');
    if (bar === -1) continue;
    const name = trimmed.slice(0, bar).trim();
    const usage = trimmed.slice(bar + 1);
    const usedToken = usage.split('/')[0];
    const mb = parseMemToMb(usedToken);
    if (!name || mb == null) continue;
    perContainer[name] = mb;
    total += mb;
  }
  return { total, perContainer };
}

// Default container probe: sum the docker stats MemUsage of every container.
function readContainersDocker() {
  const out = tryExec('docker', [
    'stats',
    '--no-stream',
    '--format',
    '{{.Name}}|{{.MemUsage}}',
  ]);
  return parseDockerStats(out);
}

// Default client RSS probe. The load client is a native k6 process, so we read
// its resident set size from the OS by pid. Returns null when unavailable.
function readClientMbByPid(pid) {
  if (!pid) return null;
  if (process.platform === 'win32') {
    const out = tryExec('tasklist', [
      '/FI',
      `PID eq ${pid}`,
      '/FO',
      'CSV',
      '/NH',
    ]);
    if (!out) return null;
    // CSV row: "k6.exe","1234","Console","1","45,123 K"
    const fields = out.trim().match(/"([^"]*)"/g);
    if (!fields || fields.length < 5) return null;
    const kb = Number(fields[4].replace(/[^0-9]/g, ''));
    if (!Number.isFinite(kb) || kb <= 0) return null;
    return kb / 1024;
  }
  const out = tryExec('ps', ['-o', 'rss=', '-p', String(pid)]);
  if (!out) return null;
  const kb = Number(out.trim());
  if (!Number.isFinite(kb) || kb <= 0) return null;
  return kb / 1024;
}

class ResourceSampler {
  constructor(opts) {
    const options = opts || {};
    this.intervalMs =
      options.intervalMs === undefined ? 1000 : options.intervalMs;
    this.safetyMb = options.safetyMb === undefined ? 4096 : options.safetyMb;
    this.clientPid = options.clientPid || null;

    this.readFree =
      options.readFree || (() => hostmetrics.freeMemMb());
    this.readContainers = options.readContainers || readContainersDocker;
    this.readClientMb =
      options.readClientMb || (() => readClientMbByPid(this.clientPid));

    this._samples = [];
    this._timer = null;
    this.tripped = false;
    this.tripReason = null;
  }

  // Take one sample and evaluate the safety floor.
  tick() {
    const freeMb = this.readFree();
    const containers = this.readContainers() || {
      total: 0,
      perContainer: {},
    };
    const clientMb = this.readClientMb();

    this._samples.push({
      t: Date.now(),
      freeMb,
      totalContainerMb: containers.total,
      clientMb,
      perContainer: containers.perContainer || {},
    });

    if (typeof freeMb === 'number' && freeMb <= this.safetyMb) {
      this.tripped = true;
      this.tripReason = `SAFETY_STOP at ${freeMb}`;
    }

    return this._samples[this._samples.length - 1];
  }

  // Begin periodic sampling. Safe to call more than once.
  start() {
    if (this._timer) return this;
    this._timer = setInterval(() => this.tick(), this.intervalMs);
    if (this._timer && typeof this._timer.unref === 'function') {
      this._timer.unref();
    }
    return this;
  }

  // Stop periodic sampling.
  stop() {
    if (this._timer) {
      clearInterval(this._timer);
      this._timer = null;
    }
    return this;
  }

  // A copy of the collected samples.
  samples() {
    return this._samples.slice();
  }
}

module.exports = {
  ResourceSampler,
  parseMemToMb,
  parseDockerStats,
  readContainersDocker,
  readClientMbByPid,
};
