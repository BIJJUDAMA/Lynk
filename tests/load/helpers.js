// Shared helpers and constants for Lynk k6 load testing suite
// 100% pure ASCII

export const BASE_URL = __ENV.API_BASE_URL || 'http://localhost:8080/api/v1';

export const commonHeaders = {
  'Content-Type': 'application/json',
  'Accept': 'application/json',
};

export const projectCategories = [
  'Engineering',
  'Design',
  'AI',
  'Product',
  'Research',
  'Writing'
];

export const searchQueries = [
  'autonomous rover firmware',
  'react typescript frontend',
  'computer vision neural net',
  'campus delivery navigation',
  'embedded c++ robotics',
  'ui ux design system',
  'distributed systems consensus',
  'natural language processing'
];

export function getRandomElement(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

// Generate realistic simulated client IP per campus student across 10.x.y.z subnets
// to accurately simulate multi-tenant campus traffic without false single-IP rate limits
export function getSimulatedClientHeaders(vuId) {
  const b = (vuId != null && vuId > 0) ? (vuId % 250) : Math.floor(Math.random() * 250) + 1;
  const c = Math.floor(Math.random() * 250) + 1;
  const d = Math.floor(Math.random() * 250) + 1;
  return {
    ...commonHeaders,
    'X-Forwarded-For': `10.${b}.${c}.${d}`,
  };
}
