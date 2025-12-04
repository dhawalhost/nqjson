# nqjson Path Expression Syntax

This document provides a comprehensive guide to all path expression syntaxes supported by nqjson for both GET and SET operations.

## Table of Contents

- [Basic Syntax](#basic-syntax)
- [Array Operations](#array-operations)
- [Advanced Expressions](#advanced-expressions)
- [Query Syntax](#query-syntax)
- [Filter Expressions](#filter-expressions)
- [Wildcard Patterns](#wildcard-patterns)
- [Modifiers](#modifiers)
- [JSON Lines Support](#json-lines-support)
- [Escape Sequences](#escape-sequences)
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

nqjson supports multiple ways to access array elements:

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

#### Appending to Arrays (SET only)

You can append values to the end of an array using the `-1` index:

```go
// Using dot notation
path := "items.-1"       // Appends to the end of the "items" array

// Using bracket notation  
path := "items[-1]"      // Equivalent to items.-1

// Nested arrays
path := "users.0.tags.-1"    // Appends to the tags array of the first user
path := "data.results[-1]"   // Appends to the results array
```

**Example:**

```go
json := `{"items":[1,2,3]}`

// Append using dot notation
result, _ := nqjson.Set([]byte(json), "items.-1", 4)
// Result: {"items":[1,2,3,4]}

// Append using bracket notation
result, _ := nqjson.Set([]byte(json), "items[-1]", 5)
// Result: {"items":[1,2,3,4,5]}

// Append object to array
json2 := `{"users":[{"name":"Alice"}]}`
result, _ := nqjson.Set([]byte(json2), "users.-1", map[string]interface{}{"name": "Bob"})
// Result: {"users":[{"name":"Alice"},{"name":"Bob"}]}
```

**Note:** The `-1` index only works for SET operations to append values. For GET operations, use standard indexing to access the last element (e.g., calculate the index based on array length).

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

## Query Syntax

nqjson supports powerful query syntax for conditional array element selection:

### First Match Query `#(condition)`

Returns the first array element matching the condition:

```go
path := "friends.#(first==\"Dale\")"       // First friend where first equals "Dale"
path := "friends.#(age>40)"                // First friend older than 40
path := "items.#(active==true)"            // First active item
```

### All Matches Query `#(condition)#`

Returns all array elements matching the condition:

```go
path := "friends.#(age>35)#"               // All friends older than 35
path := "friends.#(nets.#(==\"fb\")#)#"    // Friends with "fb" in their nets array
path := "items.#(status==\"active\")#"     // All active items
```

### Field Access After Query

Access specific fields from query results:

```go
path := "friends.#(age>35)#.first"         // Names of all friends older than 35
path := "items.#(active==true).name"       // Name of first active item
```

### Query Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `==` | Equals (strings/numbers) | `#(name=="John")` |
| `!=` | Not equals | `#(status!="inactive")` |
| `<` | Less than | `#(age<30)` |
| `<=` | Less than or equal | `#(price<=100)` |
| `>` | Greater than | `#(score>90)` |
| `>=` | Greater than or equal | `#(rating>=4)` |
| `%` | Pattern match (wildcard) | `#(name%"J*")` |
| `!%` | Negated pattern match | `#(name!%"Admin*")` |

### Pattern Matching in Queries

Use `%` for wildcard pattern matching:

```go
path := "friends.#(first%\"D*\").last"     // Last name of friends whose first starts with D
path := "friends.#(first!%\"D*\").last"    // Last name of friends whose first doesn't start with D
path := "items.#(name%\"*test*\")#"        // All items with "test" in name
```

### Direct Value Queries

Query arrays of primitive values directly:

```go
path := "tags.#(==\"featured\")"           // Find "featured" in tags array
path := "scores.#(>90)#"                   // All scores greater than 90
```

**Example:**

```json
{
  "friends": [
    {"first": "Dale", "last": "Murphy", "age": 44},
    {"first": "Roger", "last": "Craig", "age": 68},
    {"first": "Jane", "last": "Murphy", "age": 47}
  ]
}
```

- `friends.#(first=="Dale").last` → `"Murphy"`
- `friends.#(age>45)#.first` → `["Roger", "Jane"]`
- `friends.#(last%"Mur*")#.first` → `["Dale", "Jane"]`
- `friends.#` → `3` (array length)

## Filter Expressions

nqjson supports JSONPath-style filter expressions for conditional data retrieval:

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

nqjson supports two types of wildcards for flexible path matching:

### Multi-Character Wildcard `*`

Matches any number of characters (including zero):

```go
path := "child*.first"        // Match "children", "child1", etc.
path := "c?ild.first"         // Match any single character: "child", "chald", etc.
path := "*.name"              // Get "name" from all top-level keys
path := "user.*.email"        // Get "email" from all fields under "user"
```

### Single-Character Wildcard `?`

Matches exactly one character:

```go
path := "item?"               // Matches "item1", "items", etc.
path := "user?.name"          // Matches "user1.name", "userA.name", etc.
path := "c?t"                 // Matches "cat", "cut", "cot", etc.
```

### Array Wildcards

```go
path := "items.*"             // Get all values from items array
path := "items.#"             // Get all elements (alternative)
path := "users.*.preferences" // Get preferences from all users
```

### Combined Wildcards

```go
path := "*.phones.*.type"     // Get all phone types from all users
path := "user*.data.item?"    // Complex nested wildcard patterns
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

Modifiers transform results using the `@` prefix and pipe `|` syntax:

### Basic Modifier Syntax

```go
path := "items|@reverse"      // Reverse array order
path := "users|@sort"         // Sort array elements
path := "data|@flatten"       // Flatten nested arrays
```

### Available Modifiers

#### Array Transformation Modifiers

| Modifier | Description | Example |
|----------|-------------|---------|
| `@reverse` | Reverse array order | `items\|@reverse` |
| `@sort` | Sort array (ascending) | `scores\|@sort` |
| `@flatten` | Flatten nested arrays | `nested\|@flatten` |
| `@distinct` / `@unique` | Remove duplicates | `tags\|@distinct` |
| `@keys` | Get object keys as array | `user\|@keys` |
| `@values` | Get object values as array | `user\|@values` |
| `@first` | Get first element | `items\|@first` |
| `@last` | Get last element | `items\|@last` |

#### Aggregate Modifiers

| Modifier | Description | Example |
|----------|-------------|---------|
| `@sum` | Sum of numeric array | `prices\|@sum` |
| `@avg` / `@average` / `@mean` | Average of numeric array | `scores\|@avg` |
| `@min` | Minimum value | `values\|@min` |
| `@max` | Maximum value | `values\|@max` |
| `@count` / `@length` / `@len` | Array length | `items\|@count` |

#### Format Modifiers

| Modifier | Description | Example |
|----------|-------------|---------|
| `@pretty` | Pretty print JSON | `data\|@pretty` |
| `@pretty:{"indent":"\t"}` | Pretty print with custom indent | `data\|@pretty:{"indent":"\t"}` |
| `@ugly` | Minify JSON | `data\|@ugly` |
| `@valid` | Validate JSON (returns if valid) | `data\|@valid` |
| `@this` | Return current value unchanged | `@this` |

#### Type Conversion Modifiers

| Modifier | Description | Example |
|----------|-------------|---------|
| `@string` / `@str` | Convert to string | `value\|@string` |
| `@number` / `@num` | Convert to number | `value\|@number` |
| `@bool` / `@boolean` | Convert to boolean | `value\|@bool` |
| `@base64` | Base64 encode | `data\|@base64` |
| `@base64decode` | Base64 decode | `data\|@base64decode` |
| `@lower` | Convert to lowercase | `name\|@lower` |
| `@upper` | Convert to uppercase | `name\|@upper` |
| `@type` | Get JSON type as string | `value\|@type` |
| `@join` / `@join:","` | Join array to string | `tags\|@join` |

### Modifier Chaining

Chain multiple modifiers together:

```go
path := "items|@reverse|@first"           // Last item (reversed, then first)
path := "children|@reverse|0"             // First element after reversing
path := "scores|@sort|@reverse"           // Sort descending
path := "tags|@distinct|@sort"            // Unique sorted tags
path := "values|@flatten|@sum"            // Flatten then sum
```

### Modifiers with Path Continuation

Apply modifiers and continue with path access:

```go
path := "children|@reverse|0"             // First element of reversed array
path := "data|@sort|0.name"               // Name of first sorted item
```

### Standalone Modifiers

Use modifiers at the start of path:

```go
path := "@this"                           // Return entire JSON unchanged
path := "@valid"                          // Validate and return JSON
path := "@pretty"                         // Pretty print entire JSON
path := "@ugly"                           // Minify entire JSON
```

**Example:**

```json
{
  "children": ["Sara", "Alex", "Jack"],
  "scores": [85, 92, 78, 95, 88]
}
```

- `children|@reverse` → `["Jack", "Alex", "Sara"]`
- `children|@reverse|0` → `"Jack"`
- `scores|@sort` → `[78, 85, 88, 92, 95]`
- `scores|@sum` → `438`
- `scores|@avg` → `87.6`
- `scores|@min` → `78`
- `scores|@max` → `95`

## JSON Lines Support

nqjson supports JSON Lines (newline-delimited JSON) with the `..` prefix:

### Count JSON Lines

```go
path := "..#"                  // Count number of JSON lines
```

### Access JSON Lines

```go
path := "..0"                  // First JSON line
path := "..1"                  // Second JSON line
path := "..-1"                 // Last JSON line
```

### Query JSON Lines

```go
path := "..#.name"             // Get "name" from all JSON lines
path := "..#(active==true)#"   // All active records across lines
```

**Example:**

```
{"name": "Alice", "age": 30}
{"name": "Bob", "age": 25}
{"name": "Charlie", "age": 35}
```

- `..#` → `3` (count of lines)
- `..0.name` → `"Alice"`
- `..#.name` → `["Alice", "Bob", "Charlie"]`

## Escape Sequences

### Escaping Special Characters

Use backslash to escape special characters in key names:

| Escape | Character | Description |
|--------|-----------|-------------|
| `\.` | `.` | Literal dot in key name |
| `\:` | `:` | Literal colon in key name |
| `\\` | `\` | Literal backslash |

### Examples

```go
// Key with dot: {"fav.movie": "Inception"}
path := `fav\.movie`                      // Access "fav.movie" key

// Key with colon: {"user:name": "John"}
path := `user\:name`                      // Access "user:name" key

// Nested with escapes: {"a.b": {"c:d": "value"}}
path := `a\.b.c\:d`                       // Access nested keys with special chars
```

### Colon Prefix for Literal Numeric Keys

Use `:` prefix to treat numeric strings as object keys instead of array indices:

```go
// Object: {"123": "value"}
path := `:123`                            // Access "123" as object key (not array index)

// Nested: {"users": {"456": {"name": "John"}}}
path := `users.:456.name`                 // Access numeric key in nested object
```

**Example:**

```json
{
  "fav.movie": "Inception",
  "user:config": {"theme": "dark"},
  "data": {"123": "numeric key value"}
}
```

- `fav\.movie` → `"Inception"`
- `user\:config.theme` → `"dark"`
- `data.:123` → `"numeric key value"`

## SET Operation Syntax

All GET syntax patterns are supported for SET operations, with additional considerations:

### Basic SET Operations

```go
// Set simple values
nqjson.Set(json, "user.name", "Alice")
nqjson.Set(json, "config.port", 8080)
nqjson.Set(json, "settings.enabled", true)
```

### Array SET Operations

```go
// Set array elements
nqjson.Set(json, "items.0", "first item")
nqjson.Set(json, "items[1]", "second item")
nqjson.Set(json, "users.2.name", "Charlie")

// Append to array (using -1 index)
nqjson.Set(json, "items.-1", "new item")
```

### Path Creation

SET operations can create missing paths automatically:

```go
// Creates nested structure if it doesn't exist
nqjson.Set(json, "user.profile.settings.theme", "dark")
// Result: {"user": {"profile": {"settings": {"theme": "dark"}}}}
```

### SET with Filters (Limited Support)

```go
// Note: Complex filter expressions in SET operations may have limitations
// Basic replacement works, but creation through filters is not supported
nqjson.Set(json, "users[?(@.id==123)].status", "active")
```

## Path Compilation

For repeated operations, paths can be pre-compiled for better performance:

```go
// Compile once
compiledPath, err := nqjson.CompileSetPath("users.0.profile.settings")
if err != nil {
    return err
}

// Use multiple times
result1, err := nqjson.SetWithCompiledPath(json1, compiledPath, value1, nil)
result2, err := nqjson.SetWithCompiledPath(json2, compiledPath, value2, nil)
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
| `array.-1` | Append to array | `items.-1` | ❌ | ✅ |
| `array.#` | Array length | `items.#` | ✅ | ❌ |
| `array.#.key` | Key from all elements | `users.#.name` | ✅ | ❌ |
| `*` | Multi-character wildcard | `child*.name` | ✅ | ❌ |
| `?` | Single-character wildcard | `item?.value` | ✅ | ❌ |
| `#(condition)` | First match query | `#(age>30)` | ✅ | ❌ |
| `#(condition)#` | All matches query | `#(active==true)#` | ✅ | ❌ |
| `#(field%"pattern")` | Pattern match query | `#(name%"J*")` | ✅ | ❌ |
| `[?(@.key==value)]` | Filter by equality | `[?(@.active==true)]` | ✅ | Limited |
| `[?(@.key>value)]` | Filter by comparison | `[?(@.price>10)]` | ✅ | Limited |
| `path\|@modifier` | Apply modifier | `items\|@reverse` | ✅ | ❌ |
| `@this` | Return current value | `@this` | ✅ | ❌ |
| `@valid` | Validate JSON | `@valid` | ✅ | ❌ |
| `@pretty` | Pretty print JSON | `@pretty` | ✅ | ❌ |
| `@ugly` | Minify JSON | `@ugly` | ✅ | ❌ |
| `\.` | Escaped dot in key | `fav\.movie` | ✅ | ✅ |
| `\:` | Escaped colon in key | `user\:name` | ✅ | ✅ |
| `:123` | Literal numeric key | `:123` | ✅ | ✅ |
| `..#` | JSON Lines count | `..#` | ✅ | ❌ |
| `..0` | JSON Lines access | `..0.name` | ✅ | ❌ |

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
- `store.books.#` → `2` (array length)
- `store.books.#.title` → `["Go Programming", "Web Design"]`
- `store.books.#(price>25).title` → `"Go Programming"` (first match)
- `store.books.#(price>15)#.title` → `["Go Programming", "Web Design"]` (all matches)
- `store.books.0.authors[0]` → `"John Doe"`
- `store.books.#.metadata.category` → `["programming", "design"]`
- `store.books.#(metadata.category=="programming").title` → `"Go Programming"`
- `store.books.#.price|@sum` → `49.98`
- `store.books.#.price|@avg` → `24.99`
- `store.books.#.title|@reverse` → `["Web Design", "Go Programming"]`

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

This comprehensive syntax guide covers all supported path expressions in nqjson. For additional examples and use cases, refer to the [EXAMPLES.md](EXAMPLES.md) and [API.md](API.md) documentation.
