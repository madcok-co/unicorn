.PHONY: help setup up down logs clean test run build

# Colors
CYAN := \033[0;36m
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

help: ## Show this help message
	@echo "$(CYAN)Unicorn Complete Example - Available Commands:$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}'
	@echo ""

setup: ## Setup environment (copy .env.example to .env)
	@echo "$(CYAN)Setting up environment...$(NC)"
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "$(GREEN)✓ Created .env file from .env.example$(NC)"; \
	else \
		echo "$(YELLOW)⚠ .env file already exists$(NC)"; \
	fi

up: ## Start all services with docker-compose
	@echo "$(CYAN)Starting all services...$(NC)"
	@docker-compose up -d
	@echo "$(GREEN)✓ All services started!$(NC)"
	@echo ""
	@echo "$(CYAN)Services:$(NC)"
	@echo "  PostgreSQL:    localhost:5432"
	@echo "  Redis:         localhost:6379"
	@echo "  Kafka:         localhost:9092"
	@echo "  Kafka UI:      http://localhost:8090"
	@echo "  Prometheus:    http://localhost:9090"
	@echo "  Grafana:       http://localhost:3000 (admin/admin)"
	@echo "  Jaeger UI:     http://localhost:16686"
	@echo "  Adminer (DB):  http://localhost:8081"
	@echo ""

down: ## Stop all services
	@echo "$(CYAN)Stopping all services...$(NC)"
	@docker-compose down
	@echo "$(GREEN)✓ All services stopped$(NC)"

logs: ## Show logs from all services
	@docker-compose logs -f

logs-app: ## Show logs from specific service (usage: make logs-app SERVICE=postgres)
	@docker-compose logs -f $(SERVICE)

clean: ## Stop and remove all containers, volumes
	@echo "$(RED)⚠ This will remove all data!$(NC)"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		docker-compose down -v; \
		echo "$(GREEN)✓ Cleaned up$(NC)"; \
	fi

ps: ## Show running containers
	@docker-compose ps

restart: ## Restart all services
	@echo "$(CYAN)Restarting all services...$(NC)"
	@docker-compose restart
	@echo "$(GREEN)✓ Services restarted$(NC)"

build: ## Build the Go application
	@echo "$(CYAN)Building application...$(NC)"
	@go build -o bin/unicorn-example main.go
	@echo "$(GREEN)✓ Build complete: bin/unicorn-example$(NC)"

run: ## Run the application
	@echo "$(CYAN)Running application...$(NC)"
	@go run main.go

test: ## Run tests
	@echo "$(CYAN)Running tests...$(NC)"
	@go test -v ./...

deps: ## Install Go dependencies
	@echo "$(CYAN)Installing dependencies...$(NC)"
	@go mod download
	@go mod tidy
	@echo "$(GREEN)✓ Dependencies installed$(NC)"

# Database helpers
db-migrate: ## Run database migrations (placeholder)
	@echo "$(YELLOW)Database migrations not implemented yet$(NC)"

db-seed: ## Seed database with sample data (placeholder)
	@echo "$(YELLOW)Database seeding not implemented yet$(NC)"

# Testing helpers
curl-health: ## Test health endpoint
	@curl -s http://localhost:8080/health | jq .

curl-products: ## Test list products endpoint
	@curl -s http://localhost:8080/products | jq .

curl-create: ## Test create product endpoint
	@curl -X POST http://localhost:8080/products \
		-H "Content-Type: application/json" \
		-d '{"name":"Test Product","description":"Test Description","price":99.99,"stock":10}' | jq .

# Docker helpers
docker-build: ## Build Docker image for the app
	@echo "$(CYAN)Building Docker image...$(NC)"
	@docker build -t unicorn-example:latest .
	@echo "$(GREEN)✓ Docker image built$(NC)"

docker-run: ## Run app in Docker
	@echo "$(CYAN)Running app in Docker...$(NC)"
	@docker run -p 8080:8080 --env-file .env unicorn-example:latest

# Development helpers
watch: ## Watch and auto-reload (requires air: go install github.com/cosmtrek/air@latest)
	@air

format: ## Format Go code
	@echo "$(CYAN)Formatting code...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)✓ Code formatted$(NC)"

lint: ## Run linter (requires golangci-lint)
	@echo "$(CYAN)Running linter...$(NC)"
	@golangci-lint run
	@echo "$(GREEN)✓ Linting complete$(NC)"

all: setup up build run ## Setup, start services, build and run

.DEFAULT_GOAL := help
