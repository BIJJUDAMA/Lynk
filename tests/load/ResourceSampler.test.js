const test = require('node:test');
const assert = require('node:assert');
const { ResourceSampler, parseDockerStats } = require('./ResourceSampler.js');

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

test('container totals exclude non-lynk- rows', () => {
  const { total, perContainer } = parseDockerStats(
    'lynk-api-1|45.83MiB / 512MiB\nsome-other-1|100MiB / 1GiB'
  );
  assert.strictEqual(perContainer['lynk-api-1'], 45.83);
  assert.strictEqual(perContainer['some-other-1'], undefined);
  assert.strictEqual(total, 45.83);
});

test('start/stop schedules ticks and cancels cleanly', async () => {
  let count = 0;
  const s = new ResourceSampler({
    intervalMs: 5,
    safetyMb: 1,
    readFree: () => { count += 1; return 20000; },
    readContainers: () => ({ total: 0, perContainer: {} }),
    readClientMb: () => 0,
  });
  s.start();
  await new Promise((r) => setTimeout(r, 30));
  s.stop();
  const atStop = count;
  await new Promise((r) => setTimeout(r, 30));
  assert.strictEqual(count, atStop, 'no ticks after stop');
  assert.ok(atStop >= 2, 'ticks were self-scheduled repeatedly');
});
