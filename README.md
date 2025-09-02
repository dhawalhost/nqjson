# NJSON

[![Go Report Card](https://goreportcard.com/badge/github.com/dhawalhost/njson)](https://goreportcard.com/report/github.com/dhawalhost/njson)

## 📖 Documentation

- **[Installation Guide](INSTALL.md)** - Step-by-step installation and usage guide
- **[API Reference](API.md)** - Complete API documentation with all methods and types
- **[Path Syntax Guide](SYNTAX.md)** - Comprehensive guide to all supported path expression syntaxes
- **[Examples](EXAMPLES.md)** - Comprehensive examples for all use cases
- **[Performance Benchmarks](BENCHMARKS.md)** - Detailed performance comparisons with other libraries
<!-- - **[Go Doc](https://godoc.org/github.com/dhawalhost/njson)** - Online API documentation -->
[![GoDoc](https://godoc.org/github.com/dhawalhost/njson?status.svg)](https://godoc.org/github.com/dhawalhost/njson)

**njson** is a next-generation JSON manipulation library for Go that prioritizes performance and memory efficiency. Built for modern applications that need blazing-fast JSON operations with minimal allocations.

## ⚡ Performance

njson outperforms popular JSON libraries in most benchmarks:

### GET Operations vs gjson

- **SimpleSmall**: 29.7ns vs 32.5ns (8% faster, 0 allocations)
- **SimpleMedium**: 581ns vs 684ns (15% faster, 0 allocations)
- **ComplexMedium**: 345ns vs 542ns (36% faster)
- **MultiPath**: 659ns vs 729ns (10% faster)
- **Filter**: 210ns vs 88,136ns (419x faster!)
- **Wildcard**: 222ns vs 263ns (16% faster)

### SET Operations vs sjson

- **SimpleSmall**: 85.7ns vs 107.9ns (21% faster, 47% less memory)
- **ArrayAppend**: 295ns vs 416ns (29% faster, 78% less memory)
- **NestedMedium**: 357ns vs 352ns (tied, 62% less memory)

## 🚀 Features

- **Ultra-fast JSON operations** with zero-allocation parsing
- **Memory efficient** - significantly fewer allocations than alternatives
- **Path-based access** - simple dot notation and complex path expressions
- **Batch operations** - process multiple paths in one call
- **Type-safe results** - automatic type conversion and validation
- **Pretty and compact JSON** support
- **Streaming operations** for large documents
- **Thread-safe** operations

## 📦 Installation

```bash
go get github.com/dhawalhost/njson
```

## � Documentation

- **[API Reference](API.md)** - Complete API documentation with all methods and types
- **[Examples](EXAMPLES.md)** - Comprehensive examples for all use cases
- **[Performance Benchmarks](BENCHMARKS.md)** - Detailed performance comparisons with other libraries
- **[Go Doc](https://godoc.org/github.com/dhawalhost/njson)** - Online API documentation

## �🔧 Quick Start

### Basic GET Operations

```go
package main

import (
    "fmt"
    "github.com/dhawalhost/njson"
)

func main() {
    json := `{
        "name": "John Doe",
        "age": 30,
        "skills": ["Go", "Python", "JavaScript"],
        "address": {
            "city": "New York",
            "coordinates": {
                "lat": 40.7128,
                "lng": -74.0060
            }
        }
    }`

    // Simple field access
    name := njson.Get([]byte(json), "name")
    fmt.Println("Name:", name.String()) // Name: John Doe

    // Nested field access
    city := njson.Get([]byte(json), "address.city")
    fmt.Println("City:", city.String()) // City: New York

    // Array access
    firstSkill := njson.Get([]byte(json), "skills.0")
    fmt.Println("First skill:", firstSkill.String()) // First skill: Go

    // Deep nested access
    lat := njson.Get([]byte(json), "address.coordinates.lat")
    fmt.Println("Latitude:", lat.Float()) // Latitude: 40.7128
}
```

### Basic SET Operations

```go
package main

import (
    "fmt"
    "github.com/dhawalhost/njson"
)

func main() {
    json := []byte(`{"name": "John", "age": 30}`)

    // Update existing field
    result, err := njson.Set(json, "age", 31)
    if err != nil {
        panic(err)
    }

    // Add new field
    result, err = njson.Set(result, "email", "john@example.com")
    if err != nil {
        panic(err)
    }

    // Add nested object
    result, err = njson.Set(result, "address.city", "Boston")
    if err != nil {
        panic(err)
    }

    fmt.Println(string(result))
    // Output: {"name":"John","age":31,"email":"john@example.com","address":{"city":"Boston"}}
}
```

## 📖 API Reference

### GET Operations

#### `Get(json []byte, path string) Result`

Retrieves a value from JSON using a path expression.

```go
json := []byte(`{"users": [{"name": "Alice"}, {"name": "Bob"}]}`)

// Get single value
name := njson.Get(json, "users.0.name")
fmt.Println(name.String()) // Alice

// Check if value exists
if name.Exists() {
    fmt.Println("User found")
}

// Type-safe conversions
age := njson.Get(json, "users.0.age")
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

results := njson.GetMany(json, "name", "age", "city")
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
result, err := njson.Set(json, "users.-1", map[string]interface{}{
    "name": "Alice",
    "age":  25,
})

// Update nested value
result, err = njson.Set(result, "users.0.active", true)
```

#### `SetWithOptions(json []byte, path string, value interface{}, options *SetOptions) ([]byte, error)`

Sets a value with advanced options.

```go
options := &njson.SetOptions{
    MergeObjects: true,
    MergeArrays:  false,
}

result, err := njson.SetWithOptions(json, "config", newConfig, options)
```

#### `Delete(json []byte, path string) ([]byte, error)`

Removes a value at the specified path.

```go
json := []byte(`{"name": "John", "age": 30, "temp": "delete_me"}`)

result, err := njson.Delete(json, "temp")
// Result: {"name": "John", "age": 30}
```

### Path Expressions

njson supports powerful path expressions:

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
firstBook := njson.Get(json, "store.books.0.title")

// Array filtering
expensiveBooks := njson.Get(json, "store.books.#(price>25).title")

// Wildcard matching
allPrices := njson.Get(json, "store.books.#.price")

// Complex expressions
programmingBooks := njson.Get(json, "store.books.#(tags.#(#==\"programming\")).title")
```

### Result Types

The `Result` type provides type-safe access to values:

```go
result := njson.Get(json, "some.path")

// Check existence
if result.Exists() {
    // Type conversion methods
    str := result.String()
    num := result.Float()
    integer := result.Int()
    boolean := result.Bool()
    
    // Get underlying type
    switch result.Type {
    case njson.TypeString:
        fmt.Println("String value:", result.String())
    case njson.TypeNumber:
        fmt.Println("Number value:", result.Float())
    case njson.TypeBool:
        fmt.Println("Boolean value:", result.Bool())
    case njson.TypeArray:
        fmt.Println("Array with", len(result.Array()), "elements")
    case njson.TypeObject:
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
userPath, _ := njson.CompileSetPath("users.-1")
configPath, _ := njson.CompileSetPath("config.theme")

// Use compiled paths
result, _ := njson.SetWithCompiledPath(json, userPath, newUser, nil)
result, _ = njson.SetWithCompiledPath(result, configPath, "dark", nil)
```

### Error Handling

```go
result, err := njson.Set(json, "invalid..path", value)
if err != nil {
    switch err {
    case njson.ErrInvalidPath:
        fmt.Println("Path syntax error")
    case njson.ErrInvalidJSON:
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
result := njson.Get(json, "important.field")

// Avoid string allocations when possible
if result.Type == njson.TypeString {
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

Run benchmarks locally:

```bash
cd benchmark
go test -bench=. -benchmem -benchtime=5s
```

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -am 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Inspired by [gjson](https://github.com/tidwall/gjson) and [sjson](https://github.com/tidwall/sjson)
- Built with performance and memory efficiency as primary goals
- Optimized for modern Go applications and microservices
