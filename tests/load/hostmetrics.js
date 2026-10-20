// Cross-platform host metrics for the load-test suite.
//
// Exposes four best-effort probes used by the suite to make safety and
// reporting decisions. Every probe is defensive: on an unsupported host or
// any parse failure it returns null (or a fallback) instead of throwing, so
// callers degrade gracefully rather than crashing the run.
//
// 100% pure ASCII

'use strict';

const fs = require('node:fs');
const { execFileSync } = require('node:child_process');

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

// Read a file as text, or null on any failure.
function tryRead(file) {
  try {
    return fs.readFileSync(file, 'utf8');
  } catch (err) {
    return null;
  }
}

// --- freeMemMb ------------------------------------------------------------

function freeMemMbLinux() {
  const text = tryRead('/proc/meminfo');
  if (!text) return null;
  const m = text.match(/^MemAvailable:\s+(\d+)\s*kB/im);
  if (!m) return null;
  const kb = Number(m[1]);
  if (!Number.isFinite(kb)) return null;
  return Math.floor(kb / 1024);
}

function freeMemMbWindows() {
  const out = tryExec('powershell', [
    '-NoProfile',
    '-NonInteractive',
    '-Command',
    '(Get-CimInstance Win32_OperatingSystem).FreePhysicalMemory',
  ]);
  if (!out) return null;
  const m = out.match(/(\d+)/);
  if (!m) return null;
  const kb = Number(m[1]);
  if (!Number.isFinite(kb) || kb <= 0) return null;
  return Math.floor(kb / 1024);
}

// Free physical memory in MB, or null when unavailable.
function freeMemMb() {
  if (process.platform === 'linux') return freeMemMbLinux();
  if (process.platform === 'win32') return freeMemMbWindows();
  return null;
}

// --- timeWaitCount --------------------------------------------------------

// Count lines depicting a TCP socket in TIME_WAIT.
function countTimeWait(text) {
  if (!text) return null;
  let count = 0;
  for (const line of text.split(/\r?\n/)) {
    if (/\bTIME_WAIT\b/i.test(line)) count += 1;
  }
  return count;
}

function timeWaitCountLinux() {
  // Prefer `ss -s` summary when available.
  const sum = tryExec('ss', ['-s', '-t']);
  if (sum) {
    const m = sum.match(/TIME-WAIT\s+(\d+)/i) || sum.match(/TIME_WAIT\s+(\d+)/i);
    if (m) {
      const n = Number(m[1]);
      if (Number.isInteger(n) && n >= 0) return n;
    }
  }
  const netstat = tryExec('netstat', ['-an']);
  return countTimeWait(netstat);
}

function timeWaitCountWindows() {
  const netstat = tryExec('netstat', ['-an']);
  return countTimeWait(netstat);
}

// Number of sockets in TIME_WAIT, or null when unavailable.
function timeWaitCount() {
  if (process.platform === 'linux') return timeWaitCountLinux();
  if (process.platform === 'win32') return timeWaitCountWindows();
  return null;
}

// --- ephemeralPortRange ---------------------------------------------------

function ephemeralPortRangeLinux() {
  const text = tryRead('/proc/sys/net/ipv4/ip_local_port_range');
  if (!text) return null;
  const m = text.match(/(\d+)\s+(\d+)/);
  if (!m) return null;
  const start = Number(m[1]);
  const end = Number(m[2]);
  if (!Number.isInteger(start) || !Number.isInteger(end) || end <= start) return null;
  return { start, end };
}

function ephemeralPortRangeWindows() {
  const out = tryExec('netsh', ['int', 'ipv4', 'show', 'dynamicport', 'tcp']);
  if (!out) return null;
  const sm = out.match(/Start Port\s*:\s*(\d+)/i);
  const nm = out.match(/Number of Ports\s*:\s*(\d+)/i);
  if (!sm || !nm) return null;
  const start = Number(sm[1]);
  const count = Number(nm[1]);
  if (!Number.isInteger(start) || !Number.isInteger(count) || count <= 0) return null;
  const end = start + count - 1;
  if (end <= start) return null;
  return { start, end };
}

// The kernel ephemeral port range as {start, end}, or null.
function ephemeralPortRange() {
  if (process.platform === 'linux') return ephemeralPortRangeLinux();
  if (process.platform === 'win32') return ephemeralPortRangeWindows();
  return null;
}

// --- primaryEgressConnCount ----------------------------------------------

// Count ESTABLISHED TCP sockets, excluding loopback endpoints. This proxies
// "connections the host is holding on its primary egress path", which is the
// quantity the connection-scaling load mode cares about.
function countEstablishedNonLoopback(text) {
  if (!text) return null;
  let count = 0;
  for (const line of text.split(/\r?\n/)) {
    if (!/\bESTABLISHED\b/i.test(line)) continue;
    if (/\b(127\.0\.0\.1|0\.0\.0\.0|::1|\[::1\])\b/.test(line)) continue;
    count += 1;
  }
  return count;
}

function primaryEgressConnCountLinux() {
  const netstat = tryExec('netstat', ['-an']);
  const fromNetstat = countEstablishedNonLoopback(netstat);
  if (fromNetstat !== null) return fromNetstat;
  const ss = tryExec('ss', ['-tan', 'state', 'established']);
  return countEstablishedNonLoopback(ss);
}

function primaryEgressConnCountWindows() {
  const netstat = tryExec('netstat', ['-an']);
  return countEstablishedNonLoopback(netstat);
}

// Number of non-loopback ESTABLISHED TCP connections, or null.
function primaryEgressConnCount() {
  if (process.platform === 'linux') return primaryEgressConnCountLinux();
  if (process.platform === 'win32') return primaryEgressConnCountWindows();
  return null;
}

module.exports = {
  freeMemMb,
  timeWaitCount,
  ephemeralPortRange,
  primaryEgressConnCount,
};
