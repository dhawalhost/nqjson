# Benchmark Setup and Dependency Management

## Overview

njson benchmarks are in a **separate Go module** to completely isolate dependencies. The main njson library has **ZERO external dependencies**, while benchmarks have their own `go.mod` with gjson/sjson for performance comparisons.

## Architecture

### Two Independent Modules

**Main Module** (`go.mod`):
```
module github.com/dhawalhost/njson

go 1.23.10
```
- ✅ **Zero dependencies**
- ✅ Pure Go implementation
- ✅ No external packages required

**Benchmark Module** (`benchmark/go.mod`):
```
module github.com/dhawalhost/njson/benchmark

require (
    github.com/dhawalhost/njson v0.0.0
    github.com/tidwall/gjson v1.18.0
    github.com/tidwall/sjson v1.2.5
)

replace github.com/dhawalhost/njson => ../
```
- ✅ Separate dependency tree
- ✅ gjson/sjson isolated here
- ✅ Uses local njson via replace directive

## Why Separate Modules?

### ✅ Benefits

1. **Zero Dependencies for Users**: `go get github.com/dhawalhost/njson` installs ONLY njson
2. **Complete Isolation**: Benchmark dependencies never appear in user projects
3. **Cleaner Architecture**: Clear separation between library and testing
4. **Professional Structure**: Industry best practice for Go libraries
5. **No Build Tags Needed**: Modules naturally separate concerns

### 📦 What Gets Installed

**Installing njson for normal use**:
```bash
go get github.com/dhawalhost/njson
```
- ✅ Only njson library code (single module)
- ✅ **ZERO external dependencies**
- ✅ All functionality works
- ✅ Clean `go.mod` in your project

**Running benchmarks (development)**:
```bash
cd benchmark
go test -bench=. -benchmem
```
- ✅ Benchmark module has its own `go.mod`
- ✅ gjson/sjson installed only in benchmark directory
- ✅ Doesn't affect main library dependencies
- ✅ Automatic dependency management

## Running Benchmarks

### Option 1: Using Makefile (Recommended)

```bash
# Run all benchmarks (no separate dependency install needed!)
make bench

# Run specific categories
make bench-get          # GET operations only
make bench-set          # SET operations only
make bench-delete       # DELETE operations only
make bench-multipath    # Multipath queries (njson-exclusive)
make bench-modifiers    # Extended modifiers

# Save results to file
make bench-save
```

### Option 2: Direct go test Commands

```bash
# Navigate to benchmark directory
cd benchmark

# Run all benchmarks
go test -bench=. -benchmem

# Run specific benchmarks
go test -bench=BenchmarkGet_MultiPath -benchmem
go test -bench=Modifier -benchmem
```

**Note**: No need to install dependencies separately! The `benchmark/go.mod` automatically manages gjson/sjson when you run tests in that directory.

## Build Tag Implementation

### Benchmark Files Use Build Tags

All files in `benchmark/` directory have this header:

```go
// +build benchmark

package benchmark
```

This means:
- ✅ Files are **ignored** during normal `go build` or `go test`
- ✅ Files are **included** only when `-tags=benchmark` is specified
- ✅ Dependencies (gjson/sjson) are **not required** for normal builds

### Test Files (No Build Tags)

Regular test files like `njson_get_test.go` do **NOT** use build tags:
- ✅ Always run with `go test`
- ✅ Test actual njson functionality
- ✅ No external dependencies

## Dependency Management

### Current State

```
go.mod dependencies:
├── github.com/tidwall/gjson (for benchmarks only)
├── github.com/tidwall/sjson (for benchmarks only)
└── (transitive dependencies)
```

### How It Works

1. **Normal users**: When you `go get github.com/dhawalhost/njson`, the benchmark files are excluded by build tags, so gjson/sjson are not pulled in as dependencies for your project.

2. **Developers/CI**: When running `go test -tags=benchmark`, the benchmark files are included and gjson/sjson become available.

## CI/CD Integration

The GitHub Actions workflow should include:

```yaml
name: Benchmarks

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      
      - name: Install benchmark dependencies
        run: |
          go get github.com/tidwall/gjson@latest
          go get github.com/tidwall/sjson@latest
      
      - name: Run benchmarks
        run: go test -tags=benchmark -bench=. -benchmem ./benchmark/ | tee benchmark_results.txt
      
      - name: Upload results
        uses: actions/upload-artifact@v3
        with:
          name: benchmark-results
          path: benchmark_results.txt
```

## For Contributors

### Setting Up Development Environment

```bash
# Clone repository
git clone https://github.com/dhawalhost/njson.git
cd njson

# Install development tools
make install-tools

# Install benchmark dependencies
make bench-install-deps

# Run tests
make test

# Run benchmarks
make bench
```

### Adding New Benchmarks

1. Create benchmark function in `benchmark/get_bench_test.go` or `benchmark/set_bench_test.go`
2. Ensure file has `// +build benchmark` header
3. Test with: `go test -tags=benchmark -bench=YourNewBenchmark -benchmem ./benchmark/`
4. Update `BENCHMARKS.md` with results

## Troubleshooting

### Error: "package github.com/dhawalhost/njson/benchmark: build constraints exclude all Go files"

**Solution**: Add `-tags=benchmark` flag:
```bash
go test -tags=benchmark -bench=. ./benchmark/
```

### Error: "package github.com/tidwall/gjson is not in GOROOT"

**Solution**: Install benchmark dependencies:
```bash
make bench-install-deps
# OR
go get github.com/tidwall/gjson@latest
go get github.com/tidwall/sjson@latest
```

### Benchmarks run too fast/slow

**Adjust benchmark time**:
```bash
go test -tags=benchmark -bench=. -benchtime=5s ./benchmark/
```

## Best Practices

### For Library Users
- ✅ Just use `go get github.com/dhawalhost/njson`
- ✅ No need to install gjson/sjson
- ✅ All njson features work out of the box

### For Contributors
- ✅ Run `make bench-install-deps` once
- ✅ Use `make bench` to run benchmarks
- ✅ Always test with and without `-tags=benchmark` to ensure separation works

### For CI/CD
- ✅ Install benchmark deps in CI pipeline
- ✅ Run benchmarks with `-tags=benchmark`
- ✅ Save results as artifacts
- ✅ Optional: Compare with previous runs to detect regressions

## References

- [Go Build Constraints](https://pkg.go.dev/cmd/go#hdr-Build_constraints)
- [Benchmark Guidelines](https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go)
- See `benchmark/README.md` for more benchmark-specific details
