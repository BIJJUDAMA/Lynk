import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, getSimulatedClientHeaders } from '../helpers.js';

// Stage 3: PostgreSQL Connection Pool Saturation
// Pushes concurrency beyond pgxpool.MaxConns (25 connections) to verify
// that Go worker queues handle DB contention without socket crashes or 500 errors.
export const options = {
  stages: [
    { duration: '10s', target: 25 },  // Exactly fill the 25 pool connections
    { duration: '20s', target: 60 },  // Force 35+ requests to queue in Go pgxpool wait queue
    { duration: '10s', target: 0 },   // Drain pool queue
  ],
  thresholds: {
    // Queued requests may take longer, but error rate must stay under 2%
    http_req_failed: ['rate<0.02'],
    http_req_duration: ['p(95)<450'],
  },
};

export default function () {
  const headers = getSimulatedClientHeaders(__VU);

  // 1. Fetch project collection
  const listRes = http.get(`${BASE_URL}/projects?limit=20`, { headers });
  const listOk = check(listRes, {
    'pool test list 200': (r) => r.status === 200,
  });

  if (!listOk) {
    sleep(0.5);
    return;
  }

  try {
    const projects = JSON.parse(listRes.body);
    if (Array.isArray(projects) && projects.length > 0) {
      const sample = projects[__VU % projects.length];
      const targetId = sample.slug || sample.id;

      // 2. Fetch project details
      const detailRes = http.get(`${BASE_URL}/projects/${targetId}`, { headers });
      check(detailRes, {
        'pool test detail 200': (r) => r.status === 200 || r.status === 404,
      });

      // 3. Concurrently fetch team roster and modular sections
      const membersRes = http.get(`${BASE_URL}/projects/${sample.id}/members`, { headers });
      check(membersRes, {
        'pool test members 200': (r) => r.status === 200 || r.status === 404,
      });

      const sectionsRes = http.get(`${BASE_URL}/projects/${sample.id}/sections`, { headers });
      check(sectionsRes, {
        'pool test sections 200': (r) => r.status === 200 || r.status === 404,
      });
    }
  } catch (e) {
    // JSON parsing fallback
  }

  sleep(0.1 + Math.random() * 0.2);
}
