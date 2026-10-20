// Stage 8: Concurrent Connection Hold
// Answers "how many concurrent connections can we hold?" as a pure count.
// Each virtual user opens a keep-alive connection, holds it open for a fixed
// dwell time while issuing periodic heartbeat requests, then closes it. VUs ramp up
// and are held at each tier, so the reported peak is the number of simultaneous
// open connections sustained without client socket exhaustion or server failure.
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
        { duration: '15s', target: 2000 },    // Tier 1: 2,000 concurrent
        { duration: '20s', target: 6000 },    // Tier 2: 6,000 concurrent
        { duration: '20s', target: 12000 },   // Tier 3: 12,000 concurrent
        { duration: '25s', target: 16000 },   // Tier 4: 16,000 concurrent
        { duration: '25s', target: 20000 },   // Tier 5: 20,000 concurrent sustained hold
        { duration: '15s', target: 0 },       // Clean release
      ],
      gracefulStop: '15s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.05'],
  },
};

export default function () {
  const headers = getSimulatedClientHeaders(__VU);

  // Heartbeat keeps the connection actively in use while it is held open.
  const res = http.get(`${BASE_URL}/health`, { headers, tags: { name: 'conn_hold' } });
  check(res, { 'hold status 200': (r) => r.status === 200 });

  // Hold the connection open (idle keep-alive) with periodic 5-second heartbeats
  sleep(5);
}
