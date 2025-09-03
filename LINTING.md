# Code Quality and Linting Setup

This repository is configured with automated code quality checks using `golangci-lint`.

## Configuration Files

- `.golangci.yml` - Full linting configuration with all checks enabled
- `.golangci-precommit.yml` - Lightweight configuration for pre-commit hooks (critical issues only)

## Pre-commit Hook

A pre-commit hook is automatically installed that will:

1. ✅ Run critical linting checks (`golangci-lint`)
2. ✅ Run all tests (`go test ./...`)
3. ✅ Ensure `go.mod` and `go.sum` are tidy

### Running Linting Manually

```bash
# Full analysis (all linters)
golangci-lint run

# Fast analysis (critical issues only)
golangci-lint run --config .golangci-precommit.yml

# Quick formatting fixes
gofmt -w .
goimports -w .
```

### Bypassing Pre-commit (Not Recommended)

```bash
# Only use in emergency situations
git commit --no-verify
```

### Linting Rules

The full configuration includes:
- Code formatting (`gofmt`, `goimports`)
- Error checking (`errcheck`, `govet`)
- Security checks (`gosec`)
- Performance hints (`gocritic`)
- Style consistency (`revive`, `stylecheck`)
- Complexity analysis (`gocyclo`, `funlen`)

The pre-commit configuration focuses on critical issues that could break functionality.

## CI Integration

The full linting suite runs in CI to catch all style and quality issues, while the pre-commit hook prevents only critical problems from being committed.
