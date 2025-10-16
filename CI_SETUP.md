# CI/CD Setup for nqjson

This document describes the comprehensive CI/CD pipeline setup for the nqjson library.

## 🔧 CI/CD Pipeline Overview

The CI/CD pipeline includes:

### 🛡️ Security Checks

- **Vulnerability Scanning**: `govulncheck` for Go vulnerabilities
- **Security Analysis**: `gosec` for security issues
- **SARIF Upload**: Security findings uploaded to GitHub Security tab

### 🔍 Code Quality & Linting

- **golangci-lint**: Comprehensive linting with 30+ linters
- **staticcheck**: Advanced static analysis
- **go vet**: Standard Go analysis
- **go fmt**: Code formatting verification
- **ineffassign**: Dead code detection

### 🧪 Testing

- **Unit Tests**: Comprehensive test suite across multiple Go versions
- **Race Detection**: Concurrent access testing
- **Coverage**: Code coverage reporting with Codecov integration
- **Cross-Platform**: Testing on Linux, Windows, and macOS
- **Multi-Version**: Go 1.21, 1.22, and 1.23 support

### 📊 Performance

- **Benchmark Tests**: Performance regression detection
- **Memory Profiling**: Memory usage analysis

### 🏗️ Build Verification

- **Multi-Platform Builds**: Linux, Windows, macOS
- **Multi-Architecture**: amd64, arm64
- **Integration Tests**: End-to-end functionality verification

## 🚀 Local Development

### Prerequisites

```bash
# Install required tools
make install-tools
```

### Local Development Workflow

```bash
# Quick development checks
make dev

# Full CI simulation
make ci

# Security checks only
make security

# Performance testing
make perf
```

### Available Make Targets

- `make test` - Run all tests
- `make test-race` - Run tests with race detector
- `make test-coverage` - Generate coverage report
- `make lint` - Run all linting checks
- `make security` - Run security scans
- `make build` - Verify builds
- `make clean` - Clean artifacts
- `make ci` - Run complete CI pipeline locally

## 📁 Configuration Files

### Security

- `.gosec.json` - Security scanner configuration
- `SECURITY.md` - Security policy and vulnerability reporting

### Linting

- `.golangci.yml` - Comprehensive linter configuration
- Supports 30+ linters with custom rules

### CI/CD

- `.github/workflows/ci.yml` - Main CI pipeline
- `.github/dependabot.yml` - Dependency update automation

### Development

- `Makefile` - Development automation
- `coverage.out` - Coverage reports (generated)

## 🎯 Quality Gates

All CI checks must pass:

1. **Security**: No high-severity vulnerabilities
2. **Linting**: All linting rules pass
3. **Tests**: 100% test success rate
4. **Coverage**: Minimum coverage threshold
5. **Build**: Multi-platform compilation success
6. **Race Detection**: No race conditions

## 📈 Metrics & Reporting

### Code Coverage

- **Target**: >80% coverage
- **Reporting**: Codecov integration
- **Tracking**: Coverage trends over time

### Security Reporting

- **SARIF Reports**: GitHub Security tab
- **Vulnerability Database**: Regular updates
- **Security Advisories**: Automated monitoring

### Performance

- **Benchmark Results**: Tracked in CI
- **Memory Usage**: Profiling reports
- **Regression Detection**: Performance alerts

## 🔄 Workflow Triggers

### Automatic Triggers

- **Push**: `main`, `develop` branches
- **Pull Request**: Any target branch
- **Schedule**: Weekly dependency updates
- **Security**: Daily vulnerability scans

### Manual Triggers

- **Workflow Dispatch**: Manual CI runs
- **Release**: Version tagging workflow

## 📋 Troubleshooting

### Common Issues

1. **Linting Failures**

   ```bash
   make lint
   # Fix reported issues and re-run
   ```

2. **Test Failures**

   ```bash
   make test-unit
   # Debug specific test failures
   ```

3. **Security Issues**

   ```bash
   make security
   # Review gosec-results.sarif
   ```

4. **Coverage Issues**

   ```bash
   make test-coverage
   # View coverage.html report
   ```

### CI Debug Commands

```bash
# Simulate exact CI environment
docker run --rm -v "$(pwd):/workspace" -w /workspace golang:1.22 \
  bash -c "make install-tools && make ci"
```

## 🏆 Best Practices

### Development Workflow

- Run `make dev` before committing
- Use conventional commit messages
- Keep PR scope focused and small
- Include tests for new features

### Security Best Practices

- Never commit secrets or credentials
- Regular dependency updates via Dependabot
- Monitor security advisories
- Follow OWASP guidelines

### Performance Optimization

- Run benchmarks for performance-critical changes
- Profile memory usage for large refactors
- Monitor for performance regressions

## 📚 Additional Resources

- [Go Testing Best Practices](https://go.dev/doc/tutorial/add-a-test)
- [Security Guidelines](./SECURITY.md)
- [golangci-lint Documentation](https://golangci-lint.run/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)

---

## 🎉 Success Metrics

- ✅ Zero security vulnerabilities
- ✅ 100% test pass rate
- ✅ All linting rules satisfied
- ✅ Multi-platform compatibility
- ✅ Comprehensive code coverage
- ✅ Performance benchmarks stable
