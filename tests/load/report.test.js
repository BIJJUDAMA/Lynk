// Tests for the consolidated load-suite report artifact and exit codes.
// 100% pure ASCII

const test = require('node:test');
const assert = require('node:assert');
const os = require('node:os');
const fs = require('node:fs');
const path = require('node:path');
const report = require('./report.js');

test('writeReport writes a json and md artifact', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'lynkrep-'));
  const { jsonPath, mdPath } = report.writeReport({ mode: 'connections', verdict: 'SERVER_CEILING', n: 4000 }, { dir, stamp: 'test' });
  assert.ok(fs.existsSync(jsonPath));
  assert.ok(fs.existsSync(mdPath));
  assert.match(fs.readFileSync(mdPath, 'utf8'), /SERVER_CEILING/);
});

test('exitCodeFor maps verdicts', () => {
  assert.strictEqual(report.exitCodeFor({ verdict: 'SERVER_CEILING' }), 0);
  assert.strictEqual(report.exitCodeFor({ verdict: 'CLIENT_BOUND' }), 1);
  assert.strictEqual(report.exitCodeFor({ verdict: 'SAFETY_STOP' }), 1);
  assert.strictEqual(report.exitCodeFor({ mode: 'ram', verdict: 'FAILURE_KNEE' }), 0);
});
