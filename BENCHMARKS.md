# nqjson vs gojq Comprehensive Benchmark Report

**Test Environment:** Apple M3 Pro, macOS, Go 1.23.10  
**Test Duration:** 334 seconds  
**Dataset Size:** 130MB (300,000 records)

---

## Test Data

### Large Dataset (130MB)
- **Records:** 300,000 complex user objects
- **Nesting:** 4 levels deep
- **Fields per record:** 25+ including arrays, nested objects
- **Exceeds L3 Cache:** ✅ Yes

### Medium Dataset (22MB / ~23MB)
- **Records:** 50,000 users
- Used for gojq comparison benchmarks

---

## 📊 Size-Based Benchmarks (32 MiB to 1 GiB)

Fuzzer-generated data using schema-based JSON documents with reproducible random data.

### Complete Size Results

| Size | nqjson GET | gojq GET | Speedup | nqjson Allocs | gojq Allocs |
|------|------------|----------|---------|---------------|-------------|
| 32 MiB | 3.4ms | 39ms | **11x** | 0 | 67K |
| 64 MiB | 19ms | 76ms | **4x** | 0 | 134K |
| 128 MiB | 35ms | 153ms | **4x** | 0 | 268K |
| 256 MiB | 71ms | 356ms | **5x** | 0 | 537K |
| 512 MiB | 178ms | 606ms | **3.4x** | 0 | 1.07M |
| **1 GiB** | **364ms** | **5,193ms** | **14x** | 0 | 2.15M |

### Throughput Analysis

| Size | nqjson Throughput | Notes |
|------|-------------------|-------|
| 32 MiB | 9,186 MB/s | Below L3 cache |
| 64 MiB | 3,362 MB/s | At L3 cache boundary |
| 128 MiB | 3,648 MB/s | 2x L3 cache |
| 256 MiB | 3,624 MB/s | Large document |
| 512 MiB | 2,892 MB/s | Very large document |
| 1 GiB | 2,850 MB/s | Stress test |

### Key Findings
- ✅ **nqjson: 0 allocations** at ALL sizes up to 1 GiB
- ✅ **At 1 GiB: nqjson is 14x faster than gojq**
- ✅ **Consistent throughput**: ~2.8-3.6 GB/s across all sizes
- ✅ **gojq allocations grow linearly**: 2.15M allocs at 1 GiB
- ✅ **Cache effects visible**: 32 MiB (9 GB/s) vs larger sizes (~3 GB/s)

---

## 🔗 Complex Nested Data Benchmarks

Testing deeply nested structures with multiple levels of arrays containing objects.

### Hierarchy Structure (7-8 levels)
`organizations[] → departments[] → teams[] → projects[] → tasks[] → subtasks[] → comments[] → reactions[]`

| Path Depth | nqjson | gojq | Speedup | nqjson Allocs | gojq Allocs |
|------------|--------|------|---------|---------------|-------------|
| 7 levels (author) | **740µs** | 3.75ms | **5x** | 0 | 15,932 |
| 8 levels (reactions) | **745µs** | 3.37ms | **4.5x** | 0 | 15,932 |
| 4 levels (mid) | **1.16ms** | 3.58ms | **3x** | 0 | 15,932 |

### E-Commerce Orders (30MB, 10K orders)
`orders[] → lineItems[] → variants[] → modifiers[]`

| Access Pattern | nqjson | gojq | Speedup | nqjson Allocs | gojq Allocs |
|----------------|--------|------|---------|---------------|-------------|
| Deep variant modifier | **12ms** | 54ms | **4.5x** | 0 | 190,010 |
| Nested category.parent | **19ms** | 60ms | **3x** | 0 | 190,010 |
| Customer address | **23ms** | 55ms | **2.4x** | 0 | 190,009 |

### Key Findings
- ✅ **nqjson maintains 0 allocations** even with 8 levels of nesting
- ✅ **Consistent performance** regardless of nesting depth
- ✅ **gojq allocations explode** with document complexity (190K allocs)

---

## Key Benchmark Results

### 🏆 Large Data Access (130MB)

| Operation | nqjson | gojq | Winner |
|-----------|--------|------|--------|
| Simple field access | **81ms, 0 allocs** | 1.79s, 31M allocs | ✅ **nqjson 22x faster** |
| Deep nested access | **53ms, 0 allocs** | ~115ms (compiled) | ✅ **nqjson 2x faster** |
| Last element (worst case) | **71ms, 0 allocs** | ~115ms | ✅ **nqjson** |

### Medium Data Operations (22MB)

| Operation | nqjson | gojq (compiled) | Winner |
|-----------|--------|-----------------|--------|
| Simple field | **9ms, 0 allocs** | 27ms, 944 B | ✅ **nqjson 3x faster** |
| Array first | **108ns, 0 allocs** | 28ms | ✅ **nqjson 259,000x faster** |
| Array middle | **9ms, 0 allocs** | 27ms | ✅ **nqjson 3x faster** |
| Array last | **18ms, 0 allocs** | 28ms | ✅ **nqjson 1.5x faster** |
| Deep nested | **9ms, 0 allocs** | 27ms | ✅ **nqjson 3x faster** |

### Modifier/Transformation Operations

| Operation | nqjson | gojq | Winner |
|-----------|--------|------|--------|
| Sum (small array) | 2.7ms, 862KB | 654µs, 131KB | ⚠️ gojq 4x faster |
| Avg (small array) | 2.7ms, 862KB | 617µs, 131KB | ⚠️ gojq 4x faster |
| Min/Max | 2.7ms | 615-635µs | ⚠️ gojq 4x faster |
| Unique | 3.2ms | 703µs | ⚠️ gojq 4x faster |
| Filter (select) | 1.97ms | 720µs | ⚠️ gojq 3x faster |
| Projection | 270µs | 61µs | ⚠️ gojq 4x faster |

### SET/DELETE Operations (nqjson exclusive)

| Operation | nqjson | Notes |
|-----------|--------|-------|
| Set simple field | 878ns | sjson: 469ns |
| Delete simple | **182ns** | Competitive with sjson |
| Delete nested | **408ns** | Faster than sjson (446ns) |
| Increment | ~1.4µs | New helper |
| SetMany | ~2.7µs | New helper |
| DeleteMany | **368ns** | New helper |

---

## Memory Allocation Comparison

| Scenario | nqjson | gojq |
|----------|--------|------|
| **130MB simple access** | **0 B, 0 allocs** | 1.16 GB, 31M allocs |
| 22MB simple access | **0 B, 0 allocs** | 944 B, 7 allocs |
| Full JSON unmarshal | N/A | 150 MB, 3.1M allocs |

**Key Insight:** nqjson maintains **zero allocations** for data access regardless of document size.

---

## What nqjson is Missing

| Feature | Description |
|---------|-------------|
| **Variables** | `.x as $var \| .y + $var` |
| **Recursive Descent** | `..` to find values at any depth |
| **Full Pipe Chaining** | Complex multi-step expressions |
| **Conditionals in-expr** | `if .x then .y else .z end` |
| **reduce** | Accumulator pattern for custom aggregations |

---

## What gojq is Missing

| Feature | Description |
|---------|-------------|
| **Zero Allocations** | Always allocates memory |
| **Raw Byte Processing** | Requires JSON parsing first |
| **SET Operations** | No native set/update capability |
| **DELETE Operations** | No native delete capability |
| **Increment/Decrement** | No mutation helpers |
| **Custom Modifiers** | Runtime registration via Go |
| **Predictable Latency** | GC pauses from allocations |

---

## 🎯 Final Verdict

### nqjson is Better When:

| Use Case | Reason |
|----------|--------|
| ✅ **Large data (100MB+)** | 22x faster, 0 allocations vs 31M |
| ✅ **Memory-constrained** | 0 B vs 1.16 GB per operation |
| ✅ **High-throughput APIs** | No GC pauses, predictable latency |
| ✅ **Microservices** | Lower memory footprint |
| ✅ **Serverless/Lambda** | Faster cold start, less memory |
| ✅ **Data mutations** | SET/DELETE/Increment built-in |
| ✅ **Real-time systems** | No GC spikes |

### gojq is Better When:

| Use Case | Reason |
|----------|--------|
| ⚠️ **Array transformations** | 4x faster for sum/avg/filter |
| ⚠️ **Complex expressions** | Variables, conditionals, reduce |
| ⚠️ **jq compatibility** | Same syntax as CLI jq |
| ⚠️ **Scripting/exploration** | Familiar expression language |

---

## 📊 Does nqjson Work for Very Large Data?

### ✅ YES - nqjson Excels at Large Data

| Data Size | nqjson Performance | gojq Performance |
|-----------|-------------------|------------------|
| 22 MB | 9ms, 0 allocs | 27ms + parsing |
| 88 MB | 36ms, 0 allocs | 115ms + parsing |
| 130 MB | 81ms, 0 allocs | 1.79s, 31M allocs |

**Why nqjson scales better:**
1. **No parsing required** - works on raw bytes
2. **Zero allocations** - no memory growth with data size
3. **Linear scan** - O(n) traversal to target
4. **No GC pressure** - consistent performance

---

## 🏆 Overall Winner

### For Production Systems: **nqjson**

```
Large Data Access:     nqjson wins (22x faster)
Memory Usage:          nqjson wins (0 vs 1.16 GB)
SET/DELETE:            nqjson wins (gojq has none)
Latency Predictability: nqjson wins (no GC)
Custom Extensions:     nqjson wins (RegisterModifier)
```

### For Complex Transformations: **gojq**

```
Array Aggregations:    gojq wins (4x faster)
Complex Expressions:   gojq wins (variables, pipes)
jq Compatibility:      gojq wins (same syntax)
```

---

## Recommendation Summary

| Scenario | Choose |
|----------|--------|
| API server processing large JSON | **nqjson** |
| Extracting fields from 100MB+ docs | **nqjson** |
| Memory-limited container | **nqjson** |
| Complex jq expressions needed | gojq |
| One-time data transformation script | gojq |
| Need mutations (set/delete) | **nqjson** |
