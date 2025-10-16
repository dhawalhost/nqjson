# Makefile for nqjson library

.PHONY: all test test-unit test-bench test-race lint security clean help install-tools fmt vet bench bench-get bench-set bench-delete bench-multipath bench-modifiers bench-save

# Default target
all: test lint security

# Help target
help:
	@echo "Available targets:"
	@echo "  test               - Run all tests"
	@echo "  test-unit          - Run unit tests only"
	@echo "  test-race          - Run tests with race detector"
	@echo "  test-coverage      - Run tests with coverage report"
	@echo "  lint               - Run linting checks"
	@echo "  security           - Run security checks"
	@echo "  fmt                - Format code"
	@echo "  vet                - Run go vet"
	@echo "  clean              - Clean build artifacts"
	@echo "  install-tools      - Install development tools"
	@echo "  bench              - Run full benchmark suite"
	@echo "  bench-get          - Run GET benchmarks only"
	@echo "  bench-set          - Run SET benchmarks only"
	@echo "  bench-delete       - Run DELETE benchmarks only"
	@echo "  bench-multipath    - Run multipath benchmarks (nqjson-exclusive)"
	@echo "  bench-modifiers    - Run extended modifier benchmarks"
	@echo "  bench-save         - Run benchmarks and save results"
	@echo "  ci                 - Run all CI checks locally"

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

# Benchmark targets (separate module in benchmark/)
bench:
	@echo "Running full benchmark suite..."
	@echo "Note: Benchmarks have their own go.mod with gjson/sjson dependencies"
	@mkdir -p bench
	cd benchmark && go test -run=^$$ -bench=. -benchmem -benchtime=2s | tee ../bench/latest.txt
	@echo ""
	@echo "✅ Benchmark results saved to bench/latest.txt"
	@echo "📊 See BENCHMARKS.md for detailed analysis"

bench-get:
	@echo "Running GET benchmarks..."
	cd benchmark && go test -run=^$$ -bench=BenchmarkGet -benchmem

bench-set:
	@echo "Running SET benchmarks..."
	cd benchmark && go test -run=^$$ -bench=BenchmarkSet -benchmem

bench-delete:
	@echo "Running DELETE benchmarks..."
	cd benchmark && go test -run=^$$ -bench=BenchmarkDelete -benchmem

bench-multipath:
	@echo "Running multipath benchmarks (nqjson-exclusive feature)..."
	cd benchmark && go test -run=^$$ -bench=MultiPath -benchmem

bench-modifiers:
	@echo "Running extended modifier benchmarks..."
	cd benchmark && go test -run=^$$ -bench=Modifier -benchmem

bench-save:
	@echo "Running benchmarks and saving to benchmark_results.txt..."
	cd benchmark && go test -run=^$$ -bench=. -benchmem | tee ../benchmark_results.txt
	@echo "Results saved to benchmark_results.txt"

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
