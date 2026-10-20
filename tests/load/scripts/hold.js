// k6 hold script for the connection-ceiling search driver.
//
// The target VU count and dwell come from the environment:
//   TARGET_VUS  - concurrent virtual users to ramp up to (default 1000)
//   DWELL       - seconds to hold the target plateau (default 20)
//   RAMP        - seconds to ramp from 0 to TARGET_VUS (default 10)
//
// Each VU opens a keep-alive connection, issues a slow heartbeat, then sleeps.
// Because connection reuse is enabled, the VU holds one live connection for the
// whole plateau rather than churning sockets, so the peak VUs sustained is a
// direct read of concurrent connection capacity.
// 100% pure ASCII

import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, getSimulatedClientHeaders } from '../helpers.js';

const target = Number(__ENV.TARGET_VUS || 1000);
const dwell = Number(__ENV.DWELL || 20);
const ramp = Number(__ENV.RAMP || 10);

export const options = {
  // Keep connections alive so each active VU maps to one held connection.
  noConnectionReuse: false,
  scenarios: {
    hold: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: `${ramp}s`, target },
        { duration: `${dwell}s`, target },
        { duration: '5s', target: 0 },
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

  // Heartbeat keeps the held connection actively in use.
  const res = http.get(`${BASE_URL}/health`, {
    headers,
    tags: { name: 'conn_hold' },
  });
  check(res, { 'hold status 200': (r) => r.status === 200 });

  // Hold the connection open (idle but keep-alive) between heartbeats.
  sleep(dwell);
}
