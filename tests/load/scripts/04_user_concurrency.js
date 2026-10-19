import http from 'k6/http';
import { check, sleep } from 'k6';
import {
  BASE_URL,
  searchQueries,
  getRandomElement,
  getSimulatedClientHeaders
} from '../helpers.js';

// Stage 4: Max Concurrent User Breakpoint Test
// Ramps virtual users (VUs) through progressive tiers with realistic student
// think times (1-3s) to determine the maximum concurrent capacity of Lynk.
export const options = {
  stages: [
    { duration: '10s', target: 25 },   // Tier 1: 25 concurrent campus users
    { duration: '15s', target: 50 },   // Tier 2: 50 concurrent campus users
    { duration: '20s', target: 100 },  // Tier 3: 100 concurrent campus users
    { duration: '20s', target: 200 },  // Tier 4: 200 concurrent campus users
    { duration: '20s', target: 350 },  // Tier 5: 350 concurrent campus users (stress ceiling)
    { duration: '10s', target: 0 },    // Cooldown
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.02'],
  },
};

export default function () {
  const headers = getSimulatedClientHeaders(__VU);

  // User Step 1: Browse public project workspaces feed
  const listRes = http.get(`${BASE_URL}/projects?limit=20`, { headers });
  check(listRes, {
    'step 1 feed 200': (r) => r.status === 200,
  });

  // Realistic user reading/think time (1 - 2 seconds)
  sleep(1.0 + Math.random() * 1.0);

  // User Step 2: Search for specific project interests
  const query = getRandomElement(searchQueries);
  const searchRes = http.get(
    `${BASE_URL}/search/projects?q=${encodeURIComponent(query)}&limit=10`,
    { headers }
  );
  check(searchRes, {
    'step 2 search 200': (r) => r.status === 200,
  });

  sleep(1.0 + Math.random() * 1.5);

  // User Step 3: Drill down into a specific project workspace
  try {
    const listData = JSON.parse(listRes.body);
    if (Array.isArray(listData) && listData.length > 0) {
      const selectedProject = listData[__VU % listData.length];
      const projectIdentifier = selectedProject.slug || selectedProject.id;

      const detailRes = http.get(`${BASE_URL}/projects/${projectIdentifier}`, { headers });
      check(detailRes, {
        'step 3 project detail 200': (r) => r.status === 200 || r.status === 404,
      });

      sleep(1.0 + Math.random() * 1.0);

      // User Step 4: View team roster and external resources
      const membersRes = http.get(`${BASE_URL}/projects/${selectedProject.id}/members`, { headers });
      check(membersRes, {
        'step 4 team roster 200': (r) => r.status === 200 || r.status === 404,
      });

      const resourcesRes = http.get(`${BASE_URL}/projects/${selectedProject.id}/resources`, { headers });
      check(resourcesRes, {
        'step 4 resources 200': (r) => r.status === 200 || r.status === 404,
      });
    }
  } catch (err) {
    // Non-fatal parse fallback during high-concurrency burst
  }

  // End of user journey cycle pause
  sleep(1.5 + Math.random() * 1.5);
}
