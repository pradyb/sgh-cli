# sgh-cli Makefile
# Provides common development and build commands

.PHONY: help build test clean install lint format coverage docs

# Default target
help: ## Show this help message
	@echo "🚀 sgh-cli Development Commands"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
build: ## Build the sgh-cli binary
	@echo "🔨 Building sgh-cli..."
	go build -o bin/sgh .

build-linux: ## Build for Linux
	@echo "🔨 Building for Linux..."
	GOOS=linux GOARCH=amd64 go build -o bin/sgh-linux .

build-darwin: ## Build for macOS
	@echo "🔨 Building for macOS..."
	GOOS=darwin GOARCH=amd64 go build -o bin/sgh-darwin .

build-windows: ## Build for Windows
	@echo "🔨 Building for Windows..."
	GOOS=windows GOARCH=amd64 go build -o bin/sgh-windows.exe .

build-all: build-linux build-darwin build-windows ## Build for all platforms

# Development targets
install: ## Install sgh-cli locally
	@echo "📦 Installing sgh-cli..."
	go install .

dev: ## Run in development mode with hot reload
	@echo "🔄 Starting development mode..."
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "⚠️  Air not found. Install with: go install github.com/cosmtrek/air@latest"; \
		go run .; \
	fi

# Testing targets
test: ## Run all tests
	@echo "🧪 Running tests..."
	go test ./... -v

test-short: ## Run tests with short flag
	@echo "🧪 Running short tests..."
	go test ./... -v -short

test-race: ## Run tests with race detection
	@echo "🧪 Running tests with race detection..."
	go test ./... -v -race

coverage: ## Generate test coverage report
	@echo "📊 Generating coverage report..."
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "📄 Coverage report generated: coverage.html"

# Code quality targets
lint: ## Run linter
	@echo "🔍 Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

format: ## Format code
	@echo "🎨 Formatting code..."
	go fmt ./...
	go vet ./...

# Documentation targets
docs: ## Generate documentation
	@echo "📚 Generating documentation..."
	@if command -v godoc > /dev/null; then \
		godoc -http=:6060; \
	else \
		echo "⚠️  godoc not found. Install with: go install golang.org/x/tools/cmd/godoc@latest"; \
	fi

# Cleanup targets
clean: ## Clean build artifacts
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean

# Dependencies
deps: ## Download dependencies
	@echo "📥 Downloading dependencies..."
	go mod download
	go mod tidy

# Security
security: ## Run security checks
	@echo "🔒 Running security checks..."
	@if command -v gosec > /dev/null; then \
		gosec ./...; \
	else \
		echo "⚠️  gosec not found. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi

# Release
release: build-all ## Build release binaries
	@echo "🚀 Creating release..."
	@mkdir -p release
	cp bin/sgh-linux release/sgh-linux-amd64
	cp bin/sgh-darwin release/sgh-darwin-amd64
	cp bin/sgh-windows.exe release/sgh-windows-amd64.exe
	@echo "✅ Release binaries created in release/ directory"

# Docker
docker-build: ## Build Docker image
	@echo "🐳 Building Docker image..."
	docker build -t sgh-cli .

docker-run: ## Run Docker container
	@echo "🐳 Running Docker container..."
	docker run --rm -it sgh-cli

# Setup development environment
setup: ## Setup development environment
	@echo "🔧 Setting up development environment..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/cosmtrek/air@latest
	go install golang.org/x/tools/cmd/godoc@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	@echo "✅ Development environment setup complete" 