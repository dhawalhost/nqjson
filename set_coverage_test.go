package njson

import (
	"testing"
)

// TestSetOperationsCoverageBoost tests SET operations to improve coverage
func TestSetOperationsCoverageBoost(t *testing.T) {
	// Test Set with various data types
	original := `{"existing":"value"}`

	// Set string value
	result, err := Set([]byte(original), "name", "John")
	if err != nil {
		t.Errorf("Set string error: %v", err)
	}
	if Get(result, "name").String() != "John" {
		t.Errorf("Set string failed")
	}

	// Set number value
	result, err = Set(result, "age", 30)
	if err != nil {
		t.Errorf("Set number error: %v", err)
	}
	if Get(result, "age").Int() != 30 {
		t.Errorf("Set number failed")
	}

	// Set boolean value
	result, err = Set(result, "active", true)
	if err != nil {
		t.Errorf("Set boolean error: %v", err)
	}
	if !Get(result, "active").Bool() {
		t.Errorf("Set boolean failed")
	}

	// Set null value
	result, err = Set(result, "nothing", nil)
	if err != nil {
		t.Errorf("Set null error: %v", err)
	}
	if !Get(result, "nothing").IsNull() {
		t.Errorf("Set null failed")
	}

	// Set nested object
	result, err = Set(result, "nested.key", "nested value")
	if err != nil {
		t.Errorf("Set nested error: %v", err)
	}
	if Get(result, "nested.key").String() != "nested value" {
		t.Errorf("Set nested failed")
	}

	// Set array element
	result, err = Set(result, "items.0", "first item")
	if err != nil {
		t.Errorf("Set array error: %v", err)
	}
	if Get(result, "items.0").String() != "first item" {
		t.Errorf("Set array element failed")
	}
}

// TestDeleteOperationsCoverageBoost tests DELETE operations
func TestDeleteOperationsCoverageBoost(t *testing.T) {
	original := `{
		"name": "John",
		"age": 30,
		"address": {
			"street": "123 Main St",
			"city": "Anytown"
		},
		"tags": ["go", "json", "fast"]
	}`

	// Delete simple key
	result, err := Delete([]byte(original), "age")
	if err != nil {
		t.Errorf("Delete error: %v", err)
	}
	if Get(result, "age").Exists() {
		t.Errorf("Delete simple key failed")
	}

	// Delete nested key
	result, err = Delete(result, "address.street")
	if err != nil {
		t.Errorf("Delete nested error: %v", err)
	}
	if Get(result, "address.street").Exists() {
		t.Errorf("Delete nested key failed")
	}

	// Delete array element
	result, err = Delete(result, "tags.1")
	if err != nil {
		t.Errorf("Delete array error: %v", err)
	}
	remaining := Get(result, "tags").Array()
	if len(remaining) >= 3 {
		t.Errorf("Delete array element failed, still has %d elements", len(remaining))
	}
}

// TestSetWithOptionsCoverageBoost tests SetWithOptions function
func TestSetWithOptionsCoverageBoost(t *testing.T) {
	original := `{"existing":"value"}`

	// Test with various options
	opts := &SetOptions{
		Optimistic:     true,
		ReplaceInPlace: false,
	}

	result, err := SetWithOptions([]byte(original), "new", "value", opts)
	if err != nil {
		t.Errorf("SetWithOptions error: %v", err)
	}
	if Get(result, "new").String() != "value" {
		t.Errorf("SetWithOptions failed")
	}

	// Test with ReplaceInPlace option
	opts.ReplaceInPlace = true
	testData := []byte(`{"test":"data"}`)
	result, err = SetWithOptions(testData, "another", "test", opts)
	if err != nil {
		t.Errorf("SetWithOptions ReplaceInPlace error: %v", err)
	}
	if Get(result, "another").String() != "test" {
		t.Errorf("SetWithOptions with ReplaceInPlace failed")
	}
}

// TestCompileSetPathCoverageBoost tests CompileSetPath function
func TestCompileSetPathCoverageBoost(t *testing.T) {
	// Test compiling various path patterns
	paths := []string{
		"simple",
		"nested.key",
		"array.0",
		"complex[0].nested",
		"deep.path.with.many.levels",
	}

	for _, path := range paths {
		compiled, err := CompileSetPath(path)
		if err != nil {
			t.Errorf("CompileSetPath error for path %s: %v", path, err)
		}
		if compiled == nil {
			t.Errorf("CompileSetPath failed for path: %s", path)
		}
	}
}

// TestSpecialCharacterHandlingBoost tests handling of special characters
func TestSpecialCharacterHandlingBoost(t *testing.T) {
	// Test with various special characters in keys and values
	data := `{}`

	// Test setting keys with special characters
	result, err := Set([]byte(data), "key with spaces", "value")
	if err != nil {
		t.Errorf("Special character key error: %v", err)
	}
	if Get(result, "key with spaces").String() != "value" {
		t.Errorf("Special character key failed")
	}

	// Test values with special characters
	result, err = Set(result, "special", "value with\nnewlines\tand\ttabs")
	if err != nil {
		t.Errorf("Special character value error: %v", err)
	}
	if !Get(result, "special").Exists() {
		t.Errorf("Special character value failed")
	}

	// Test unicode characters
	result, err = Set(result, "unicode", "🚀 emoji test 中文")
	if err != nil {
		t.Errorf("Unicode value error: %v", err)
	}
	if !Get(result, "unicode").Exists() {
		t.Errorf("Unicode value failed")
	}
}

// TestArrayOperationsBoost tests array operations to trigger more coverage
func TestArrayOperationsBoost(t *testing.T) {
	// Test with empty array
	emptyArray := `[]`
	result, err := Set([]byte(emptyArray), "0", "first")
	if err != nil {
		t.Errorf("Set on empty array error: %v", err)
	}
	if Get(result, "0").String() != "first" {
		t.Errorf("Set on empty array failed")
	}

	// Test with large array indices
	largeArray := `[1,2,3,4,5]`
	result, err = Set([]byte(largeArray), "10", "new item")
	if err != nil {
		t.Errorf("Set large index error: %v", err)
	}
	if Get(result, "10").String() != "new item" {
		t.Errorf("Set large index failed")
	}

	// Test array in object
	objWithArray := `{"items":[1,2,3]}`
	result, err = Set([]byte(objWithArray), "items.5", "added")
	if err != nil {
		t.Errorf("Set array in object error: %v", err)
	}
	if Get(result, "items.5").String() != "added" {
		t.Errorf("Set array in object failed")
	}
}

// TestErrorConditions tests error conditions to improve coverage
func TestErrorConditions(t *testing.T) {
	// Test with invalid JSON
	invalidJson := `{"invalid": json}`
	_, err := Set([]byte(invalidJson), "key", "value")
	if err == nil {
		t.Errorf("Expected error for invalid JSON")
	}

	// Test with invalid path
	validJson := `{"valid":"json"}`
	_, err = Set([]byte(validJson), "", "value")
	if err == nil {
		t.Errorf("Expected error for empty path")
	}

	// Test Delete with invalid JSON
	_, err = Delete([]byte(invalidJson), "key")
	if err == nil {
		t.Errorf("Expected error for Delete with invalid JSON")
	}
}

// generateLargeStringForCoverage creates a string of specified length for testing
func generateLargeStringForCoverage(length int) string {
	result := make([]byte, length)
	for i := range result {
		result[i] = byte('a' + (i % 26))
	}
	return string(result)
}
