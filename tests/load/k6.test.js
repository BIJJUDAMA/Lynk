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

test('buildK6Args forwards env as -e flags in the docker form only', () => {
  const env = { API_BASE_URL: 'http://x/api/v1', TARGET_VUS: '500', DWELL: '20' };

  const native = k6.buildK6Args({ script: 'tests/load/scripts/hold.js', summaryExport: '/tmp/s.json', env, native: true });
  assert.ok(!native.args.includes('-e'));

  const docker = k6.buildK6Args({ script: 'tests/load/scripts/hold.js', summaryExport: '/tmp/s.json', env, native: false });
  assert.ok(docker.args.includes('-e'));
  assert.ok(docker.args.includes('API_BASE_URL=http://x/api/v1'));
  assert.ok(docker.args.includes('TARGET_VUS=500'));
  assert.ok(docker.args.includes('DWELL=20'));

  const imageIdx = docker.args.indexOf('grafana/k6');
  const envIdx = docker.args.indexOf('API_BASE_URL=http://x/api/v1');
  assert.ok(envIdx > -1 && envIdx < imageIdx, '-e must precede the image name');
});

test('native detection is a boolean', () => {
  assert.strictEqual(typeof k6.hasNativeK6(), 'boolean');
});
