# Fraud Detection System

Real-time fraud detection system built with Go, combining rule-based engines and machine learning for transaction analysis.

## Quick Start

```bash
# Build the application
make build

# Run tests
make test

# Start the service
make run

# Or run with Docker Compose (includes PostgreSQL, NATS)
make docker-run
```

## Key Features

- Real-time transaction processing with <1ms latency
- Hybrid detection: rule-based + ML scoring (60/40 weights)
- Production ML: XGBoost model with Python subprocess integration
- Enterprise security: JWT authentication, CORS, rate limiting, HTTPS via reverse proxy
- Event-driven architecture with NATS messaging
- 50+ Prometheus metrics for monitoring
- 95%+ test coverage
- Model management API: upload, activate, deactivate, version models
- IP address capture and geolocation analysis
- Docker deployment with PostgreSQL, pgAdmin, and NATS
- Load testing with K6 (2923+ transactions/sec)

## Architecture

```
HTTP API → Decision Service → ML Models
    ↓              ↓              ↓
Repository → Rules Engine → Feature Engineering
    ↓              ↓              ↓
PostgreSQL ← NATS Queue ← Prometheus
```

## Tech Stack

- **Language**: Go 1.25+
- **Web**: Gin framework with JWT auth
- **Database**: PostgreSQL + Redis
- **ML**: XGBoost, ONNX Runtime
- **Messaging**: NATS
- **Monitoring**: Prometheus
- **Testing**: Go test suite + K6 load testing

## Testing

```bash
# Unit tests
go test ./... -v

# Load testing
k6 run load-testing/simple-load-test.js

# Coverage
go test -cover ./...
```

## Deployment

```bash
# Docker (recommended - includes PostgreSQL, pgAdmin, NATS)
cd deploy && docker-compose up -d

# Build binary
go build -o fraudsvc ./cmd/fraudsvc

# Production
./fraudsvc --config configs/config.yaml
```

## API Endpoints

### Authentication
```bash
# Login
POST /api/v1/auth/login
{
  "username": "admin",
  "password": "Admin123!"
}
```

### Transactions
```bash
# Create transaction
POST /api/v1/transactions
Authorization: Bearer <token>

# Get transaction history
GET /api/v1/transactions?limit=10&offset=0
Authorization: Bearer <token>
```

### Model Management
```bash
# List models
GET /api/v1/models
Authorization: Bearer <token>

# Upload model
POST /api/v1/models/upload
Authorization: Bearer <token>

# Activate model
POST /api/v1/models/{model-id}/activate
Authorization: Bearer <token>
```

## Project Structure

```
cmd/           # Application entrypoints
internal/       # Private application code
  domain/       # Business models
  usecase/      # Business logic
  repository/   # Data access
  infrastructure/ # External integrations
configs/        # Configuration files
tests/          # Test files
load-testing/   # Performance tests
ml/             # ML models and training
  models/       # ML model files
  training/     # Training scripts
  experiments/  # Jupyter notebooks
deploy/         # Docker deployment files
```

## Health & Metrics

```bash
# Health check
curl http://localhost:8080/health

# Metrics
curl http://localhost:8080/metrics
```

## Configuration

Key environment variables:
- `ML_MODEL_TYPE`: xgboost, onnx, or mock
- `ML_MODEL_PATH`: Path to model file (default: ./ml/models/fraud_model.onnx)
- `RULE_WEIGHT`: Rule engine weight (default: 0.6)
- `ML_WEIGHT`: ML model weight (default: 0.4)
- `DB_HOST`: Database host (default: localhost)
- `DB_PORT`: Database port (default: 5432)
- `NATS_URL`: NATS server URL (default: nats://localhost:4222)

### Docker Environment
See `deploy/.env` for complete Docker configuration.
