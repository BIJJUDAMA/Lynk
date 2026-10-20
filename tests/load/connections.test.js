// Tests for the self-searching connection ceiling driver.
// 100% pure ASCII

const test = require('node:test');
const assert = require('node:assert');
const { searchCeiling, classify, prepareEnv } = require('./connections.js');

test('classify: server failure with healthy client is SERVER_CEILING', () => {
  const v = classify({ exitCode: 0, failedRate: 0.9, dropped: 0, tripped: false, timeWaitHigh: false });
  assert.strictEqual(v.verdict, 'SERVER_CEILING');
});

test('classify: client death is CLIENT_BOUND', () => {
  const v = classify({ exitCode: 137, failedRate: 0.5, dropped: 9999, tripped: false, timeWaitHigh: true });
  assert.strictEqual(v.verdict, 'CLIENT_BOUND');
});

test('classify: safety trip is SAFETY_STOP', () => {
  const v = classify({ exitCode: 0, failedRate: 0.0, dropped: 0, tripped: true, timeWaitHigh: false });
  assert.strictEqual(v.verdict, 'SAFETY_STOP');
});

test('classify: k6 threshold exit 99 with server failures is SERVER_CEILING', () => {
  // k6 exits 99 when a threshold is breached; that is NOT a client crash.
  const v = classify({ exitCode: 99, failedRate: 0.9, dropped: 0, tripped: false, timeWaitHigh: false });
  assert.strictEqual(v.verdict, 'SERVER_CEILING');
});

test('classify: client port exhaustion with server failures is CLIENT_BOUND', () => {
  // Port-bound client (unsupported host, TIME_WAIT unreadable) must never be
  // reported as the server's ceiling.
  const v = classify({ exitCode: 0, failedRate: 0.9, dropped: 0, tripped: false, timeWaitHigh: false, clientPortsExhausted: true });
  assert.strictEqual(v.verdict, 'CLIENT_BOUND');
});

test('searchCeiling converges into the bracket', () => {
  // Stubbed runner: server "fails" above 8000 connections.
  const deps = {
    runK6: (target) => ({ exitCode: 0, failedRate: target > 8000 ? 0.9 : 0.0, dropped: 0, vusMax: target }),
    serverConns: () => null,
    sampler: { tripped: false, tick() {} },
  };
  const res = searchCeiling({ max: 64000, dwell: 1, tolerance: 0.5, iterations: 8 }, deps);
  assert.strictEqual(res.verdict, 'SERVER_CEILING');
  assert.ok(res.n >= 4000 && res.n <= 16000, `expected between 4k and 16k, got ${res.n}`);
});

test('prepareEnv forwards only the variables the k6 script reads', () => {
  process.env.SECRET_TOKEN = 'do-not-forward';
  try {
    const run = prepareEnv(500, 20);
    assert.deepStrictEqual(
      Object.keys(run.env).sort(),
      ['API_BASE_URL', 'DWELL', 'TARGET_VUS']
    );
    assert.ok(!('PATH' in run.env), 'PATH must not leak into the docker fallback');
    assert.ok(!('SECRET_TOKEN' in run.env), 'secrets must not leak into the docker fallback');
  } finally {
    delete process.env.SECRET_TOKEN;
  }
});
