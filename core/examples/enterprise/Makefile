.PHONY: help run build test clean docker-build docker-up docker-down install deps

# Default target
help:
	@echo "Unicorn Enterprise SaaS Platform - Available Commands:"
	@echo ""
	@echo "  make run           - Run the application locally"
	@echo "  make build         - Build the application binary"
	@echo "  make test          - Run tests"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make install       - Install dependencies"
	@echo "  make deps          - Download Go dependencies"
	@echo ""
	@echo "  make docker-build  - Build Docker image"
	@echo "  make docker-up     - Start Docker containers"
	@echo "  make docker-down   - Stop Docker containers"
	@echo "  make docker-logs   - View Docker logs"
	@echo ""
	@echo "  make setup-oauth   - Instructions for OAuth setup"
	@echo "  make test-curl     - Test API with curl commands"
	@echo ""

# Run application locally
run:
	@echo "Starting Enterprise SaaS Platform..."
	go run main.go

# Build binary
build:
	@echo "Building application..."
	go build -o bin/enterprise-saas main.go
	@echo "Binary created at: bin/enterprise-saas"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	go clean

# Install dependencies
install: deps
	@echo "Installing development tools..."
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod verify
	go mod tidy

# Docker commands
docker-build:
	@echo "Building Docker image..."
	docker-compose build

docker-up:
	@echo "Starting Docker containers..."
	docker-compose up -d
	@echo ""
	@echo "Application is running at:"
	@echo "  http://localhost:8080"
	@echo "  http://acme.myapp.com (add to /etc/hosts)"
	@echo "  http://techcorp.myapp.com (add to /etc/hosts)"

docker-down:
	@echo "Stopping Docker containers..."
	docker-compose down

docker-logs:
	docker-compose logs -f app

docker-restart: docker-down docker-up

# OAuth setup instructions
setup-oauth:
	@echo "=========================================="
	@echo "Google OAuth Setup Instructions"
	@echo "=========================================="
	@echo ""
	@echo "1. Go to: https://console.cloud.google.com/apis/credentials"
	@echo "2. Create a new OAuth 2.0 Client ID"
	@echo "3. Application type: Web application"
	@echo "4. Authorized redirect URIs:"
	@echo "   - http://localhost:8080/api/v1/auth/callback"
	@echo "   - http://acme.myapp.com/api/v1/auth/callback"
	@echo "5. Copy Client ID and Client Secret"
	@echo "6. Update .env file or export environment variables:"
	@echo "   export GOOGLE_CLIENT_ID='your-client-id'"
	@echo "   export GOOGLE_CLIENT_SECRET='your-client-secret'"
	@echo ""

# Test with curl
test-curl:
	@echo "=========================================="
	@echo "Testing Enterprise Features with curl"
	@echo "=========================================="
	@echo ""
	@echo "1. List Projects (Acme tenant):"
	@echo '   curl -H "Host: acme.myapp.com" http://localhost:8080/api/v1/projects'
	@echo ""
	@echo "2. List Projects (TechCorp tenant):"
	@echo '   curl -H "Host: techcorp.myapp.com" http://localhost:8080/api/v1/projects'
	@echo ""
	@echo "3. Get specific project:"
	@echo '   curl -H "Host: acme.myapp.com" http://localhost:8080/api/v1/projects/proj-1'
	@echo ""
	@echo "4. Create project:"
	@echo '   curl -X POST -H "Host: acme.myapp.com" -H "Content-Type: application/json" \'
	@echo '     -d '"'"'{"name":"Test Project","description":"Testing"}'"'"' \'
	@echo '     http://localhost:8080/api/v1/projects'
	@echo ""
	@echo "5. List with pagination (V1 - offset):"
	@echo '   curl "http://localhost:8080/api/v1/projects?page=1&limit=10&sort=created_at&order=desc"'
	@echo ""
	@echo "6. List with cursor pagination (V2):"
	@echo '   curl "http://localhost:8080/api/v2/projects?limit=10"'
	@echo ""
	@echo "7. Batch update (V2):"
	@echo '   curl -X POST -H "Content-Type: application/json" \'
	@echo '     -d '"'"'{"project_ids":["proj-1","proj-2"],"updates":{"status":"archived"}}'"'"' \'
	@echo '     http://localhost:8080/api/v2/projects/batch'
	@echo ""

# Setup hosts file
setup-hosts:
	@echo "Add these lines to /etc/hosts (Linux/Mac) or C:\Windows\System32\drivers\etc\hosts (Windows):"
	@echo ""
	@echo "127.0.0.1  acme.myapp.com"
	@echo "127.0.0.1  techcorp.myapp.com"
	@echo ""
	@echo "On Linux/Mac, run:"
	@echo "  sudo sh -c 'echo \"127.0.0.1  acme.myapp.com\" >> /etc/hosts'"
	@echo "  sudo sh -c 'echo \"127.0.0.1  techcorp.myapp.com\" >> /etc/hosts'"

# Development mode with hot reload
dev:
	@echo "Starting development server with hot reload..."
	air

# Lint code
lint:
	@echo "Running linter..."
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Security scan
security:
	@echo "Running security scan..."
	gosec ./...

# All-in-one setup
setup: install setup-hosts
	@echo ""
	@echo "=========================================="
	@echo "Setup Complete!"
	@echo "=========================================="
	@echo ""
	@echo "Next steps:"
	@echo "1. Copy .env.example to .env"
	@echo "2. Run 'make setup-oauth' for OAuth setup instructions"
	@echo "3. Run 'make run' to start the application"
	@echo "4. Run 'make test-curl' for testing examples"
	@echo ""
