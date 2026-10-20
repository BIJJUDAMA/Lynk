// Tests for the resource curve driver (Tasks 5).
// 100% pure ASCII

'use strict';

const test = require('node:test');
const assert = require('node:assert');

const { runLevels, runRamp, assertNoMemLimit } = require('./resources.js');

test('assertNoMemLimit rejects a capped config', () => {
  assert.throws(() =>
    assertNoMemLimit('services:\n  api:\n    mem_limit: 512m\n')
  );
  assert.doesNotThrow(() =>
    assertNoMemLimit('services:\n  api:\n    image: lynk-api\n')
  );
});

test('runLevels samples each fixed level', () => {
  const deps = {
    runLevel: (rps) => ({ rps, p95: 5, failedRate: 0 }),
    sampler: {
      samples: () => [{ totalContainerMb: 800, clientMb: 300 }],
      tick() {},
      tripped: false,
    },
  };
  const out = runLevels({ levels: [500, 2000] }, deps);
  assert.strictEqual(out.mode, 'levels');
  assert.strictEqual(out.table.length, 2);
  assert.strictEqual(out.table[0].rps, 500);
});

test('runRamp stops at the sampler trip', () => {
  const deps = {
    runRamp: (opts) => ({ rps: opts.ceiling, p95: 9, failedRate: 0 }),
    sampler: {
      samples: () => [
        { totalContainerMb: 1200, clientMb: 250 },
        { totalContainerMb: 1800, clientMb: 400 },
      ],
      tick() {},
      tripped: true,
    },
  };
  const out = runRamp({ ceiling: 8000 }, deps);
  assert.strictEqual(out.mode, 'ramp');
  assert.strictEqual(out.verdict, 'SAFETY_STOP');
  assert.strictEqual(out.stackPeakMb, 1800);
  assert.strictEqual(out.clientPeakMb, 400);
  assert.ok(Array.isArray(out.curve));
});

test('runRamp returns the failure-knee verdict when the ceiling is reached', () => {
  const deps = {
    runRamp: (opts) => ({ rps: opts.ceiling, p95: 12, failedRate: 0.2 }),
    sampler: {
      samples: () => [{ totalContainerMb: 900, clientMb: 200 }],
      tick() {},
      tripped: false,
    },
  };
  const out = runRamp({ ceiling: 6000 }, deps);
  assert.strictEqual(out.verdict, 'FAILURE_KNEE');
  assert.strictEqual(out.stackPeakMb, 900);
});
