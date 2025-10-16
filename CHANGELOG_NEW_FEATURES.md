# New Features - Multipath Queries & Extended Modifiers

## Summary

Successfully implemented and debugged gjson feature parity extensions for nqjson:

### ✅ Multipath Queries (nqjson-exclusive)
- **Feature**: Query multiple paths in a single call using comma-separated syntax
- **Syntax**: `Get(json, "user.name,user.email,user.age")`
- **Returns**: JSON array containing results for each path
- **Performance**: 1.2-3.3µs for 2-5 fields
- **Tests**: `TestGetMultiPath` - all passing
- **Benchmarks**: 
  - 2 fields: 1,418 ns/op, 368 B/op
  - 5 fields: 3,305 ns/op, 1,016 B/op
  - Mixed: 2,015 ns/op, 600 B/op

### ✅ JSON Lines Support (gjson parity)
- **Feature**: Query JSON Lines format (newline-delimited JSON)
- **Syntax**: `Get(jsonLines, "..#.name")` - all names from all lines
- **Syntax**: `Get(jsonLines, "..2.age")` - age from line 2
- **Performance**: 3-4µs per query
- **Tests**: `TestGetMultiPath` includes JSON Lines tests
- **Benchmarks**:
  - Projection: 3,064 ns/op (nqjson) vs 2,311 ns/op (gjson)
  - Indexed: 4,559 ns/op (nqjson) vs 457 ns/op (gjson)

### ✅ Extended Modifiers
**Supported by gjson**:
- `@reverse` - Reverse array order
- `@flatten` - Flatten nested arrays

**New nqjson-exclusive modifiers**:
- `@distinct` - Remove duplicate values (5,178 ns/op)
- `@sort` - Sort array elements (3,617 ns/op)
- `@first` - Get first element (1,617 ns/op)
- `@last` - Get last element (1,972 ns/op)
- `@sum` - Sum numeric array (2,126 ns/op)
- `@avg` - Average of numeric array (3,021 ns/op)
- `@min` - Minimum value (2,231 ns/op)
- `@max` - Maximum value (2,357 ns/op)

**Example Usage**:
```go
// Get reversed array
result := nqjson.Get(json, "nums|@reverse")

// Get sum of scores
total := nqjson.Get(json, "scores|@sum")

// Get unique IDs
ids := nqjson.Get(json, "items.#.id|@distinct")

// Combined: multipath with modifiers
results := nqjson.Get(json, "nums|@reverse,scores|@avg")
```

## Bug Fix - Root Cause Analysis

### Problem
Extended modifiers were returning `undefined` even though the framework was correctly implemented.

### Root Cause
The `scanKey` function (line 4099-4112 in `nqjson_get.go`) only checked for `.` and `[` as path terminators but didn't check for `|` and `@` (modifier separators).

This caused paths like `"nums|@reverse"` to be incorrectly classified as "simple paths", routing them through `getSimplePath` instead of `getComplexPath`, which meant modifiers were never tokenized or applied.

### Solution
Added `&& path[p] != '|' && path[p] != '@'` to the loop condition in `scanKey`:

```go
// Before:
for p < len(path) && path[p] != '.' && path[p] != '[' {

// After:
for p < len(path) && path[p] != '.' && path[p] != '[' && path[p] != '|' && path[p] != '@' {
```

This ensures paths containing modifiers are correctly identified as complex paths and routed through the proper tokenization and execution pipeline.

## Test Coverage

### Test Files
- `nqjson_get_multipath_test.go` - Comprehensive tests for multipath and extended modifiers
- `nqjson_modifier_debug_test.go` - Debug test showing correct routing behavior

### Test Results
```bash
$ go test -v -run TestGetMultiPath
=== RUN   TestGetMultiPath
--- PASS: TestGetMultiPath (0.00s)
PASS

$ go test -v -run TestExtendedModifiers  
=== RUN   TestExtendedModifiers
--- PASS: TestExtendedModifiers (0.00s)
PASS
```

## Benchmark Suite Expansion

### New Benchmarks Added

**Multipath (nqjson-exclusive)**:
- `BenchmarkGet_MultiPath_TwoFields_NQJSON`
- `BenchmarkGet_MultiPath_FiveFields_NQJSON`
- `BenchmarkGet_MultiPath_Mixed_NQJSON`
- `BenchmarkGet_MultiPath_WithModifier_NQJSON`

**Extended Modifiers**:
- `BenchmarkGet_Modifier_Distinct_NQJSON`
- `BenchmarkGet_Modifier_Sort_NQJSON`
- `BenchmarkGet_Modifier_First_NQJSON`
- `BenchmarkGet_Modifier_Last_NQJSON`
- `BenchmarkGet_Modifier_Sum_NQJSON`
- `BenchmarkGet_Modifier_Avg_NQJSON`
- `BenchmarkGet_Modifier_Min_NQJSON`
- `BenchmarkGet_Modifier_Max_NQJSON`

**SET/DELETE Expanded**:
- Array element updates/deletions
- Deep nested operations
- Multiple update sequences
- Complex value types (objects, arrays)

**Total Benchmark Count**: 71 benchmarks (GET: 38, SET: 21, DELETE: 4, Multi-op: 2)

## Performance Characteristics

### Where nqjson Excels
1. ✅ **DELETE operations** on simple/nested fields (faster than sjson)
2. ✅ **Extended modifiers** - 8 exclusive aggregation/transformation operations
3. ✅ **Multipath queries** - fetch multiple fields in single query (no gjson equivalent)
4. ✅ **Feature completeness** - gjson parity + extensions

### Where gjson/sjson Excel
1. **Simple GET paths** - gjson 2-16x faster on basic queries
2. **Large arrays** - gjson's statistical jump optimization
3. **SET operations** - sjson 1.5-10x faster for most SET scenarios
4. **Memory efficiency** - gjson minimal allocations for simple paths

## Files Modified

### Core Implementation
- `nqjson_get.go`:
  - Fixed `scanKey` function (line ~4101)
  - Added `getWithOptions` internal dispatcher
  - Added `getMultiPathResult` for multipath handling
  - Added `getJSONLinesResult` for JSON Lines support
  - Added 8 new modifier implementations
  - Added `splitMultiPath`, `extractJSONLinesValues` helpers

### Tests
- `nqjson_get_multipath_test.go` - NEW file with comprehensive tests
- `nqjson_modifier_debug_test.go` - NEW debug test file

### Benchmarks
- `benchmark/get_bench_test.go` - Expanded with 20+ new benchmarks
- `benchmark/set_bench_test.go` - Expanded with 12+ new benchmarks
- `benchmark_results.txt` - Updated with full results

### Documentation
- `BENCHMARKS.md` - Completely rewritten with new data
- `CHANGELOG_NEW_FEATURES.md` - THIS FILE

## Running the Features

```bash
# Run all tests
go test -v

# Run multipath tests only
go test -v -run TestGetMultiPath

# Run extended modifier tests only  
go test -v -run TestExtendedModifiers

# Run all benchmarks
go test -bench=. -benchmem ./benchmark/

# Run multipath benchmarks only
go test -bench=MultiPath -benchmem ./benchmark/

# Run modifier benchmarks only
go test -bench=Modifier -benchmem ./benchmark/
```

## Migration Guide

### Using Multipath Queries

**Before** (multiple Get calls):
```go
name := nqjson.Get(json, "user.name")
email := nqjson.Get(json, "user.email")
age := nqjson.Get(json, "user.age")
```

**After** (single multipath query):
```go
results := nqjson.Get(json, "user.name,user.email,user.age")
// results is JSON array: ["Alice","alice@example.com",28]
```

### Using Extended Modifiers

```go
// Get sum of all scores
total := nqjson.Get(json, "scores|@sum")  // Returns: 464.8

// Get average score
avg := nqjson.Get(json, "scores|@avg")    // Returns: 92.96

// Get unique IDs (remove duplicates)
ids := nqjson.Get(json, "items.#.id|@distinct")  // Returns: [1,2,3]

// Get sorted numbers
sorted := nqjson.Get(json, "nums|@sort")  // Returns: [1,2,3,4,5,8,9]

// First and last elements
first := nqjson.Get(json, "items|@first")
last := nqjson.Get(json, "items|@last")
```

### Combined Usage

```go
// Multipath with modifiers
results := nqjson.Get(json, "nums|@reverse,scores|@avg,items.#.id|@distinct")
// Returns array with all three results
```

## Future Enhancements

Potential additions for future versions:
- [ ] Custom modifiers API
- [ ] Modifier chaining (`nums|@sort|@reverse|@first`)
- [ ] Statistical modifiers (`@median`, `@stddev`, `@percentile`)
- [ ] String modifiers (`@upper`, `@lower`, `@trim`)
- [ ] Date/time modifiers
- [ ] JSONPath filter expressions
- [ ] Performance optimizations for large arrays (adopt gjson's jump algorithms)

## Credits

- **gjson** by Josh Baker - Inspiration and compatibility target
- **sjson** by Josh Baker - SET operation reference implementation
- **nqjson** - Extended feature implementation

---

**Status**: ✅ All features implemented, tested, and benchmarked
**Version**: Ready for release
**Date**: October 16, 2025
