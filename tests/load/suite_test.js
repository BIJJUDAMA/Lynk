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
  runSmoke,
  runRamRamp,
  runRamLevels,
  runThroughput,
  parseSummaryResult,
  runFlow,
  SMOKE_SCRIPT,
} = require('./suite.js');
const { ResourceSampler } = require('./ResourceSampler.js');
const resources = require('./resources.js');

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

test('parseArgs rejects non-numeric and non-positive safety flags', () => {
  assert.throws(() => parseArgs(['--mode=ram', '--safety-mb=abc']), /safety-mb/);
  assert.throws(() => parseArgs(['--mode=ram', '--tolerance=0']), /tolerance/);
  assert.throws(() => parseArgs(['--mode=ram', '--iterations=-1']), /iterations/);
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

test('main-style flow runs the mem-limit guard after stack bring-up', async () => {
  // Bring-up succeeds, readiness passes, then the guard inspects the config the
  // running stack produced. A capped post-bring-up config must fail the ram path
  // before the driver runs.
  const capped = [];
  const cappedCode = await runFlow(
    { mode: 'ram', safetyMb: 4096, levels: null },
    {
      upStack: () => { capped.push('upStack'); return { ok: true, log: '' }; },
      waitForReady: async () => { capped.push('waitForReady'); return true; },
      readComposeConfig: () => {
        capped.push('readComposeConfig');
        return 'services:\n  api:\n    mem_limit: 512m\n';
      },
      assertNoMemLimit: resources.assertNoMemLimit,
      suiteDeps: { runRamp: () => { capped.push('driver'); return 0; } },
      error: () => {},
    }
  );
  assert.strictEqual(cappedCode, 2, 'a capped ram run must exit 2');
  assert.deepStrictEqual(
    capped,
    ['upStack', 'waitForReady', 'readComposeConfig'],
    'guard must run after bring-up and the driver must not run when capped'
  );

  // A non-capped config proceeds from the guard to the driver.
  const ok = [];
  const okCode = await runFlow(
    { mode: 'ram', safetyMb: 4096, levels: null },
    {
      upStack: () => { ok.push('upStack'); return { ok: true, log: '' }; },
      waitForReady: async () => { ok.push('waitForReady'); return true; },
      readComposeConfig: () => {
        ok.push('readComposeConfig');
        return 'services:\n  api:\n    image: lynk-api\n';
      },
      assertNoMemLimit: resources.assertNoMemLimit,
      suiteDeps: { runRamp: () => { ok.push('driver'); return 0; } },
      error: () => {},
    }
  );
  assert.strictEqual(okCode, 0, 'a non-capped ram run must proceed to the driver');
  assert.deepStrictEqual(ok, ['upStack', 'waitForReady', 'readComposeConfig', 'driver']);
});

test('main-style flow runs the guard without bring-up when --no-up', async () => {
  const events = [];
  const code = await runFlow(
    { mode: 'ram', safetyMb: 4096, noUp: true, levels: null },
    {
      upStack: () => { events.push('upStack'); return { ok: true, log: '' }; },
      waitForReady: async () => { events.push('waitForReady'); return true; },
      readComposeConfig: () => {
        events.push('readComposeConfig');
        return 'services:\n  api:\n    mem_limit: 512m\n';
      },
      assertNoMemLimit: resources.assertNoMemLimit,
      suiteDeps: { runRamp: () => { events.push('driver'); return 0; } },
      error: () => {},
    }
  );
  assert.strictEqual(code, 2, 'the guard still runs after an explicit no-up');
  assert.deepStrictEqual(events, ['readComposeConfig']);
});

test('runRamLevels drives each level at its own arrival rate', async () => {
  const targets = [];
  const emitted = [];
  await runRamLevels({ levels: [2000], safetyMb: 4096 }, {
    readComposeConfig: () => 'services:\n  api:\n    image: lynk-api\n',
    runArrivalJob: async (opts) => {
      targets.push(opts.arrivalTarget);
      return { exitCode: 0, p95: 6, failedRate: 0 };
    },
    sampler: fakeSampler(),
    emit: (r) => emitted.push(r),
  });
  assert.deepStrictEqual(targets, [2000]);
  assert.strictEqual(emitted[0].mode, 'levels');
  assert.strictEqual(emitted[0].table[0].rps, 2000);
});

test('runRamLevels exits 1 when the supervisor trips', async () => {
  const emitted = [];
  const code = await runRamLevels({ levels: [2000], safetyMb: 4096 }, {
    readComposeConfig: () => 'services:\n  api:\n    image: lynk-api\n',
    runArrivalJob: async () => ({ exitCode: 0, p95: 6, failedRate: 0 }),
    sampler: fakeSampler(),
    runWithSupervisor: async () => ({
      result: [{ rps: 2000, p95: 6, stackMb: 1, clientMb: 1 }],
      aborted: true,
      tripped: true,
    }),
    report: () => ({ jsonPath: 'json', mdPath: 'md' }),
    out: { write() {} },
    emit: (r) => emitted.push(r),
  });
  assert.strictEqual(code, 1);
  assert.strictEqual(emitted[0].verdict, 'SAFETY_STOP');
});

test('runRamLevels stays exit 0 when the supervisor does not trip', async () => {
  const emitted = [];
  const code = await runRamLevels({ levels: [2000], safetyMb: 4096 }, {
    readComposeConfig: () => 'services:\n  api:\n    image: lynk-api\n',
    runArrivalJob: async () => ({ exitCode: 0, p95: 6, failedRate: 0 }),
    sampler: fakeSampler(),
    runWithSupervisor: async () => ({
      result: [{ rps: 2000, p95: 6, stackMb: 1, clientMb: 1 }],
      aborted: false,
      tripped: false,
    }),
    report: () => ({ jsonPath: 'json', mdPath: 'md' }),
    out: { write() {} },
    emit: (r) => emitted.push(r),
  });
  assert.strictEqual(code, 0);
  assert.strictEqual(emitted[0].verdict, undefined);
});

test('parseSummaryResult surfaces failedRate, p95 and RPS', () => {
  const fixture = {
    metrics: {
      http_reqs: { count: 60000, rate: 2000 },
      http_req_failed: { value: 0.01 },
      http_req_duration: { 'p(95)': 12 },
    },
  };
  const r = parseSummaryResult(fixture);
  assert.strictEqual(r.rps, 2000);
  assert.strictEqual(r.failedRate, 0.01);
  assert.strictEqual(r.p95, 12);
});

test('runThroughput surfaces RPS from the injected job', async () => {
  let emitted = null;
  const code = await runThroughput({}, {
    runArrivalJob: async () => ({ exitCode: 0, failedRate: 0, p95: 5, rps: 1234 }),
    emit: (r) => { emitted = r; },
  });
  assert.strictEqual(code, 0);
  assert.strictEqual(emitted.rps, 1234);
});

test('SMOKE_SCRIPT declares failure thresholds so a down health check fails', () => {
  assert.match(SMOKE_SCRIPT, /thresholds/);
  assert.match(SMOKE_SCRIPT, /http_req_failed:\s*\['rate<0\.01'\]/);
  assert.match(SMOKE_SCRIPT, /checks:\s*\['rate>0\.99'\]/);
});

test('runSmoke returns non-zero when k6 exits non-zero', () => {
  let seen = null;
  const code = runSmoke({}, {
    runK6: (run) => { seen = run; return { status: 99 }; },
    report: () => ({ jsonPath: 'json', mdPath: 'md' }),
    out: { write() {} },
  });
  assert.notStrictEqual(code, 0);
  assert.ok(seen, 'the injected runner must be used');
  assert.ok(
    seen.env && seen.env.API_BASE_URL,
    'API_BASE_URL must be forwarded to the runner'
  );
});

test('runSmoke returns 0 when k6 exits 0', () => {
  const code = runSmoke({}, {
    runK6: () => ({ status: 0 }),
    report: () => ({ jsonPath: 'json', mdPath: 'md' }),
    out: { write() {} },
  });
  assert.strictEqual(code, 0);
});

test('runThroughput treats a k6 threshold exit as a successful saturation', async () => {
  let emitted = null;
  const code = await runThroughput({}, {
    runArrivalJob: async () => ({ exitCode: 99, failedRate: 0.42, p95: 12, rps: 6000 }),
    emit: (r) => { emitted = r; },
    report: () => ({ jsonPath: 'json', mdPath: 'md' }),
    out: { write() {} },
  });
  assert.strictEqual(code, 0);
  assert.strictEqual(emitted.exitCode, 0);
  assert.strictEqual(emitted.verdict, 'SERVER_CEILING');
});

test('runThroughput fails on a genuine crash with no summary', async () => {
  const code = await runThroughput({}, {
    runArrivalJob: async () => ({ exitCode: 1, failedRate: 0, p95: null, rps: null }),
    emit: () => {},
    report: () => ({ jsonPath: 'json', mdPath: 'md' }),
    out: { write() {} },
  });
  assert.notStrictEqual(code, 0);
});
