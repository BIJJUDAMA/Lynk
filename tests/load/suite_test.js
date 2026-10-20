// Tests for the parameterized load suite entrypoint.
// 100% pure ASCII

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const {
  parseArgs,
  MODES,
  runSuite,
  runWithSupervisor,
  runRamRamp,
  runRamLevels,
  runThroughput,
  parseSummaryResult,
} = require('./suite.js');
const { ResourceSampler } = require('./ResourceSampler.js');

// A minimal sampler double for exercising the ram orchestration without a host.
function fakeSampler() {
  return {
    clientPid: null,
    tripped: false,
    started: false,
    stopped: false,
    start() { this.started = true; return this; },
    stop() { this.stopped = true; return this; },
    tick() {},
    samples() { return []; },
  };
}

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

test('parseArgs coerces numeric flags to integers', () => {
  const o = parseArgs(['--mode=connections', '--target=500', '--max=64000', '--dwell=20']);
  assert.strictEqual(o.max, 64000);
  assert.strictEqual(typeof o.max, 'number');
  assert.strictEqual(o.dwell, 20);
  assert.strictEqual(typeof o.dwell, 'number');
  assert.strictEqual(o.target, 500);
  assert.strictEqual(typeof o.target, 'number');
});

test('parseArgs rejects a non-integer max', () => {
  assert.throws(() => parseArgs(['--mode=connections', '--max=x']));
});

test('MODES is the fixed set', () => {
  assert.deepStrictEqual(MODES, ['throughput', 'connections', 'ram', 'smoke']);
});

test('nginx.conf declares stub_status', () => {
  const conf = path.resolve(__dirname, '..', '..', '.docker', 'nginx', 'nginx.conf');
  const c = fs.readFileSync(conf, 'utf8');
  assert.match(c, /stub_status/);
});

test('runSuite dispatches each mode to the right driver', () => {
  const calls = [];
  const deps = {
    runSmoke: () => { calls.push('smoke'); return 0; },
    searchCeiling: () => { calls.push('connections'); return 0; },
    runRamp: () => { calls.push('ram-ramp'); return 0; },
    runLevels: () => { calls.push('ram-levels'); return 0; },
    runThroughput: () => { calls.push('throughput'); return 0; },
  };
  assert.strictEqual(runSuite({ mode: 'smoke' }, deps), 0);
  assert.strictEqual(runSuite({ mode: 'connections' }, deps), 0);
  assert.strictEqual(runSuite({ mode: 'ram' }, deps), 0);
  assert.strictEqual(runSuite({ mode: 'ram', levels: [500] }, deps), 0);
  assert.strictEqual(runSuite({ mode: 'throughput' }, deps), 0);
  assert.deepStrictEqual(calls, ['smoke', 'connections', 'ram-ramp', 'ram-levels', 'throughput']);
});

test('runWithSupervisor starts/stops the sampler and records the child pid', async () => {
  const sampler = fakeSampler();
  const fakeChild = { pid: 4242, exitCode: null, signalCode: null, kill() { this.exitCode = 0; } };
  const res = await runWithSupervisor((onChild) => {
    onChild(fakeChild);
    return Promise.resolve({ exitCode: 0 });
  }, { sampler, pollMs: 5, graceMs: 5 });

  assert.ok(sampler.started, 'sampler.start() must be called');
  assert.ok(sampler.stopped, 'sampler.stop() must be called');
  assert.strictEqual(sampler.clientPid, 4242);
  assert.strictEqual(res.tripped, false);
});

test('runWithSupervisor kills the child when the sampler trips', async () => {
  const kills = [];
  const sampler = fakeSampler();
  const res = await runWithSupervisor((onChild) => {
    onChild({
      pid: 7,
      exitCode: null,
      signalCode: null,
      kill(sig) { kills.push(sig); this.exitCode = 0; },
    });
    setTimeout(() => { sampler.tripped = true; }, 5);
    return new Promise((resolve) => setTimeout(() => resolve({ exitCode: 0 }), 40));
  }, { sampler, pollMs: 5, graceMs: 10 });

  assert.ok(kills.includes('SIGTERM'), `expected SIGTERM, got ${JSON.stringify(kills)}`);
  assert.strictEqual(res.aborted, true);
  assert.strictEqual(res.tripped, true);
});

test('runWithSupervisor populates clientMb from an injected reader and child pid', async () => {
  let seenPid = null;
  const sampler = new ResourceSampler({
    intervalMs: 100000,
    safetyMb: 0,
    readFree: () => 1000000,
    readContainers: () => ({ total: 10, perContainer: {} }),
    readClientMb: () => {
      seenPid = sampler.clientPid;
      return sampler.clientPid ? 321 : null;
    },
  });

  const fakeChild = { pid: 4242, exitCode: null, signalCode: null, kill() { this.exitCode = 0; } };
  await runWithSupervisor((onChild) => {
    onChild(fakeChild);
    return Promise.resolve({ exitCode: 0 });
  }, { sampler, pollMs: 5, graceMs: 5 });

  sampler.tick();
  assert.strictEqual(sampler.clientPid, 4242);
  assert.strictEqual(seenPid, 4242);
  assert.ok(sampler.samples().some((s) => s.clientMb === 321));
});

test('runRamRamp calls the mem-limit guard before load', async () => {
  let guarded = false;
  const emitted = [];
  await runRamRamp({ safetyMb: 4096 }, {
    readComposeConfig: () => { guarded = true; return 'services:\n  api:\n    image: lynk-api\n'; },
    runArrivalJob: async () => ({ exitCode: 0, p95: 4, failedRate: 0 }),
    sampler: fakeSampler(),
    emit: (r) => emitted.push(r),
  });
  assert.ok(guarded, 'readComposeConfig must be consulted');
  assert.strictEqual(emitted.length, 1);
  assert.strictEqual(emitted[0].mode, 'ramp');
});

test('runRamRamp fails when the compose config caps memory', async () => {
  await assert.rejects(
    () => runRamRamp({ safetyMb: 4096 }, {
      readComposeConfig: () => 'services:\n  api:\n    mem_limit: 512m\n',
    }),
    /mem_limit/
  );
});
