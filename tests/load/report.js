// Consolidated load-suite report artifact and verdict-to-exit-code mapping.
//
// Every load-suite mode funnels its result object through here after it is
// computed: writeReport() emits a machine-readable JSON file plus a short
// human-readable Markdown summary, and exitCodeFor() turns the run's verdict
// into the process exit code the suite (and CI) gates on.
//
// The artifact names are mode- and stamp-qualified on purpose. The suite never
// writes SUMMARY.md or REPORT.md: those names live under tests/load/results/
// and belong to the earlier hand-curated stages, so a suite run must not
// clobber them.
//
// 100% pure ASCII

'use strict';

const fs = require('node:fs');
const path = require('node:path');

// Verdicts that mean the run reached a reportable, usable end: a real server
// ceiling, the failure knee of a ramp, or a healthy pass. These exit 0.
const OK_VERDICTS = new Set(['SERVER_CEILING', 'FAILURE_KNEE', 'HEALTHY']);

// Verdicts that mean the run is not usable as a result: the load generator was
// the bottleneck, or the host safety floor tripped. These exit 1.
const FAIL_VERDICTS = new Set(['CLIENT_BOUND', 'SAFETY_STOP', 'FAILURE']);

// Numeric fields worth surfacing in the Markdown summary, in a stable order.
const SUMMARY_FIELDS = [
  'n',
  'rps',
  'p95',
  'failedRate',
  'stackPeakMb',
  'clientPeakMb',
  'serverConns',
];

// Map a result's verdict (or a bare failure signal) to a process exit code.
//
//   0  SERVER_CEILING / FAILURE_KNEE / HEALTHY / no verdict (healthy)
//   1  CLIENT_BOUND / SAFETY_STOP / FAILURE, a non-zero exitCode, or ok:false
//
// A verdict takes precedence over the transport-level exitCode so a completed
// k6 run that merely breached a threshold is judged on its classified verdict.
function exitCodeFor(result) {
  const r = result || {};
  const verdict = r.verdict;

  if (OK_VERDICTS.has(verdict)) return 0;
  if (FAIL_VERDICTS.has(verdict)) return 1;

  // No recognised verdict: fall back to explicit failure signals.
  if (typeof r.exitCode === 'number' && r.exitCode !== 0) return 1;
  if (r.ok === false || r.failed === true) return 1;

  return 0;
}

// Render one scalar line for the Markdown summary.
function fieldLine(key, value) {
  if (value === undefined || value === null) return null;
  if (typeof value === 'number') {
    return `- ${key}: ${Number.isFinite(value) ? value : 'n/a'}`;
  }
  return `- ${key}: ${String(value)}`;
}

// Render the fixed level table (ram --levels) as a Markdown grid.
function levelsTable(rows) {
  const header = '| rps | p95 | stackMb | clientMb |';
  const sep = '| ---: | ---: | ---: | ---: |';
  const body = rows.map((row) => {
    const r = row || {};
    const cell = (v) => (v === undefined || v === null ? '' : String(v));
    return `| ${cell(r.rps)} | ${cell(r.p95)} | ${cell(r.stackMb)} | ${cell(r.clientMb)} |`;
  });
  return [header, sep, ...body].join('\n');
}

// Build the short Markdown summary of a result: title, key scalars, verdict,
// the exit code it maps to, and any per-level table.
function renderMarkdown(result, stamp) {
  const r = result || {};
  const lines = [];

  lines.push(`# Load suite report: ${r.mode || 'unknown'}`);
  lines.push('');
  lines.push(`- stamp: ${stamp}`);
  if (r.verdict !== undefined && r.verdict !== null) {
    lines.push(`- verdict: ${r.verdict}`);
  }
  if (r.reason !== undefined && r.reason !== null) {
    lines.push(`- reason: ${r.reason}`);
  }
  lines.push(`- exit code: ${exitCodeFor(r)}`);

  for (const key of SUMMARY_FIELDS) {
    const line = fieldLine(key, r[key]);
    if (line) lines.push(line);
  }

  if (Array.isArray(r.table)) {
    lines.push('');
    lines.push('## levels');
    lines.push('');
    lines.push(r.table.length ? levelsTable(r.table) : '_no levels recorded_');
  }

  if (Array.isArray(r.curve)) {
    lines.push('');
    lines.push(`- samples: ${r.curve.length}`);
  }

  lines.push('');
  return `${lines.join('\n')}\n`;
}

// Write the JSON + Markdown artifacts for one result and return both paths.
//
//   result: the mode's result object (carries `mode` and usually `verdict`)
//   opts:   { dir, stamp } -- output directory and filename stamp
//
// The stamp is injected (normally Date.now()) so tests get deterministic names.
// Never writes SUMMARY.md or REPORT.md.
function writeReport(result, opts) {
  const r = result || {};
  const o = opts || {};
  const dir = o.dir || path.join(__dirname, 'results');
  const stamp = o.stamp === undefined || o.stamp === null ? Date.now() : o.stamp;
  const mode = r.mode === undefined || r.mode === null ? 'unknown' : String(r.mode);

  fs.mkdirSync(dir, { recursive: true });

  const jsonPath = path.join(dir, `suite-${mode}-${stamp}.json`);
  const mdPath = path.join(dir, `suite-${mode}-${stamp}.md`);

  fs.writeFileSync(jsonPath, `${JSON.stringify(r, null, 2)}\n`, 'utf8');
  fs.writeFileSync(mdPath, renderMarkdown(r, stamp), 'utf8');

  return { jsonPath, mdPath };
}

module.exports = {
  writeReport,
  exitCodeFor,
  renderMarkdown,
  OK_VERDICTS,
  FAIL_VERDICTS,
};
