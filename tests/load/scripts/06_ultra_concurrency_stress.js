// Stage 6: Ultra-Concurrency Stress Test (2,500 VUs)
// Pushes concurrency to 2,500 VUs to evaluate Nginx micro-caching with request collapsing,
// TwoTierCache (L1 in-memory + L2 Redis), and PostgreSQL 16 RAM buffer tuning under extreme load.
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
    { duration: '15s', target: 500 },    // Warm-up to 500 VUs
    { duration: '20s', target: 1500 },   // Tier 2: 1,500 VUs
    { duration: '30s', target: 2500 },   // Tier 3: 2,500 VUs (Ultra-Scale ceiling)
    { duration: '15s', target: 0 },      // Cooldown
  ],
  thresholds: {
    http_req_duration: ['p(95)<150', 'p(99)<300'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  const headers = getSimulatedClientHeaders(__VU);

  // 1. Project list feed (exercises Nginx microcache + TwoTierCache L1 memory + ETag)
  const listRes = http.get(`${BASE_URL}/projects?limit=20`, { headers });
  check(listRes, { 'feed status 200': (r) => r.status === 200 });

  sleep(0.5 + Math.random() * 0.5);

  // 2. Search query (exercises PostgreSQL 1GB shared buffers in RAM + covering index)
  const query = getRandomElement(searchQueries);
  const searchRes = http.get(
    `${BASE_URL}/search/projects?q=${encodeURIComponent(query)}&limit=10`,
    { headers }
  );
  check(searchRes, { 'search status 200': (r) => r.status === 200 });

  sleep(0.5 + Math.random() * 1.0);
}
