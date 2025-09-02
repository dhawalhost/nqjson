# njson Installation and Usage Guide

Quick guide to get started with njson.

## Installation

### Using Go Modules (Recommended)

```bash
go mod init your-project
go get github.com/dhawalhost/njson
```

### Using go get

```bash
go get github.com/dhawalhost/njson
```

### Verify Installation

Create a simple test file:

```go
// test.go
package main

import (
    "fmt"
    "github.com/dhawalhost/njson"
)

func main() {
    json := []byte(`{"name": "Alice", "age": 30}`)
    name := njson.Get(json, "name")
    fmt.Printf("Name: %s\n", name.String())
}
```

Run it:
```bash
go run test.go
```

Expected output:
```
Name: Alice
```

## Basic Usage

### Reading JSON

```go
package main

import (
    "fmt"
    "github.com/dhawalhost/njson"
)

func main() {
    // Sample JSON data
    jsonData := []byte(`{
        "user": {
            "id": 123,
            "name": "John Doe",
            "email": "john@example.com",
            "active": true,
            "scores": [85, 92, 78, 96]
        }
    }`)

    // Get simple fields
    name := njson.Get(jsonData, "user.name")
    fmt.Println("Name:", name.String())

    // Get numbers
    id := njson.Get(jsonData, "user.id")
    fmt.Println("ID:", id.Int())

    // Get booleans
    active := njson.Get(jsonData, "user.active")
    fmt.Println("Active:", active.Bool())

    // Get array elements
    firstScore := njson.Get(jsonData, "user.scores.0")
    fmt.Println("First score:", firstScore.Int())

    // Get multiple values at once
    results := njson.GetMany(jsonData, "user.name", "user.email", "user.active")
    for i, result := range results {
        fmt.Printf("Field %d: %s\n", i, result.String())
    }
}
```

### Writing JSON

```go
package main

import (
    "fmt"
    "github.com/dhawalhost/njson"
)

func main() {
    // Start with some JSON
    jsonData := []byte(`{"user": {"name": "John"}}`)

    // Add a new field
    result, err := njson.Set(jsonData, "user.age", 30)
    if err != nil {
        panic(err)
    }

    // Add nested data
    result, err = njson.Set(result, "user.address.city", "New York")
    if err != nil {
        panic(err)
    }

    // Add to an array
    result, err = njson.Set(result, "user.hobbies", []string{"reading", "coding"})
    if err != nil {
        panic(err)
    }

    // Append to array
    result, err = njson.Set(result, "user.hobbies.-1", "gaming")
    if err != nil {
        panic(err)
    }

    fmt.Println(string(result))
}
```

### Error Handling

```go
package main

import (
    "fmt"
    "github.com/dhawalhost/njson"
)

func main() {
    jsonData := []byte(`{"user": {"name": "John"}}`)

    // Safe field access
    email := njson.Get(jsonData, "user.email")
    if email.Exists() {
        fmt.Println("Email:", email.String())
    } else {
        fmt.Println("Email not found")
    }

    // Safe type conversion
    age := njson.Get(jsonData, "user.age")
    if age.Exists() && age.Type == njson.TypeNumber {
        fmt.Println("Age:", age.Int())
    } else {
        fmt.Println("Age not available or not a number")
    }

    // Error handling for SET operations
    result, err := njson.Set(jsonData, "invalid..path", "value")
    if err != nil {
        fmt.Printf("Set operation failed: %v\n", err)
    } else {
        fmt.Println("Success:", string(result))
    }
}
```

## Common Use Cases

### Configuration Files

```go
// Reading configuration
config := njson.Get(configJSON, "database.host")
host := config.String()

port := njson.Get(configJSON, "database.port")
portNum := port.Int()

// Updating configuration
newConfig, err := njson.Set(configJSON, "database.maxConnections", 100)
```

### API Responses

```go
// Parse API response
response := []byte(`{
    "status": "success",
    "data": {
        "users": [
            {"id": 1, "name": "Alice"},
            {"id": 2, "name": "Bob"}
        ]
    }
}`)

// Extract data
status := njson.Get(response, "status").String()
users := njson.Get(response, "data.users")

if users.IsArray() {
    for i, user := range users.Array() {
        id := njson.Get([]byte(user.Raw), "id").Int()
        name := njson.Get([]byte(user.Raw), "name").String()
        fmt.Printf("User %d: %s (ID: %d)\n", i+1, name, id)
    }
}
```

### Building JSON

```go
// Start with empty object
json := []byte(`{}`)

// Build user profile
json, _ = njson.Set(json, "user.id", 123)
json, _ = njson.Set(json, "user.name", "Alice Johnson")
json, _ = njson.Set(json, "user.email", "alice@example.com")
json, _ = njson.Set(json, "user.preferences.theme", "dark")
json, _ = njson.Set(json, "user.preferences.notifications", true)

fmt.Println(string(json))
```

## Performance Tips

### For High Performance

```go
// 1. Compile paths for repeated use
userPath, _ := njson.CompileSetPath("users.-1")
for _, user := range users {
    json, _ = njson.SetWithCompiledPath(json, userPath, user, nil)
}

// 2. Use GetMany for multiple fields
results := njson.GetMany(json, "user.name", "user.email", "user.age")

// 3. Use Raw for zero-copy access
name := njson.Get(json, "user.name")
if name.Exists() {
    // Use name.Raw instead of name.String() to avoid allocation
    fmt.Printf("Name: %s\n", name.Raw)
}
```

### For Memory Efficiency

```go
// Process results immediately
users := njson.Get(json, "users")
if users.IsArray() {
    for _, user := range users.Array() {
        // Process immediately instead of storing
        processUser(user)
    }
}

// Reuse byte slices
var buffer []byte
for i := 0; i < count; i++ {
    buffer = buffer[:0] // Reset without reallocating
    // Use buffer for temporary operations
}
```

## Migration from Other Libraries

### From encoding/json

```go
// Before (encoding/json)
var data map[string]interface{}
json.Unmarshal(jsonBytes, &data)
name := data["user"].(map[string]interface{})["name"].(string)

// After (njson)
name := njson.Get(jsonBytes, "user.name").String()
```

### From gjson

```go
// Before (gjson)
import "github.com/tidwall/gjson"
result := gjson.GetBytes(jsonBytes, "user.name")
name := result.String()

// After (njson) - mostly compatible
import "github.com/dhawalhost/njson"
result := njson.Get(jsonBytes, "user.name")
name := result.String()
```

### From sjson

```go
// Before (sjson)
import "github.com/tidwall/sjson"
result, err := sjson.SetBytes(jsonBytes, "user.age", 30)

// After (njson) - mostly compatible
import "github.com/dhawalhost/njson"
result, err := njson.Set(jsonBytes, "user.age", 30)
```

## Troubleshooting

### Common Issues

1. **Empty Results**
   ```go
   result := njson.Get(json, "missing.field")
   if !result.Exists() {
       fmt.Println("Field not found")
   }
   ```

2. **Type Mismatches**
   ```go
   age := njson.Get(json, "user.age")
   if age.Type != njson.TypeNumber {
       fmt.Printf("Expected number, got %s\n", age.Type)
   }
   ```

3. **Invalid Paths**
   ```go
   result, err := njson.Set(json, "invalid..path", value)
   if err == njson.ErrInvalidPath {
       fmt.Println("Path syntax error")
   }
   ```

### Debug Tips

```go
// Print raw JSON for debugging
result := njson.Get(json, "complex.path")
fmt.Printf("Raw JSON: %s\n", result.Raw)
fmt.Printf("Type: %s\n", result.Type)
fmt.Printf("Exists: %t\n", result.Exists())
```

## Next Steps

- Read the [complete API documentation](API.md)
- Check out [comprehensive examples](EXAMPLES.md)
- See [performance benchmarks](BENCHMARKS.md)
- Browse the [Go Doc](https://godoc.org/github.com/dhawalhost/njson) online

## Getting Help

1. Check the documentation files in this repository
2. Look at the examples for similar use cases
3. Create an issue on GitHub if you find bugs
4. Read the source code for advanced usage patterns

Happy JSON processing with njson! 🚀
