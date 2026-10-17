import http from 'k6/http';

export const BASE_URL = __ENV.API_BASE_URL || 'http://localhost:8080';

export const commonHeaders = {
  'Content-Type': 'application/json',
  'Accept': 'application/json',
};

export const departments = [
  'Computer Science',
  'Electrical Engineering',
  'Design',
  'Business Administration',
  'Mechanical Engineering'
];

export const searchQueries = [
  'frontend react',
  'python backend',
  'machine learning',
  'ui ux design',
  'data analysis',
  'tutor mathematics',
  'mobile flutter',
  'devops kubernetes'
];

export function getRandomElement(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}
