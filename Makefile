# Fraud Detection Service Makefile

.PHONY: help build test clean docker-build docker-run docker-stop lint format deps dev run ml-train ml-jupyter ml-setup

# Default target
help: ## Show this help message
	@echo "Fraud Detection Service - Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

# Development commands
build: ## Build application
	go build -o bin/fraudsvc ./cmd/fraudsvc

run: ## Run application locally
	go run ./cmd/fraudsvc/main.go

dev: ## Run in development mode with hot reload
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "Air not installed. Install with: go install github.com/cosmtrek/air@latest"; \
		go run ./cmd/fraudsvc/main.go; \
	fi

test: ## Run all tests
	go test ./tests/... -v

test-coverage: ## Run tests with coverage
	go test ./tests/... -coverprofile=coverage.out -v
	go tool cover -html=coverage.out -o coverage.html

lint: ## Run linter
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install from: https://golangci-lint.run/usage/install/"; \
		go vet ./...; \
	fi

format: ## Format code
	gofmt -s -w .
	goimports -w .

deps: ## Download dependencies
	go mod download
	go mod tidy

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean

# Docker commands
docker-build: ## Build Docker image
	docker build -t fraud-detection:latest -f deploy/Dockerfile .

docker-run: ## Run with Docker Compose
	docker-compose -f deploy/docker-compose.yml up -d

docker-stop: ## Stop Docker Compose services
	docker-compose -f deploy/docker-compose.yml down

# ML commands
ml-train: ## Run ML training pipeline
	@cd ml && docker-compose up ml-training --build

ml-jupyter: ## Start ML Jupyter notebook server
	@cd ml && docker-compose --profile jupyter up ml-jupyter --build

ml-setup: ## Setup ML development environment
	@echo "Setting up ML directories..."
	@mkdir -p ml/data/{raw,processed,validation}
	@mkdir -p ml/models
	@echo "ML environment setup complete!"

# Quick development setup
setup: deps ## Setup development environment
	@echo "Installing development tools..."
	@go install github.com/cosmtrek/air@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@echo "Development environment setup complete!"