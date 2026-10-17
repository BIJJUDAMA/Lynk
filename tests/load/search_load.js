import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, commonHeaders, searchQueries, getRandomElement } from './helpers.js';

export const options = {
  stages: [
    { duration: '15s', target: 20 },
    { duration: '30s', target: 50 },
    { duration: '15s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<150', 'p(99)<300'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  const query = getRandomElement(searchQueries);
  const url = `${BASE_URL}/api/v1/jobs?query=${encodeURIComponent(query)}&limit=10`;

  const res = http.get(url, { headers: commonHeaders });

  check(res, {
    'status is 200': (r) => r.status === 200,
    'has jobs array': (r) => {
      try {
        const body = JSON.parse(r.body);
        return Array.isArray(body.jobs) || Array.isArray(body);
      } catch (e) {
        return false;
      }
    },
  });

  sleep(0.1 + Math.random() * 0.2);
}
