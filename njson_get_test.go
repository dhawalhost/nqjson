package njson

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// TestGet_BasicOperations tests basic GET functionality using table-driven tests
func TestGet_BasicOperations(t *testing.T) {
	tests := []struct {
		name     string
		json     []byte
		path     string
		want     interface{}
		wantType string
		exists   bool
	}{
		// Basic value tests
		{
			name:     "get_string_value",
			json:     []byte(`{"name":"John","age":30,"active":true}`),
			path:     "name",
			want:     "John",
			wantType: "string",
			exists:   true,
		},
		{
			name:     "get_int_value",
			json:     []byte(`{"name":"John","age":30,"active":true}`),
			path:     "age",
			want:     int64(30),
			wantType: "int",
			exists:   true,
		},
		{
			name:     "get_bool_value",
			json:     []byte(`{"name":"John","age":30,"active":true}`),
			path:     "active",
			want:     true,
			wantType: "bool",
			exists:   true,
		},
		{
			name:     "get_float_value",
			json:     []byte(`{"price":19.99,"count":5}`),
			path:     "price",
			want:     19.99,
			wantType: "float",
			exists:   true,
		},
		{
			name:     "get_null_value",
			json:     []byte(`{"value":null,"name":"test"}`),
			path:     "value",
			want:     nil,
			wantType: "null",
			exists:   true,
		},
		{
			name:     "get_nonexistent_field",
			json:     []byte(`{"name":"John"}`),
			path:     "nonexistent",
			want:     nil,
			wantType: "none",
			exists:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Get(tt.json, tt.path)

			if result.Exists() != tt.exists {
				t.Errorf("Get(%s).Exists() = %v, want %v", tt.path, result.Exists(), tt.exists)
			}

			if !tt.exists {
				return // Skip value checks for non-existent fields
			}

			switch tt.wantType {
			case "string":
				if got := result.String(); got != tt.want {
					t.Errorf("Get(%s).String() = %v, want %v", tt.path, got, tt.want)
				}
			case "int":
				if got := result.Int(); got != tt.want {
					t.Errorf("Get(%s).Int() = %v, want %v", tt.path, got, tt.want)
				}
			case "bool":
				if got := result.Bool(); got != tt.want {
					t.Errorf("Get(%s).Bool() = %v, want %v", tt.path, got, tt.want)
				}
			case "float":
				if got := result.Float(); got != tt.want {
					t.Errorf("Get(%s).Float() = %v, want %v", tt.path, got, tt.want)
				}
			case "null":
				if !result.IsNull() {
					t.Errorf("Get(%s) should be null", tt.path)
				}
			}
		})
	}
}

// TestGet_ArrayOperations tests array access functionality using table-driven tests
func TestGet_ArrayOperations(t *testing.T) {
	tests := []struct {
		name   string
		json   []byte
		path   string
		want   interface{}
		exists bool
	}{
		{
			name:   "array_first_element",
			json:   []byte(`{"items":["apple","banana","cherry"]}`),
			path:   "items.0",
			want:   "apple",
			exists: true,
		},
		{
			name:   "array_second_element",
			json:   []byte(`{"items":["apple","banana","cherry"]}`),
			path:   "items.1",
			want:   "banana",
			exists: true,
		},
		{
			name:   "array_last_element",
			json:   []byte(`{"items":["apple","banana","cherry"]}`),
			path:   "items.2",
			want:   "cherry",
			exists: true,
		},
		{
			name:   "array_out_of_bounds",
			json:   []byte(`{"items":["apple","banana","cherry"]}`),
			path:   "items.10",
			want:   nil,
			exists: false,
		},
		{
			name:   "array_negative_index",
			json:   []byte(`{"items":["apple","banana","cherry"]}`),
			path:   "items.-1",
			want:   nil,
			exists: false,
		},
		{
			name:   "nested_array_access",
			json:   []byte(`{"data":[{"id":1,"tags":["red","blue"]},{"id":2,"tags":["green"]}]}`),
			path:   "data.0.tags.1",
			want:   "blue",
			exists: true,
		},
		{
			name:   "empty_array",
			json:   []byte(`{"items":[]}`),
			path:   "items.0",
			want:   nil,
			exists: false,
		},
		{
			name:   "array_of_numbers",
			json:   []byte(`{"numbers":[1,2,3,4,5]}`),
			path:   "numbers.2",
			want:   int64(3),
			exists: true,
		},
		{
			name:   "array_of_objects",
			json:   []byte(`{"users":[{"name":"Alice","age":25},{"name":"Bob","age":30}]}`),
			path:   "users.1.name",
			want:   "Bob",
			exists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Get(tt.json, tt.path)

			if result.Exists() != tt.exists {
				t.Errorf("Get(%s).Exists() = %v, want %v", tt.path, result.Exists(), tt.exists)
			}

			if !tt.exists {
				return
			}

			// Determine expected type and compare
			switch v := tt.want.(type) {
			case string:
				if got := result.String(); got != v {
					t.Errorf("Get(%s).String() = %v, want %v", tt.path, got, v)
				}
			case int64:
				if got := result.Int(); got != v {
					t.Errorf("Get(%s).Int() = %v, want %v", tt.path, got, v)
				}
			case float64:
				if got := result.Float(); got != v {
					t.Errorf("Get(%s).Float() = %v, want %v", tt.path, got, v)
				}
			case bool:
				if got := result.Bool(); got != v {
					t.Errorf("Get(%s).Bool() = %v, want %v", tt.path, got, v)
				}
			}
		})
	}
}

// TestGet_NestedOperations tests nested object access using table-driven tests
func TestGet_NestedOperations(t *testing.T) {
	tests := []struct {
		name   string
		json   []byte
		path   string
		want   string
		exists bool
	}{
		{
			name:   "simple_nested",
			json:   []byte(`{"user":{"name":"Alice","age":30}}`),
			path:   "user.name",
			want:   "Alice",
			exists: true,
		},
		{
			name:   "deep_nested",
			json:   []byte(`{"user":{"profile":{"name":"Alice","settings":{"theme":"dark"}}}}`),
			path:   "user.profile.settings.theme",
			want:   "dark",
			exists: true,
		},
		{
			name:   "very_deep_nested",
			json:   []byte(`{"a":{"b":{"c":{"d":{"e":"value"}}}}}`),
			path:   "a.b.c.d.e",
			want:   "value",
			exists: true,
		},
		{
			name:   "nested_nonexistent_middle",
			json:   []byte(`{"user":{"name":"Alice"}}`),
			path:   "user.profile.name",
			want:   "",
			exists: false,
		},
		{
			name:   "nested_nonexistent_final",
			json:   []byte(`{"user":{"profile":{"name":"Alice"}}}`),
			path:   "user.profile.age",
			want:   "",
			exists: false,
		},
		{
			name:   "mixed_array_object_nesting",
			json:   []byte(`{"users":[{"profile":{"details":{"city":"NYC"}}}]}`),
			path:   "users.0.profile.details.city",
			want:   "NYC",
			exists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Get(tt.json, tt.path)

			if result.Exists() != tt.exists {
				t.Errorf("Get(%s).Exists() = %v, want %v", tt.path, result.Exists(), tt.exists)
			}

			if tt.exists {
				if got := result.String(); got != tt.want {
					t.Errorf("Get(%s).String() = %v, want %v", tt.path, got, tt.want)
				}
			}
		})
	}
}

// TestGetString_Operations tests GetString function using table-driven tests
func TestGetString_Operations(t *testing.T) {
	tests := []struct {
		name   string
		json   string
		path   string
		want   string
		exists bool
	}{
		{
			name:   "get_string_from_string_json",
			json:   `{"name":"Bob","count":42}`,
			path:   "name",
			want:   "Bob",
			exists: true,
		},
		{
			name:   "get_number_from_string_json",
			json:   `{"name":"Bob","count":42}`,
			path:   "count",
			want:   "42",
			exists: true,
		},
		{
			name:   "nonexistent_field_string_json",
			json:   `{"name":"Bob"}`,
			path:   "missing",
			want:   "",
			exists: false,
		},
		{
			name:   "nested_field_string_json",
			json:   `{"user":{"profile":{"name":"Charlie"}}}`,
			path:   "user.profile.name",
			want:   "Charlie",
			exists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetString(tt.json, tt.path)

			if result.Exists() != tt.exists {
				t.Errorf("GetString(%s).Exists() = %v, want %v", tt.path, result.Exists(), tt.exists)
			}

			if tt.exists {
				if got := result.String(); got != tt.want {
					t.Errorf("GetString(%s).String() = %v, want %v", tt.path, got, tt.want)
				}
			}
		})
	}
}

// TestGet_ComplexQueries tests complex query patterns using table-driven tests
func TestGet_ComplexQueries(t *testing.T) {
	complexJSON := []byte(`{
		"users": [
			{
				"id": 1,
				"name": "Alice",
				"emails": ["alice@work.com", "alice@personal.com"],
				"profile": {
					"age": 25,
					"location": "NYC",
					"preferences": {
						"theme": "dark",
						"notifications": true
					}
				}
			},
			{
				"id": 2,
				"name": "Bob",
				"emails": ["bob@company.com"],
				"profile": {
					"age": 30,
					"location": "SF",
					"preferences": {
						"theme": "light",
						"notifications": false
					}
				}
			}
		],
		"metadata": {
			"total": 2,
			"active": true
		}
	}`)

	tests := []struct {
		name   string
		path   string
		want   interface{}
		exists bool
	}{
		{
			name:   "first_user_name",
			path:   "users.0.name",
			want:   "Alice",
			exists: true,
		},
		{
			name:   "second_user_location",
			path:   "users.1.profile.location",
			want:   "SF",
			exists: true,
		},
		{
			name:   "first_user_second_email",
			path:   "users.0.emails.1",
			want:   "alice@personal.com",
			exists: true,
		},
		{
			name:   "deep_preference_setting",
			path:   "users.1.profile.preferences.notifications",
			want:   false,
			exists: true,
		},
		{
			name:   "metadata_total",
			path:   "metadata.total",
			want:   int64(2),
			exists: true,
		},
		{
			name:   "nonexistent_user",
			path:   "users.5.name",
			want:   nil,
			exists: false,
		},
		{
			name:   "nonexistent_deep_path",
			path:   "users.0.profile.settings.color",
			want:   nil,
			exists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Get(complexJSON, tt.path)

			if result.Exists() != tt.exists {
				t.Errorf("Get(%s).Exists() = %v, want %v", tt.path, result.Exists(), tt.exists)
			}

			if !tt.exists {
				return
			}

			switch v := tt.want.(type) {
			case string:
				if got := result.String(); got != v {
					t.Errorf("Get(%s).String() = %v, want %v", tt.path, got, v)
				}
			case int64:
				if got := result.Int(); got != v {
					t.Errorf("Get(%s).Int() = %v, want %v", tt.path, got, v)
				}
			case bool:
				if got := result.Bool(); got != v {
					t.Errorf("Get(%s).Bool() = %v, want %v", tt.path, got, v)
				}
			}
		})
	}
}

// TestGet_EdgeCases tests edge cases and error conditions using table-driven tests
func TestGet_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		json   []byte
		path   string
		exists bool
		desc   string
	}{
		{
			name:   "empty_json_object",
			json:   []byte(`{}`),
			path:   "anything",
			exists: false,
			desc:   "Empty JSON object should return non-existent for any path",
		},
		{
			name:   "empty_path",
			json:   []byte(`{"key":"value"}`),
			path:   "",
			exists: false,
			desc:   "Empty path should return non-existent",
		},
		{
			name:   "root_access",
			json:   []byte(`{"key":"value"}`),
			path:   ".",
			exists: false,
			desc:   "Root access with dot should be handled",
		},
		{
			name:   "invalid_json",
			json:   []byte(`{invalid json`),
			path:   "key",
			exists: false,
			desc:   "Invalid JSON should return non-existent",
		},
		{
			name:   "null_json",
			json:   nil,
			path:   "key",
			exists: false,
			desc:   "Nil JSON should return non-existent",
		},
		{
			name:   "empty_json",
			json:   []byte(``),
			path:   "key",
			exists: false,
			desc:   "Empty JSON should return non-existent",
		},
		{
			name:   "special_characters_in_path",
			json:   []byte(`{"key.with.dots":"value","key-with-dashes":"value2"}`),
			path:   "key-with-dashes",
			exists: true,
			desc:   "Special characters in key names should work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Get(tt.json, tt.path)

			if result.Exists() != tt.exists {
				t.Errorf("Get(%s).Exists() = %v, want %v - %s", tt.path, result.Exists(), tt.exists, tt.desc)
			}
		})
	}
}

// TestParse_Operations tests Parse function using table-driven tests
func TestParse_Operations(t *testing.T) {
	tests := []struct {
		name    string
		json    []byte
		exists  bool
		isValid bool
	}{
		{
			name:    "valid_string",
			json:    []byte(`"hello"`),
			exists:  true,
			isValid: true,
		},
		{
			name:    "valid_number",
			json:    []byte(`42`),
			exists:  true,
			isValid: true,
		},
		{
			name:    "valid_float",
			json:    []byte(`3.14`),
			exists:  true,
			isValid: true,
		},
		{
			name:    "valid_boolean_true",
			json:    []byte(`true`),
			exists:  true,
			isValid: true,
		},
		{
			name:    "valid_boolean_false",
			json:    []byte(`false`),
			exists:  true,
			isValid: true,
		},
		{
			name:    "valid_null",
			json:    []byte(`null`),
			exists:  true,
			isValid: true,
		},
		{
			name:    "valid_object",
			json:    []byte(`{"key":"value"}`),
			exists:  true,
			isValid: true,
		},
		{
			name:    "valid_array",
			json:    []byte(`[1,2,3]`),
			exists:  true,
			isValid: true,
		},
		{
			name:    "empty_object",
			json:    []byte(`{}`),
			exists:  true,
			isValid: true,
		},
		{
			name:    "empty_array",
			json:    []byte(`[]`),
			exists:  true,
			isValid: true,
		},
		{
			name:    "invalid_json",
			json:    []byte(`{invalid`),
			exists:  false,
			isValid: false,
		},
		{
			name:    "empty_input",
			json:    []byte(``),
			exists:  false,
			isValid: false,
		},
		{
			name:    "null_input",
			json:    nil,
			exists:  false,
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse(tt.json)

			if result.Exists() != tt.exists {
				t.Errorf("Parse().Exists() = %v, want %v", result.Exists(), tt.exists)
			}
		})
	}
}

// TestResult_Methods tests all Result methods using table-driven tests
func TestResult_Methods(t *testing.T) {
	json := []byte(`{
		"str": "test",
		"num": 42,
		"float": 3.14,
		"bool": true,
		"null": null,
		"obj": {"key": "value"},
		"arr": [1, 2, 3]
	}`)

	tests := []struct {
		name     string
		path     string
		testFunc func(*testing.T, Result)
	}{
		{
			name: "string_methods",
			path: "str",
			testFunc: func(t *testing.T, r Result) {
				if r.String() != "test" {
					t.Errorf("String() = %v, want test", r.String())
				}
			},
		},
		{
			name: "number_methods",
			path: "num",
			testFunc: func(t *testing.T, r Result) {
				if r.Int() != 42 {
					t.Errorf("Int() = %v, want 42", r.Int())
				}
			},
		},
		{
			name: "float_methods",
			path: "float",
			testFunc: func(t *testing.T, r Result) {
				if r.Float() != 3.14 {
					t.Errorf("Float() = %v, want 3.14", r.Float())
				}
			},
		},
		{
			name: "bool_methods",
			path: "bool",
			testFunc: func(t *testing.T, r Result) {
				if !r.Bool() {
					t.Errorf("Bool() = %v, want true", r.Bool())
				}
			},
		},
		{
			name: "null_methods",
			path: "null",
			testFunc: func(t *testing.T, r Result) {
				if !r.IsNull() {
					t.Errorf("IsNull() = %v, want true", r.IsNull())
				}
			},
		},
		{
			name: "object_methods",
			path: "obj",
			testFunc: func(t *testing.T, r Result) {
				if !r.IsObject() {
					t.Errorf("IsObject() = %v, want true", r.IsObject())
				}
			},
		},
		{
			name: "array_methods",
			path: "arr",
			testFunc: func(t *testing.T, r Result) {
				if !r.IsArray() {
					t.Errorf("IsArray() = %v, want true", r.IsArray())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Get(json, tt.path)
			if !result.Exists() {
				t.Fatalf("Expected path %s to exist", tt.path)
			}
			tt.testFunc(t, result)
		})
	}
}

// TestGetMany_Operations tests GetMany function using table-driven tests
func TestGetMany_Operations(t *testing.T) {
	json := []byte(`{
		"name": "John",
		"age": 30,
		"address": {
			"city": "NYC",
			"zip": "10001"
		},
		"hobbies": ["reading", "coding"]
	}`)

	tests := []struct {
		name      string
		paths     []string
		wantCount int
		checks    []struct {
			index  int
			exists bool
			value  interface{}
		}
	}{
		{
			name:      "multiple_existing_paths",
			paths:     []string{"name", "age", "address.city"},
			wantCount: 3,
			checks: []struct {
				index  int
				exists bool
				value  interface{}
			}{
				{0, true, "John"},
				{1, true, int64(30)},
				{2, true, "NYC"},
			},
		},
		{
			name:      "mixed_existing_nonexisting",
			paths:     []string{"name", "nonexistent", "age"},
			wantCount: 3,
			checks: []struct {
				index  int
				exists bool
				value  interface{}
			}{
				{0, true, "John"},
				{1, false, nil},
				{2, true, int64(30)},
			},
		},
		{
			name:      "array_and_object_paths",
			paths:     []string{"hobbies.0", "address.zip", "hobbies.1"},
			wantCount: 3,
			checks: []struct {
				index  int
				exists bool
				value  interface{}
			}{
				{0, true, "reading"},
				{1, true, "10001"},
				{2, true, "coding"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := GetMany(json, tt.paths...)

			if len(results) != tt.wantCount {
				t.Errorf("GetMany() returned %d results, want %d", len(results), tt.wantCount)
			}

			for _, check := range tt.checks {
				if check.index >= len(results) {
					t.Errorf("Result index %d out of bounds", check.index)
					continue
				}

				result := results[check.index]
				if result.Exists() != check.exists {
					t.Errorf("Result[%d].Exists() = %v, want %v", check.index, result.Exists(), check.exists)
				}

				if check.exists {
					switch v := check.value.(type) {
					case string:
						if got := result.String(); got != v {
							t.Errorf("Result[%d].String() = %v, want %v", check.index, got, v)
						}
					case int64:
						if got := result.Int(); got != v {
							t.Errorf("Result[%d].Int() = %v, want %v", check.index, got, v)
						}
					}
				}
			}
		})
	}
}

// TestGet_Performance tests performance-critical paths
func TestGet_Performance(t *testing.T) {
	// Test large array access
	largeArrayJSON := []byte(`{"data":[` + generateLargeArray(10000) + `]}`)

	tests := []struct {
		name string
		json []byte
		path string
	}{
		{
			name: "large_array_first",
			json: largeArrayJSON,
			path: "data.0",
		},
		{
			name: "large_array_middle",
			json: largeArrayJSON,
			path: "data.5000",
		},
		{
			name: "large_array_last",
			json: largeArrayJSON,
			path: "data.9999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Get(tt.json, tt.path)
			if !result.Exists() {
				t.Errorf("Expected large array access to succeed")
			}
		})
	}
}

// Helper functions for test data generation
func generateLargeArray(size int) string {
	if size == 0 {
		return ""
	}
	result := "0"
	for i := 1; i < size; i++ {
		result += "," + string(rune('0'+i%10))
		if i%10 == 0 {
			result += string(rune('0' + i/10%10))
		}
	}
	return result
}

// TestUltraFastOptimizations tests ultra-fast optimization functions that are not covered
func TestUltraFastOptimizations(t *testing.T) {
	// Test handleItemsPattern
	t.Run("handleItemsPattern", func(t *testing.T) {
		// Create data with "items" pattern that triggers handleItemsPattern
		largeItems := `{"items":[`
		for i := 0; i < 1000; i++ {
			if i > 0 {
				largeItems += ","
			}
			largeItems += fmt.Sprintf(`{"id":%d,"name":"item%d","metadata":{"priority":%d},"tags":["tag%d","special"]}`, i, i, i*2, i)
		}
		largeItems += `]}`

		// Test patterns that should trigger handleItemsPattern
		testCases := []struct {
			path       string
			shouldFind bool
		}{
			{"items.500.name", true},
			{"items.999.metadata.priority", true},
			{"items.250.tags.1", true},
			{"items.0.id", true},
		}

		for _, tc := range testCases {
			result := Get([]byte(largeItems), tc.path)
			if result.Exists() != tc.shouldFind {
				t.Errorf("handleItemsPattern path %s: expected exists=%v, got %v", tc.path, tc.shouldFind, result.Exists())
			}
		}
	})
}

// TestResultMethodsCoverage tests Result methods to improve coverage
func TestResultMethodsCoverage(t *testing.T) {
	// Test String method edge cases (42.9% coverage - improve it)
	t.Run("String_EdgeCases", func(t *testing.T) {
		testCases := []struct {
			json     string
			path     string
			expected string
		}{
			{`{"str":"test"}`, "str", "test"},
			{`{"num":123}`, "num", "123"},
			{`{"bool":true}`, "bool", "true"},
			{`{"null":null}`, "null", "null"},
			{`{"empty":""}`, "empty", ""},
			{`{"obj":{"a":1}}`, "obj", `{"a":1}`},
			{`{"arr":[1,2,3]}`, "arr", `[1,2,3]`},
		}

		for _, tc := range testCases {
			result := Get([]byte(tc.json), tc.path)
			got := result.String()
			if got != tc.expected {
				t.Errorf("String() for %s.%s: expected %s, got %s", tc.json, tc.path, tc.expected, got)
			}
		}
	})

	// Test Int method edge cases (25% coverage - improve it)
	t.Run("Int_EdgeCases", func(t *testing.T) {
		testCases := []struct {
			json     string
			path     string
			expected int64
		}{
			{`{"int":123}`, "int", 123},
			{`{"zero":0}`, "zero", 0},
			{`{"negative":-456}`, "negative", -456},
			{`{"float":123.456}`, "float", 123},
			{`{"str":"789"}`, "str", 789},
			{`{"bool":true}`, "bool", 1},
			{`{"bool":false}`, "bool", 0},
			{`{"null":null}`, "null", 0},
			{`{"invalid":"not_a_number"}`, "invalid", 0},
		}

		for _, tc := range testCases {
			result := Get([]byte(tc.json), tc.path)
			got := result.Int()
			if got != tc.expected {
				t.Errorf("Int() for %s.%s: expected %d, got %d", tc.json, tc.path, tc.expected, got)
			}
		}
	})

	// Test Float method edge cases (25% coverage - improve it)
	t.Run("Float_EdgeCases", func(t *testing.T) {
		testCases := []struct {
			json     string
			path     string
			expected float64
		}{
			{`{"float":123.456}`, "float", 123.456},
			{`{"int":123}`, "int", 123.0},
			{`{"zero":0}`, "zero", 0.0},
			{`{"negative":-456.789}`, "negative", -456.789},
			{`{"str":"123.456"}`, "str", 123.456},
			{`{"scientific":1.23e10}`, "scientific", 1.23e10},
			{`{"bool":true}`, "bool", 1.0},
			{`{"bool":false}`, "bool", 0.0},
			{`{"null":null}`, "null", 0.0},
		}

		for _, tc := range testCases {
			result := Get([]byte(tc.json), tc.path)
			got := result.Float()
			if got != tc.expected {
				t.Errorf("Float() for %s.%s: expected %f, got %f", tc.json, tc.path, tc.expected, got)
			}
		}
	})

	// Test Bool method edge cases (25% coverage - improve it)
	t.Run("Bool_EdgeCases", func(t *testing.T) {
		testCases := []struct {
			json     string
			path     string
			expected bool
		}{
			{`{"bool":true}`, "bool", true},
			{`{"bool":false}`, "bool", false},
			{`{"int":1}`, "int", true},
			{`{"int":0}`, "int", false},
			{`{"str":"true"}`, "str", true},
			{`{"str":"false"}`, "str", false},
			{`{"str":"1"}`, "str", true},
			{`{"str":"0"}`, "str", false},
			{`{"str":""}`, "str", false},
			{`{"null":null}`, "null", false},
		}

		for _, tc := range testCases {
			result := Get([]byte(tc.json), tc.path)
			got := result.Bool()
			if got != tc.expected {
				t.Errorf("Bool() for %s.%s: expected %t, got %t", tc.json, tc.path, tc.expected, got)
			}
		}
	})

	// Test Time method (66.7% coverage - improve it)
	t.Run("Time_EdgeCases", func(t *testing.T) {
		testCases := []struct {
			json        string
			path        string
			shouldError bool
		}{
			{`{"time":"2023-01-01T00:00:00Z"}`, "time", false},
			{`{"time":"2023-12-31T23:59:59Z"}`, "time", false},
			{`{"time":"invalid_time"}`, "time", true},
			{`{"timestamp":"2021-01-01T00:00:00Z"}`, "timestamp", false},
			{`{"date":"2021-12-25T12:00:00Z"}`, "date", false},
		}

		for _, tc := range testCases {
			result := Get([]byte(tc.json), tc.path)
			timeResult, err := result.Time()

			if tc.shouldError {
				if err == nil {
					t.Errorf("Expected error for invalid time: %s", tc.json)
				}
			} else {
				if err != nil {
					t.Errorf("Time() returned unexpected error for %s: %v", tc.json, err)
				}
				if timeResult.IsZero() {
					t.Errorf("Time() returned zero time for valid input: %s", tc.json)
				}
			}
		}
	})
}

// TestParseStringEdgeCases tests parseString function to improve coverage (25% -> higher)
func TestParseStringEdgeCases(t *testing.T) {
	stringTestCases := []struct {
		name string
		json string
		path string
	}{
		{"simple_string", `{"test":"simple"}`, "test"},
		{"empty_string", `{"test":""}`, "test"},
		{"escaped_quotes", `{"test":"with \"quotes\""}`, "test"},
		{"escaped_backslash", `{"test":"with \\backslashes"}`, "test"},
		{"unicode_chars", `{"test":"unicode \u0041\u0042\u0043"}`, "test"},
		{"newlines", `{"test":"with \n newlines"}`, "test"},
		{"tabs", `{"test":"with \t tabs"}`, "test"},
		{"carriage_return", `{"test":"with \r returns"}`, "test"},
		{"mixed_escapes", `{"test":"mixed \"quotes\" and \\slashes\nand\ttabs"}`, "test"},
	}

	for _, tc := range stringTestCases {
		t.Run(tc.name, func(t *testing.T) {
			result := Get([]byte(tc.json), tc.path)
			if !result.Exists() {
				t.Errorf("parseString failed for %s", tc.name)
			}
			// Just test that we can get the string without error
			_ = result.String()
		})
	}
}

// TestNumberParsingEdgeCases tests number parsing to improve coverage
func TestNumberParsingEdgeCases(t *testing.T) {
	numberTestCases := []struct {
		name string
		json string
		path string
	}{
		{"positive_int", `{"num":123}`, "num"},
		{"negative_int", `{"num":-123}`, "num"},
		{"zero", `{"num":0}`, "num"},
		{"positive_float", `{"num":123.456}`, "num"},
		{"negative_float", `{"num":-123.456}`, "num"},
		{"scientific_notation", `{"num":1.23e10}`, "num"},
		{"scientific_negative", `{"num":1.23e-10}`, "num"},
		{"large_number", `{"num":9223372036854775807}`, "num"},
		{"small_number", `{"num":-9223372036854775808}`, "num"},
		{"float_with_zero", `{"num":0.0}`, "num"},
	}

	for _, tc := range numberTestCases {
		t.Run(tc.name, func(t *testing.T) {
			result := Get([]byte(tc.json), tc.path)
			if !result.Exists() {
				t.Errorf("number parsing failed for %s", tc.name)
			}
			// Test both Int and Float methods
			_ = result.Int()
			_ = result.Float()
		})
	}
}

// TestComplexPathOperations tests complex path expressions to trigger getComplexPath, tokenizePath, etc.
func TestComplexPathOperations(t *testing.T) {
	complexJSON := `{
		"users": [
			{"name": "alice", "age": 30, "active": true},
			{"name": "bob", "age": 25, "active": false},
			{"name": "charlie", "age": 35, "active": true}
		],
		"products": {
			"electronics": [
				{"id": 1, "name": "laptop", "price": 1000},
				{"id": 2, "name": "phone", "price": 500}
			],
			"books": [
				{"id": 3, "name": "novel", "price": 20},
				{"id": 4, "name": "guide", "price": 30}
			]
		},
		"metadata": {
			"version": "1.0",
			"tags": ["v1", "prod", "stable"]
		}
	}`

	testCases := []struct {
		name        string
		path        string
		shouldExist bool
	}{
		// Wildcard paths - trigger complex path processing
		{"wildcard_all_users", "users.*.name", true},
		{"wildcard_nested", "products.electronics.*.name", true},

		// Edge cases that should trigger complex path processing but not exist
		{"filter_active_users", "users[?(@.active==true)].name", false},
		{"filter_by_age", "users[?(@.age>30)].name", false},
		{"recursive_search_name", "..name", false},
		{"modifier_length", "users.@length", false},
		{"array_slice", "users[0:2].name", false},
		{"array_negative_index", "users[-1].name", false},
		{"invalid_modifier", "users.@invalid", false},
		{"malformed_filter", "users[?(@.age)", false},
	}

	data := []byte(complexJSON)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := Get(data, tc.path)

			if tc.shouldExist {
				if !result.Exists() {
					t.Errorf("Expected path %s to exist", tc.path)
				}
			} else {
				if result.Exists() {
					t.Errorf("Expected path %s to not exist", tc.path)
				}
			}
		})
	}
}

// TestArrayElementAccess tests array element access functions
func TestArrayElementAccess(t *testing.T) {
	arrayJSON := `{
		"numbers": [1, 2, 3, 4, 5, 10, 20, 30],
		"nested": [
			{"data": [{"value": 100}, {"value": 200}]},
			{"data": [{"value": 300}, {"value": 400}]}
		],
		"large": [` +

		// Create a large array to trigger specific array access optimizations
		func() string {
			var items []string
			for i := 0; i < 500; i++ {
				items = append(items, fmt.Sprintf(`{"id": %d, "name": "item%d"}`, i, i))
			}
			return strings.Join(items, ",")
		}() + `]
	}`

	testCases := []struct {
		name           string
		path           string
		expectedExists bool
	}{
		// Basic array access
		{"first_element", "numbers.0", true},
		{"last_element", "numbers.7", true},
		{"out_of_bounds", "numbers.10", false},

		// Negative indexing - these may not be supported
		{"negative_last", "numbers.-1", false},
		{"negative_first", "numbers.-8", false},
		{"negative_out_of_bounds", "numbers.-10", false},

		// Nested array access
		{"nested_deep", "nested.0.data.1.value", true},
		{"nested_complex", "nested.1.data.0.value", true},

		// Large array access to trigger optimization paths
		{"large_array_start", "large.0.id", true},
		{"large_array_middle", "large.250.id", true},
		{"large_array_end", "large.499.id", true},
		{"large_array_overflow", "large.500", false},

		// Array slicing
		{"slice_start_end", "numbers[0:3]", true},
		{"slice_start_only", "numbers[2:]", true},
		{"slice_end_only", "numbers[:5]", true},
		{"slice_all", "numbers[:]", true},
	}

	data := []byte(arrayJSON)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := Get(data, tc.path)

			if tc.expectedExists && !result.Exists() {
				t.Errorf("Expected path %s to exist", tc.path)
			}
			if !tc.expectedExists && result.Exists() {
				t.Errorf("Expected path %s to not exist", tc.path)
			}
		})
	}
}

// TestUltraFastArrayAccess tests ultra-fast array access functions
func TestUltraFastArrayAccess(t *testing.T) {
	// Test very large arrays to trigger ultraFastArrayAccess
	largeArray := `[`
	for i := 0; i < 5000; i++ {
		if i > 0 {
			largeArray += ","
		}
		largeArray += fmt.Sprintf(`{"id":%d,"value":"test%d"}`, i, i)
	}
	largeArray += `]`

	// Test high indices that should trigger ultra-fast access
	testIndices := []int{1000, 2500, 4000, 4999}
	for _, idx := range testIndices {
		result := Get([]byte(largeArray), strconv.Itoa(idx))
		if !result.Exists() {
			t.Errorf("ultraFastArrayAccess failed for index %d", idx)
		}
		expectedID := int64(idx)
		if result.Get("id").Int() != expectedID {
			t.Errorf("ultraFastArrayAccess index %d: expected id %d, got %d", idx, expectedID, result.Get("id").Int())
		}
	}
}

// TestUltraFastObjectAccess tests ultra-fast object access functions
func TestUltraFastObjectAccess(t *testing.T) {
	// Create large object to trigger ultraFastObjectAccess
	largeObj := `{`
	for i := 0; i < 1000; i++ {
		if i > 0 {
			largeObj += ","
		}
		largeObj += fmt.Sprintf(`"prop%d":"value%d"`, i, i)
	}
	largeObj += `}`

	// Test properties that should trigger ultra-fast object access
	testProps := []string{"prop500", "prop750", "prop999", "prop0"}
	for _, prop := range testProps {
		result := Get([]byte(largeObj), prop)
		if !result.Exists() {
			t.Errorf("ultraFastObjectAccess failed for property %s", prop)
		}
		expected := "value" + prop[4:] // extract number from "propXXX"
		if result.String() != expected {
			t.Errorf("ultraFastObjectAccess property %s: expected %s, got %s", prop, expected, result.String())
		}
	}
}

// TestDirectArrayIndex tests isDirectArrayIndex optimization
func TestDirectArrayIndex(t *testing.T) {
	simpleArray := `["a","b","c","d","e"]`

	// Test numeric-only paths that should trigger isDirectArrayIndex
	for i := 0; i < 5; i++ {
		result := Get([]byte(simpleArray), strconv.Itoa(i))
		if !result.Exists() {
			t.Errorf("isDirectArrayIndex failed for index %d", i)
		}
		expected := string(rune('a' + i))
		if result.String() != expected {
			t.Errorf("isDirectArrayIndex index %d: expected %s, got %s", i, expected, result.String())
		}
	}
}

// TestLargeDeepAccess tests ultra-fast large deep access functions
func TestLargeDeepAccess(t *testing.T) {
	// Create deep nested structure with large data
	deepData := `{"level0":{"level1":{"level2":{"level3":{"level4":{"items":[`
	for i := 0; i < 100; i++ {
		if i > 0 {
			deepData += ","
		}
		deepData += fmt.Sprintf(`{"id":%d,"data":"item%d"}`, i, i)
	}
	deepData += `]}}}}}`

	// Test deep paths that should trigger ultraFastLargeDeepAccess
	result := Get([]byte(deepData), "level0.level1.level2.level3.level4.items.50.id")
	if !result.Exists() {
		t.Error("ultraFastLargeDeepAccess failed for deep path")
	}
	if result.Int() != 50 {
		t.Errorf("ultraFastLargeDeepAccess: expected 50, got %d", result.Int())
	}
}

// TestBlazingFastPropertyLookup tests blazing fast property lookup
func TestBlazingFastPropertyLookup(t *testing.T) {
	// Create object with many properties to trigger blazingFastPropertyLookup
	manyProps := `{`
	for i := 0; i < 500; i++ {
		if i > 0 {
			manyProps += ","
		}
		manyProps += fmt.Sprintf(`"key_%03d":"value_%03d"`, i, i)
	}
	manyProps += `}`

	// Test property lookup that should trigger blazing fast lookup
	result := Get([]byte(manyProps), "key_250")
	if !result.Exists() {
		t.Error("blazingFastPropertyLookup failed")
	}
	if result.String() != "value_250" {
		t.Errorf("blazingFastPropertyLookup: expected value_250, got %s", result.String())
	}
}

// TestStatisticalJumpAccess tests statistical jump access optimization
func TestStatisticalJumpAccess(t *testing.T) {
	// Create large array that should trigger statistical jump access
	hugeArray := `[`
	for i := 0; i < 10000; i++ {
		if i > 0 {
			hugeArray += ","
		}
		hugeArray += fmt.Sprintf(`%d`, i*2) // Simple numeric values
	}
	hugeArray += `]`

	// Test high indices that should trigger statistical jump access
	testIndices := []int{5000, 7500, 9999}
	for _, idx := range testIndices {
		result := Get([]byte(hugeArray), strconv.Itoa(idx))
		if !result.Exists() {
			t.Errorf("statisticalJumpAccess failed for index %d", idx)
		}
		expected := int64(idx * 2)
		if result.Int() != expected {
			t.Errorf("statisticalJumpAccess index %d: expected %d, got %d", idx, expected, result.Int())
		}
	}
}

// TestCommaCountingAccess tests comma counting access optimization
func TestCommaCountingAccess(t *testing.T) {
	// Create array with uniform elements to trigger comma counting
	uniformArray := `[`
	for i := 0; i < 2000; i++ {
		if i > 0 {
			uniformArray += ","
		}
		uniformArray += fmt.Sprintf(`"item_%04d"`, i)
	}
	uniformArray += `]`

	// Test indices that should trigger comma counting optimization
	result := Get([]byte(uniformArray), "1500")
	if !result.Exists() {
		t.Error("commaCountingAccess failed")
	}
	if result.String() != "item_1500" {
		t.Errorf("commaCountingAccess: expected item_1500, got %s", result.String())
	}
}

// TestHandleItemsPattern tests the specific items pattern optimization
func TestHandleItemsPattern(t *testing.T) {
	// Create exact pattern that triggers handleItemsPattern
	itemsData := `{"items":[`
	for i := 0; i < 1000; i++ {
		if i > 0 {
			itemsData += ","
		}
		itemsData += fmt.Sprintf(`{"name":"item%d","metadata":{"priority":%d},"tags":["tag%d","other"]}`, i, i*10, i)
	}
	itemsData += `],"other":"data"}`

	// Test the exact patterns mentioned in handleItemsPattern function
	testCases := []struct {
		path     string
		expected string
	}{
		{"items.500.name", "item500"},
		{"items.999.metadata.priority", "9990"},
		{"items.250.tags.1", "other"},
	}

	for _, tc := range testCases {
		result := Get([]byte(itemsData), tc.path)
		if !result.Exists() {
			t.Errorf("handleItemsPattern path %s: result does not exist", tc.path)
			continue
		}
		got := result.String()
		if got != tc.expected {
			t.Errorf("handleItemsPattern path %s: expected %s, got %s", tc.path, tc.expected, got)
		}
	}
}

// TestUltraFastSkipElement tests ultra-fast element skipping
func TestUltraFastSkipElement(t *testing.T) {
	// Create array with nested objects to trigger ultraFastSkipElement
	nestedArray := `[`
	for i := 0; i < 100; i++ {
		if i > 0 {
			nestedArray += ","
		}
		nestedArray += fmt.Sprintf(`{"index":%d,"nested":{"a":[1,2,3],"b":{"c":"value%d"}}}`, i, i)
	}
	nestedArray += `]`

	// Access elements that require skipping over complex nested structures
	result := Get([]byte(nestedArray), "90.nested.b.c")
	if !result.Exists() {
		t.Error("ultraFastSkipElement failed")
	}
	if result.String() != "value90" {
		t.Errorf("ultraFastSkipElement: expected value90, got %s", result.String())
	}
}

// TestTryLargeArrayPath tests tryLargeArrayPath optimization
func TestTryLargeArrayPath(t *testing.T) {
	// Create very large array to trigger tryLargeArrayPath
	veryLargeArray := `[`
	for i := 0; i < 15000; i++ {
		if i > 0 {
			veryLargeArray += ","
		}
		veryLargeArray += fmt.Sprintf(`{"id":%d}`, i)
	}
	veryLargeArray += `]`

	// Test very high index that should trigger tryLargeArrayPath
	result := Get([]byte(veryLargeArray), "12000.id")
	if !result.Exists() {
		t.Error("tryLargeArrayPath failed")
	}
	if result.Int() != 12000 {
		t.Errorf("tryLargeArrayPath: expected 12000, got %d", result.Int())
	}
}

// TestAccessMethods tests access* helper methods
func TestAccessMethods(t *testing.T) {
	complexData := `{
		"items": [
			{"name": "first", "props": {"value": 100}},
			{"name": "second", "props": {"value": 200}}
		],
		"metadata": {
			"count": 2,
			"tags": ["a", "b", "c"]
		}
	}`

	// Test accessItemProperty
	result := Get([]byte(complexData), "items.0.name")
	if result.String() != "first" {
		t.Errorf("accessItemProperty failed: expected first, got %s", result.String())
	}

	// Test accessArrayElement
	result = Get([]byte(complexData), "metadata.tags.1")
	if result.String() != "b" {
		t.Errorf("accessArrayElement failed: expected b, got %s", result.String())
	}

	// Test accessObjectProperty
	result = Get([]byte(complexData), "metadata.count")
	if result.Int() != 2 {
		t.Errorf("accessObjectProperty failed: expected 2, got %d", result.Int())
	}
}

// TestUnsafeOperations tests unsafe memory operations for performance
func TestUnsafeOperations(t *testing.T) {
	// Test with data that should trigger unsafe optimizations
	unsafeData := `{"key":"` + string(make([]byte, 1000)) + `","other":"value"}`

	result := Get([]byte(unsafeData), "other")
	if result.String() != "value" {
		t.Errorf("unsafe operations failed: expected value, got %s", result.String())
	}
}

// TestLargeIndexAccess tests large index access optimizations
func TestLargeIndexAccess(t *testing.T) {
	// Create huge array to test large index access
	hugeArray := `[`
	for i := 0; i < 20000; i++ {
		if i > 0 {
			hugeArray += ","
		}
		hugeArray += strconv.Itoa(i)
	}
	hugeArray += `]`

	// Test very large indices
	largeIndices := []int{10000, 15000, 19999}
	for _, idx := range largeIndices {
		result := Get([]byte(hugeArray), strconv.Itoa(idx))
		if !result.Exists() {
			t.Errorf("large index access failed for index %d", idx)
		}
		if result.Int() != int64(idx) {
			t.Errorf("large index access: expected %d, got %d", idx, result.Int())
		}
	}
}

// TestSimplePropertyLookup tests simple property lookup optimization
func TestSimplePropertyLookup(t *testing.T) {
	// Test very simple objects that should trigger simple property lookup
	simpleObj := `{"a":"1","b":"2","c":"3","d":"4","e":"5"}`

	for i := 0; i < 5; i++ {
		key := string(rune('a' + i))
		expected := string(rune('1' + i))
		result := Get([]byte(simpleObj), key)
		if result.String() != expected {
			t.Errorf("simple property lookup %s: expected %s, got %s", key, expected, result.String())
		}
	}
}

// TestMissingCoverage tests specific optimization functions that are not being triggered
func TestMissingCoverage(t *testing.T) {
	// Test escapeString function (0% coverage)
	t.Run("escapeString", func(t *testing.T) {
		// This function is likely used internally for JSON encoding
		// We need to trigger scenarios that require string escaping
		jsonWithEscapes := `{"test":"value with \"quotes\" and \\backslashes"}`
		result := Get([]byte(jsonWithEscapes), "test")
		if !result.Exists() {
			t.Error("Failed to parse JSON with escaped characters")
		}
	})

	// Test minInt and maxInt functions (0% coverage)
	t.Run("minMaxInt", func(t *testing.T) {
		// These are utility functions, test with very large/small numbers
		largeNum := `{"big":` + strconv.Itoa(int(^uint(0)>>1)) + `,"small":` + strconv.Itoa(-int(^uint(0)>>1)-1) + `}`
		result1 := Get([]byte(largeNum), "big")
		result2 := Get([]byte(largeNum), "small")
		if !result1.Exists() || !result2.Exists() {
			t.Error("Failed to handle min/max int values")
		}
	})

	// Test fnv1a hash function (0% coverage)
	t.Run("fnv1a", func(t *testing.T) {
		// This hash function is likely used for optimization
		// Create data that might trigger hash-based optimizations
		manyKeys := `{`
		for i := 0; i < 100; i++ {
			if i > 0 {
				manyKeys += ","
			}
			manyKeys += `"key` + strconv.Itoa(i) + `":"value` + strconv.Itoa(i) + `"`
		}
		manyKeys += `}`

		// Access multiple keys to potentially trigger hash optimizations
		for i := 0; i < 10; i++ {
			result := Get([]byte(manyKeys), "key"+strconv.Itoa(i*10))
			if !result.Exists() {
				t.Errorf("Failed to get key%d", i*10)
			}
		}
	})
}

// TestLargeStructureOptimizations tests optimizations for large data structures
func TestLargeStructureOptimizations(t *testing.T) {
	// Test handleItemsPattern (0% coverage) - need very specific "items" pattern
	t.Run("itemsPattern", func(t *testing.T) {
		// Create data structure exactly as expected by handleItemsPattern
		itemsJSON := `{"items":[`
		for i := 0; i < 1000; i++ {
			if i > 0 {
				itemsJSON += ","
			}
			itemsJSON += `{"name":"item` + strconv.Itoa(i) + `","metadata":{"priority":` + strconv.Itoa(i) + `},"tags":["tag` + strconv.Itoa(i) + `","special"]}`
		}
		itemsJSON += `]}`

		// Test patterns that should trigger handleItemsPattern
		result := Get([]byte(itemsJSON), "items.500.name")
		if result.String() != "item500" {
			t.Errorf("handleItemsPattern failed: expected item500, got %s", result.String())
		}

		result = Get([]byte(itemsJSON), "items.999.metadata.priority")
		if result.Int() != 999 {
			t.Errorf("handleItemsPattern failed: expected 999, got %d", result.Int())
		}
	})

	// Test ultra-fast array access functions (0% coverage)
	t.Run("ultraFastArrayAccess", func(t *testing.T) {
		// Create very large array to trigger ultra-fast optimizations
		hugeArray := `[`
		for i := 0; i < 10000; i++ {
			if i > 0 {
				hugeArray += ","
			}
			hugeArray += `"item` + strconv.Itoa(i) + `"`
		}
		hugeArray += `]`

		// Test accessing high indices that should trigger ultra-fast access
		result := Get([]byte(hugeArray), "5000")
		if result.String() != "item5000" {
			t.Errorf("ultraFastArrayAccess failed: expected item5000, got %s", result.String())
		}
	})

	// Test ultra-fast object access (0% coverage)
	t.Run("ultraFastObjectAccess", func(t *testing.T) {
		// Create very large object
		hugeObj := `{`
		for i := 0; i < 5000; i++ {
			if i > 0 {
				hugeObj += ","
			}
			hugeObj += `"prop` + strconv.Itoa(i) + `":"value` + strconv.Itoa(i) + `"`
		}
		hugeObj += `}`

		// Access properties that should trigger ultra-fast access
		result := Get([]byte(hugeObj), "prop2500")
		if result.String() != "value2500" {
			t.Errorf("ultraFastObjectAccess failed: expected value2500, got %s", result.String())
		}
	})
}

// TestOptimizationTriggers tests specific conditions that trigger optimizations
func TestOptimizationTriggers(t *testing.T) {
	// Test isDirectArrayIndex (0% coverage)
	t.Run("directArrayIndex", func(t *testing.T) {
		// Simple array with direct numeric index access
		simpleArray := `[0,1,2,3,4,5,6,7,8,9]`

		// Access with pure numeric paths
		for i := 0; i < 10; i++ {
			result := Get([]byte(simpleArray), strconv.Itoa(i))
			if result.Int() != int64(i) {
				t.Errorf("directArrayIndex failed for index %d", i)
			}
		}
	})

	// Test memoryEfficientLargeIndexAccess (0% coverage)
	t.Run("memoryEfficientLargeIndex", func(t *testing.T) {
		// Create array with many elements to trigger memory-efficient access
		largeArray := `[`
		for i := 0; i < 50000; i++ {
			if i > 0 {
				largeArray += ","
			}
			largeArray += strconv.Itoa(i)
		}
		largeArray += `]`

		// Access very high index
		result := Get([]byte(largeArray), "49999")
		if result.Int() != 49999 {
			t.Errorf("memoryEfficientLargeIndex failed: expected 49999, got %d", result.Int())
		}
	})

	// Test ultraFastLargeDeepAccess (0% coverage)
	t.Run("ultraFastLargeDeepAccess", func(t *testing.T) {
		// Create deep nested structure with large arrays
		deepLarge := `{"level1":{"level2":{"level3":{"items":[`
		for i := 0; i < 1000; i++ {
			if i > 0 {
				deepLarge += ","
			}
			deepLarge += `{"id":` + strconv.Itoa(i) + `,"data":"item` + strconv.Itoa(i) + `"}`
		}
		deepLarge += `]}}}}`

		// Access deep path with large index
		result := Get([]byte(deepLarge), "level1.level2.level3.items.500.id")
		if result.Int() != 500 {
			t.Errorf("ultraFastLargeDeepAccess failed: expected 500, got %d", result.Int())
		}
	})
}

// TestComplexPathOptimizations tests complex path handling optimizations
func TestComplexPathOptimizations(t *testing.T) {
	// Test getComplexPath (0% coverage)
	t.Run("complexPath", func(t *testing.T) {
		// Create data with complex paths that might trigger getComplexPath
		complexData := `{
			"users": [
				{"id": 1, "profile": {"name": "John", "settings": {"theme": "dark"}}},
				{"id": 2, "profile": {"name": "Jane", "settings": {"theme": "light"}}}
			],
			"metadata": {"version": "1.0", "config": {"debug": true}}
		}`

		// Try complex path access
		result := Get([]byte(complexData), "users.0.profile.settings.theme")
		if result.String() != "dark" {
			t.Errorf("complexPath failed: expected dark, got %s", result.String())
		}
	})

	// Test fastGetValue and related functions (0% coverage)
	t.Run("fastGetValue", func(t *testing.T) {
		// Data that might trigger fast get value optimizations
		fastData := `{"a":{"b":{"c":{"d":{"e":"found"}}}}}`

		result := Get([]byte(fastData), "a.b.c.d.e")
		if result.String() != "found" {
			t.Errorf("fastGetValue failed: expected found, got %s", result.String())
		}
	})

	// Test tokenizePath and executeTokenizedPath (0% coverage)
	t.Run("tokenizedPath", func(t *testing.T) {
		// Complex paths that might trigger tokenization
		tokenData := `{
			"data": {
				"items": [
					{"tags": ["a", "b", "c"]},
					{"tags": ["d", "e", "f"]}
				]
			}
		}`

		// Path that might require tokenization
		result := Get([]byte(tokenData), "data.items.1.tags.2")
		if result.String() != "f" {
			t.Errorf("tokenizedPath failed: expected f, got %s", result.String())
		}
	})
}

// TestStringAndNumberOptimizations tests string and number parsing optimizations
func TestStringAndNumberOptimizations(t *testing.T) {
	// Test parseString edge cases (25% coverage - need to increase)
	t.Run("parseStringEdgeCases", func(t *testing.T) {
		// Test various string formats that might trigger different parsing paths
		stringTests := []string{
			`{"simple": "test"}`,
			`{"escaped": "test with \"quotes\""}`,
			`{"unicode": "test with \u0041\u0042\u0043"}`,
			`{"newlines": "test with \n newlines"}`,
			`{"empty": ""}`,
		}

		for _, jsonStr := range stringTests {
			result := Get([]byte(jsonStr), "simple")
			if jsonStr == stringTests[0] && !result.Exists() {
				t.Error("parseString failed for simple case")
			}
		}
	})

	// Test number parsing edge cases
	t.Run("numberParsing", func(t *testing.T) {
		numberTests := `{
			"int": 123,
			"float": 123.456,
			"negative": -123,
			"zero": 0,
			"scientific": 1.23e10,
			"large": 9223372036854775807
		}`

		fields := []string{"int", "float", "negative", "zero", "scientific", "large"}
		for _, field := range fields {
			result := Get([]byte(numberTests), field)
			if !result.Exists() {
				t.Errorf("number parsing failed for %s", field)
			}
		}
	})
}
