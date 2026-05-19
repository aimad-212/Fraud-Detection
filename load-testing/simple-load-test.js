import http from 'k6/http';
import { check, sleep } from 'k6';

// Test configuration
export const options = {
  vus: 10,              // 10 virtual users
  duration: '30s',      // Test duration

  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests should be below 500ms
    http_req_failed: ['rate<0.1'],    // Error rate should be below 10%
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Get auth token once at the start
let authToken = null;

export function setup() {
  const loginResponse = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
    username: 'admin',
    password: 'Admin123!'
  }), {
    headers: {
      'Content-Type': 'application/json',
    },
  });
  
  if (loginResponse.status === 200) {
    const tokenData = JSON.parse(loginResponse.body);
    authToken = tokenData.access_token;
  }
  
  return { authToken };
}

// Generate random transaction data
function generateTransactionData() {
  return {
    user_id: '550e8400-e29b-41d4-a716-446655440000',
    amount: Math.random() * 1000 + 10, // Random amount between 10-1010
    currency: 'USD',
    merchant: 'test-merchant-' + Math.floor(Math.random() * 100),
  };
}

export default function (data) {
  // Use token from setup
  const token = data.authToken || authToken;
  
  // Health check
  const healthResponse = http.get(`${BASE_URL}/health`);
  check(healthResponse, {
    'health check is successful': (r) => r.status === 200,
  });

  // Create transaction
  const transactionData = generateTransactionData();
  const createResponse = http.post(
    `${BASE_URL}/api/v1/transactions`,
    JSON.stringify(transactionData),
    {
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
      },
    }
  );

  check(createResponse, {
    'transaction creation is successful': (r) => r.status === 201,
    'response time is acceptable': (r) => r.timings.duration < 1000,
  });

  // Small delay between requests
  sleep(0.1);
}