package njson

import (
	"testing"
)

// TestUltraSimplePaths tests the ultra-fast path optimizations with simple JSON and simple paths
func TestUltraSimplePaths(t *testing.T) {
	// Test ultra-simple path (data < 1024 bytes, no special chars in path)
	simpleData := `{"name":"John","age":30,"active":true,"score":95.5}`

	// Simple keys without special characters should trigger ultra-fast path
	result := Get([]byte(simpleData), "name")
	if result.String() != "John" {
		t.Errorf("Expected 'John', got %s", result.String())
	}

	result = Get([]byte(simpleData), "age")
	if result.Int() != 30 {
		t.Errorf("Expected 30, got %d", result.Int())
	}

	result = Get([]byte(simpleData), "active")
	if result.Bool() != true {
		t.Errorf("Expected true, got %t", result.Bool())
	}

	result = Get([]byte(simpleData), "score")
	if result.Float() != 95.5 {
		t.Errorf("Expected 95.5, got %f", result.Float())
	}
}

// TestParseFunction tests the Parse function to get coverage
func TestParseFunction(t *testing.T) {
	// Test parsing different JSON types

	// Object
	obj := `{"key":"value"}`
	result := Parse([]byte(obj))
	if result.Type != TypeObject {
		t.Errorf("Expected TypeObject, got %v", result.Type)
	}

	// Array
	arr := `[1,2,3]`
	result = Parse([]byte(arr))
	if result.Type != TypeArray {
		t.Errorf("Expected TypeArray, got %v", result.Type)
	}

	// String
	str := `"hello"`
	result = Parse([]byte(str))
	if result.Type != TypeString {
		t.Errorf("Expected TypeString, got %v", result.Type)
	}

	// Number
	num := `42`
	result = Parse([]byte(num))
	if result.Type != TypeNumber {
		t.Errorf("Expected TypeNumber, got %v", result.Type)
	}

	// Boolean true
	boolTrue := `true`
	result = Parse([]byte(boolTrue))
	if result.Type != TypeBoolean {
		t.Errorf("Expected TypeBoolean, got %v", result.Type)
	}

	// Boolean false
	boolFalse := `false`
	result = Parse([]byte(boolFalse))
	if result.Type != TypeBoolean {
		t.Errorf("Expected TypeBoolean, got %v", result.Type)
	}

	// Null
	nullVal := `null`
	result = Parse([]byte(nullVal))
	if result.Type != TypeNull {
		t.Errorf("Expected TypeNull, got %v", result.Type)
	}

	// Empty data
	result = Parse([]byte(""))
	if result.Type != TypeUndefined {
		t.Errorf("Expected TypeUndefined, got %v", result.Type)
	}

	// Whitespace only
	result = Parse([]byte("   "))
	if result.Type != TypeUndefined {
		t.Errorf("Expected TypeUndefined, got %v", result.Type)
	}
}

// TestEmptyPathHandling tests empty path handling
func TestEmptyPathHandling(t *testing.T) {
	data := `{"key":"value"}`

	// Empty path should parse the whole document
	result := Get([]byte(data), "")
	if result.Type != TypeObject {
		t.Errorf("Expected TypeObject for empty path, got %v", result.Type)
	}
}

// TestDirectArrayIndexWithSimplePath tests array access with simple numeric paths
func TestDirectArrayIndexWithSimplePath(t *testing.T) {
	// Small array to stay under 1024 bytes for ultra-fast path
	arr := `[10,20,30,40,50]`

	// Direct numeric access should trigger simple path optimizations
	result := Get([]byte(arr), "0")
	if result.Int() != 10 {
		t.Errorf("Expected 10, got %d", result.Int())
	}

	result = Get([]byte(arr), "2")
	if result.Int() != 30 {
		t.Errorf("Expected 30, got %d", result.Int())
	}

	result = Get([]byte(arr), "4")
	if result.Int() != 50 {
		t.Errorf("Expected 50, got %d", result.Int())
	}
}

// TestNestedSimplePaths tests nested access with simple dot notation
func TestNestedSimplePaths(t *testing.T) {
	// Small nested object
	nested := `{"user":{"name":"Alice","age":25}}`

	// Simple dot notation should trigger simple path processing
	result := Get([]byte(nested), "user.name")
	if result.String() != "Alice" {
		t.Errorf("Expected 'Alice', got %s", result.String())
	}

	result = Get([]byte(nested), "user.age")
	if result.Int() != 25 {
		t.Errorf("Expected 25, got %d", result.Int())
	}
}

// TestComplexPathsToTriggerAdvanced tests complex paths that should trigger advanced processing
func TestComplexPathsToTriggerAdvanced(t *testing.T) {
	data := `{"items":[{"id":1,"tags":["a","b"]},{"id":2,"tags":["c","d"]}]}`

	// Complex path with array indices and brackets should trigger complex path processing
	result := Get([]byte(data), "items[0].id")
	if result.Int() != 1 {
		t.Errorf("Expected 1, got %d", result.Int())
	}

	result = Get([]byte(data), "items[1].tags[0]")
	if result.String() != "c" {
		t.Errorf("Expected 'c', got %s", result.String())
	}
}

// TestLargeDataToBypassUltraFast tests data larger than 1024 bytes to bypass ultra-fast path
func TestLargeDataToBypassUltraFast(t *testing.T) {
	// Create JSON larger than 1024 bytes
	largeData := `{"data":"` + generateLargeString(1100) + `","key":"value"}`

	// Should not use ultra-fast path due to size
	result := Get([]byte(largeData), "key")
	if result.String() != "value" {
		t.Errorf("Expected 'value', got %s", result.String())
	}
}

// generateLargeString creates a string of specified length
func generateLargeString(length int) string {
	result := make([]byte, length)
	for i := range result {
		result[i] = 'x'
	}
	return string(result)
}
