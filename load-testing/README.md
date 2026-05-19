# Load Testing

This directory contains load testing scripts and configurations for the Fraud Detection Service.

## Prerequisites

### Install k6

```bash
# macOS
brew install k6

# Ubuntu/Debian
sudo apt update
sudo apt install k6

# Or download from: https://k6.io/docs/get-started/installation/
```

### Start the Application

Make sure the fraud detection service is running:

```bash
# Using Docker Compose (recommended for testing)
make docker-run

# Or using Kubernetes
make k8s-deploy
make k8s-port-forward

# Or locally
make run
```

## Running Load Tests

### Simple Load Test

Run a basic load test with 10 virtual users for 30 seconds:

```bash
# Local service
k6 run load-testing/simple-load-test.js

# Docker service
k6 run -e BASE_URL=http://localhost:8080 load-testing/simple-load-test.js

# Kubernetes service
k6 run -e BASE_URL=http://localhost:8080 load-testing/simple-load-test.js
```

### Comprehensive Load Test

Run the full load testing suite with multiple scenarios:

```bash
# Ramp up test (gradual increase in load)
k6 run --tag test_type=ramp_up load-testing/k6-load-test.js

# Constant load test (steady load)
k6 run --tag test_type=constant_load load-testing/k6-load-test.js

# Spike test (sudden load increase)
k6 run --tag test_type=spike load-testing/k6-load-test.js

# Stress test (breaking point testing)
k6 run --tag test_type=stress load-testing/k6-load-test.js

# Run all scenarios
k6 run load-testing/k6-load-test.js
```

### Custom Configuration

Override default settings using environment variables:

```bash
# Set base URL
export K6_BASE_URL=http://localhost:8080

# Run with custom virtual users
k6 run -e VUS=50 -e DURATION=2m load-testing/simple-load-test.js

# Run with custom thresholds
k6 run --threshold 'http_req_duration{expected_response:"true"}:p(95)<200' load-testing/simple-load-test.js
```

## Test Scenarios

### 1. Ramp Up Test
- Gradually increases load from 0 to 100 users
- Tests how the system handles increasing load
- Identifies performance degradation points

### 2. Constant Load Test
- Maintains steady load of 50 users for 3 minutes
- Tests system stability under sustained load
- Monitors resource usage and response times

### 3. Spike Test
- Sudden increase from 10 to 200 users
- Tests system resilience to traffic spikes
- Evaluates auto-scaling capabilities

### 4. Stress Test
- Continuously increases load until system failure
- Determines maximum capacity and breaking points
- Identifies system limits and bottlenecks

## Performance Metrics

The load tests measure:

- **Response Times**: p95, p99, average response times
- **Error Rates**: HTTP error percentages
- **Throughput**: Requests per second
- **Custom Metrics**:
  - Transaction creation success rate
  - API response time trends

## Thresholds

Current performance thresholds:

```javascript
thresholds: {
  http_req_duration: ['p(95)<500'],     // 95% of requests < 500ms
  http_req_duration: ['p(99)<1000'],    // 99% of requests < 1000ms
  http_req_failed: ['rate<0.01'],       // Error rate < 1%
  transaction_creation_success: ['rate>0.99'], // Success rate > 99%
}
```

## Output and Reports

### Console Output
k6 provides real-time metrics in the console during test execution.

### Summary Reports
After test completion, the comprehensive test generates:

- **JSON Summary**: `load-test-summary.json` - Detailed metrics data
- **HTML Report**: `load-test-report.html` - Visual performance report
- **Custom Metrics**: `custom-metrics.json` - Application-specific metrics

### Viewing HTML Report

```bash
# Open the HTML report in browser
open load-test-report.html
```

## Interpreting Results

### Key Metrics to Monitor

1. **Response Time Percentiles**
   - p95 < 500ms: Good performance
   - p95 > 1000ms: Performance issues

2. **Error Rate**
   - < 1%: Acceptable
   - > 5%: Investigate issues

3. **Throughput**
   - Should scale linearly with user load
   - Identify saturation points

4. **Resource Usage**
   - Monitor CPU, memory, and network usage
   - Check for memory leaks or resource exhaustion

### Common Issues

1. **High Response Times**
   - Database connection pooling issues
   - Inefficient queries
   - Resource constraints

2. **High Error Rates**
   - Service unavailability
   - Database connection issues
   - Resource exhaustion

3. **Memory Leaks**
   - Goroutine leaks
   - Improper resource cleanup

## Integration with CI/CD

Add load testing to your CI/CD pipeline:

```yaml
# .github/workflows/ci-cd.yml
- name: Load Testing
  run: |
    k6 run --out json=results.json load-testing/simple-load-test.js
    # Check results and fail if thresholds not met
```

## Performance Optimization

Based on load test results, consider these optimizations:

1. **Database**
   - Connection pooling
   - Query optimization
   - Indexing

2. **Application**
   - Caching layers
   - Async processing
   - Horizontal scaling

3. **Infrastructure**
   - Load balancing
   - Auto-scaling
   - Resource limits

## Troubleshooting

### k6 Issues

```bash
# Check k6 version
k6 version

# Run with verbose logging
k6 run --verbose load-testing/simple-load-test.js

# Check system resources
k6 run --system-tags load-testing/simple-load-test.js
```

### Service Issues

```bash
# Check service health
curl http://localhost:8080/health

# Check service logs
make docker-logs

# Check resource usage
docker stats
```

### Network Issues

```bash
# Test connectivity
curl -v http://localhost:8080/health

# Check network latency
ping localhost
```

## Best Practices

1. **Test Environment**: Use staging environment that mirrors production
2. **Gradual Load**: Start with low load and gradually increase
3. **Monitor Resources**: Track CPU, memory, disk, and network usage
4. **Baseline Tests**: Establish performance baselines for regression testing
5. **Realistic Data**: Use production-like data and scenarios
6. **Regular Testing**: Include load testing in CI/CD pipeline

## Performance Benchmarks

Run Go benchmarks to measure code performance:

```bash
# Run all benchmarks
go test -bench=. ./...

# Run specific benchmark
go test -bench=BenchmarkTransactionProcessing ./tests/

# Run with memory allocation tracking
go test -bench=. -benchmem ./...

# Generate CPU profile
go test -bench=. -cpuprofile=cpu.prof ./...
go tool pprof cpu.prof
```