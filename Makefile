# Makefile for njson library

.PHONY: all test test-unit test-bench test-race lint security clean help install-tools fmt vet

# Default target
all: test lint security

# Help target
help:
	@echo "Available targets:"
	@echo "  test          - Run all tests"
	@echo "  test-unit     - Run unit tests only"
	@echo "  test-race     - Run tests with race detector"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  lint          - Run linting checks"
	@echo "  security      - Run security checks"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet"
	@echo "  clean         - Clean build artifacts"
	@echo "  install-tools - Install development tools"
	@echo "  ci            - Run all CI checks locally"

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/gordonklaus/ineffassign@latest
	@echo "Tools installed successfully!"

# Test targets
test: test-unit

test-unit:
	@echo "Running unit tests..."
	go test -v -timeout 30s ./...

test-race:
	@echo "Running tests with race detector..."
	go test -v -race -timeout 60s ./...

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Code quality targets
fmt:
	@echo "Formatting code..."
	go fmt ./...

vet:
	@echo "Running go vet..."
	go vet ./...

lint: fmt vet
	@echo "Running golangci-lint..."
	golangci-lint run --timeout=5m
	@echo "Running staticcheck..."
	staticcheck ./...
	@echo "Running ineffassign..."
	ineffassign ./...

# Security targets
security:
	@echo "Running security checks..."
	@echo "Checking for vulnerabilities..."
	govulncheck ./...
	@echo "Running gosec..."
	gosec -fmt text ./... || true
	@echo "Security scan complete. Review output above for details."

# Build verification
build:
	@echo "Building library..."
	go build -v ./...

# Dependency management
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod verify

tidy:
	@echo "Tidying go.mod..."
	go mod tidy

# Clean up
clean:
	@echo "Cleaning up..."
	rm -f coverage.out coverage.html
	go clean -testcache

# Run all CI checks locally
ci: deps tidy build test-race lint security
	@echo "All CI checks completed successfully! ✅"

# Development workflow
dev: fmt vet test-unit
	@echo "Development checks completed!"

# Generate documentation
docs:
	@echo "Generating documentation..."
	go doc -all . > docs.txt
	@echo "Documentation generated: docs.txt"

# Check for potential issues
check: vet lint
	@echo "Running additional checks..."
	go list -m all
	go version
	@echo "System check complete!"

# Release preparation
release-check: ci
	@echo "Release checks completed. Ready for release! 🚀"
