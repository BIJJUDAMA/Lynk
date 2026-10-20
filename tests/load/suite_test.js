// Tests for the parameterized load suite entrypoint.
// 100% pure ASCII

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const { parseArgs, MODES, runSuite } = require('./suite.js');

test('parseArgs defaults', () => {
  const o = parseArgs(['--mode=ram']);
  assert.strictEqual(o.mode, 'ram');
  assert.strictEqual(o.safetyMb, 4096);
  assert.strictEqual(o.tolerance, 0.05);
  assert.strictEqual(o.iterations, 8);
  assert.strictEqual(o.levels, null);
});

test('parseArgs parses levels and overrides', () => {
  const o = parseArgs(['--mode=ram', '--levels=500,2000,8000', '--safety-mb=2048']);
  assert.deepStrictEqual(o.levels, [500, 2000, 8000]);
  assert.strictEqual(o.safetyMb, 2048);
});

test('parseArgs rejects unknown mode', () => {
  assert.throws(() => parseArgs(['--mode=nope']));
});

test('parseArgs coerces numeric flags to integers', () => {
  const o = parseArgs(['--mode=connections', '--target=500', '--max=64000', '--dwell=20']);
  assert.strictEqual(o.max, 64000);
  assert.strictEqual(typeof o.max, 'number');
  assert.strictEqual(o.dwell, 20);
  assert.strictEqual(typeof o.dwell, 'number');
  assert.strictEqual(o.target, 500);
  assert.strictEqual(typeof o.target, 'number');
});

test('parseArgs rejects a non-integer max', () => {
  assert.throws(() => parseArgs(['--mode=connections', '--max=x']));
});

test('MODES is the fixed set', () => {
  assert.deepStrictEqual(MODES, ['throughput', 'connections', 'ram', 'smoke']);
});

test('nginx.conf declares stub_status', () => {
  const conf = path.resolve(__dirname, '..', '..', '.docker', 'nginx', 'nginx.conf');
  const c = fs.readFileSync(conf, 'utf8');
  assert.match(c, /stub_status/);
});

test('runSuite dispatches each mode to the right driver', () => {
  const calls = [];
  const deps = {
    runSmoke: () => { calls.push('smoke'); return 0; },
    searchCeiling: () => { calls.push('connections'); return 0; },
    runRamp: () => { calls.push('ram-ramp'); return 0; },
    runLevels: () => { calls.push('ram-levels'); return 0; },
    runThroughput: () => { calls.push('throughput'); return 0; },
  };
  assert.strictEqual(runSuite({ mode: 'smoke' }, deps), 0);
  assert.strictEqual(runSuite({ mode: 'connections' }, deps), 0);
  assert.strictEqual(runSuite({ mode: 'ram' }, deps), 0);
  assert.strictEqual(runSuite({ mode: 'ram', levels: [500] }, deps), 0);
  assert.strictEqual(runSuite({ mode: 'throughput' }, deps), 0);
  assert.deepStrictEqual(calls, ['smoke', 'connections', 'ram-ramp', 'ram-levels', 'throughput']);
});
