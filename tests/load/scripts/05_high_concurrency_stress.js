// Stage 5: High-Concurrency Stress Test (1,000 VUs)
// Pushes concurrency to 1,000 VUs to evaluate PgBouncer transaction pooling,
// Redis cache-aside, HTTP ETags, Gzip compression, multi-container scaling,
// and compound covering database indexes under peak campus load conditions.
// 100% pure ASCII

import http from 'k6/http';
import { check, sleep } from 'k6';
import {
  BASE_URL,
  searchQueries,
  getRandomElement,
  getSimulatedClientHeaders,
} from '../helpers.js';

export const options = {
  stages: [
    { duration: '15s', target: 200 },   // Warm-up to 200 VUs
    { duration: '20s', target: 500 },   // Tier 2: 500 VUs
    { duration: '30s', target: 1000 },  // Tier 3: 1,000 VUs (Scale ceiling test)
    { duration: '15s', target: 0 },     // Cooldown
  ],
  thresholds: {
    http_req_duration: ['p(95)<250', 'p(99)<500'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  const headers = getSimulatedClientHeaders(__VU);

  // 1. Project list feed (tests Redis cache + ETag + Gzip)
  const listRes = http.get(`${BASE_URL}/projects?limit=20`, { headers });
  check(listRes, { 'feed status 200': (r) => r.status === 200 });

  sleep(1.0 + Math.random() * 1.0);

  // 2. Search query (tests covering index)
  const query = getRandomElement(searchQueries);
  const searchRes = http.get(
    `${BASE_URL}/search/projects?q=${encodeURIComponent(query)}&limit=10`,
    { headers }
  );
  check(searchRes, { 'search status 200': (r) => r.status === 200 });

  sleep(1.0 + Math.random() * 1.5);
}
