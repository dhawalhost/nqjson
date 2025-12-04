# nqjson Examples

This document provides comprehensive examples of using nqjson for various JSON manipulation tasks.

## Table of Contents

- [Basic Operations](#basic-operations)
- [Path Expressions](#path-expressions)
- [Type Handling](#type-handling)
- [Array Operations](#array-operations)
- [Object Operations](#object-operations)
- [Advanced Patterns](#advanced-patterns)
- [Query Syntax](#query-syntax)
- [Modifier Examples](#modifier-examples)
- [Escape Sequences](#escape-sequences)
- [JSON Lines Support](#json-lines-support)
- [Performance Optimizations](#performance-optimizations)
- [Error Handling](#error-handling)

## Basic Operations

### Simple Field Access

```go
package main

import (
    "fmt"
    "github.com/dhawalhost/nqjson"
)

func main() {
    json := `{
        "user": {
            "id": 123,
            "name": "Alice Johnson",
            "email": "alice@example.com",
            "active": true
        }
    }`

    // Get simple fields
    id := nqjson.Get([]byte(json), "user.id")
    name := nqjson.Get([]byte(json), "user.name")
    email := nqjson.Get([]byte(json), "user.email")
    active := nqjson.Get([]byte(json), "user.active")

    fmt.Printf("ID: %d\n", id.Int())           // ID: 123
    fmt.Printf("Name: %s\n", name.String())    // Name: Alice Johnson
    fmt.Printf("Email: %s\n", email.String())  // Email: alice@example.com
    fmt.Printf("Active: %t\n", active.Bool())  // Active: true
}
```

### Setting Values

```go
func updateUser() {
    json := []byte(`{"user": {"id": 123, "name": "Alice"}}`)

    // Update existing field
    result, err := nqjson.Set(json, "user.name", "Alice Johnson")
    if err != nil {
        panic(err)
    }

    // Add new field
    result, err = nqjson.Set(result, "user.email", "alice@example.com")
    if err != nil {
        panic(err)
    }

    // Add nested object
    result, err = nqjson.Set(result, "user.preferences.theme", "dark")
    if err != nil {
        panic(err)
    }

    fmt.Println(string(result))
    // Output: {"user":{"id":123,"name":"Alice Johnson","email":"alice@example.com","preferences":{"theme":"dark"}}}
}
```

## Path Expressions

### Dot Notation

```go
func dotNotationExamples() {
    json := `{
        "company": {
            "name": "TechCorp",
            "departments": {
                "engineering": {
                    "head": "John Doe",
                    "budget": 1000000
                }
            }
        }
    }`

    // Deep nested access
    head := nqjson.Get([]byte(json), "company.departments.engineering.head")
    budget := nqjson.Get([]byte(json), "company.departments.engineering.budget")

    fmt.Printf("Engineering Head: %s\n", head.String())
    fmt.Printf("Budget: $%.0f\n", budget.Float())
}
```

### Array Indexing

```go
func arrayIndexingExamples() {
    json := `{
        "products": [
            {"id": 1, "name": "Laptop", "price": 999.99},
            {"id": 2, "name": "Mouse", "price": 29.99},
            {"id": 3, "name": "Keyboard", "price": 79.99}
        ]
    }`

    // Access by index
    firstProduct := nqjson.Get([]byte(json), "products.0.name")
    lastProductPrice := nqjson.Get([]byte(json), "products.2.price")

    fmt.Printf("First Product: %s\n", firstProduct.String())
    fmt.Printf("Last Product Price: $%.2f\n", lastProductPrice.Float())

    // Get array length
    products := nqjson.Get([]byte(json), "products")
    if products.IsArray() {
        fmt.Printf("Total Products: %d\n", len(products.Array()))
    }
}
```

### Complex Path Expressions

```go
func complexPathExamples() {
    json := `{
        "store": {
            "books": [
                {
                    "title": "Go Programming",
                    "author": "John Doe",
                    "price": 29.99,
                    "categories": ["programming", "go", "backend"]
                },
                {
                    "title": "Web Design Basics",
                    "author": "Jane Smith",
                    "price": 19.99,
                    "categories": ["design", "web", "frontend"]
                },
                {
                    "title": "Advanced Go",
                    "author": "Bob Wilson",
                    "price": 39.99,
                    "categories": ["programming", "go", "advanced"]
                }
            ]
        }
    }`

    // Find books with specific category
    goBooks := nqjson.Get([]byte(json), "store.books.#(categories.#(#==\"go\")).title")
    fmt.Println("Go Books:", goBooks.String())

    // Find expensive books (price > 25)
    expensiveBooks := nqjson.Get([]byte(json), "store.books.#(price>25).title")
    fmt.Println("Expensive Books:", expensiveBooks.String())

    // Get all prices
    allPrices := nqjson.Get([]byte(json), "store.books.#.price")
    if allPrices.IsArray() {
        for _, price := range allPrices.Array() {
            fmt.Printf("Price: $%.2f\n", price.Float())
        }
    }
}
```

## Type Handling

### Type-Safe Access

```go
func typeSafeAccess() {
    json := `{
        "user": {
            "id": 123,
            "name": "Alice",
            "balance": 1234.56,
            "active": true,
            "tags": ["premium", "verified"],
            "metadata": {
                "lastLogin": "2023-01-15",
                "attempts": 5
            }
        }
    }`

    data := []byte(json)

    // Safe type conversion with existence check
    if id := nqjson.Get(data, "user.id"); id.Exists() {
        switch id.Type {
        case nqjson.TypeNumber:
            fmt.Printf("User ID: %d\n", id.Int())
        default:
            fmt.Printf("User ID (as string): %s\n", id.String())
        }
    }

    // Handle different number types
    balance := nqjson.Get(data, "user.balance")
    if balance.Exists() && balance.Type == nqjson.TypeNumber {
        fmt.Printf("Balance: $%.2f\n", balance.Float())
    }

    // Array handling
    tags := nqjson.Get(data, "user.tags")
    if tags.IsArray() {
        fmt.Print("Tags: ")
        for i, tag := range tags.Array() {
            if i > 0 {
                fmt.Print(", ")
            }
            fmt.Print(tag.String())
        }
        fmt.Println()
    }

    // Object handling
    metadata := nqjson.Get(data, "user.metadata")
    if metadata.Type == nqjson.TypeObject {
        metaMap := metadata.Map()
        for key, value := range metaMap {
            fmt.Printf("Metadata %s: %s\n", key, value.String())
        }
    }
}
```

### Default Values

```go
func defaultValueHandling() {
    json := `{"user": {"name": "Alice"}}`
    data := []byte(json)

    // Provide defaults for missing values
    name := nqjson.Get(data, "user.name")
    email := nqjson.Get(data, "user.email")
    age := nqjson.Get(data, "user.age")

    fmt.Printf("Name: %s\n", getStringOrDefault(name, "Unknown"))
    fmt.Printf("Email: %s\n", getStringOrDefault(email, "no-email@example.com"))
    fmt.Printf("Age: %d\n", getIntOrDefault(age, 0))
}

func getStringOrDefault(result nqjson.Result, defaultValue string) string {
    if result.Exists() {
        return result.String()
    }
    return defaultValue
}

func getIntOrDefault(result nqjson.Result, defaultValue int64) int64 {
    if result.Exists() {
        return result.Int()
    }
    return defaultValue
}
```

## Array Operations

### Appending to Arrays

nqjson supports appending elements to arrays using the `-1` index in both dot and bracket notation:

```go
func appendingToArrays() {
    // Starting with a simple array
    json := []byte(`{"items": [1, 2, 3]}`)
    
    // Append using dot notation
    result, err := nqjson.Set(json, "items.-1", 4)
    if err != nil {
        panic(err)
    }
    fmt.Println(string(result))  // {"items":[1,2,3,4]}
    
    // Append using bracket notation
    result, err = nqjson.Set(result, "items[-1]", 5)
    if err != nil {
        panic(err)
    }
    fmt.Println(string(result))  // {"items":[1,2,3,4,5]}
    
    // Append an object
    json2 := []byte(`{"users": [{"name": "Alice"}]}`)
    newUser := map[string]interface{}{
        "name": "Bob",
        "age":  30,
    }
    result, err = nqjson.Set(json2, "users.-1", newUser)
    if err != nil {
        panic(err)
    }
    fmt.Println(string(result))
    // {"users":[{"name":"Alice"},{"name":"Bob","age":30}]}
    
    // Append to nested arrays
    json3 := []byte(`{"groups": [{"members": ["Alice", "Bob"]}]}`)
    result, err = nqjson.Set(json3, "groups.0.members.-1", "Charlie")
    if err != nil {
        panic(err)
    }
    fmt.Println(string(result))
    // {"groups":[{"members":["Alice","Bob","Charlie"]}]}
    
    // Multiple appends
    json4 := []byte(`{"tags": []}`)
    tags := []string{"golang", "json", "performance"}
    for _, tag := range tags {
        json4, err = nqjson.Set(json4, "tags.-1", tag)
        if err != nil {
            panic(err)
        }
    }
    fmt.Println(string(json4))
    // {"tags":["golang","json","performance"]}
}
```

### Array Manipulation

```go
func arrayManipulation() {
    json := []byte(`{"items": [1, 2, 3]}`)

    // Replace specific element
    result, err := nqjson.Set(json, "items.1", 1.5)
    if err != nil {
        panic(err)
    }
    fmt.Println(string(result))  // {"items":[1,1.5,3]}

    // Expand array by setting high index (fills with nulls)
    result, err = nqjson.Set(json, "items.5", 6)
    if err != nil {
        panic(err)
    }
    fmt.Println(string(result))  // {"items":[1,2,3,null,null,6]}

    // Add object to array
    newItem := map[string]interface{}{
        "id":   5,
        "name": "New Item",
    }
    result, err = nqjson.Set(result, "items.-1", newItem)
    if err != nil {
        panic(err)
    }

    fmt.Println(string(result))
}
```

### Array Filtering and Processing

```go
func arrayProcessing() {
    json := `{
        "employees": [
            {"id": 1, "name": "Alice", "department": "Engineering", "salary": 90000},
            {"id": 2, "name": "Bob", "department": "Sales", "salary": 70000},
            {"id": 3, "name": "Carol", "department": "Engineering", "salary": 95000},
            {"id": 4, "name": "David", "department": "Marketing", "salary": 65000}
        ]
    }`

    data := []byte(json)

    // Get all engineering employees
    engineers := nqjson.Get(data, "employees.#(department==\"Engineering\").name")
    fmt.Println("Engineers:", engineers.String())

    // Get high-salary employees
    highEarners := nqjson.Get(data, "employees.#(salary>80000)")
    if highEarners.IsArray() {
        fmt.Println("High Earners:")
        for _, emp := range highEarners.Array() {
            name := nqjson.Get([]byte(emp.Raw), "name")
            salary := nqjson.Get([]byte(emp.Raw), "salary")
            fmt.Printf("  %s: $%.0f\n", name.String(), salary.Float())
        }
    }

    // Calculate average salary
    salaries := nqjson.Get(data, "employees.#.salary")
    if salaries.IsArray() {
        var total float64
        count := 0
        for _, salary := range salaries.Array() {
            total += salary.Float()
            count++
        }
        if count > 0 {
            fmt.Printf("Average Salary: $%.2f\n", total/float64(count))
        }
    }
}
```

## Object Operations

### Dynamic Object Building

```go
func dynamicObjectBuilding() {
    // Start with empty object
    json := []byte(`{}`)

    // Build user profile dynamically
    fields := map[string]interface{}{
        "user.id":                1,
        "user.name":              "Alice Johnson",
        "user.email":             "alice@example.com",
        "user.profile.bio":       "Software Engineer",
        "user.profile.location":  "San Francisco",
        "user.preferences.theme": "dark",
        "user.preferences.lang":  "en",
    }

    result := json
    for path, value := range fields {
        var err error
        result, err = nqjson.Set(result, path, value)
        if err != nil {
            panic(err)
        }
    }

    fmt.Println(string(result))
}
```

### Object Merging

```go
func objectMerging() {
    base := []byte(`{
        "user": {
            "id": 1,
            "name": "Alice",
            "preferences": {
                "theme": "light",
                "notifications": true
            }
        }
    }`)

    // Merge new preferences
    options := &nqjson.SetOptions{
        MergeObjects: true,
    }

    newPrefs := map[string]interface{}{
        "theme":    "dark",
        "language": "en",
        "timezone": "UTC",
    }

    result, err := nqjson.SetWithOptions(base, "user.preferences", newPrefs, options)
    if err != nil {
        panic(err)
    }

    fmt.Println(string(result))
    // Existing notifications setting is preserved, theme is updated, new fields are added
}
```

## Advanced Patterns

### Batch Processing

```go
func batchProcessing() {
    json := []byte(`{
        "users": [
            {"id": 1, "name": "Alice", "status": "active"},
            {"id": 2, "name": "Bob", "status": "inactive"},
            {"id": 3, "name": "Carol", "status": "active"}
        ]
    }`)

    // Get multiple fields efficiently
    results := nqjson.GetMany(json,
        "users.0.name",
        "users.1.name",
        "users.2.name",
    )

    fmt.Println("User names:")
    for i, result := range results {
        if result.Exists() {
            fmt.Printf("  User %d: %s\n", i+1, result.String())
        }
    }

    // Batch updates using compiled paths for performance
    nameTemplate, _ := nqjson.CompileSetPath("users.%d.lastLogin")
    
    result := json
    for i := 0; i < 3; i++ {
        path := fmt.Sprintf("users.%d.lastLogin", i)
        var err error
        result, err = nqjson.Set(result, path, "2023-01-15T10:00:00Z")
        if err != nil {
            panic(err)
        }
    }

    fmt.Println(string(result))
}
```

### Conditional Updates

```go
func conditionalUpdates() {
    json := []byte(`{
        "products": [
            {"id": 1, "name": "Laptop", "price": 999, "inStock": true},
            {"id": 2, "name": "Mouse", "price": 29, "inStock": false},
            {"id": 3, "name": "Keyboard", "price": 79, "inStock": true}
        ]
    }`)

    result := json

    // Apply discount to in-stock items
    products := nqjson.Get(json, "products")
    if products.IsArray() {
        for i, product := range products.Array() {
            inStock := nqjson.Get([]byte(product.Raw), "inStock")
            price := nqjson.Get([]byte(product.Raw), "price")
            
            if inStock.Bool() && price.Float() > 50 {
                // Apply 10% discount
                newPrice := price.Float() * 0.9
                path := fmt.Sprintf("products.%d.price", i)
                var err error
                result, err = nqjson.Set(result, path, newPrice)
                if err != nil {
                    panic(err)
                }
                
                // Add discount flag
                discountPath := fmt.Sprintf("products.%d.discounted", i)
                result, err = nqjson.Set(result, discountPath, true)
                if err != nil {
                    panic(err)
                }
            }
        }
    }

    fmt.Println(string(result))
}
```

### Data Transformation

```go
func dataTransformation() {
    // Transform nested structure to flat structure
    nested := []byte(`{
        "user": {
            "personal": {
                "firstName": "Alice",
                "lastName": "Johnson"
            },
            "contact": {
                "email": "alice@example.com",
                "phone": "+1-555-0123"
            }
        }
    }`)

    // Extract and flatten
    firstName := nqjson.Get(nested, "user.personal.firstName")
    lastName := nqjson.Get(nested, "user.personal.lastName")
    email := nqjson.Get(nested, "user.contact.email")
    phone := nqjson.Get(nested, "user.contact.phone")

    // Build flat structure
    flat := []byte(`{}`)
    flatData := map[string]string{
        "fullName": firstName.String() + " " + lastName.String(),
        "email":    email.String(),
        "phone":    phone.String(),
    }

    result := flat
    for key, value := range flatData {
        var err error
        result, err = nqjson.Set(result, key, value)
        if err != nil {
            panic(err)
        }
    }

    fmt.Println("Flattened:", string(result))
}
```

## Performance Optimizations

### Reusing Compiled Paths

```go
func optimizedOperations() {
    // Compile paths for reuse
    userPath, _ := nqjson.CompileSetPath("users.-1")
    statusPath, _ := nqjson.CompileSetPath("users.%d.status")

    json := []byte(`{"users": []}`)
    result := json

    // Add multiple users efficiently
    users := []map[string]interface{}{
        {"id": 1, "name": "Alice"},
        {"id": 2, "name": "Bob"},
        {"id": 3, "name": "Carol"},
    }

    for _, user := range users {
        var err error
        result, err = nqjson.SetWithCompiledPath(result, userPath, user, nil)
        if err != nil {
            panic(err)
        }
    }

    fmt.Println("Added users:", string(result))
}
```

### Memory-Efficient Processing

```go
func memoryEfficientProcessing() {
    json := []byte(`{"data": {"values": [1, 2, 3, 4, 5]}}`)

    // Use Raw for zero-copy string access
    values := nqjson.Get(json, "data.values")
    if values.IsArray() {
        fmt.Print("Values: ")
        for i, value := range values.Array() {
            if i > 0 {
                fmt.Print(", ")
            }
            // Use Raw to avoid string allocation
            fmt.Print(string(value.Raw))
        }
        fmt.Println()
    }

    // Reuse byte slices when possible
    var buffer []byte
    for i := 0; i < 5; i++ {
        path := fmt.Sprintf("data.newValue%d", i)
        var err error
        json, err = nqjson.Set(json, path, i*10)
        if err != nil {
            panic(err)
        }
    }

    fmt.Println("Updated:", string(json))
}
```

## Error Handling

### Comprehensive Error Handling

```go
func errorHandlingExamples() {
    json := []byte(`{"user": {"name": "Alice"}}`)

    // Handle different error types
    result, err := nqjson.Set(json, "invalid..path", "value")
    if err != nil {
        switch err {
        case nqjson.ErrInvalidPath:
            fmt.Println("Path syntax error:", err)
        case nqjson.ErrInvalidJSON:
            fmt.Println("JSON parsing error:", err)
        default:
            fmt.Println("Other error:", err)
        }
        return
    }

    // Validate results
    name := nqjson.Get(result, "user.name")
    if !name.Exists() {
        fmt.Println("Warning: Expected field not found")
        return
    }

    // Type validation
    if name.Type != nqjson.TypeString {
        fmt.Printf("Warning: Expected string, got %s\n", name.Type)
        return
    }

    fmt.Println("Success:", name.String())
}
```

### Safe Operations

```go
func safeOperations() {
    json := []byte(`{"users": [{"name": "Alice"}]}`)

    // Safe array access
    secondUser := nqjson.Get(json, "users.1.name")
    if secondUser.Exists() {
        fmt.Println("Second user:", secondUser.String())
    } else {
        fmt.Println("Second user not found")
    }

    // Safe type conversion
    age := nqjson.Get(json, "users.0.age")
    if age.Exists() {
        if age.Type == nqjson.TypeNumber {
            fmt.Printf("Age: %d\n", age.Int())
        } else {
            fmt.Printf("Age (as string): %s\n", age.String())
        }
    } else {
        fmt.Println("Age not specified")
    }

    // Safe object access
    profile := nqjson.Get(json, "users.0.profile")
    if profile.Exists() && profile.Type == nqjson.TypeObject {
        fmt.Println("Profile found:", string(profile.Raw))
    } else {
        fmt.Println("No profile information")
    }
}
```

## Query Syntax

### First Match Query #(condition)

```go
func queryFirstMatch() {
    json := []byte(`{
        "friends": [
            {"first": "Dale", "last": "Murphy", "age": 44},
            {"first": "Roger", "last": "Craig", "age": 68},
            {"first": "Jane", "last": "Murphy", "age": 47}
        ]
    }`)

    // Find first friend named Dale
    dale := nqjson.Get(json, `friends.#(first=="Dale").last`)
    fmt.Println("Dale's last name:", dale.String()) // Murphy

    // Find first friend older than 45
    older := nqjson.Get(json, `friends.#(age>45).first`)
    fmt.Println("First friend over 45:", older.String()) // Roger

    // Find first Murphy
    murphy := nqjson.Get(json, `friends.#(last=="Murphy").first`)
    fmt.Println("First Murphy:", murphy.String()) // Dale
}
```

### All Matches Query #(condition)#

```go
func queryAllMatches() {
    json := []byte(`{
        "friends": [
            {"first": "Dale", "last": "Murphy", "age": 44, "nets": ["ig", "fb"]},
            {"first": "Roger", "last": "Craig", "age": 68, "nets": ["fb", "tw"]},
            {"first": "Jane", "last": "Murphy", "age": 47, "nets": ["ig", "tw"]}
        ]
    }`)

    // Find all friends older than 45
    olderFriends := nqjson.Get(json, `friends.#(age>45)#.first`)
    fmt.Println("Friends over 45:", olderFriends.Raw) // ["Roger","Jane"]

    // Find all Murphys
    murphys := nqjson.Get(json, `friends.#(last=="Murphy")#.first`)
    fmt.Println("All Murphys:", murphys.Raw) // ["Dale","Jane"]

    // Find all friends with Facebook
    fbFriends := nqjson.Get(json, `friends.#(nets.#(=="fb"))#.first`)
    fmt.Println("Facebook friends:", fbFriends.Raw) // ["Dale","Roger"]
}
```

### Pattern Matching in Queries

```go
func queryPatternMatching() {
    json := []byte(`{
        "users": [
            {"name": "John Smith", "email": "john@company.com"},
            {"name": "Jane Doe", "email": "jane@external.org"},
            {"name": "Jack Wilson", "email": "jack@company.com"}
        ]
    }`)

    // Find users whose name starts with "J"
    jUsers := nqjson.Get(json, `users.#(name%"J*")#.email`)
    fmt.Println("J users:", jUsers.Raw) // All J users

    // Find users with company.com email
    companyUsers := nqjson.Get(json, `users.#(email%"*@company.com")#.name`)
    fmt.Println("Company users:", companyUsers.Raw) // ["John Smith","Jack Wilson"]

    // Find users NOT matching a pattern
    externalUsers := nqjson.Get(json, `users.#(email!%"*@company.com")#.name`)
    fmt.Println("External users:", externalUsers.Raw) // ["Jane Doe"]
}
```

## Modifier Examples

### Array Transformation Modifiers

```go
func arrayModifiers() {
    json := []byte(`{
        "children": ["Sara", "Alex", "Jack"],
        "scores": [85, 92, 78, 95, 88]
    }`)

    // Reverse array
    reversed := nqjson.Get(json, "children|@reverse")
    fmt.Println("Reversed:", reversed.Raw) // ["Jack","Alex","Sara"]

    // Sort array
    sorted := nqjson.Get(json, "scores|@sort")
    fmt.Println("Sorted:", sorted.Raw) // [78,85,88,92,95]

    // Sort descending (chain sort and reverse)
    sortedDesc := nqjson.Get(json, "scores|@sort|@reverse")
    fmt.Println("Sorted desc:", sortedDesc.Raw) // [95,92,88,85,78]

    // First and last
    first := nqjson.Get(json, "children|@first")
    last := nqjson.Get(json, "children|@last")
    fmt.Println("First:", first.String(), "Last:", last.String())
}
```

### Aggregate Modifiers

```go
func aggregateModifiers() {
    json := []byte(`{
        "prices": [19.99, 29.99, 39.99, 49.99],
        "quantities": [2, 5, 3, 1]
    }`)

    // Sum
    total := nqjson.Get(json, "prices|@sum")
    fmt.Printf("Total: $%.2f\n", total.Float()) // 139.96

    // Average
    avg := nqjson.Get(json, "prices|@avg")
    fmt.Printf("Average: $%.2f\n", avg.Float()) // 34.99

    // Min and Max
    min := nqjson.Get(json, "prices|@min")
    max := nqjson.Get(json, "prices|@max")
    fmt.Printf("Price range: $%.2f - $%.2f\n", min.Float(), max.Float())

    // Count
    count := nqjson.Get(json, "prices|@count")
    fmt.Printf("Number of prices: %d\n", count.Int()) // 4
}
```

### Object Modifiers

```go
func objectModifiers() {
    json := []byte(`{
        "user": {
            "name": "Alice",
            "age": 30,
            "email": "alice@example.com",
            "city": "NYC"
        }
    }`)

    // Get object keys
    keys := nqjson.Get(json, "user|@keys")
    fmt.Println("Keys:", keys.Raw) // ["name","age","email","city"]

    // Get object values
    values := nqjson.Get(json, "user|@values")
    fmt.Println("Values:", values.Raw) // ["Alice",30,"alice@example.com","NYC"]
}
```

### Format Modifiers

```go
func formatModifiers() {
    json := []byte(`{"name":"Alice","age":30,"city":"NYC"}`)

    // Pretty print
    pretty := nqjson.Get(json, "@pretty")
    fmt.Println("Pretty:", pretty.Raw)
    // {
    //   "name": "Alice",
    //   "age": 30,
    //   "city": "NYC"
    // }

    // Pretty with custom indent
    prettyTab := nqjson.Get(json, `@pretty:{"indent":"\t"}`)
    fmt.Println("Pretty with tabs:", prettyTab.Raw)

    // Minify (ugly)
    ugly := nqjson.Get([]byte(`{
        "name": "Alice",
        "age": 30
    }`), "@ugly")
    fmt.Println("Minified:", ugly.Raw) // {"name":"Alice","age":30}

    // Validate JSON
    valid := nqjson.Get(json, "@valid")
    fmt.Println("Valid JSON:", valid.Exists()) // true

    // Identity (@this)
    identity := nqjson.Get(json, "@this")
    fmt.Println("Identity:", identity.Raw) // Same as input
}
```

### Modifier Chaining with Path Continuation

```go
func modifierChaining() {
    json := []byte(`{
        "items": [
            {"name": "Apple", "price": 1.50},
            {"name": "Banana", "price": 0.75},
            {"name": "Cherry", "price": 2.00}
        ]
    }`)

    // Reverse and get first (effectively last item)
    lastItem := nqjson.Get(json, "items|@reverse|0")
    fmt.Println("Last item:", lastItem.Raw) // {"name":"Cherry","price":2.00}

    // Get name of last item
    lastName := nqjson.Get(json, "items|@reverse|0.name")
    fmt.Println("Last item name:", lastName.String()) // Cherry

    // Chain multiple operations
    prices := nqjson.Get(json, "items.#.price|@sort|@reverse")
    fmt.Println("Prices (high to low):", prices.Raw) // [2,1.5,0.75]
}
```

## Escape Sequences

### Keys with Special Characters

```go
func escapeSequences() {
    // JSON with special characters in keys
    json := []byte(`{
        "fav.movie": "Inception",
        "user:config": {"theme": "dark"},
        "path\\to\\file": "readme.txt"
    }`)

    // Access key with dot using \. escape
    movie := nqjson.Get(json, `fav\.movie`)
    fmt.Println("Favorite movie:", movie.String()) // Inception

    // Access key with colon using \: escape
    theme := nqjson.Get(json, `user\:config.theme`)
    fmt.Println("Theme:", theme.String()) // dark

    // SET with escaped keys
    result, _ := nqjson.Set(json, `user\:config.language`, []byte(`"en"`))
    fmt.Println("Updated:", string(result))
}
```

### Numeric Keys as Object Properties

```go
func numericKeys() {
    // JSON with numeric string keys (not array indices)
    json := []byte(`{
        "data": {
            "123": "first value",
            "456": {"nested": "second value"}
        }
    }`)

    // Use : prefix to treat as object key, not array index
    first := nqjson.Get(json, "data.:123")
    fmt.Println("Value at '123':", first.String()) // first value

    second := nqjson.Get(json, "data.:456.nested")
    fmt.Println("Nested value:", second.String()) // second value

    // SET with numeric object keys
    result, _ := nqjson.Set(json, "data.:789", []byte(`"new value"`))
    fmt.Println("Updated:", string(result))
}
```

## JSON Lines Support

### Processing JSON Lines

```go
func jsonLinesProcessing() {
    // JSON Lines format (newline-delimited JSON)
    jsonLines := []byte(`{"name": "Alice", "age": 30}
{"name": "Bob", "age": 25}
{"name": "Charlie", "age": 35}`)

    // Count lines
    count := nqjson.Get(jsonLines, "..#")
    fmt.Printf("Number of lines: %d\n", count.Int()) // 3

    // Access specific line
    firstLine := nqjson.Get(jsonLines, "..0")
    fmt.Println("First line:", firstLine.Raw)

    secondName := nqjson.Get(jsonLines, "..1.name")
    fmt.Println("Second person:", secondName.String()) // Bob

    // Get field from all lines
    allNames := nqjson.Get(jsonLines, "..#.name")
    fmt.Println("All names:", allNames.Raw) // ["Alice","Bob","Charlie"]

    // Query across lines
    olderThan30 := nqjson.Get(jsonLines, "..#(age>30)#.name")
    fmt.Println("Older than 30:", olderThan30.Raw) // ["Charlie"]
}
```

These examples demonstrate the versatility and power of nqjson for various JSON manipulation tasks. The library's performance optimizations and type-safe operations make it suitable for both simple scripts and high-performance applications.
