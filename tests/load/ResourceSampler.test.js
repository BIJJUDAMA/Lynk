const test = require('node:test');
const assert = require('node:assert');
const { ResourceSampler } = require('./ResourceSampler.js');

test('trips when free memory falls to the floor', () => {
  let t = 0;
  const feed = [8000, 6000, 4096, 2048]; // MiB free
  const s = new ResourceSampler({
    intervalMs: 0,
    safetyMb: 4096,
    readFree: () => feed[Math.min(t++, feed.length - 1)],
    readContainers: () => ({ total: 900, perContainer: {} }),
    readClientMb: () => 100,
  });
  s.tick(); s.tick();
  assert.strictEqual(s.tripped, false, 'not tripped at 6000 free');
  s.tick();
  assert.strictEqual(s.tripped, true, 'tripped at 4096 free');
  assert.match(s.tripReason, /SAFETY_STOP/);
});

test('attributes client vs container memory separately', () => {
  const s = new ResourceSampler({
    intervalMs: 0, safetyMb: 1,
    readFree: () => 20000,
    readContainers: () => ({ total: 850, perContainer: { 'lynk-api-1': 60 } }),
    readClientMb: () => 1200,
  });
  s.tick();
  const [sample] = s.samples();
  assert.strictEqual(sample.totalContainerMb, 850);
  assert.strictEqual(sample.clientMb, 1200);
  assert.strictEqual(sample.perContainer['lynk-api-1'], 60);
});
