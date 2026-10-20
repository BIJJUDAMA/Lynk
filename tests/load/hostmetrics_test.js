// Tests for the cross-platform host metrics abstraction.
// 100% pure ASCII

const test = require('node:test');
const assert = require('node:assert');
const hm = require('./hostmetrics.js');

test('freeMemMb returns positive on this host', () => {
  assert.ok(hm.freeMemMb() > 0);
});

test('timeWaitCount returns null or a non-negative int', () => {
  const v = hm.timeWaitCount();
  assert.ok(v === null || (Number.isInteger(v) && v >= 0));
});

test('ephemeralPortRange returns null or a valid range', () => {
  const r = hm.ephemeralPortRange();
  assert.ok(r === null || (Number.isInteger(r.start) && Number.isInteger(r.end) && r.end > r.start));
});
