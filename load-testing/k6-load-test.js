import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const transactionCreationRate = new Rate('transaction_creation_success');
const apiResponseTime = new Trend('api_response_time');

// Test configuration
export const options = {
  scenarios: {
    // Ramp up test
    ramp_up: {
      executor: 'ramping-vus',
      stages: [
        { duration: '30s', target: 10 },   // Ramp up to 10 users over 30s
        { duration: '1m', target: 50 },    // Ramp up to 50 users over 1m
        { duration: '2m', target: 100 },   // Ramp up to 100 users over 2m
        { duration: '1m', target: 100 },   // Stay at 100 users for 1m
        { duration: '30s', target: 0 },    // Ramp down to 0 users
      ],
      tags: { test_type: 'ramp_up' },
    },

    // Constant load test
    constant_load: {
      executor: 'constant-vus',
      vus: 50,
      duration: '3m',
      tags: { test_type: 'constant_load' },
    },

    // Spike test
    spike_test: {
      executor: 'ramping-vus',
      stages: [
        { duration: '10s', target: 10 },   // Normal load
        { duration: '10s', target: 200 },  // Spike to 200 users
        { duration: '30s', target: 200 },  // Stay at spike
        { duration: '10s', target: 10 },   // Back to normal
        { duration: '10s', target: 0 },    // Ramp down
      ],
      tags: { test_type: 'spike' },
    },

    // Stress test
    stress_test: {
      executor: 'ramping-vus',
      stages: [
        { duration: '1m', target: 100 },
        { duration: '2m', target: 200 },
        { duration: '2m', target: 300 },
        { duration: '2m', target: 400 },
        { duration: '2m', target: 500 },
        { duration: '1m', target: 0 },
      ],
      tags: { test_type: 'stress' },
    },
  },

  thresholds: {
    // HTTP request duration should be < 500ms for 95% of requests
    http_req_duration: ['p(95)<500'],

    // HTTP request duration should be < 1000ms for 99% of requests
    http_req_duration: ['p(99)<1000'],

    // Error rate should be < 1%
    http_req_failed: ['rate<0.01'],

    // Transaction creation success rate should be > 99%
    transaction_creation_success: ['rate>0.99'],
  },
};

// Base URL - can be overridden with K6_BASE_URL environment variable
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Generate random transaction data
function generateTransactionData() {
  const users = [
    '550e8400-e29b-41d4-a716-446655440000',
    '550e8400-e29b-41d4-a716-446655440001',
    '550e8400-e29b-41d4-a716-446655440002',
    '550e8400-e29b-41d4-a716-446655440003',
    '550e8400-e29b-41d4-a716-446655440004',
  ];

  const merchants = [
    'amazon.com',
    'walmart.com',
    'target.com',
    'bestbuy.com',
    'costco.com',
    'suspicious-merchant', // For testing fraud detection
  ];

  const currencies = ['USD', 'EUR', 'GBP'];

  return {
    user_id: users[Math.floor(Math.random() * users.length)],
    amount: Math.random() * 5000 + 10, // Random amount between 10-5010
    currency: currencies[Math.floor(Math.random() * currencies.length)],
    merchant: merchants[Math.floor(Math.random() * merchants.length)],
  };
}

// Main test function
export default function () {
  // Test 1: Health check
  const healthResponse = http.get(`${BASE_URL}/health`);
  check(healthResponse, {
    'health check status is 200': (r) => r.status === 200,
    'health check response time < 100ms': (r) => r.timings.duration < 100,
  });

  // Test 2: Create transaction
  const transactionData = generateTransactionData();
  const createResponse = http.post(
    `${BASE_URL}/api/v1/transactions`,
    JSON.stringify(transactionData),
    {
      headers: {
        'Content-Type': 'application/json',
      },
    }
  );

  const createSuccess = check(createResponse, {
    'create transaction status is 201': (r) => r.status === 201,
    'create transaction response time < 200ms': (r) => r.timings.duration < 200,
    'create transaction returns transaction ID': (r) => r.json().transaction && r.json().transaction.id,
  });

  transactionCreationRate.add(createSuccess);
  apiResponseTime.add(createResponse.timings.duration);

  if (createSuccess && createResponse.status === 201) {
    const transactionId = createResponse.json().transaction.id;

    // Test 3: Get transaction (if creation was successful)
    const getResponse = http.get(`${BASE_URL}/api/v1/transactions/${transactionId}`);
    check(getResponse, {
      'get transaction status is 200': (r) => r.status === 200,
      'get transaction response time < 100ms': (r) => r.timings.duration < 100,
    });

    // Test 4: List transactions
    const listResponse = http.get(`${BASE_URL}/api/v1/transactions?limit=10`);
    check(listResponse, {
      'list transactions status is 200': (r) => r.status === 200,
      'list transactions response time < 150ms': (r) => r.timings.duration < 150,
    });
  }

  // Test 5: Get user transactions
  const userId = transactionData.user_id;
  const userTransactionsResponse = http.get(`${BASE_URL}/api/v1/users/${userId}/transactions`);
  check(userTransactionsResponse, {
    'get user transactions status is 200': (r) => r.status === 200,
    'get user transactions response time < 150ms': (r) => r.timings.duration < 150,
  });

  // Test 6: Metrics endpoint (if enabled)
  const metricsResponse = http.get(`${BASE_URL}/metrics`);
  check(metricsResponse, {
    'metrics endpoint accessible': (r) => r.status === 200 || r.status === 404, // 404 if metrics disabled
    'metrics response time < 100ms': (r) => r.timings.duration < 100,
  });

  // Random sleep between 0.1-1 second to simulate realistic user behavior
  sleep(Math.random() * 0.9 + 0.1);
}

// Setup function - runs before the test starts
export function setup() {
  console.log('Starting load test setup...');

  // Verify the service is healthy before starting the test
  const healthCheck = http.get(`${BASE_URL}/health`);
  if (healthCheck.status !== 200) {
    console.error(`Service health check failed: ${healthCheck.status}`);
    return;
  }

  console.log('Service is healthy, proceeding with load test...');
}

// Teardown function - runs after the test completes
export function teardown(data) {
  console.log('Load test completed');
}

// Handle summary - custom summary output
export function handleSummary(data) {
  const summary = {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    'load-test-summary.json': JSON.stringify(data, null, 2),
    'load-test-report.html': htmlReport(data),
  };

  // Custom metrics summary
  const customMetrics = {
    total_requests: data.metrics.http_reqs.values.count,
    avg_response_time: data.metrics.http_req_duration.values.avg,
    p95_response_time: data.metrics.http_req_duration.values['p(95)'],
    p99_response_time: data.metrics.http_req_duration.values['p(99)'],
    error_rate: data.metrics.http_req_failed.values.rate,
    transaction_creation_rate: data.metrics.transaction_creation_success?.values.rate || 0,
  };

  summary['custom-metrics.json'] = JSON.stringify(customMetrics, null, 2);

  return summary;
}

function textSummary(data, options) {
  return `
📊 Load Test Summary
==================

Test Duration: ${data.metrics.iteration_duration.values.avg}ms avg iteration
Total Requests: ${data.metrics.http_reqs.values.count}
Failed Requests: ${data.metrics.http_req_failed.values.rate * 100}%

Response Times:
  Average: ${Math.round(data.metrics.http_req_duration.values.avg)}ms
  95th percentile: ${Math.round(data.metrics.http_req_duration.values['p(95)'])}ms
  99th percentile: ${Math.round(data.metrics.http_req_duration.values['p(99)'])}ms

Transaction Creation Success Rate: ${(data.metrics.transaction_creation_success?.values.rate * 100 || 0).toFixed(2)}%

Virtual Users: ${data.metrics.vus.values.max} max concurrent users

Threshold Results:
${Object.entries(data.metrics)
  .filter(([key, metric]) => metric.thresholds)
  .map(([key, metric]) => {
    const thresholds = metric.thresholds;
    return Object.entries(thresholds).map(([threshold, result]) => {
      const status = result.ok ? '✅' : '❌';
      return `${status} ${key}: ${threshold}`;
    }).join('\n');
  })
  .flat()
  .join('\n')}
`;
}

function htmlReport(data) {
  return `
<!DOCTYPE html>
<html>
<head>
    <title>Fraud Detection Load Test Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .metric { background: #f5f5f5; padding: 10px; margin: 10px 0; border-radius: 5px; }
        .success { color: green; }
        .failure { color: red; }
        h1, h2 { color: #333; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <h1>🚀 Fraud Detection Load Test Report</h1>

    <div class="metric">
        <h2>📈 Test Overview</h2>
        <p><strong>Total Requests:</strong> ${data.metrics.http_reqs.values.count}</p>
        <p><strong>Duration:</strong> ${Math.round(data.metrics.iteration_duration.values.avg)}ms avg iteration</p>
        <p><strong>Error Rate:</strong> ${(data.metrics.http_req_failed.values.rate * 100).toFixed(2)}%</p>
        <p><strong>Max Virtual Users:</strong> ${data.metrics.vus.values.max}</p>
    </div>

    <div class="metric">
        <h2>⚡ Response Times</h2>
        <table>
            <tr><th>Metric</th><th>Value</th></tr>
            <tr><td>Average</td><td>${Math.round(data.metrics.http_req_duration.values.avg)}ms</td></tr>
            <tr><td>95th Percentile</td><td>${Math.round(data.metrics.http_req_duration.values['p(95)'])}ms</td></tr>
            <tr><td>99th Percentile</td><td>${Math.round(data.metrics.http_req_duration.values['p(99)'])}ms</td></tr>
            <tr><td>Min</td><td>${Math.round(data.metrics.http_req_duration.values.min)}ms</td></tr>
            <tr><td>Max</td><td>${Math.round(data.metrics.http_req_duration.values.max)}ms</td></tr>
        </table>
    </div>

    <div class="metric">
        <h2>🎯 Custom Metrics</h2>
        <p><strong>Transaction Creation Success Rate:</strong> ${(data.metrics.transaction_creation_success?.values.rate * 100 || 0).toFixed(2)}%</p>
        <p><strong>API Response Time (avg):</strong> ${Math.round(data.metrics.api_response_time?.values.avg || 0)}ms</p>
    </div>

    <div class="metric">
        <h2>📊 Threshold Results</h2>
        ${Object.entries(data.metrics)
          .filter(([key, metric]) => metric.thresholds)
          .map(([key, metric]) => {
            const thresholds = metric.thresholds;
            return Object.entries(thresholds).map(([threshold, result]) => {
              const statusClass = result.ok ? 'success' : 'failure';
              const statusIcon = result.ok ? '✅' : '❌';
              return `<p class="${statusClass}">${statusIcon} ${key}: ${threshold}</p>`;
            }).join('');
          })
          .flat()
          .join('')}
    </div>
</body>
</html>
`;
}