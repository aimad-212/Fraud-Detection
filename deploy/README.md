# Fraud Detection Service - Local Development

## Quick Start

### Option 1: Automated Startup (Recommended)
```bash
# Navigate to deploy directory
cd deploy

# Run the startup script
./start.sh
```

### Option 2: Manual Docker Compose
```bash
# Navigate to deploy directory
cd deploy

# Copy environment file
cp .env.example .env

# Start services
docker-compose up --build -d
```

## Services

- **Fraud Detection API**: http://localhost:8080
- **Health Check**: http://localhost:8080/health
- **API Metrics**: http://localhost:8080/metrics
- **pgAdmin**: http://localhost:8081
  - Email: admin@fraud-detection.com
  - Password: admin123
- **NATS Monitoring**: http://localhost:8222

## Environment Configuration

Copy `.env.example` to `.env` and modify as needed:

```bash
cp .env.example .env
```

Key environment variables:
- `PORT`: Application port (default: 8080)
- `DB_HOST`: PostgreSQL host (default: postgres)
- `DB_NAME`: Database name (default: fraud_detection)
- `NATS_URL`: NATS connection URL (default: nats://nats:4222)

## Database Setup

The application uses PostgreSQL with automatic schema migration. The database will be initialized automatically when you start the services.

## Useful Commands

```bash
# View all services
docker-compose ps

# View logs
docker-compose logs -f [service-name]

# Stop all services
docker-compose down

# Restart services
docker-compose restart

# Access PostgreSQL
docker-compose exec postgres psql -U fraud_user -d fraud_detection

# Access application container
docker-compose exec fraud-detection sh
```

## Service Dependencies

The application depends on:
- **PostgreSQL**: Primary database
- **NATS**: Message broker for async processing
- **pgAdmin**: Database management interface

All services include health checks and will wait for dependencies to be ready before starting.

## Development Workflow

1. Make changes to the code
2. Rebuild and restart: `docker-compose up --build -d`
3. View logs: `docker-compose logs -f fraud-detection`
4. Test API: http://localhost:8080/health

## Monitoring

- **Health Checks**: `/health`, `/health/ready`, `/health/detailed`
- **Metrics**: `/metrics` (Prometheus format)
- **Application Logs**: `docker-compose logs -f fraud-detection`