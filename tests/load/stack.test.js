// Tests for the stack bring-up and readiness helpers (Task 2).
// 100% pure ASCII

'use strict';

const test = require('node:test');
const assert = require('node:assert');
const { spawnSync } = require('node:child_process');
const stack = require('./stack.js');

test('waitForReady resolves true once the probe returns healthy', async () => {
  let calls = 0;
  const probe = async () => { calls++; return calls >= 3; };
  const ok = await stack.waitForReady({ url: 'x', timeoutMs: 1000, intervalMs: 1, probe });
  assert.strictEqual(ok, true);
  assert.strictEqual(calls, 3);
});

test('waitForReady resolves false on timeout', async () => {
  const probe = async () => false;
  const ok = await stack.waitForReady({ url: 'x', timeoutMs: 30, intervalMs: 10, probe });
  assert.strictEqual(ok, false);
});

test('HEALTH_URL points at the gateway health path', () => {
  assert.match(stack.HEALTH_URL, /\/api\/v1\/health$/);
});

// The poll sleep must be a REF'd timer: an unref'd one lets Node exit mid-wait, so
// the process would terminate (code 0) before the gate resolves. Run the wait in a
// real child process with no other handles and assert it survives two sleeps.
test('waitForReady keeps the process alive across polls', () => {
  const stackPath = require.resolve('./stack.js');
  const script = `
    const stack = require(${JSON.stringify(stackPath)});
    let calls = 0;
    stack.waitForReady({
      url: 'x',
      timeoutMs: 2000,
      intervalMs: 20,
      probe: async () => { calls++; return calls >= 3; },
    }).then((ok) => { console.log('RESOLVED:' + ok + ':' + calls); });
  `;
  const out = spawnSync(process.execPath, ['-e', script], { encoding: 'utf8' });
  assert.strictEqual(out.status, 0);
  assert.match(out.stdout, /RESOLVED:true:3/);
});
