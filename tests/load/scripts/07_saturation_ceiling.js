// Stage 7: Constant-Arrival Saturation Ceiling
// Unlike the VU-based stages, this uses k6's constant-arrival-rate executor with
// NO think time, so throughput is driven by a fixed arrival target rather than by
// how many virtual users the client can hold. It ramps the arrival rate upward
// until the server saturates, revealing the true per-pod server ceiling.
// 100% pure ASCII

import http from 'k6/http';
import { check } from 'k6';
import {
  BASE_URL,
  searchQueries,
  getRandomElement,
  getSimulatedClientHeaders,
} from '../helpers.js';

// Target arrivals per second, ramped in tiers. Start far above the VU-bound
// measurements so the client, not the VU count, is the only limit.
export const options = {
  discardResponseBodies: true,
  scenarios: {
    saturation: {
      executor: 'ramping-arrival-rate',
      startRate: 500,
      timeUnit: '1s',
      preAllocatedVUs: 400,
      maxVUs: 4000,
      stages: [
        { target: 1000, duration: '15s' },
        { target: 2000, duration: '20s' },
        { target: 4000, duration: '20s' },
        { target: 6000, duration: '20s' },
        { target: 8000, duration: '20s' },
        { target: 0, duration: '10s' },
      ],
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.10'],
  },
};

export default function () {
  const headers = getSimulatedClientHeaders(__VU);
  const query = getRandomElement(searchQueries);
  const res = http.get(`${BASE_URL}/search/projects?q=${encodeURIComponent(query)}&limit=10`, { headers });
  check(res, { 'search status 200': (r) => r.status === 200 });
}
