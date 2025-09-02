# njson Examples

This document provides comprehensive examples of using njson for various JSON manipulation tasks.

## Table of Contents

- [Basic Operations](#basic-operations)
- [Path Expressions](#path-expressions)
- [Type Handling](#type-handling)
- [Array Operations](#array-operations)
- [Object Operations](#object-operations)
- [Advanced Patterns](#advanced-patterns)
- [Performance Optimizations](#performance-optimizations)
- [Error Handling](#error-handling)

## Basic Operations

### Simple Field Access

```go
package main

import (
    "fmt"
    "github.com/dhawalhost/njson"
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
    id := njson.Get([]byte(json), "user.id")
    name := njson.Get([]byte(json), "user.name")
    email := njson.Get([]byte(json), "user.email")
    active := njson.Get([]byte(json), "user.active")

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
    result, err := njson.Set(json, "user.name", "Alice Johnson")
    if err != nil {
        panic(err)
    }

    // Add new field
    result, err = njson.Set(result, "user.email", "alice@example.com")
    if err != nil {
        panic(err)
    }

    // Add nested object
    result, err = njson.Set(result, "user.preferences.theme", "dark")
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
    head := njson.Get([]byte(json), "company.departments.engineering.head")
    budget := njson.Get([]byte(json), "company.departments.engineering.budget")

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
    firstProduct := njson.Get([]byte(json), "products.0.name")
    lastProductPrice := njson.Get([]byte(json), "products.2.price")

    fmt.Printf("First Product: %s\n", firstProduct.String())
    fmt.Printf("Last Product Price: $%.2f\n", lastProductPrice.Float())

    // Get array length
    products := njson.Get([]byte(json), "products")
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
    goBooks := njson.Get([]byte(json), "store.books.#(categories.#(#==\"go\")).title")
    fmt.Println("Go Books:", goBooks.String())

    // Find expensive books (price > 25)
    expensiveBooks := njson.Get([]byte(json), "store.books.#(price>25).title")
    fmt.Println("Expensive Books:", expensiveBooks.String())

    // Get all prices
    allPrices := njson.Get([]byte(json), "store.books.#.price")
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
    if id := njson.Get(data, "user.id"); id.Exists() {
        switch id.Type {
        case njson.TypeNumber:
            fmt.Printf("User ID: %d\n", id.Int())
        default:
            fmt.Printf("User ID (as string): %s\n", id.String())
        }
    }

    // Handle different number types
    balance := njson.Get(data, "user.balance")
    if balance.Exists() && balance.Type == njson.TypeNumber {
        fmt.Printf("Balance: $%.2f\n", balance.Float())
    }

    // Array handling
    tags := njson.Get(data, "user.tags")
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
    metadata := njson.Get(data, "user.metadata")
    if metadata.Type == njson.TypeObject {
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
    name := njson.Get(data, "user.name")
    email := njson.Get(data, "user.email")
    age := njson.Get(data, "user.age")

    fmt.Printf("Name: %s\n", getStringOrDefault(name, "Unknown"))
    fmt.Printf("Email: %s\n", getStringOrDefault(email, "no-email@example.com"))
    fmt.Printf("Age: %d\n", getIntOrDefault(age, 0))
}

func getStringOrDefault(result njson.Result, defaultValue string) string {
    if result.Exists() {
        return result.String()
    }
    return defaultValue
}

func getIntOrDefault(result njson.Result, defaultValue int64) int64 {
    if result.Exists() {
        return result.Int()
    }
    return defaultValue
}
```

## Array Operations

### Array Manipulation

```go
func arrayManipulation() {
    json := []byte(`{"items": [1, 2, 3]}`)

    // Append to array
    result, err := njson.Set(json, "items.-1", 4)
    if err != nil {
        panic(err)
    }

    // Insert at specific position
    result, err = njson.Set(result, "items.1", 1.5)
    if err != nil {
        panic(err)
    }

    // Add object to array
    newItem := map[string]interface{}{
        "id":   5,
        "name": "New Item",
    }
    result, err = njson.Set(result, "items.-1", newItem)
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
    engineers := njson.Get(data, "employees.#(department==\"Engineering\").name")
    fmt.Println("Engineers:", engineers.String())

    // Get high-salary employees
    highEarners := njson.Get(data, "employees.#(salary>80000)")
    if highEarners.IsArray() {
        fmt.Println("High Earners:")
        for _, emp := range highEarners.Array() {
            name := njson.Get([]byte(emp.Raw), "name")
            salary := njson.Get([]byte(emp.Raw), "salary")
            fmt.Printf("  %s: $%.0f\n", name.String(), salary.Float())
        }
    }

    // Calculate average salary
    salaries := njson.Get(data, "employees.#.salary")
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
        result, err = njson.Set(result, path, value)
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
    options := &njson.SetOptions{
        MergeObjects: true,
    }

    newPrefs := map[string]interface{}{
        "theme":    "dark",
        "language": "en",
        "timezone": "UTC",
    }

    result, err := njson.SetWithOptions(base, "user.preferences", newPrefs, options)
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
    results := njson.GetMany(json,
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
    nameTemplate, _ := njson.CompileSetPath("users.%d.lastLogin")
    
    result := json
    for i := 0; i < 3; i++ {
        path := fmt.Sprintf("users.%d.lastLogin", i)
        var err error
        result, err = njson.Set(result, path, "2023-01-15T10:00:00Z")
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
    products := njson.Get(json, "products")
    if products.IsArray() {
        for i, product := range products.Array() {
            inStock := njson.Get([]byte(product.Raw), "inStock")
            price := njson.Get([]byte(product.Raw), "price")
            
            if inStock.Bool() && price.Float() > 50 {
                // Apply 10% discount
                newPrice := price.Float() * 0.9
                path := fmt.Sprintf("products.%d.price", i)
                var err error
                result, err = njson.Set(result, path, newPrice)
                if err != nil {
                    panic(err)
                }
                
                // Add discount flag
                discountPath := fmt.Sprintf("products.%d.discounted", i)
                result, err = njson.Set(result, discountPath, true)
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
    firstName := njson.Get(nested, "user.personal.firstName")
    lastName := njson.Get(nested, "user.personal.lastName")
    email := njson.Get(nested, "user.contact.email")
    phone := njson.Get(nested, "user.contact.phone")

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
        result, err = njson.Set(result, key, value)
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
    userPath, _ := njson.CompileSetPath("users.-1")
    statusPath, _ := njson.CompileSetPath("users.%d.status")

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
        result, err = njson.SetWithCompiledPath(result, userPath, user, nil)
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
    values := njson.Get(json, "data.values")
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
        json, err = njson.Set(json, path, i*10)
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
    result, err := njson.Set(json, "invalid..path", "value")
    if err != nil {
        switch err {
        case njson.ErrInvalidPath:
            fmt.Println("Path syntax error:", err)
        case njson.ErrInvalidJSON:
            fmt.Println("JSON parsing error:", err)
        default:
            fmt.Println("Other error:", err)
        }
        return
    }

    // Validate results
    name := njson.Get(result, "user.name")
    if !name.Exists() {
        fmt.Println("Warning: Expected field not found")
        return
    }

    // Type validation
    if name.Type != njson.TypeString {
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
    secondUser := njson.Get(json, "users.1.name")
    if secondUser.Exists() {
        fmt.Println("Second user:", secondUser.String())
    } else {
        fmt.Println("Second user not found")
    }

    // Safe type conversion
    age := njson.Get(json, "users.0.age")
    if age.Exists() {
        if age.Type == njson.TypeNumber {
            fmt.Printf("Age: %d\n", age.Int())
        } else {
            fmt.Printf("Age (as string): %s\n", age.String())
        }
    } else {
        fmt.Println("Age not specified")
    }

    // Safe object access
    profile := njson.Get(json, "users.0.profile")
    if profile.Exists() && profile.Type == njson.TypeObject {
        fmt.Println("Profile found:", string(profile.Raw))
    } else {
        fmt.Println("No profile information")
    }
}
```

These examples demonstrate the versatility and power of njson for various JSON manipulation tasks. The library's performance optimizations and type-safe operations make it suitable for both simple scripts and high-performance applications.
