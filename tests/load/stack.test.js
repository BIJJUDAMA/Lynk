// Tests for the stack bring-up and readiness helpers (Task 2).
// 100% pure ASCII

'use strict';

const test = require('node:test');
const assert = require('node:assert');
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
