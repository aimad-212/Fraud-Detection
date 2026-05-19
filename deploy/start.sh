#!/bin/bash

# Fraud Detection Application Startup Script
# This script starts all services using Docker Compose

set -e

echo "🚀 Starting Fraud Detection Application..."

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker first."
    exit 1
fi

# Check if docker-compose is available
if ! command -v docker-compose > /dev/null 2>&1 && ! docker compose version > /dev/null 2>&1; then
    echo "❌ docker-compose is not installed. Please install docker-compose first."
    exit 1
fi

# Navigate to the deploy directory
cd "$(dirname "$0")"

# Check if .env file exists, if not copy from .env.example
if [ ! -f .env ]; then
    echo "📝 Creating .env file from .env.example..."
    cp .env.example .env
    echo "✅ .env file created. Please review and modify if needed."
fi

# Build and start services
echo "🔨 Building and starting services..."
if command -v docker-compose > /dev/null 2>&1; then
    docker-compose up --build -d
else
    docker compose up --build -d
fi

# Wait for services to be healthy
echo "⏳ Waiting for services to be healthy..."
sleep 10

# Check service health
echo "🔍 Checking service health..."
for service in postgres nats pgadmin; do
    if command -v docker-compose > /dev/null 2>&1; then
        service_status=$(docker-compose ps $service)
    else
        service_status=$(docker compose ps $service)
    fi
    
    if echo "$service_status" | grep -q "Up (healthy)"; then
        echo "✅ $service is healthy"
    else
        echo "❌ $service is not healthy"
        echo "📋 Last 10 lines of $service logs:"
        if command -v docker-compose > /dev/null 2>&1; then
            docker-compose logs --tail=10 $service
        else
            docker compose logs --tail=10 $service
        fi
        echo ""
    fi
done

# Show service URLs
echo ""
echo "🌐 Service URLs:"
echo "   • Fraud Detection API: http://localhost:8080"
echo "   • pgAdmin: http://localhost:8081"
echo "   • NATS Monitoring: http://localhost:8222"
echo ""
echo "📊 pgAdmin Login:"
echo "   • Email: admin@fraud-detection.com"
echo "   • Password: admin123"
echo ""
echo "🔧 Useful Commands:"
if command -v docker-compose > /dev/null 2>&1; then
    echo "   • View logs: docker-compose logs -f [service-name]"
    echo "   • Stop services: docker-compose down"
    echo "   • Restart services: docker-compose restart"
else
    echo "   • View logs: docker compose logs -f [service-name]"
    echo "   • Stop services: docker compose down"
    echo "   • Restart services: docker compose restart"
fi
echo ""

echo "🎉 Fraud Detection Application is ready!"