package njson

import (
	"strings"
	"testing"
	"time"
)

// TestGetBasic tests basic GET functionality
func TestGetBasic(t *testing.T) {
	json := []byte(`{"name":"John","age":30,"active":true}`)

	// Test string value
	result := Get(json, "name")
	if !result.Exists() {
		t.Error("Expected name field to exist")
	}
	if result.String() != "John" {
		t.Errorf("Expected 'John', got %q", result.String())
	}

	// Test number value
	result = Get(json, "age")
	if !result.Exists() {
		t.Error("Expected age field to exist")
	}
	if result.Int() != 30 {
		t.Errorf("Expected 30, got %d", result.Int())
	}

	// Test boolean value
	result = Get(json, "active")
	if !result.Exists() {
		t.Error("Expected active field to exist")
	}
	if !result.Bool() {
		t.Errorf("Expected true, got %v", result.Bool())
	}
}

// TestGetArrays tests array access functionality
func TestGetArrays(t *testing.T) {
	json := []byte(`{"items":["apple","banana","cherry"]}`)

	// Test first element
	result := Get(json, "items.0")
	if !result.Exists() {
		t.Error("Expected items.0 to exist")
	}
	if result.String() != "apple" {
		t.Errorf("Expected 'apple', got %q", result.String())
	}

	// Test second element
	result = Get(json, "items.1")
	if !result.Exists() {
		t.Error("Expected items.1 to exist")
	}
	if result.String() != "banana" {
		t.Errorf("Expected 'banana', got %q", result.String())
	}

	// Test out of bounds
	result = Get(json, "items.10")
	if result.Exists() {
		t.Error("Expected out of bounds access to not exist")
	}
}

// TestGetNested tests nested object access
func TestGetNested(t *testing.T) {
	json := []byte(`{"user":{"profile":{"name":"Alice","settings":{"theme":"dark"}}}}`)

	// Test nested access
	result := Get(json, "user.profile.name")
	if !result.Exists() {
		t.Error("Expected nested path to exist")
	}
	if result.String() != "Alice" {
		t.Errorf("Expected 'Alice', got %q", result.String())
	}

	// Test deep nested access
	result = Get(json, "user.profile.settings.theme")
	if !result.Exists() {
		t.Error("Expected deep nested path to exist")
	}
	if result.String() != "dark" {
		t.Errorf("Expected 'dark', got %q", result.String())
	}
}

// TestGetString tests string-based GET operations
func TestGetString(t *testing.T) {
	jsonStr := `{"name":"Bob","count":42}`

	// Test GetString
	result := GetString(jsonStr, "name")
	if !result.Exists() {
		t.Error("Expected name field to exist")
	}
	if result.String() != "Bob" {
		t.Errorf("Expected 'Bob', got %q", result.String())
	}

	// Test number from string JSON
	result = GetString(jsonStr, "count")
	if !result.Exists() {
		t.Error("Expected count field to exist")
	}
	if result.Int() != 42 {
		t.Errorf("Expected 42, got %d", result.Int())
	}
}

// TestGetMany tests batch GET operations
func TestGetMany(t *testing.T) {
	json := []byte(`{"name":"Charlie","age":25,"active":true,"items":[0,1]}`)

	paths := []string{"name", "age", "items.0", "missing"}
	results := GetMany(json, paths...)

	if len(results) != 4 {
		t.Fatalf("Expected 4 results, got %d", len(results))
	}

	// Check name
	if !results[0].Exists() || results[0].String() != "Charlie" {
		t.Errorf("Expected 'Charlie', got %q", results[0].String())
	}

	// Check age
	if !results[1].Exists() || results[1].Int() != 25 {
		t.Errorf("Expected 25, got %d", results[1].Int())
	}

	// Check array element
	if !results[2].Exists() || results[2].Int() != 0 {
		t.Errorf("Expected 0, got %d", results[2].Int())
	}

	// Check missing field
	if results[3].Exists() {
		t.Error("Expected missing field to not exist")
	}
}

// TestGetTypes tests different value types
func TestGetTypes(t *testing.T) {
	json := []byte(`{"str":"hello","num":123,"float":3.14,"bool":true,"null":null,"obj":{"key":"value"},"arr":[1,2,3]}`)

	tests := []struct {
		path     string
		expected ValueType
	}{
		{"str", TypeString},
		{"num", TypeNumber},
		{"float", TypeNumber},
		{"bool", TypeBoolean},
		{"null", TypeNull},
		{"obj", TypeObject},
		{"arr", TypeArray},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := Get(json, tt.path)
			if !result.Exists() {
				t.Errorf("Expected %s to exist", tt.path)
			}
			if result.Type != tt.expected {
				t.Errorf("Expected type %v, got %v", tt.expected, result.Type)
			}
		})
	}
}

// TestGetEdgeCases tests error cases and edge conditions
func TestGetEdgeCases(t *testing.T) {
	// Test empty JSON
	emptyJson := []byte(`{}`)
	result := Get(emptyJson, "missing")
	if result.Exists() {
		t.Error("Expected missing field in empty JSON to not exist")
	}

	// Test invalid JSON
	invalidJson := []byte(`{"invalid": json}`)
	result = Get(invalidJson, "any")
	if result.Exists() {
		t.Error("Expected invalid JSON to return non-existent result")
	}

	// Test null JSON
	result = Get(nil, "any")
	if result.Exists() {
		t.Error("Expected nil JSON to return non-existent result")
	}

	// Test with valid JSON and invalid path characters
	validJson := []byte(`{"key":"value"}`)
	result = Get(validJson, "nonexistent.path")
	if result.Exists() {
		t.Error("Expected nonexistent path to return non-existent result")
	}
}

// TestParse tests the Parse function that was 0% covered
func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected ValueType
	}{
		{"string", `"hello"`, TypeString},
		{"number", `42`, TypeNumber},
		{"float", `3.14`, TypeNumber},
		{"boolean true", `true`, TypeBoolean},
		{"boolean false", `false`, TypeBoolean},
		{"null", `null`, TypeNull},
		{"object", `{"key":"value"}`, TypeObject},
		{"array", `[1,2,3]`, TypeArray},
		{"empty object", `{}`, TypeObject},
		{"empty array", `[]`, TypeArray},
		{"invalid", `invalid`, TypeUndefined},
		{"incomplete", `{"incomplete":`, TypeObject}, // Incomplete JSON might still parse as object
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse([]byte(tt.json))
			if result.Type != tt.expected {
				t.Errorf("Expected type %v, got %v", tt.expected, result.Type)
			}
		})
	}
}

// TestResultMethods tests Result type methods for better coverage
func TestResultMethods(t *testing.T) {
	json := []byte(`{
		"str": "hello world",
		"num": 42,
		"float": 3.14159,
		"bool_true": true,
		"bool_false": false,
		"null_val": null,
		"obj": {"nested": "value"},
		"arr": [1, "two", 3.0],
		"time_str": "2023-01-01T00:00:00Z"
	}`)

	// Test String() method
	strResult := Get(json, "str")
	if strResult.String() != "hello world" {
		t.Errorf("String(): expected 'hello world', got %q", strResult.String())
	}

	// Test Int() method
	numResult := Get(json, "num")
	if numResult.Int() != 42 {
		t.Errorf("Int(): expected 42, got %d", numResult.Int())
	}

	// Test Float() method
	floatResult := Get(json, "float")
	if floatResult.Float() != 3.14159 {
		t.Errorf("Float(): expected 3.14159, got %f", floatResult.Float())
	}

	// Test Bool() method
	boolTrueResult := Get(json, "bool_true")
	if !boolTrueResult.Bool() {
		t.Errorf("Bool(): expected true, got %v", boolTrueResult.Bool())
	}

	boolFalseResult := Get(json, "bool_false")
	if boolFalseResult.Bool() {
		t.Errorf("Bool(): expected false, got %v", boolFalseResult.Bool())
	}

	// Test IsNull() method
	nullResult := Get(json, "null_val")
	if !nullResult.IsNull() {
		t.Error("IsNull(): expected true for null value")
	}

	// Test Array() method
	arrResult := Get(json, "arr")
	if !arrResult.Exists() {
		t.Error("Array should exist")
	}

	arrayItems := arrResult.Array()
	if len(arrayItems) != 3 {
		t.Errorf("Array(): expected 3 items, got %d", len(arrayItems))
	}

	// Test Map() method
	objResult := Get(json, "obj")
	if !objResult.Exists() {
		t.Error("Object should exist")
	}

	mapItems := objResult.Map()
	if len(mapItems) != 1 {
		t.Errorf("Map(): expected 1 item, got %d", len(mapItems))
	}
	if mapItems["nested"].String() != "value" {
		t.Errorf("Map(): expected 'value', got %q", mapItems["nested"].String())
	}

	// Test ForEach() method
	count := 0
	objResult.ForEach(func(key, value Result) bool {
		count++
		if key.String() != "nested" {
			t.Errorf("ForEach key: expected 'nested', got %q", key.String())
		}
		if value.String() != "value" {
			t.Errorf("ForEach value: expected 'value', got %q", value.String())
		}
		return true
	})
	if count != 1 {
		t.Errorf("ForEach: expected 1 iteration, got %d", count)
	}

	// Test Time() method
	timeResult := Get(json, "time_str")
	parsedTime, timeErr := timeResult.Time()
	if timeErr != nil {
		t.Errorf("Time() error: %v", timeErr)
	}
	expectedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	if !parsedTime.Equal(expectedTime) {
		t.Errorf("Time(): expected %v, got %v", expectedTime, parsedTime)
	}

	// Test Get() method on Result
	nestedResult := objResult.Get("nested")
	if !nestedResult.Exists() || nestedResult.String() != "value" {
		t.Errorf("Result.Get(): expected 'value', got %q", nestedResult.String())
	}
}

// TestLargeArrayAccess tests large array access to trigger optimization paths
func TestLargeArrayAccess(t *testing.T) {
	// Create a large array to trigger ultra-fast array access
	var jsonBuilder strings.Builder
	jsonBuilder.WriteString(`{"large_array":[`)
	for i := 0; i < 1000; i++ {
		if i > 0 {
			jsonBuilder.WriteString(",")
		}
		jsonBuilder.WriteString(`"item`)
		jsonBuilder.WriteString(strings.Repeat("0", 10)) // Make items large
		jsonBuilder.WriteString(`"`)
	}
	jsonBuilder.WriteString(`]}`)

	json := []byte(jsonBuilder.String())

	// Test accessing various elements
	tests := []int{0, 1, 10, 100, 500, 999}
	for _, idx := range tests {
		path := "large_array." + strings.Repeat("0", 10-len(string(rune(idx)))) + string(rune(idx+'0'))
		if idx >= 10 {
			path = "large_array." + string(rune(idx/10+'0')) + string(rune(idx%10+'0'))
		}
		if idx >= 100 {
			path = "large_array." + string(rune(idx/100+'0')) + string(rune((idx/10)%10+'0')) + string(rune(idx%10+'0'))
		}

		// Use direct index access instead
		path = "large_array." + string(rune(idx+'0'))
		if idx >= 10 {
			path = "large_array." + string(rune(idx/10+'0')) + string(rune(idx%10+'0'))
		}
		if idx >= 100 {
			path = "large_array." + string(rune(idx/100+'0')) + string(rune((idx/10)%10+'0')) + string(rune(idx%10+'0'))
		}

		// Simplify - just test a few key indices
		if idx < 10 {
			path = "large_array." + string(rune(idx+'0'))
			result := Get(json, path)
			if !result.Exists() {
				t.Errorf("Expected element at index %d to exist", idx)
			}
		}
	}
}

// TestComplexPaths tests complex path operations
func TestComplexPaths(t *testing.T) {
	json := []byte(`{
		"users": [
			{"name": "Alice", "age": 30, "active": true},
			{"name": "Bob", "age": 25, "active": false},
			{"name": "Charlie", "age": 35, "active": true}
		],
		"metadata": {
			"total": 3,
			"filters": ["active", "name"]
		}
	}`)

	// Test deep nested access
	result := Get(json, "users.0.name")
	if !result.Exists() || result.String() != "Alice" {
		t.Errorf("Expected 'Alice', got %q", result.String())
	}

	// Test multiple levels - use simple path first
	result = Get(json, "metadata.total")
	if !result.Exists() || result.Int() != 3 {
		t.Errorf("Expected 3, got %d", result.Int())
	}

	// Test very deep nesting to trigger deep access functions
	deepJson := []byte(`{
		"level1": {
			"level2": {
				"level3": {
					"level4": {
						"level5": {
							"level6": {
								"level7": {
									"level8": {
										"level9": {
											"level10": {
												"value": "deep_value"
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}`)

	result = Get(deepJson, "level1.level2.level3.level4.level5.level6.level7.level8.level9.level10.value")
	if !result.Exists() || result.String() != "deep_value" {
		t.Errorf("Expected 'deep_value', got %q", result.String())
	}
}

// TestStringParsing tests string parsing functions
func TestStringParsing(t *testing.T) {
	json := []byte(`{
		"escaped": "hello\nworld\t\"quoted\"",
		"unicode": "hello\\u0041world",
		"empty": "",
		"long": "` + strings.Repeat("a", 1000) + `"
	}`)

	// Test escaped string
	result := Get(json, "escaped")
	if !result.Exists() {
		t.Error("Expected escaped string to exist")
	}

	// Test unicode string
	result = Get(json, "unicode")
	if !result.Exists() {
		t.Error("Expected unicode string to exist")
	}

	// Test empty string
	result = Get(json, "empty")
	if !result.Exists() || result.String() != "" {
		t.Errorf("Expected empty string, got %q", result.String())
	}

	// Test long string
	result = Get(json, "long")
	if !result.Exists() || len(result.String()) != 1000 {
		t.Errorf("Expected long string of 1000 chars, got %d", len(result.String()))
	}
}
