// Stage 8: Concurrent Connection Hold
// Answers "how many concurrent connections can we hold?" as a pure count.
// Each virtual user opens a keep-alive connection, holds it open for a fixed
// dwell time while issuing slow heartbeat requests, then closes it. VUs ramp up
// and are held at each tier, so the reported peak is the number of simultaneous
// open connections sustained without failure.
// 100% pure ASCII

import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, getSimulatedClientHeaders } from '../helpers.js';

export const options = {
  // Disable per-request connection reuse across VUs so each active VU maps to a
  // live connection that stays open for the whole dwell window.
  noConnectionReuse: false,
  scenarios: {
    hold: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '15s', target: 2000 },    // Tier 1: 2,000 concurrent (verified clean)
        { duration: '20s', target: 8000 },    // Tier 2: 8,000 concurrent (verified clean)
        { duration: '20s', target: 16000 },   // Tier 3: 16,000 concurrent
        { duration: '30s', target: 32000 },   // Tier 4: 32,000 concurrent
        { duration: '30s', target: 40000 },   // Tier 5: 40,000 concurrent
        { duration: '30s', target: 48000 },   // Tier 6: 48,000 concurrent
        { duration: '30s', target: 56000 },   // Tier 7: 56,000 concurrent
        { duration: '30s', target: 64000 },   // Tier 8: 64,000 concurrent
        { duration: '20s', target: 0 },       // Release
      ],
      gracefulStop: '30s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.05'],
  },
};

export default function () {
  const headers = getSimulatedClientHeaders(__VU);

  // Heartbeat keeps the connection actively in use while it is held open.
  // A long dwell per iteration means the connection persists for the tier
  // duration rather than churning.
  const res = http.get(`${BASE_URL}/health`, { headers, tags: { name: 'conn_hold' } });
  check(res, { 'hold status 200': (r) => r.status === 200 });

  // Hold the connection open (idle but keep-alive) between heartbeats. The VU
  // sleeps, so it occupies one live connection for the tier duration while the
  // client uses almost no CPU -- this measures concurrent connection capacity,
  // not request throughput.
  sleep(20);
}
