import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, commonHeaders } from './helpers.js';

export const options = {
  // Burst requests within 10 seconds from a single IP to exceed the 120 req/min limit
  scenarios: {
    burst_rate_limit: {
      executor: 'constant-arrival-rate',
      rate: 30, // 30 requests per second
      timeUnit: '1s',
      duration: '10s',
      preAllocatedVUs: 20,
      maxVUs: 50,
    },
  },
  thresholds: {
    // We expect some requests to return 429 Too Many Requests
    'http_req_failed{status:429}': ['rate>0.05'],
  },
};

export default function () {
  const url = `${BASE_URL}/api/v1/jobs?limit=5`;
  const res = http.get(url, { headers: commonHeaders });

  check(res, {
    'status is 200 or 429': (r) => r.status === 200 || r.status === 429,
  });
}
