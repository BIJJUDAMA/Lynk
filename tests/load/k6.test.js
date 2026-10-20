// Tests for the k6 runner module: native detection plus Docker fallback.
// 100% pure ASCII

'use strict';

const test = require('node:test');
const assert = require('node:assert');
const k6 = require('./k6.js');

test('buildK6Args uses native k6 when available', () => {
  const { cmd, args } = k6.buildK6Args({ script: 'tests/load/scripts/hold.js', summaryExport: '/tmp/s.json', native: true });
  assert.strictEqual(cmd, 'k6');
  assert.deepStrictEqual(args.slice(0, 2), ['run', '--summary-export=/tmp/s.json']);
});

test('buildK6Args uses docker fallback when not native', () => {
  const { cmd, args } = k6.buildK6Args({ script: 'tests/load/scripts/hold.js', summaryExport: '/tmp/s.json', native: false });
  assert.strictEqual(cmd, 'docker');
  assert.ok(args.includes('--network=host'));
  assert.ok(args.includes('grafana/k6'));
  assert.ok(args.join(' ').includes('run'));
});

test('native detection is a boolean', () => {
  assert.strictEqual(typeof k6.hasNativeK6(), 'boolean');
});
