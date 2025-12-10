# nqjson API Documentation

Complete API reference for the nqjson library.

## Table of Contents

- [Core Types](#core-types)
- [GET Operations](#get-operations)
- [SET Operations](#set-operations)
- [DELETE Operations](#delete-operations)
- [Path Escape Utilities](#path-escape-utilities)
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
result := nqjson.Get(json, "user.name")
if result.Exists() {
    fmt.Println("Name found:", result.String())
}
```

##### `String() string`
Returns the string representation of the value.

```go
name := nqjson.Get(json, "user.name").String()
```

##### `Int() int64`
Returns the integer representation of the value.

```go
age := nqjson.Get(json, "user.age").Int()
```

##### `Uint() uint64`
Returns the unsigned integer representation of the value.

```go
id := nqjson.Get(json, "user.id").Uint()
```

##### `Float() float64`
Returns the floating-point representation of the value.

```go
price := nqjson.Get(json, "product.price").Float()
```

##### `Bool() bool`
Returns the boolean representation of the value.

```go
active := nqjson.Get(json, "user.active").Bool()
```

##### `Time() time.Time`
Parses the value as a time.Time using RFC3339 format.

```go
created := nqjson.Get(json, "user.createdAt").Time()
```

##### `Array() []Result`
Returns the value as an array of Results.

```go
items := nqjson.Get(json, "items")
if items.IsArray() {
    for _, item := range items.Array() {
        fmt.Println(item.String())
    }
}
```

##### `Map() map[string]Result`
Returns the value as a map of string keys to Results.

```go
user := nqjson.Get(json, "user")
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
name := nqjson.Get(json, "user.name")
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
results := nqjson.GetMany(json, "user.name", "user.age", "user.city")
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
result, err := nqjson.Set(json, "user.age", 30)
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
options := &nqjson.SetOptions{
    MergeObjects: true,
    MergeArrays:  false,
}
result, err := nqjson.SetWithOptions(json, "user.preferences", newPrefs, options)
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
compiledPath, err := nqjson.CompileSetPath("users.-1")
if err != nil {
    panic(err)
}

for _, user := range users {
    json, err = nqjson.SetWithCompiledPath(json, compiledPath, user, nil)
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
result, err := nqjson.Delete(json, "user.temp")
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

## Path Escape Utilities

### `EscapePathSegment(segment string) string`

Escapes special characters in a path segment to create a literal key name. This function ensures that keys containing special path characters (like `.`, `@`, `*`, etc.) are properly escaped for use in path expressions.

**Parameters:**
- `segment string`: The path segment to escape

**Returns:**
- `string`: The escaped segment with special characters prefixed with backslash

**Escapes the following characters:**
- `\` (backslash) → `\\`
- `.` (dot) → `\.`
- `:` (colon) → `\:` (unless it's a leading colon prefix)
- `|` (pipe) → `\|`
- `@` (at) → `\@`
- `*` (asterisk) → `\*`
- `?` (question mark) → `\?`
- `#` (hash) → `\#`
- `,` (comma) → `\,`
- `(` (left parenthesis) → `\(`
- `)` (right parenthesis) → `\)`
- `=` (equals) → `\=`
- `!` (exclamation) → `\!`
- `<` (less than) → `\<`
- `>` (greater than) → `\>`
- `~` (tilde) → `\~`

**Note:** Leading colon prefix (`:`) used to force numeric keys as object properties is preserved.

**Example:**
```go
// Escape a key with special characters
key := "foo.bar@baz"
escaped := nqjson.EscapePathSegment(key)
fmt.Println(escaped) // "foo\\.bar\\@baz"

// Use in path expressions
json := []byte(`{"config": {"foo.bar@baz": "value"}}`)
result := nqjson.Get(json, "config."+escaped)
fmt.Println(result.String()) // "value"

// Numeric key as object property (colon prefix preserved)
escaped := nqjson.EscapePathSegment(":123")
fmt.Println(escaped) // ":123" (colon prefix not escaped)

// Unicode characters are preserved
escaped := nqjson.EscapePathSegment("aaa_æåø")
fmt.Println(escaped) // "aaa_æåø" (no escaping needed)
```

### `BuildEscapedPath(segments ...string) string`

Builds a complete path expression from multiple segments, automatically escaping special characters in each segment and joining them with dots.

**Parameters:**
- `segments ...string`: Variable number of path segments to escape and join

**Returns:**
- `string`: Complete path with all segments properly escaped and joined with `.`

**Example:**
```go
// Build path from multiple segments with special characters
path := nqjson.BuildEscapedPath("config", "foo.bar@baz", "*weird#key")
fmt.Println(path) // "config.foo\\.bar\\@baz.\\*weird\\#key"

// Use in Set/Get operations
json := []byte(`{}`)
json, _ = nqjson.Set(json, path, 42)
value := nqjson.Get(json, path)
fmt.Println(value.Int()) // 42

// Numeric object keys with colon prefix
path := nqjson.BuildEscapedPath("data", ":123", "value")
fmt.Println(path) // "data.:123.value"

// Unicode keys are preserved
path := nqjson.BuildEscapedPath("users", "aaa_æåø", "name")
fmt.Println(path) // "users.aaa_æåø.name"
```

**Use Cases:**
1. **Dynamic keys from user input**: Safely use user-provided strings as JSON keys
2. **Database field names**: Handle column names with special characters
3. **Configuration keys**: Work with config keys containing dots or other special chars
4. **API responses**: Navigate JSON with keys that contain path syntax characters
5. **Internationalized keys**: Preserve Unicode characters in multi-language data

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
userPath, err := nqjson.CompileSetPath("users.-1")
if err != nil {
    panic(err)
}

// Reuse compiled path for better performance
for _, user := range users {
    json, err = nqjson.SetWithCompiledPath(json, userPath, user, nil)
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

options := &nqjson.SetOptions{MergeObjects: true}
result, err := nqjson.SetWithOptions(existing, "user", newData, options)
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
result, err := nqjson.Set(json, "some.path", value)
if err != nil {
    switch err {
    case nqjson.ErrInvalidPath:
        // Handle path syntax errors
        fmt.Println("Invalid path syntax")
    case nqjson.ErrInvalidJSON:
        // Handle JSON parsing errors
        fmt.Println("Invalid JSON input")
    case nqjson.ErrPathNotFound:
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
result := nqjson.Get(json, "user.name")
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
- `items.#` - Array length (count of elements)
- `items.#.name` - Get the `name` field from all array elements
- `items.#(price>10)` - First element where price > 10
- `items.#(price>10)#` - All elements where price > 10
- `items.#(name=="Alice")` - First element where name equals "Alice"
- `items.#(name%"A*")#` - All elements where name matches pattern "A*"
- `items.#(tags.#(=="urgent"))` - Complex nested filtering

### Wildcard Matching
- `*.name` - Get `name` from all top-level objects
- `user.*.email` - Get `email` from all fields under `user`
- `child*.first` - Match keys starting with "child" (e.g., children, child1)
- `item?.value` - Match single character wildcard (e.g., item1, itemA)

### Modifiers

Use the pipe `|` syntax to apply modifiers to results:

#### Array Transformation Modifiers
- `items|@reverse` - Reverse array order
- `items|@sort` - Sort array ascending
- `items|@flatten` - Flatten nested arrays
- `items|@distinct` or `items|@unique` - Remove duplicates
- `items|@first` - Get first element
- `items|@last` - Get last element

#### Object Modifiers
- `user|@keys` - Get object keys as array
- `user|@values` - Get object values as array

#### Aggregate Modifiers (for numeric arrays)
- `prices|@sum` - Sum of all values
- `prices|@avg` or `@average` or `@mean` - Average of values
- `prices|@min` - Minimum value
- `prices|@max` - Maximum value
- `items|@count` or `@length` or `@len` - Count of elements

#### Format Modifiers
- `@this` - Return current value unchanged
- `@valid` - Validate JSON (returns if valid, empty if invalid)
- `@pretty` - Pretty print JSON with 2-space indent
- `@pretty:{"indent":"\t"}` - Pretty print with custom indent
- `@ugly` - Minify JSON (remove whitespace)

#### Type Conversion Modifiers
- `value|@string` or `@str` - Convert to string
- `value|@number` or `@num` - Convert to number
- `value|@bool` or `@boolean` - Convert to boolean
- `value|@base64` - Base64 encode
- `value|@base64decode` - Base64 decode
- `value|@lower` - Convert string to lowercase
- `value|@upper` - Convert string to uppercase
- `value|@type` - Get JSON type as string
- `tags|@join` or `@join:","` - Join array elements to string

#### Modifier Chaining
- `items|@sort|@reverse` - Sort descending (sort then reverse)
- `children|@reverse|0` - First element of reversed array
- `tags|@distinct|@sort` - Unique sorted values
- `nested|@flatten|@sum` - Flatten then sum

### Escape Sequences
- `fav\.movie` - Access key with literal dot: `{"fav.movie": "value"}`
- `user\:name` - Access key with literal colon: `{"user:name": "value"}`
- `:123` - Access numeric string as object key (not array index)
- `users.:456.name` - Nested numeric object keys

### JSON Lines Support
- `..#` - Count of JSON lines
- `..0` - First JSON line
- `..1` - Second JSON line
- `..#.name` - Get `name` from all JSON lines

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
