// Tests for the parameterized load suite entrypoint.
// 100% pure ASCII

const test = require('node:test');
const assert = require('node:assert');
const { parseArgs, MODES } = require('./suite.js');

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

test('MODES is the fixed set', () => {
  assert.deepStrictEqual(MODES, ['throughput', 'connections', 'ram', 'smoke']);
});
