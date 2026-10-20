// Stage 1: Rate Limiter Enforcement
// Tests that Go sliding-window limiter blocks single-IP bursts above 120 req/min.
// 100% pure ASCII

import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, commonHeaders } from '../helpers.js';

export const options = {
  scenarios: {
    single_ip_burst: {
      executor: 'constant-arrival-rate',
      rate: 30, // 30 req/sec from a single IP
      timeUnit: '1s',
      duration: '10s', // 300 requests total in 10s (limit is 120/min)
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
  // Pass a consistent single-IP header and cache-bypass headers to test the Go sliding window limiter
  const headers = {
    ...commonHeaders,
    'X-Forwarded-For': '198.51.100.42',
    'Cache-Control': 'no-cache',
  };

  // Target search endpoint (which is not micro-cached) to ensure requests reach the Go limiter
  const res = http.get(`${BASE_URL}/projects/search?q=test-rate-limit`, { headers });

  check(res, {
    'status is 200 or 429': (r) => r.status === 200 || r.status === 429,
    'rate limit response under 20ms': (r) => r.timings.duration < 20,
  });
}
