# njson Path Expression Syntax

This document provides a comprehensive guide to all path expression syntaxes supported by njson for both GET and SET operations.

## Table of Contents

- [Basic Syntax](#basic-syntax)
- [Array Operations](#array-operations)
- [Advanced Expressions](#advanced-expressions)
- [Filter Expressions](#filter-expressions)
- [Wildcard Patterns](#wildcard-patterns)
- [Modifiers](#modifiers)
- [SET Operation Syntax](#set-operation-syntax)
- [Path Compilation](#path-compilation)
- [Syntax Reference](#syntax-reference)

## Basic Syntax

### Dot Notation

The fundamental way to navigate JSON objects is through dot notation:

```go
// Basic object access
path := "user.name"           // Gets the "name" field from "user" object
path := "user.address.city"   // Deep nested access
path := "config.database.host" // Multi-level nesting
```

**Example:**

```json
{
  "user": {
    "name": "Alice",
    "address": {
      "city": "New York",
      "zip": "10001"
    }
  }
}
```

- `user.name` → `"Alice"`
- `user.address.city` → `"New York"`
- `user.address.zip` → `"10001"`

### Root Access

```go
path := ""               // Returns the entire JSON document
path := "."              // Also returns the entire JSON document
```

## Array Operations

### Array Indexing

njson supports multiple ways to access array elements:

#### Dot Notation with Index

```go
path := "items.0"        // First element (index 0)
path := "items.1"        // Second element (index 1)
path := "users.2.name"   // Name field of the third user
```

#### Bracket Notation

```go
path := "items[0]"       // First element (equivalent to items.0)
path := "items[1]"       // Second element
path := "users[2].name"  // Name field of the third user
```

#### Mixed Notation

```go
path := "items.0.tags[1]"     // Second tag of first item
path := "users[0].phones.1"   // Second phone of first user
```

### Special Array Indices

#### Last Element (SET only)

```go
path := "items.-1"       // For SET operations, appends to the end of array
```

#### All Elements

```go
path := "items.#"        // Gets all elements as an array
path := "users.#.name"   // Gets the "name" field from all users
```

**Example:**

```json
{
  "users": [
    {"name": "Alice", "age": 30},
    {"name": "Bob", "age": 25},
    {"name": "Charlie", "age": 35}
  ]
}
```

- `users.0.name` → `"Alice"`
- `users[1].age` → `25`
- `users.#.name` → `["Alice", "Bob", "Charlie"]`
- `users.#` → `[{"name": "Alice", "age": 30}, {"name": "Bob", "age": 25}, {"name": "Charlie", "age": 35}]`

## Advanced Expressions

### Complex Array Access

```go
path := "items[500].metadata.priority"  // Deep array access with high index
path := "data.results[0].nested[2].id"  // Multiple nested arrays
```

### Nested Object and Array Combinations

```go
path := "store.books.0.authors[1].name"    // Author name from first book
path := "api.endpoints[3].params.0.type"   // Parameter type from fourth endpoint
```

## Filter Expressions

njson supports JSONPath-style filter expressions for conditional data retrieval:

### Basic Filter Syntax

```go
path := "items[?(@.key==value)]"         // Filter where field equals value
path := "users[?(@.age>30)]"             // Filter where age is greater than 30
path := "products[?(@.price<=100)]"      // Filter where price is less than or equal to 100
```

### String Equality Filters

```go
path := `items[?(@.type=="work")]`       // Filter where type equals "work"
path := `users[?(@.status=="active")]`   // Filter where status equals "active"
path := `phones[?(@.type=="mobile")]`    // Filter where type equals "mobile"
```

### Numeric Comparison Filters

```go
path := "items[?(@.price>10)]"           // Price greater than 10
path := "users[?(@.age>=18)]"            // Age 18 or older
path := "products[?(@.rating<3)]"        // Rating less than 3
path := "orders[?(@.total<=500)]"        // Total 500 or less
```

### Inequality Filters

```go
path := `items[?(@.status!="archived")]` // Status not equal to "archived"
path := `users[?(@.type!="admin")]`      // Type not equal to "admin"
```

### Field Extraction with Filters

```go
path := `phones[?(@.type=="work")].number`     // Get numbers of work phones
path := `items[?(@.priority>3)].name`          // Get names of high-priority items
path := `users[?(@.active==true)].email`      // Get emails of active users
```

### Supported Filter Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `==` | Equals | `[?(@.type=="mobile")]` |
| `!=` | Not equals | `[?(@.status!="inactive")]` |
| `>` | Greater than | `[?(@.age>21)]` |
| `>=` | Greater than or equal | `[?(@.score>=80)]` |
| `<` | Less than | `[?(@.price<100)]` |
| `<=` | Less than or equal | `[?(@.rating<=3)]` |
| `=~` | Regular expression match | `[?(@.email=~".*@company.com")]` |

**Example:**

```json
{
  "employees": [
    {"name": "Alice", "age": 30, "department": "engineering", "active": true},
    {"name": "Bob", "age": 25, "department": "marketing", "active": false},
    {"name": "Charlie", "age": 35, "department": "engineering", "active": true}
  ]
}
```

- `employees[?(@.active==true)].name` → `["Alice", "Charlie"]`
- `employees[?(@.age>28)].department` → `["engineering", "engineering"]`
- `employees[?(@.department=="engineering")].age` → `[30, 35]`

## Wildcard Patterns

### Object Wildcard

```go
path := "*.name"              // Get "name" from all top-level objects
path := "user.*.email"        // Get "email" from all fields under "user"
path := "data.*.status"       // Get "status" from all objects in "data"
```

### Array Wildcard

```go
path := "items.*"             // Get all values from items array
path := "users.*.preferences" // Get preferences from all users
```

### Combined Wildcards

```go
path := "*.phones.*.type"     // Get all phone types from all users
```

**Example:**

```json
{
  "users": {
    "alice": {"email": "alice@example.com", "active": true},
    "bob": {"email": "bob@example.com", "active": false},
    "charlie": {"email": "charlie@example.com", "active": true}
  }
}
```

- `users.*.email` → `["alice@example.com", "bob@example.com", "charlie@example.com"]`
- `users.*.active` → `[true, false, true]`

## Modifiers

Modifiers can be applied to path expressions to transform results:

### Available Modifiers

```go
path := "items@reverse"       // Reverse array order
path := "users@sort"          // Sort array elements
path := "data@group"          // Group array elements
```

### Combined with Paths

```go
path := "users.#.name@sort"   // Get all names and sort them
path := "items.#@reverse"     // Get all items in reverse order
```

## SET Operation Syntax

All GET syntax patterns are supported for SET operations, with additional considerations:

### Basic SET Operations

```go
// Set simple values
njson.Set(json, "user.name", "Alice")
njson.Set(json, "config.port", 8080)
njson.Set(json, "settings.enabled", true)
```

### Array SET Operations

```go
// Set array elements
njson.Set(json, "items.0", "first item")
njson.Set(json, "items[1]", "second item")
njson.Set(json, "users.2.name", "Charlie")

// Append to array (using -1 index)
njson.Set(json, "items.-1", "new item")
```

### Path Creation

SET operations can create missing paths automatically:

```go
// Creates nested structure if it doesn't exist
njson.Set(json, "user.profile.settings.theme", "dark")
// Result: {"user": {"profile": {"settings": {"theme": "dark"}}}}
```

### SET with Filters (Limited Support)

```go
// Note: Complex filter expressions in SET operations may have limitations
// Basic replacement works, but creation through filters is not supported
njson.Set(json, "users[?(@.id==123)].status", "active")
```

## Path Compilation

For repeated operations, paths can be pre-compiled for better performance:

```go
// Compile once
compiledPath, err := njson.CompileSetPath("users.0.profile.settings")
if err != nil {
    return err
}

// Use multiple times
result1, err := njson.SetWithCompiledPath(json1, compiledPath, value1, nil)
result2, err := njson.SetWithCompiledPath(json2, compiledPath, value2, nil)
```

### Valid Path Characters

Path segments can contain:

- Letters: `a-z`, `A-Z`
- Numbers: `0-9`
- Special characters: `_`, `-`

Invalid characters will result in compilation errors.

## Syntax Reference

### Quick Reference Table

| Pattern | Description | Example | GET | SET |
|---------|-------------|---------|-----|-----|
| `key` | Simple key access | `name` | ✅ | ✅ |
| `key.subkey` | Nested object access | `user.name` | ✅ | ✅ |
| `array.0` | Array index (dot notation) | `items.0` | ✅ | ✅ |
| `array[0]` | Array index (bracket notation) | `items[0]` | ✅ | ✅ |
| `array.-1` | Last element / append | `items.-1` | ❌ | ✅ |
| `array.#` | All array elements | `items.#` | ✅ | ❌ |
| `array.#.key` | Key from all elements | `users.#.name` | ✅ | ❌ |
| `*.key` | Wildcard object access | `*.name` | ✅ | ❌ |
| `array[?(@.key==value)]` | Filter by equality | `users[?(@.active==true)]` | ✅ | Limited |
| `array[?(@.key>value)]` | Filter by comparison | `items[?(@.price>10)]` | ✅ | Limited |
| `path@modifier` | Apply modifier | `items@reverse` | ✅ | ❌ |

### Complex Example

```json
{
  "store": {
    "books": [
      {
        "title": "Go Programming",
        "price": 29.99,
        "authors": ["John Doe", "Jane Smith"],
        "metadata": {
          "isbn": "978-0123456789",
          "category": "programming",
          "tags": ["go", "programming", "backend"]
        }
      },
      {
        "title": "Web Design",
        "price": 19.99,
        "authors": ["Alice Johnson"],
        "metadata": {
          "isbn": "978-0987654321",
          "category": "design",
          "tags": ["html", "css", "frontend"]
        }
      }
    ]
  }
}
```

**Path Examples:**

- `store.books.0.title` → `"Go Programming"`
- `store.books[1].price` → `19.99`
- `store.books.#.title` → `["Go Programming", "Web Design"]`
- `store.books[?(@.price>25)].title` → `["Go Programming"]`
- `store.books.0.authors[0]` → `"John Doe"`
- `store.books.#.metadata.category` → `["programming", "design"]`
- `store.books[?(@.metadata.category=="programming")].authors.0` → `["John Doe"]`

## Error Handling

### Common Syntax Errors

1. **Invalid Path Characters:**

   ```go
   "user.name@" // Invalid character @
   "user.na me" // Spaces not allowed
   ```

2. **Malformed Array Access:**

   ```go
   "items[abc]" // Non-numeric index
   "items["     // Missing closing bracket
   ```

3. **Invalid Filter Syntax:**

   ```go
   "items[?(@.price)]"          // Missing operator
   "items[?(@.price==\"10\")]"  // Missing closing )
   ```

### Performance Considerations

1. **Simple Paths:** Use basic dot notation when possible for best performance
2. **Compiled Paths:** Pre-compile paths for repeated operations
3. **Filter Expressions:** Use filters sparingly for large datasets
4. **Wildcard Operations:** Consider performance impact on large objects

## Best Practices

1. **Use specific paths when possible** instead of wildcards for better performance
2. **Compile paths for repeated operations** to avoid parsing overhead
3. **Validate paths early** in your application to catch syntax errors
4. **Consider using batch operations** for multiple SET operations on the same document
5. **Test complex filter expressions** thoroughly to ensure they work as expected

This comprehensive syntax guide covers all supported path expressions in njson. For additional examples and use cases, refer to the [EXAMPLES.md](EXAMPLES.md) and [API.md](API.md) documentation.
