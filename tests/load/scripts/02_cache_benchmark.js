import http from 'k6/http';
import { check, sleep } from 'k6';
import {
  BASE_URL,
  searchQueries,
  projectCategories,
  getRandomElement,
  getSimulatedClientHeaders
} from '../helpers.js';

// Stage 2: Cache & Read Path Benchmark
// Measures throughput and latency on project list and search endpoints.
// Uses simulated multi-tenant IPs to evaluate raw server throughput.
export const options = {
  stages: [
    { duration: '10s', target: 20 },  // Warmup
    { duration: '20s', target: 40 },  // Steady throughput
    { duration: '10s', target: 0 },   // Cooldown
  ],
  thresholds: {
    http_req_duration: ['p(95)<250', 'p(99)<500'],
    http_req_failed: ['rate<0.02'],
  },
};

export default function () {
  const headers = getSimulatedClientHeaders(__VU);

  // Warm read path: Top project workspace listing
  const listRes = http.get(`${BASE_URL}/projects?limit=10`, { headers });
  check(listRes, {
    'list projects 200': (r) => r.status === 200,
  });

  // Cold read path: Dynamic search query with pgvector / full-text match
  const query = getRandomElement(searchQueries);
  const searchRes = http.get(
    `${BASE_URL}/search/projects?q=${encodeURIComponent(query)}&limit=5`,
    { headers }
  );
  check(searchRes, {
    'search projects 200': (r) => r.status === 200,
  });

  // Filtered read path: Filter by category
  const category = getRandomElement(projectCategories);
  const filterRes = http.get(
    `${BASE_URL}/projects?category=${encodeURIComponent(category)}&limit=10`,
    { headers }
  );
  check(filterRes, {
    'category filter 200': (r) => r.status === 200,
  });

  sleep(0.2 + Math.random() * 0.3);
}
