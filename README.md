# NQJSON

[![Go Report Card](https://goreportcard.com/badge/github.com/dhawalhost/nqjson)](https://goreportcard.com/report/github.com/dhawalhost/nqjson) [![GoDoc](https://godoc.org/github.com/dhawalhost/nqjson?status.svg)](https://godoc.org/github.com/dhawalhost/nqjson) [![Static Badge](https://img.shields.io/badge/nqjson-playground-blue)](https://dhawalhost.github.io/nqjson-playground/)

**nqjson** is a high-performance JSON manipulation library for Go that delivers **blazing-fast operations** with **zero allocations** on critical paths. Built for modern applications requiring extreme performance, minimal memory overhead, and advanced JSON processing capabilities.

## ⚡ Why nqjson?

### 🚀 **Zero-Allocation Performance**
- **0 allocations** on simple GET operations
- **No GC pressure** - Perfect for high-throughput systems
- **Predictable latency** - No GC pauses in critical paths
- **Memory efficient** - Minimal overhead even with complex queries

### 🎯 **Powerful Features**
- **Multipath Queries** - Get multiple values in one call: `user.name,user.email,user.age`
- **8 Advanced Modifiers** - `@distinct`, `@sort`, `@sum`, `@avg`, `@min`, `@max`, `@first`, `@last`
- **Path Caching** - 2-5x speedup with `GetCached()` for hot paths
- **JSON Lines Support** - Native newline-delimited JSON processing
- **Complete CRUD** - GET, SET, DELETE with atomic operations

### 💪 **Production Ready**
- **Thread-safe** - Concurrent access without locks
- **Battle-tested** - 73.9% test coverage with 168 comprehensive tests
- **Zero dependencies** - No external runtime dependencies
- **Type-safe** - Automatic type conversion with validation

## 🌟 Key Features

### 📊 Advanced Query Operations

```go
// Multipath - Get multiple fields in one call
result := nqjson.Get(json, "user.name,user.email,user.age,user.status")
// Returns: ["John Doe","john@example.com",30,"active"]

// Statistical aggregations
totals := nqjson.Get(json, "orders.#.amount|@sum")      // Sum all amounts
average := nqjson.Get(json, "ratings.#.score|@avg")     // Average rating
highest := nqjson.Get(json, "products.#.price|@max")    // Highest price

// Array transformations
unique := nqjson.Get(json, "tags|@distinct|@sort")      // Unique sorted tags
sorted := nqjson.Get(json, "items.#.name|@sort")        // Alphabetically sorted names
```

### 🔍 Flexible Path Expressions

```go
// Dot notation
city := nqjson.Get(json, "address.city")

// Array indexing (multiple styles)
first := nqjson.Get(json, "items.0")        // Bracket-free
last := nqjson.Get(json, "items.-1")        // Negative indexing
range := nqjson.Get(json, "items.0:5")      // Range slicing

// Wildcards
allNames := nqjson.Get(json, "users.*.name")           // All user names
allEmails := nqjson.Get(json, "users.#.email")         // Array iteration

// Filters and queries
adults := nqjson.Get(json, "users.#(age>=18).name")    // Conditional filtering
active := nqjson.Get(json, "users.#(status=active)")   // Exact match
premium := nqjson.Get(json, "users.#(tier=premium).email")  // Complex queries
```

### ⚙️ Complete CRUD Operations

```go
// GET - Zero-allocation reads
value := nqjson.Get(json, "user.profile.email")

// SET - Atomic updates
updated, _ := nqjson.Set(json, "user.status", "active")

// DELETE - Safe removal
cleaned, _ := nqjson.Delete(json, "user.temp_data")

// BATCH - Multiple operations
results := nqjson.GetMany(json, "name", "email", "age")
```

### 🎨 Data Transformations

```go
// Sort arrays
sorted := nqjson.Get(json, "scores|@sort")              // Ascending
reversed := nqjson.Get(json, "scores|@sort|@reverse")   // Descending

// Extract unique values
unique := nqjson.Get(json, "categories|@distinct")

// Statistical operations
sum := nqjson.Get(json, "values|@sum")
avg := nqjson.Get(json, "values|@avg")
min := nqjson.Get(json, "values|@min")
max := nqjson.Get(json, "values|@max")

// Array manipulation
first := nqjson.Get(json, "items|@first")
last := nqjson.Get(json, "items|@last")
flattened := nqjson.Get(json, "nested.arrays|@flatten")
```

### ⚡ Performance Optimization

```go
// Path caching for hot paths (2-5x faster)
result := nqjson.GetCached(json, "frequently.accessed.path")

// Batch operations
results := nqjson.GetMany(json, 
    "user.name",
    "user.email", 
    "user.status",
    "user.lastLogin",
)

// Zero-copy string access
if result.Type == nqjson.TypeString {
    // Use Raw for zero allocations
    rawValue := result.Raw
}
```

## 📈 Performance

### Benchmark Results

| Operation | Time | Allocations | Memory |
|-----------|------|-------------|--------|
| Simple GET | 86 ns/op | 0 allocs/op | 0 B/op |
| Nested GET (4 levels) | 224 ns/op | 0 allocs/op | 0 B/op |
| Deep GET (8 levels) | 365 ns/op | 0 allocs/op | 0 B/op |
| Array access (middle) | 10.8 μs/op | 0 allocs/op | 0 B/op |
| Multipath (5 fields) | 4.7 μs/op | 4 allocs/op | 880 B/op |
| Simple SET | 1.01 μs/op | 2 allocs/op | 592 B/op |
| Simple DELETE | 244 ns/op | 1 alloc/op | 248 B/op |

**Key Advantages:**
- ✅ Zero allocations on all simple GET operations
- ✅ No GC pressure for read-heavy workloads
- ✅ Predictable performance under load
- ✅ Excellent scalability for high-throughput systems

## 📖 Documentation

- **[Installation Guide](INSTALL.md)** - Step-by-step installation and setup
- **[API Reference](API.md)** - Complete API documentation with examples
- **[Path Syntax Guide](SYNTAX.md)** - Comprehensive path expression reference
- **[Examples](EXAMPLES.md)** - Real-world usage patterns and recipes
- **[Performance Benchmarks](BENCHMARKS.md)** - Detailed performance analysis
- **[Performance Summary](PERFORMANCE_SUMMARY.md)** - Production optimization guide

## 📦 Installation

```bash
go get github.com/dhawalhost/nqjson
```

## 🧩 Go Version Compatibility

nqjson is compatible with >= 1.23.10

- **Go 1.23.10+**:
  ```bash
  go get github.com/dhawalhost/nqjson@latest
  ```

The public API remains consistent across versions.

## � Quick Start

### Simple GET Operations

```go
import "github.com/dhawalhost/nqjson"

json := []byte(`{
    "name": "John Doe",
    "age": 30,
    "skills": ["Go", "Python", "JavaScript"],
    "address": {
        "city": "New York",
        "coordinates": {"lat": 40.7128, "lng": -74.0060}
    }
}`)

// Zero-allocation field access
name := nqjson.Get(json, "name")
fmt.Println(name.String())  // John Doe

// Deep nested access
lat := nqjson.Get(json, "address.coordinates.lat")
fmt.Println(lat.Float())  // 40.7128

// Array access
skill := nqjson.Get(json, "skills.0")
fmt.Println(skill.String())  // Go
```

### Multipath Queries (Unique to nqjson!)

```go
json := []byte(`{
    "user": {
        "name": "Alice",
        "email": "alice@example.com",
        "age": 28,
        "status": "active"
    }
}`)

// Get multiple fields in ONE call - Super efficient!
result := nqjson.Get(json, "user.name,user.email,user.age,user.status")
fmt.Println(result.String())
// ["Alice","alice@example.com",28,"active"]
```

### Advanced Modifiers

```go
json := []byte(`{
    "scores": [85, 92, 78, 95, 88, 92, 85],
    "sales": [1200, 1500, 980, 2100, 1800]
}`)

// Statistical operations
sum := nqjson.Get(json, "sales|@sum")
fmt.Println("Total:", sum.Float())  // 7580

avg := nqjson.Get(json, "scores|@avg")
fmt.Println("Average:", avg.Float())  // 87.857

// Unique and sorted values
unique := nqjson.Get(json, "scores|@distinct|@sort")
fmt.Println(unique.String())  // [78,85,88,92,95]

// Min/Max operations
highest := nqjson.Get(json, "scores|@max")  // 95
lowest := nqjson.Get(json, "scores|@min")   // 78
```

### SET and DELETE Operations

```go
json := []byte(`{"name": "John", "age": 30}`)

// Update field
json, _ = nqjson.Set(json, "age", 31)

// Add nested field (auto-creates structure!)
json, _ = nqjson.Set(json, "address.city", "Boston")

// Delete field
json, _ = nqjson.Delete(json, "age")

fmt.Println(string(json))
// {"name":"John","address":{"city":"Boston"}}
```

### Complex Filtering

```go
json := []byte(`{
    "products": [
        {"name": "Laptop", "price": 999, "stock": 15},
        {"name": "Mouse", "price": 25, "stock": 150},
        {"name": "Keyboard", "price": 75, "stock": 80},
        {"name": "Monitor", "price": 299, "stock": 45}
    ]
}`)

// Filter by condition
expensive := nqjson.Get(json, "products.#(price>100).name")
fmt.Println(expensive.String())  // ["Laptop","Monitor"]

// Calculate total value
totalValue := nqjson.Get(json, "products.#.price|@sum")
fmt.Println("Total:", totalValue.Float())  // 1398
```

## 📖 API Reference

### GET Operations

#### `Get(json []byte, path string) Result`

Retrieves a value from JSON using a path expression.

```go
json := []byte(`{"users": [{"name": "Alice"}, {"name": "Bob"}]}`)

// Get single value
name := nqjson.Get(json, "users.0.name")
fmt.Println(name.String()) // Alice

// Check if value exists
if name.Exists() {
    fmt.Println("User found")
}

// Type-safe conversions
age := nqjson.Get(json, "users.0.age")
if age.Exists() {
    fmt.Println("Age:", age.Int())
} else {
    fmt.Println("Age not found")
}
```

#### `GetMany(json []byte, paths ...string) []Result`

Retrieves multiple values in a single operation.

```go
json := []byte(`{"name": "John", "age": 30, "city": "NYC"}`)

results := nqjson.GetMany(json, "name", "age", "city")
for i, result := range results {
    fmt.Printf("Field %d: %s\n", i, result.String())
}
```

### SET Operations

#### `Set(json []byte, path string, value interface{}) ([]byte, error)`

Sets a value at the specified path.

```go
json := []byte(`{"users": []}`)

// Add to array
result, err := nqjson.Set(json, "users.-1", map[string]interface{}{
    "name": "Alice",
    "age":  25,
})

// Update nested value
result, err = nqjson.Set(result, "users.0.active", true)
```

#### `SetWithOptions(json []byte, path string, value interface{}, options *SetOptions) ([]byte, error)`

Sets a value with advanced options.

```go
options := &nqjson.SetOptions{
    MergeObjects: true,
    MergeArrays:  false,
}

result, err := nqjson.SetWithOptions(json, "config", newConfig, options)
```

#### `Delete(json []byte, path string) ([]byte, error)`

Removes a value at the specified path.

```go
json := []byte(`{"name": "John", "age": 30, "temp": "delete_me"}`)

result, err := nqjson.Delete(json, "temp")
// Result: {"name": "John", "age": 30}
```

### Path Expressions

nqjson supports powerful path expressions:

```go
json := []byte(`{
    "store": {
        "books": [
            {"title": "Go Programming", "price": 29.99, "tags": ["programming", "go"]},
            {"title": "Python Guide", "price": 24.99, "tags": ["programming", "python"]},
            {"title": "Web Design", "price": 19.99, "tags": ["design", "web"]}
        ]
    }
}`)

// Array indexing
firstBook := nqjson.Get(json, "store.books.0.title")

// Array filtering
expensiveBooks := nqjson.Get(json, "store.books.#(price>25).title")

// Wildcard matching
allPrices := nqjson.Get(json, "store.books.#.price")

// Complex expressions
programmingBooks := nqjson.Get(json, "store.books.#(tags.#(#==\"programming\")).title")
```

### Result Types

The `Result` type provides type-safe access to values:

```go
result := nqjson.Get(json, "some.path")

// Check existence
if result.Exists() {
    // Type conversion methods
    str := result.String()
    num := result.Float()
    integer := result.Int()
    boolean := result.Bool()
    
    // Get underlying type
    switch result.Type {
    case nqjson.TypeString:
        fmt.Println("String value:", result.String())
    case nqjson.TypeNumber:
        fmt.Println("Number value:", result.Float())
    case nqjson.TypeBool:
        fmt.Println("Boolean value:", result.Bool())
    case nqjson.TypeArray:
        fmt.Println("Array with", len(result.Array()), "elements")
    case nqjson.TypeObject:
        fmt.Println("Object value:", result.Map())
    }
}
```

## 🎯 Advanced Usage

### Batch Processing

```go
// Process multiple operations efficiently
json := []byte(`{"users": [], "config": {}}`)

// Compile paths for reuse (performance optimization)
userPath, _ := nqjson.CompileSetPath("users.-1")
configPath, _ := nqjson.CompileSetPath("config.theme")

// Use compiled paths
result, _ := nqjson.SetWithCompiledPath(json, userPath, newUser, nil)
result, _ = nqjson.SetWithCompiledPath(result, configPath, "dark", nil)
```

### Error Handling

```go
result, err := nqjson.Set(json, "invalid..path", value)
if err != nil {
    switch err {
    case nqjson.ErrInvalidPath:
        fmt.Println("Path syntax error")
    case nqjson.ErrInvalidJSON:
        fmt.Println("Invalid JSON input")
    default:
        fmt.Println("Operation failed:", err)
    }
}
```

### Memory Optimization

```go
// For high-performance scenarios, reuse byte slices
var buffer []byte

json := getData()
result := nqjson.Get(json, "important.field")

// Avoid string allocations when possible
if result.Type == nqjson.TypeString {
    // Use result.Raw for zero-copy access
    rawBytes := result.Raw
    // Process rawBytes directly
}
```

## 🔍 Performance Tips

1. **Use byte slices**: Work with `[]byte` instead of strings when possible
2. **Compile paths**: For repeated operations, use `CompileSetPath` and `SetWithCompiledPath`
3. **Batch operations**: Use `GetMany` for multiple field access
4. **Zero-copy**: Use `Result.Raw` for string values to avoid allocations
5. **Preallocate**: When building JSON, preallocate result slices

## 📊 Benchmarks

nqjson benchmarks are in a **separate Go module** (`benchmark/`) with **zero impact** on your dependencies.

```bash
# Navigate to benchmark directory
cd benchmark

# Run all benchmarks
go test -bench=. -benchmem

# Run specific categories
go test -bench=BenchmarkGet -benchmem        # GET operations
go test -bench=BenchmarkSet -benchmem        # SET operations
go test -bench=MultiPath -benchmem           # Multipath queries
go test -bench=Modifier -benchmem            # Extended modifiers
```

**Why separate module?** The benchmark directory has its own `go.mod` file. This means:
- ✅ Main nqjson library has **ZERO dependencies**
- ✅ Benchmark dependencies (gjson/sjson) completely isolated
- ✅ Your `go.mod` stays clean when you install nqjson

For detailed performance analysis, see [BENCHMARKS.md](BENCHMARKS.md).

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -am 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🎯 Use Cases

**nqjson is perfect for:**

- 🚀 **High-throughput APIs** - Zero allocations = no GC pressure
- 📊 **Data Processing Pipelines** - Advanced modifiers for transformations
- 🔥 **Real-time Systems** - Predictable latency without GC pauses
- 📱 **Microservices** - Lightweight with zero dependencies
- 🎮 **Gaming Backends** - Performance-critical JSON operations
- 📈 **Analytics Systems** - Statistical aggregations built-in
- 🔍 **Log Processing** - Native JSON Lines support

## 🌟 Why Choose nqjson?

1. **Zero Allocations** - No memory overhead on hot paths
2. **Advanced Features** - Multipath, aggregations, and more
3. **Production Ready** - Battle-tested with high test coverage
4. **Developer Friendly** - Intuitive API with comprehensive docs
5. **Type Safe** - Automatic type conversion with validation
6. **Zero Dependencies** - Minimal attack surface, easy deployment

## 🙏 Acknowledgments

Built with performance, memory efficiency, and developer experience as primary goals. Optimized for modern Go applications and microservices architecture.
