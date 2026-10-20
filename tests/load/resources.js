// Resource curve driver for the load-test suite (Task 5).
//
// Drives the RAM/CPU curve in one of two modes:
//   - ramp (default): a ramping-arrival load is run while the ResourceSampler
//     ticks continuously. The run stops either when the sampler trips its
//     safety floor (verdict SAFETY_STOP) or when the arrival ceiling is
//     reached (verdict taken from the failure knee).
//   - levels: a fixed list of arrival levels is run one at a time, producing a
//     row per level so the caller can see where the stack bends.
//
// assertNoMemLimit guards the measured curve: a run whose compose config caps
// container memory with `mem_limit` is not natural usage, so we refuse it.
//
// Every external touchpoint (the k6 runner, the sampler) is injectable through
// `deps`, so the driver can be exercised without a real stack.
//
// 100% pure ASCII

'use strict';

// Compute the peak of a numeric field across sampler samples, ignoring
// non-numeric probes. Returns null when no usable sample exists.
function peakOf(samples, field) {
  let peak = null;
  for (const sample of samples || []) {
    if (!sample) continue;
    const value = sample[field];
    if (typeof value === 'number' && Number.isFinite(value)) {
      peak = peak === null ? value : Math.max(peak, value);
    }
  }
  return peak;
}

// Run a fixed list of arrival levels. Each level is executed via deps.runLevel,
// the sampler is ticked while it runs, and one row is recorded per level.
function runLevels(opts, deps) {
  const options = opts || {};
  const depsIn = deps || {};
  const sampler = depsIn.sampler;
  const runLevel = depsIn.runLevel;

  const levels = options.levels || [];
  const table = [];

  for (const rps of levels) {
    const result = (runLevel && runLevel(rps)) || {};
    if (sampler && typeof sampler.tick === 'function') sampler.tick();
    const samples = sampler ? sampler.samples() || [] : [];
    table.push({
      rps,
      p95: typeof result.p95 === 'number' ? result.p95 : null,
      stackMb: peakOf(samples, 'totalContainerMb'),
      clientMb: peakOf(samples, 'clientMb'),
    });
  }

  return { mode: 'levels', table };
}

// Run a ramping-arrival load while sampling continuously. Stops at the
// sampler's safety trip (SAFETY_STOP) or at the arrival ceiling (verdict from
// the failure knee).
function runRamp(opts, deps) {
  const options = opts || {};
  const depsIn = deps || {};
  const sampler = depsIn.sampler;
  const runRampFn = depsIn.runRamp;

  const result = (runRampFn && runRampFn(options)) || {};
  if (sampler && typeof sampler.tick === 'function') sampler.tick();

  const samples = sampler ? sampler.samples() || [] : [];

  if (sampler && sampler.tripped) {
    return {
      mode: 'ramp',
      curve: samples,
      stackPeakMb: peakOf(samples, 'totalContainerMb'),
      clientPeakMb: peakOf(samples, 'clientMb'),
      verdict: 'SAFETY_STOP',
    };
  }

  const failed = typeof result.failedRate === 'number' ? result.failedRate : 0;
  const threshold =
    typeof options.failureRate === 'number' ? options.failureRate : 0.1;
  const verdict = failed > threshold ? 'FAILURE_KNEE' : 'CEILING_REACHED';

  return {
    mode: 'ramp',
    curve: samples,
    stackPeakMb: peakOf(samples, 'totalContainerMb'),
    clientPeakMb: peakOf(samples, 'clientMb'),
    verdict,
  };
}

// Reject a compose config that caps container memory. `docker compose config`
// output containing `mem_limit` is a hard failure: a capped run cannot be
// reported as natural resource usage.
function assertNoMemLimit(composeConfigText) {
  const text = composeConfigText == null ? '' : String(composeConfigText);
  if (/mem_limit/.test(text)) {
    throw new Error(
      'compose config contains mem_limit: refusing to report a capped run as natural usage'
    );
  }
}

module.exports = {
  runLevels,
  runRamp,
  assertNoMemLimit,
  peakOf,
};
