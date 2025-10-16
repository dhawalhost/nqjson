# Benchmarks

This directory contains performance benchmarks comparing njson against gjson (GET operations) and sjson (SET/DELETE operations).

## Separate Go Module

This directory has its own `go.mod` file, making it a **separate Go module** from the main njson library. This ensures:
- ✅ Main njson library has **ZERO dependencies**
- ✅ Benchmark dependencies (gjson/sjson) are completely isolated
- ✅ Users installing njson don't download benchmark deps

## Running Benchmarks

### Quick Start

```bash
# Navigate to benchmark directory
cd benchmark

# Run all benchmarks
go test -bench=. -benchmem
```

That's it! No need to install dependencies - the `go.mod` in this directory handles everything automatically.

### Run Specific Benchmark Categories

**GET Operations:**
```bash
go test -bench=BenchmarkGet -benchmem
```

**SET Operations:**
```bash
go test -bench=BenchmarkSet -benchmem
```

**DELETE Operations:**
```bash
go test -bench=BenchmarkDelete -benchmem
```

**Multipath Queries (njson-exclusive):**
```bash
go test -bench=MultiPath -benchmem
```

**Extended Modifiers:**
```bash
go test -bench=Modifier -benchmem
```

## Dependencies

This benchmark module requires:
- `github.com/tidwall/gjson` - for GET operation comparisons
- `github.com/tidwall/sjson` - for SET/DELETE operation comparisons

These are **automatically managed** by the `go.mod` file in this directory and are **NOT** required for normal njson usage.

## Module Structure

```
njson/
├── go.mod                    # Main module - ZERO dependencies
├── njson_get.go
├── njson_get_test.go        # Tests - no external deps
└── benchmark/
    ├── go.mod               # Benchmark module - has gjson/sjson
    ├── get_bench_test.go    # Benchmarks comparing with gjson
    └── set_bench_test.go    # Benchmarks comparing with sjson
```

The `replace` directive in `benchmark/go.mod` points to the parent directory, so benchmarks always use the local development version of njson.

## Benchmark Results

Latest benchmark results are saved in `../benchmark_results.txt` at the repository root.

For detailed performance analysis, see `../BENCHMARKS.md`.

## No Build Tags Required

Unlike the previous approach, this module-based separation doesn't require build tags. The benchmark directory is a completely separate Go module with its own dependency tree.

**Benefits:**
- ✅ Simpler to use - just `cd benchmark && go test -bench=.`
- ✅ True dependency isolation at the module level
- ✅ No need for `-tags=benchmark` flags
- ✅ Automatic dependency management via go.mod

## CI/CD Integration

See `../.github/workflows/benchmarks.yml` for automated benchmark execution in CI.
