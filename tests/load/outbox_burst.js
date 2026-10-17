import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, commonHeaders } from './helpers.js';

export const options = {
  stages: [
    { duration: '10s', target: 15 },
    { duration: '20s', target: 30 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    // Health and metrics endpoints must stay sub-50ms even during load
    http_req_duration: ['p(95)<50'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  const res = http.get(`${BASE_URL}/metrics`, { headers: commonHeaders });

  check(res, {
    'metrics status is 200': (r) => r.status === 200,
    'contains prometheus metric': (r) => r.body.includes('http_requests_total') || r.body.includes('go_goroutines'),
  });

  sleep(0.2);
}
