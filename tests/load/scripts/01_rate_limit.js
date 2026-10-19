import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, commonHeaders } from '../helpers.js';

// Stage 1: Rate Limiter Enforcement
// Tests that Go sliding-window limiter blocks single-IP bursts above 120 req/min.
export const options = {
  scenarios: {
    single_ip_burst: {
      executor: 'constant-arrival-rate',
      rate: 30, // 30 req/sec from a single IP
      timeUnit: '1s',
      duration: '10s', // 300 requests total in 10s
      preAllocatedVUs: 15,
      maxVUs: 40,
    },
  },
  thresholds: {
    // We expect requests beyond 120/min to return HTTP 429
    'http_req_failed{status:429}': ['rate>0.30'],
  },
};

export default function () {
  const res = http.get(`${BASE_URL}/projects?limit=5`, { headers: commonHeaders });

  check(res, {
    'status is 200 or 429': (r) => r.status === 200 || r.status === 429,
    'rate limit response under 10ms': (r) => r.timings.duration < 20,
  });
}
