# njson API Documentation

Complete API reference for the njson library.

## Table of Contents

- [Core Types](#core-types)
- [GET Operations](#get-operations)
- [SET Operations](#set-operations)
- [DELETE Operations](#delete-operations)
- [Path Compilation](#path-compilation)
- [Options and Configuration](#options-and-configuration)
- [Error Types](#error-types)
- [Type Constants](#type-constants)

## Core Types

### Result

The `Result` type represents a value retrieved from JSON.

```go
type Result struct {
    Type  Type    // The type of the result
    Raw   string  // The raw JSON representation
    Str   string  // String value (for strings)
    Num   float64 // Numeric value (for numbers)
    Index int     // Array index (for array elements)
}
```

#### Methods

##### `Exists() bool`
Returns true if the result exists in the JSON.

```go
result := njson.Get(json, "user.name")
if result.Exists() {
    fmt.Println("Name found:", result.String())
}
```

##### `String() string`
Returns the string representation of the value.

```go
name := njson.Get(json, "user.name").String()
```

##### `Int() int64`
Returns the integer representation of the value.

```go
age := njson.Get(json, "user.age").Int()
```

##### `Uint() uint64`
Returns the unsigned integer representation of the value.

```go
id := njson.Get(json, "user.id").Uint()
```

##### `Float() float64`
Returns the floating-point representation of the value.

```go
price := njson.Get(json, "product.price").Float()
```

##### `Bool() bool`
Returns the boolean representation of the value.

```go
active := njson.Get(json, "user.active").Bool()
```

##### `Time() time.Time`
Parses the value as a time.Time using RFC3339 format.

```go
created := njson.Get(json, "user.createdAt").Time()
```

##### `Array() []Result`
Returns the value as an array of Results.

```go
items := njson.Get(json, "items")
if items.IsArray() {
    for _, item := range items.Array() {
        fmt.Println(item.String())
    }
}
```

##### `Map() map[string]Result`
Returns the value as a map of string keys to Results.

```go
user := njson.Get(json, "user")
if user.IsObject() {
    for key, value := range user.Map() {
        fmt.Printf("%s: %s\n", key, value.String())
    }
}
```

##### `IsObject() bool`
Returns true if the result is a JSON object.

```go
if result.IsObject() {
    // Handle as object
}
```

##### `IsArray() bool`
Returns true if the result is a JSON array.

```go
if result.IsArray() {
    // Handle as array
}
```

##### `ForEach(iterator func(key, value Result) bool)`
Iterates over arrays and objects.

```go
result.ForEach(func(key, value Result) bool {
    fmt.Printf("Key: %s, Value: %s\n", key.String(), value.String())
    return true // continue iteration
})
```

### Type

Enumeration of JSON value types.

```go
type Type int

const (
    TypeNull Type = iota
    TypeFalse
    TypeNumber
    TypeString
    TypeTrue
    TypeJSON
    TypeArray
    TypeObject
    TypeUndefined
)
```

## GET Operations

### `Get(json []byte, path string) Result`

Retrieves a single value from JSON using a path expression.

**Parameters:**
- `json []byte`: The JSON data to query
- `path string`: The path expression to the desired value

**Returns:**
- `Result`: The result containing the value and metadata

**Example:**
```go
json := []byte(`{"user": {"name": "Alice", "age": 30}}`)
name := njson.Get(json, "user.name")
fmt.Println(name.String()) // "Alice"
```

### `GetBytes(json []byte, path string) Result`

Alias for `Get()` for consistency.

### `GetMany(json []byte, paths ...string) []Result`

Retrieves multiple values from JSON in a single operation.

**Parameters:**
- `json []byte`: The JSON data to query
- `paths ...string`: Variable number of path expressions

**Returns:**
- `[]Result`: Slice of results corresponding to each path

**Example:**
```go
json := []byte(`{"user": {"name": "Alice", "age": 30, "city": "NYC"}}`)
results := njson.GetMany(json, "user.name", "user.age", "user.city")
for i, result := range results {
    fmt.Printf("Field %d: %s\n", i, result.String())
}
```

### `GetManyBytes(json []byte, paths ...string) []Result`

Alias for `GetMany()` for consistency.

## SET Operations

### `Set(json []byte, path string, value interface{}) ([]byte, error)`

Sets a value at the specified path in JSON.

**Parameters:**
- `json []byte`: The JSON data to modify
- `path string`: The path where to set the value
- `value interface{}`: The value to set (any JSON-serializable type)

**Returns:**
- `[]byte`: The modified JSON
- `error`: Error if the operation failed

**Example:**
```go
json := []byte(`{"user": {"name": "Alice"}}`)
result, err := njson.Set(json, "user.age", 30)
if err != nil {
    panic(err)
}
fmt.Println(string(result)) // {"user":{"name":"Alice","age":30}}
```

### `SetBytes(json []byte, path string, value interface{}) ([]byte, error)`

Alias for `Set()` for consistency.

### `SetWithOptions(json []byte, path string, value interface{}, options *SetOptions) ([]byte, error)`

Sets a value with advanced configuration options.

**Parameters:**
- `json []byte`: The JSON data to modify
- `path string`: The path where to set the value
- `value interface{}`: The value to set
- `options *SetOptions`: Configuration options (can be nil for defaults)

**Returns:**
- `[]byte`: The modified JSON
- `error`: Error if the operation failed

**Example:**
```go
options := &njson.SetOptions{
    MergeObjects: true,
    MergeArrays:  false,
}
result, err := njson.SetWithOptions(json, "user.preferences", newPrefs, options)
```

### `SetWithCompiledPath(json []byte, path *CompiledSetPath, value interface{}, options *SetOptions) ([]byte, error)`

Sets a value using a pre-compiled path for better performance.

**Parameters:**
- `json []byte`: The JSON data to modify
- `path *CompiledSetPath`: Pre-compiled path expression
- `value interface{}`: The value to set
- `options *SetOptions`: Configuration options (can be nil)

**Returns:**
- `[]byte`: The modified JSON
- `error`: Error if the operation failed

**Example:**
```go
compiledPath, err := njson.CompileSetPath("users.-1")
if err != nil {
    panic(err)
}

for _, user := range users {
    json, err = njson.SetWithCompiledPath(json, compiledPath, user, nil)
    if err != nil {
        panic(err)
    }
}
```

## DELETE Operations

### `Delete(json []byte, path string) ([]byte, error)`

Removes a value at the specified path from JSON.

**Parameters:**
- `json []byte`: The JSON data to modify
- `path string`: The path of the value to remove

**Returns:**
- `[]byte`: The modified JSON with the value removed
- `error`: Error if the operation failed

**Example:**
```go
json := []byte(`{"user": {"name": "Alice", "temp": "remove_me"}}`)
result, err := njson.Delete(json, "user.temp")
if err != nil {
    panic(err)
}
fmt.Println(string(result)) // {"user":{"name":"Alice"}}
```

### `DeleteBytes(json []byte, path string) ([]byte, error)`

Alias for `Delete()` for consistency.

### `DeleteWithOptions(json []byte, path string, options *SetOptions) ([]byte, error)`

Removes a value with advanced configuration options.

**Parameters:**
- `json []byte`: The JSON data to modify
- `path string`: The path of the value to remove
- `options *SetOptions`: Configuration options

**Returns:**
- `[]byte`: The modified JSON
- `error`: Error if the operation failed

## Path Compilation

### `CompileSetPath(path string) (*CompiledSetPath, error)`

Compiles a path expression for reuse in multiple SET operations.

**Parameters:**
- `path string`: The path expression to compile

**Returns:**
- `*CompiledSetPath`: Compiled path object
- `error`: Error if the path is invalid

**Example:**
```go
userPath, err := njson.CompileSetPath("users.-1")
if err != nil {
    panic(err)
}

// Reuse compiled path for better performance
for _, user := range users {
    json, err = njson.SetWithCompiledPath(json, userPath, user, nil)
    if err != nil {
        panic(err)
    }
}
```

### CompiledSetPath

Represents a compiled path expression.

```go
type CompiledSetPath struct {
    // Internal fields (implementation-specific)
}
```

## Options and Configuration

### SetOptions

Configuration options for SET and DELETE operations.

```go
type SetOptions struct {
    MergeObjects  bool // Whether to merge objects instead of replacing
    MergeArrays   bool // Whether to merge arrays instead of replacing
    ReplaceInPlace bool // Whether to attempt in-place replacement (advanced)
}
```

#### Default Options

```go
var DefaultSetOptions = SetOptions{
    MergeObjects:   false,
    MergeArrays:    false,
    ReplaceInPlace: false,
}
```

**Field Descriptions:**

- **MergeObjects**: When true, setting an object value will merge it with existing object instead of replacing it entirely
- **MergeArrays**: When true, setting an array value will merge it with existing array instead of replacing it entirely  
- **ReplaceInPlace**: Advanced option for performance optimization (use with caution)

**Example:**
```go
// Merge objects example
existing := []byte(`{"user": {"name": "Alice", "age": 30}}`)
newData := map[string]interface{}{
    "email": "alice@example.com",
    "age":   31,
}

options := &njson.SetOptions{MergeObjects: true}
result, err := njson.SetWithOptions(existing, "user", newData, options)
// Result: {"user":{"name":"Alice","age":31,"email":"alice@example.com"}}
```

## Error Types

### Standard Errors

```go
var (
    ErrInvalidPath = errors.New("invalid path")
    ErrInvalidJSON = errors.New("invalid json")
    ErrPathNotFound = errors.New("path not found")
    ErrTypeMismatch = errors.New("type mismatch")
)
```

### Error Handling

```go
result, err := njson.Set(json, "some.path", value)
if err != nil {
    switch err {
    case njson.ErrInvalidPath:
        // Handle path syntax errors
        fmt.Println("Invalid path syntax")
    case njson.ErrInvalidJSON:
        // Handle JSON parsing errors
        fmt.Println("Invalid JSON input")
    case njson.ErrPathNotFound:
        // Handle missing path errors
        fmt.Println("Path does not exist")
    default:
        // Handle other errors
        fmt.Printf("Operation failed: %v\n", err)
    }
}
```

## Type Constants

### JSON Type Constants

```go
const (
    TypeNull      Type = iota // JSON null
    TypeFalse                 // JSON false
    TypeNumber                // JSON number (integer or float)
    TypeString                // JSON string
    TypeTrue                  // JSON true
    TypeJSON                  // Raw JSON (complex values)
    TypeArray                 // JSON array
    TypeObject                // JSON object
    TypeUndefined             // Value does not exist
)
```

### Type Methods

```go
func (t Type) String() string
```

Returns the string representation of the type.

**Example:**
```go
result := njson.Get(json, "user.name")
fmt.Printf("Type: %s\n", result.Type.String()) // "String"
```

## Path Expression Reference

### Basic Dot Notation
- `user.name` - Access the `name` field of the `user` object
- `user.address.city` - Deep nested access

### Array Access
- `items.0` - First element of the `items` array
- `items.3.name` - The `name` field of the 4th element
- `items.-1` - Last element of the array (for SET operations, appends)

### Complex Expressions
- `items.#` - Get all elements of array
- `items.#.name` - Get the `name` field from all array elements
- `items.#(price>10)` - Filter array elements where price > 10
- `items.#(name="Alice")` - Filter elements where name equals "Alice"
- `items.#(tags.#(#=="urgent"))` - Complex nested filtering

### Wildcard Matching
- `*.name` - Get `name` from all top-level objects
- `user.*.email` - Get `email` from all fields under `user`

### Modifiers
- `@reverse` - Reverse array order
- `@sort` - Sort array elements
- `@group` - Group array elements

## Performance Notes

1. **Compiled Paths**: Use `CompileSetPath()` for repeated operations on the same path
2. **Batch Operations**: Use `GetMany()` to retrieve multiple values efficiently
3. **Memory Usage**: Use `Result.Raw` for zero-copy string access when possible
4. **Type Checking**: Check `Result.Exists()` before accessing values
5. **Error Handling**: Always check errors from SET/DELETE operations

## Thread Safety

All GET operations are thread-safe and can be called concurrently. SET and DELETE operations modify the input JSON and return new byte slices, so they are safe to use concurrently as long as each goroutine works with its own copy of the JSON data.

## Best Practices

1. **Check Existence**: Always check `result.Exists()` before using values
2. **Type Safety**: Use appropriate type conversion methods (`Int()`, `Float()`, etc.)
3. **Error Handling**: Handle all possible error conditions
4. **Performance**: Compile paths for repeated operations
5. **Memory**: Reuse byte slices when possible to reduce allocations
