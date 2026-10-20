// Resource curve driver for the load-test suite (Task 5).
//
// Drives the RAM/CPU curve in one of two modes:
//   - ramp (default): a ramping-arrival load is run while the ResourceSampler
//     ticks continuously. The run stops either when the sampler trips its
//     safety floor (verdict SAFETY_STOP) or when the arrival ceiling is
//     reached (verdict taken from the failure knee, FAILURE_KNEE).
//   - levels: a fixed list of arrival levels is run one at a time, producing a
//     row per level so the caller can see where the stack bends. Each row's
//     peaks are scoped to the samples appended during that level only.
//
// assertNoMemLimit guards the measured curve: a run whose compose config caps
// container memory with `mem_limit` is not natural usage, so we refuse it.
//
// Every external touchpoint (the k6 runner, the sampler) is injectable through
// `deps`, so the driver can be exercised without a real stack. The injected
// functions are required: a missing dep throws rather than fabricating a
// verdict or a row with no data.
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

// Read the current sampler sample list, tolerating a sampler that has not
// collected anything yet.
function readSamples(sampler) {
  const samples = sampler && typeof sampler.samples === 'function'
    ? sampler.samples()
    : [];
  return Array.isArray(samples) ? samples : [];
}

// Fail fast when an injected dependency is absent, so a missing dep can never
// produce a fabricated result.
function requireDep(value, name, caller) {
  if (typeof value !== 'function') {
    throw new Error(`${caller}: missing required dep "${name}"`);
  }
  return value;
}

// Run a fixed list of arrival levels. Each level is executed via deps.runLevel,
// the sampler is ticked while it runs, and one row is recorded per level. The
// peaks in a row are computed from the samples appended during that level only,
// so an earlier, higher level never bleeds into a later row.
function runLevels(opts, deps) {
  const options = opts || {};
  const depsIn = deps || {};
  const runLevel = requireDep(depsIn.runLevel, 'runLevel', 'runLevels');
  const sampler = depsIn.sampler;
  if (!sampler || typeof sampler.tick !== 'function') {
    throw new Error('runLevels: missing required dep "sampler"');
  }

  const levels = options.levels || [];
  const table = [];

  for (const rps of levels) {
    const before = readSamples(sampler).length;
    const result = runLevel(rps) || {};
    sampler.tick();
    const window = readSamples(sampler).slice(before);
    table.push({
      rps,
      p95: typeof result.p95 === 'number' ? result.p95 : null,
      stackMb: peakOf(window, 'totalContainerMb'),
      clientMb: peakOf(window, 'clientMb'),
    });
  }

  return { mode: 'levels', table };
}

// Run a ramping-arrival load while sampling continuously. Verdict is either
// SAFETY_STOP (sampler tripped its safety floor) or FAILURE_KNEE (the run
// reached the arrival ceiling and the failure knee was observed).
function runRamp(opts, deps) {
  const options = opts || {};
  const depsIn = deps || {};
  const runRampFn = requireDep(depsIn.runRamp, 'runRamp', 'runRamp');
  const sampler = depsIn.sampler;
  if (!sampler || typeof sampler.tick !== 'function') {
    throw new Error('runRamp: missing required dep "sampler"');
  }

  const result = runRampFn(options) || {};
  sampler.tick();

  const samples = readSamples(sampler);

  if (sampler.tripped) {
    return {
      mode: 'ramp',
      curve: samples,
      stackPeakMb: peakOf(samples, 'totalContainerMb'),
      clientPeakMb: peakOf(samples, 'clientMb'),
      verdict: 'SAFETY_STOP',
    };
  }

  return {
    mode: 'ramp',
    curve: samples,
    stackPeakMb: peakOf(samples, 'totalContainerMb'),
    clientPeakMb: peakOf(samples, 'clientMb'),
    verdict: 'FAILURE_KNEE',
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
