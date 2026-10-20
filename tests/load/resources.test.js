// Tests for the resource curve driver (Task 5).
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

test('runLevels attributes peaks to the level that produced them', () => {
  // The sampler accumulates samples; each level appends its own. Rows must
  // reflect only the samples appended while that level ran, not the running
  // cumulative peak. A pre-existing high sample must not bleed into any row.
  const store = [{ totalContainerMb: 9999, clientMb: 999 }];
  const deps = {
    runLevel: (rps) => {
      store.push({ totalContainerMb: rps, clientMb: rps / 10 });
      return { rps, p95: 5, failedRate: 0 };
    },
    sampler: {
      samples: () => store.slice(),
      tick() {},
      tripped: false,
    },
  };
  const out = runLevels({ levels: [500, 2000] }, deps);
  assert.strictEqual(out.table.length, 2);
  assert.strictEqual(out.table[0].rps, 500);
  assert.strictEqual(out.table[0].stackMb, 500);
  assert.strictEqual(out.table[0].clientMb, 50);
  assert.strictEqual(out.table[1].stackMb, 2000);
  assert.strictEqual(out.table[1].clientMb, 200);
});

test('runLevels throws when a required dep is missing', () => {
  assert.throws(
    () => runLevels({ levels: [500] }, { sampler: { samples: () => [], tick() {}, tripped: false } }),
    /runLevel/
  );
  assert.throws(
    () => runLevels({ levels: [500] }, { runLevel: () => ({}) }),
    /sampler/
  );
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

test('runRamp returns the failure-knee verdict on high failedRate', () => {
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

test('runRamp verdict is FAILURE_KNEE even when failures are within tolerance', () => {
  const deps = {
    runRamp: (opts) => ({ rps: opts.ceiling, p95: 3, failedRate: 0.01 }),
    sampler: {
      samples: () => [{ totalContainerMb: 700, clientMb: 150 }],
      tick() {},
      tripped: false,
    },
  };
  const out = runRamp({ ceiling: 5000 }, deps);
  assert.ok(['SAFETY_STOP', 'FAILURE_KNEE'].includes(out.verdict));
  assert.strictEqual(out.verdict, 'FAILURE_KNEE');
});

test('runRamp throws when a required dep is missing', () => {
  assert.throws(
    () => runRamp({ ceiling: 6000 }, { sampler: { samples: () => [], tick() {}, tripped: false } }),
    /runRamp/
  );
  assert.throws(
    () => runRamp({ ceiling: 6000 }, { runRamp: () => ({}) }),
    /sampler/
  );
});
