package njson

import (
	"strconv"
	"strings"
	"testing"
)

// Common validation helpers for table-driven tests
func validateStringValue(expectedValue string) func(t *testing.T, result []byte) {
	return func(t *testing.T, result []byte) {
		t.Helper()
		getValue := Get(result, "name")
		if !getValue.Exists() || getValue.String() != expectedValue {
			t.Errorf("Expected %q, got %q", expectedValue, getValue.String())
		}
	}
}

func validateIntValue(path string, expectedValue int64) func(t *testing.T, result []byte) {
	return func(t *testing.T, result []byte) {
		t.Helper()
		getValue := Get(result, path)
		if !getValue.Exists() || getValue.Int() != expectedValue {
			t.Errorf("Expected %d, got %d", expectedValue, getValue.Int())
		}
	}
}

func validateFloatValue(path string, expectedValue float64) func(t *testing.T, result []byte) {
	return func(t *testing.T, result []byte) {
		t.Helper()
		getValue := Get(result, path)
		if !getValue.Exists() || getValue.Float() != expectedValue {
			t.Errorf("Expected %f, got %f", expectedValue, getValue.Float())
		}
	}
}

func validateBoolValue(path string, expectedValue bool) func(t *testing.T, result []byte) {
	return func(t *testing.T, result []byte) {
		t.Helper()
		getValue := Get(result, path)
		if !getValue.Exists() || getValue.Bool() != expectedValue {
			t.Errorf("Expected %v, got %v", expectedValue, getValue.Bool())
		}
	}
}

func validateNullValue(path string) func(t *testing.T, result []byte) {
	return func(t *testing.T, result []byte) {
		t.Helper()
		getValue := Get(result, path)
		if !getValue.Exists() || !getValue.IsNull() {
			t.Error("Expected null value")
		}
	}
}

func validateFieldExists(path string, expectedValue string) func(t *testing.T, result []byte) {
	return func(t *testing.T, result []byte) {
		t.Helper()
		getValue := Get(result, path)
		if !getValue.Exists() || getValue.String() != expectedValue {
			t.Errorf("Expected %q at path %s, got %q", expectedValue, path, getValue.String())
		}
	}
}

func validateMultipleFields(validations map[string]interface{}) func(t *testing.T, result []byte) {
	return func(t *testing.T, result []byte) {
		t.Helper()
		for path, expected := range validations {
			getValue := Get(result, path)
			if !getValue.Exists() {
				t.Errorf("Expected field %s to exist", path)
				continue
			}

			switch expectedVal := expected.(type) {
			case string:
				if getValue.String() != expectedVal {
					t.Errorf("Expected %q at path %s, got %q", expectedVal, path, getValue.String())
				}
			case int64:
				if getValue.Int() != expectedVal {
					t.Errorf("Expected %d at path %s, got %d", expectedVal, path, getValue.Int())
				}
			case int:
				if getValue.Int() != int64(expectedVal) {
					t.Errorf("Expected %d at path %s, got %d", expectedVal, path, getValue.Int())
				}
			case bool:
				if getValue.Bool() != expectedVal {
					t.Errorf("Expected %v at path %s, got %v", expectedVal, path, getValue.Bool())
				}
			}
		}
	}
}

// TestSet_BasicOperations tests basic SET functionality using table-driven tests
func TestSet_BasicOperations(t *testing.T) {
	tests := []struct {
		name      string
		json      []byte
		path      string
		value     interface{}
		wantError bool
		validate  func(t *testing.T, result []byte)
	}{
		{
			name:      "set_string_value",
			json:      []byte(`{"name":"John","age":30}`),
			path:      "name",
			value:     "Jane",
			wantError: false,
			validate:  validateStringValue("Jane"),
		},
		{
			name:      "set_int_value",
			json:      []byte(`{"name":"John","age":30}`),
			path:      "age",
			value:     31,
			wantError: false,
			validate:  validateIntValue("age", 31),
		},
		{
			name:      "add_new_field",
			json:      []byte(`{"name":"John","age":30}`),
			path:      "email",
			value:     "jane@example.com",
			wantError: false,
			validate:  validateFieldExists("email", "jane@example.com"),
		},
		{
			name:      "set_float_value",
			json:      []byte(`{"price":10.5}`),
			path:      "price",
			value:     19.99,
			wantError: false,
			validate:  validateFloatValue("price", 19.99),
		},
		{
			name:      "set_bool_value",
			json:      []byte(`{"active":false}`),
			path:      "active",
			value:     true,
			wantError: false,
			validate:  validateBoolValue("active", true),
		},
		{
			name:      "set_null_value",
			json:      []byte(`{"value":"something"}`),
			path:      "value",
			value:     nil,
			wantError: false,
			validate:  validateNullValue("value"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Set(tt.json, tt.path, tt.value)

			if (err != nil) != tt.wantError {
				t.Errorf("Set() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestSet_ArrayOperations tests array SET functionality using table-driven tests
func TestSet_ArrayOperations(t *testing.T) {
	tests := []struct {
		name      string
		json      []byte
		path      string
		value     interface{}
		wantError bool
		validate  func(t *testing.T, result []byte)
	}{
		{
			name:      "update_array_element",
			json:      []byte(`{"items":["apple","banana","cherry"]}`),
			path:      "items.1",
			value:     "orange",
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				getValue := Get(result, "items.1")
				if !getValue.Exists() || getValue.String() != "orange" {
					t.Errorf("Expected 'orange', got %q", getValue.String())
				}
			},
		},
		{
			name:      "append_to_array",
			json:      []byte(`{"items":["apple","banana"]}`),
			path:      "items.2",
			value:     "cherry",
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				getValue := Get(result, "items.2")
				if !getValue.Exists() || getValue.String() != "cherry" {
					t.Errorf("Expected 'cherry', got %q", getValue.String())
				}
			},
		},
		{
			name:      "update_array_object_property",
			json:      []byte(`{"users":[{"name":"Alice","age":25},{"name":"Bob","age":30}]}`),
			path:      "users.1.name",
			value:     "Robert",
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				getValue := Get(result, "users.1.name")
				if !getValue.Exists() || getValue.String() != "Robert" {
					t.Errorf("Expected 'Robert', got %q", getValue.String())
				}
			},
		},
		{
			name:      "add_object_to_array",
			json:      []byte(`{"users":[{"name":"Alice"}]}`),
			path:      "users.1",
			value:     map[string]interface{}{"name": "Bob", "age": 30},
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				getValue := Get(result, "users.1.name")
				if !getValue.Exists() || getValue.String() != "Bob" {
					t.Errorf("Expected 'Bob', got %q", getValue.String())
				}
				getValue = Get(result, "users.1.age")
				if !getValue.Exists() || getValue.Int() != 30 {
					t.Errorf("Expected 30, got %d", getValue.Int())
				}
			},
		},
		{
			name:      "extend_array_with_gap",
			json:      []byte(`{"items":["a","b"]}`),
			path:      "items.5",
			value:     "f",
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				getValue := Get(result, "items.5")
				if !getValue.Exists() || getValue.String() != "f" {
					t.Errorf("Expected 'f', got %q", getValue.String())
				}
				// Check that nulls were added for gaps
				getValue = Get(result, "items.2")
				if !getValue.Exists() || !getValue.IsNull() {
					t.Errorf("Expected null at items.2")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Set(tt.json, tt.path, tt.value)

			if (err != nil) != tt.wantError {
				t.Errorf("Set() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestSet_NestedOperations tests nested SET functionality using table-driven tests
func TestSet_NestedOperations(t *testing.T) {
	tests := []struct {
		name      string
		json      []byte
		path      string
		value     interface{}
		wantError bool
		validate  func(t *testing.T, result []byte)
	}{
		{
			name:      "update_nested_property",
			json:      []byte(`{"user":{"name":"Alice","profile":{"age":25}}}`),
			path:      "user.profile.age",
			value:     26,
			wantError: false,
			validate:  validateIntValue("user.profile.age", 26),
		},
		{
			name:      "create_deep_nested_path",
			json:      []byte(`{"user":{"name":"Alice"}}`),
			path:      "user.profile.settings.theme",
			value:     "dark",
			wantError: false,
			validate:  validateFieldExists("user.profile.settings.theme", "dark"),
		},
		{
			name:      "create_very_deep_path",
			json:      []byte(`{}`),
			path:      "a.b.c.d.e.f.g",
			value:     "deep",
			wantError: false,
			validate:  validateFieldExists("a.b.c.d.e.f.g", "deep"),
		},
		{
			name:      "mixed_array_object_creation",
			json:      []byte(`{}`),
			path:      "users.0.profile.tags.1",
			value:     "admin",
			wantError: false,
			validate:  validateFieldExists("users.0.profile.tags.1", "admin"),
		},
		{
			name:      "create_nested_in_array",
			json:      []byte(`{}`),
			path:      "items.0.nested.property",
			value:     "value",
			wantError: false,
			validate:  validateFieldExists("items.0.nested.property", "value"),
		},
		{
			name:      "deep_array_nesting",
			json:      []byte(`{}`),
			path:      "level1.2.level2.1.level3.0",
			value:     "deep_array_value",
			wantError: false,
			validate:  validateFieldExists("level1.2.level2.1.level3.0", "deep_array_value"),
		},
		{
			name:      "multiple_nested_updates",
			json:      []byte(`{"config":{"db":{"host":"localhost"}}}`),
			path:      "config.db.port",
			value:     5432,
			wantError: false,
			validate: validateMultipleFields(map[string]interface{}{
				"config.db.host": "localhost",
				"config.db.port": 5432,
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Set(tt.json, tt.path, tt.value)

			if (err != nil) != tt.wantError {
				t.Errorf("Set() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestSetString_Operations tests SetString function using table-driven tests
func TestSetString_Operations(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		path      string
		value     interface{}
		wantError bool
		validate  func(t *testing.T, result string)
	}{
		{
			name:      "set_string_from_string_input",
			json:      `{"name":"John","age":30}`,
			path:      "name",
			value:     "Jane",
			wantError: false,
			validate: func(t *testing.T, result string) {
				getValue := GetString(result, "name")
				if !getValue.Exists() || getValue.String() != "Jane" {
					t.Errorf("Expected 'Jane', got %q", getValue.String())
				}
			},
		},
		{
			name:      "add_field_string_input",
			json:      `{"name":"John"}`,
			path:      "email",
			value:     "john@example.com",
			wantError: false,
			validate: func(t *testing.T, result string) {
				getValue := GetString(result, "email")
				if !getValue.Exists() || getValue.String() != "john@example.com" {
					t.Errorf("Expected 'john@example.com', got %q", getValue.String())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SetString(tt.json, tt.path, tt.value)

			if (err != nil) != tt.wantError {
				t.Errorf("SetString() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestSetWithOptions_Operations tests SetWithOptions function using table-driven tests
func TestSetWithOptions_Operations(t *testing.T) {
	tests := []struct {
		name      string
		json      []byte
		path      string
		value     interface{}
		options   *SetOptions
		wantError bool
		validate  func(t *testing.T, result []byte)
	}{
		{
			name:  "merge_objects",
			json:  []byte(`{"user":{"name":"Alice","age":25}}`),
			path:  "user",
			value: map[string]interface{}{"email": "alice@example.com", "city": "NYC"},
			options: &SetOptions{
				MergeObjects: true,
			},
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				// Check original fields are preserved
				getValue := Get(result, "user.name")
				if !getValue.Exists() || getValue.String() != "Alice" {
					t.Errorf("Expected original name 'Alice', got %q", getValue.String())
				}
				getValue = Get(result, "user.age")
				if !getValue.Exists() || getValue.Int() != 25 {
					t.Errorf("Expected original age 25, got %d", getValue.Int())
				}
				// Check new fields are added
				getValue = Get(result, "user.email")
				if !getValue.Exists() || getValue.String() != "alice@example.com" {
					t.Errorf("Expected new email 'alice@example.com', got %q", getValue.String())
				}
				getValue = Get(result, "user.city")
				if !getValue.Exists() || getValue.String() != "NYC" {
					t.Errorf("Expected new city 'NYC', got %q", getValue.String())
				}
			},
		},
		{
			name:  "merge_arrays",
			json:  []byte(`{"tags":["tag1","tag2"]}`),
			path:  "tags",
			value: []interface{}{"tag3", "tag4"},
			options: &SetOptions{
				MergeArrays: true,
			},
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				// Check array was extended
				getValue := Get(result, "tags.0")
				if !getValue.Exists() || getValue.String() != "tag1" {
					t.Errorf("Expected original tag1, got %q", getValue.String())
				}
				getValue = Get(result, "tags.2")
				if !getValue.Exists() || getValue.String() != "tag3" {
					t.Errorf("Expected new tag3, got %q", getValue.String())
				}
				getValue = Get(result, "tags.3")
				if !getValue.Exists() || getValue.String() != "tag4" {
					t.Errorf("Expected new tag4, got %q", getValue.String())
				}
			},
		},
		{
			name:  "replace_in_place",
			json:  []byte(`{"user":{"name":"Alice","age":25}}`),
			path:  "user.name",
			value: "Bob",
			options: &SetOptions{
				ReplaceInPlace: true,
			},
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				getValue := Get(result, "user.name")
				if !getValue.Exists() || getValue.String() != "Bob" {
					t.Errorf("Expected 'Bob', got %q", getValue.String())
				}
			},
		},
		{
			name:  "optimistic_mode",
			json:  []byte(`{"user":{"name":"Alice"}}`),
			path:  "user.name",
			value: "Bob",
			options: &SetOptions{
				Optimistic: true,
			},
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				getValue := Get(result, "user.name")
				if !getValue.Exists() || getValue.String() != "Bob" {
					t.Errorf("Expected 'Bob', got %q", getValue.String())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SetWithOptions(tt.json, tt.path, tt.value, tt.options)

			if (err != nil) != tt.wantError {
				t.Errorf("SetWithOptions() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestCompileSetPath_Operations tests CompileSetPath function using table-driven tests
func TestCompileSetPath_Operations(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError bool
		validate  func(t *testing.T, compiled *SetPath)
	}{
		{
			name:      "simple_path",
			path:      "name",
			wantError: false,
			validate: func(t *testing.T, compiled *SetPath) {
				if compiled == nil {
					t.Error("Expected compiled path to not be nil")
				}
			},
		},
		{
			name:      "nested_path",
			path:      "user.profile.name",
			wantError: false,
			validate: func(t *testing.T, compiled *SetPath) {
				if compiled == nil {
					t.Error("Expected compiled path to not be nil")
				}
			},
		},
		{
			name:      "array_path",
			path:      "users.0.name",
			wantError: false,
			validate: func(t *testing.T, compiled *SetPath) {
				if compiled == nil {
					t.Error("Expected compiled path to not be nil")
				}
			},
		},
		{
			name:      "complex_path",
			path:      "data.items.5.properties.tags.2",
			wantError: false,
			validate: func(t *testing.T, compiled *SetPath) {
				if compiled == nil {
					t.Error("Expected compiled path to not be nil")
				}
			},
		},
		{
			name:      "empty_path",
			path:      "",
			wantError: true,
			validate:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := CompileSetPath(tt.path)

			if (err != nil) != tt.wantError {
				t.Errorf("CompileSetPath() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && tt.validate != nil {
				tt.validate(t, compiled)
			}
		})
	}
}

// TestSetWithCompiledPath_Operations tests SetWithCompiledPath function
func TestSetWithCompiledPath_Operations(t *testing.T) {
	// Pre-compile paths for testing
	simplePath, _ := CompileSetPath("name")
	nestedPath, _ := CompileSetPath("user.profile.name")
	arrayPath, _ := CompileSetPath("users.0.name")

	tests := []struct {
		name      string
		json      []byte
		path      *SetPath
		value     interface{}
		options   *SetOptions
		wantError bool
		validate  func(t *testing.T, result []byte)
	}{
		{
			name:      "simple_compiled_path",
			json:      []byte(`{"name":"John","age":30}`),
			path:      simplePath,
			value:     "Jane",
			options:   nil,
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				getValue := Get(result, "name")
				if !getValue.Exists() || getValue.String() != "Jane" {
					t.Errorf("Expected 'Jane', got %q", getValue.String())
				}
			},
		},
		{
			name:      "nested_compiled_path",
			json:      []byte(`{"user":{"profile":{"name":"Alice"}}}`),
			path:      nestedPath,
			value:     "Bob",
			options:   nil,
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				getValue := Get(result, "user.profile.name")
				if !getValue.Exists() || getValue.String() != "Bob" {
					t.Errorf("Expected 'Bob', got %q", getValue.String())
				}
			},
		},
		{
			name:      "array_compiled_path",
			json:      []byte(`{"users":[{"name":"Alice"},{"name":"Bob"}]}`),
			path:      arrayPath,
			value:     "Charlie",
			options:   nil,
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				getValue := Get(result, "users.0.name")
				if !getValue.Exists() || getValue.String() != "Charlie" {
					t.Errorf("Expected 'Charlie', got %q", getValue.String())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SetWithCompiledPath(tt.json, tt.path, tt.value, tt.options)

			if (err != nil) != tt.wantError {
				t.Errorf("SetWithCompiledPath() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestDelete_Operations tests Delete function using table-driven tests
func TestDelete_Operations(t *testing.T) {
	tests := []struct {
		name      string
		json      []byte
		path      string
		wantError bool
		validate  func(t *testing.T, result []byte)
	}{
		{
			name:      "delete_simple_field",
			json:      []byte(`{"name":"John","age":30,"city":"NYC"}`),
			path:      "city",
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				getValue := Get(result, "city")
				if getValue.Exists() {
					t.Error("Expected city field to be deleted")
				}
				// Ensure other fields remain
				getValue = Get(result, "name")
				if !getValue.Exists() || getValue.String() != "John" {
					t.Errorf("Expected name to remain")
				}
			},
		},
		{
			name:      "delete_nested_field",
			json:      []byte(`{"user":{"name":"Alice","age":25,"email":"alice@example.com"}}`),
			path:      "user.email",
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				getValue := Get(result, "user.email")
				if getValue.Exists() {
					t.Error("Expected email field to be deleted")
				}
				// Ensure other fields remain
				getValue = Get(result, "user.name")
				if !getValue.Exists() || getValue.String() != "Alice" {
					t.Error("Expected name to remain")
				}
			},
		},
		{
			name:      "delete_array_element",
			json:      []byte(`{"items":["apple","banana","cherry"]}`),
			path:      "items.1",
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				// Check array was compacted
				getValue := Get(result, "items.1")
				if !getValue.Exists() || getValue.String() != "cherry" {
					t.Errorf("Expected array to be compacted, items.1 should be 'cherry', got %q", getValue.String())
				}
				getValue = Get(result, "items.2")
				if getValue.Exists() {
					t.Error("Expected array to be shortened")
				}
			},
		},
		{
			name:      "delete_entire_object",
			json:      []byte(`{"user":{"name":"Alice"},"settings":{"theme":"dark"}}`),
			path:      "user",
			wantError: false,
			validate: func(t *testing.T, result []byte) {
				getValue := Get(result, "user")
				if getValue.Exists() {
					t.Error("Expected user object to be deleted")
				}
				getValue = Get(result, "settings")
				if !getValue.Exists() {
					t.Error("Expected settings to remain")
				}
			},
		},
		{
			name:      "delete_nonexistent_field",
			json:      []byte(`{"name":"John"}`),
			path:      "nonexistent",
			wantError: true, // Delete should return error when field doesn't exist
			validate: func(t *testing.T, result []byte) {
				// No validation needed for error case
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Delete(tt.json, tt.path)

			if (err != nil) != tt.wantError {
				t.Errorf("Delete() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestDeleteString_Operations tests DeleteString function using table-driven tests
func TestDeleteString_Operations(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		path      string
		wantError bool
		validate  func(t *testing.T, result string)
	}{
		{
			name:      "delete_from_string_json",
			json:      `{"name":"John","age":30,"city":"NYC"}`,
			path:      "city",
			wantError: false,
			validate: func(t *testing.T, result string) {
				getValue := GetString(result, "city")
				if getValue.Exists() {
					t.Error("Expected city field to be deleted")
				}
			},
		},
		{
			name:      "delete_nested_from_string",
			json:      `{"user":{"name":"Alice","email":"alice@example.com"}}`,
			path:      "user.email",
			wantError: false,
			validate: func(t *testing.T, result string) {
				getValue := GetString(result, "user.email")
				if getValue.Exists() {
					t.Error("Expected email field to be deleted")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DeleteString(tt.json, tt.path)

			if (err != nil) != tt.wantError {
				t.Errorf("DeleteString() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestSet_EdgeCases tests edge cases and error conditions using table-driven tests
func TestSet_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		json      []byte
		path      string
		value     interface{}
		wantError bool
		desc      string
	}{
		{
			name:      "empty_json_object",
			json:      []byte(`{}`),
			path:      "name",
			value:     "John",
			wantError: false,
			desc:      "Setting in empty object should work",
		},
		{
			name:      "empty_path",
			json:      []byte(`{"name":"John"}`),
			path:      "",
			value:     "value",
			wantError: true,
			desc:      "Empty path should return error",
		},
		{
			name:      "invalid_json",
			json:      []byte(`{invalid json`),
			path:      "key",
			value:     "value",
			wantError: true,
			desc:      "Invalid JSON should return error",
		},
		{
			name:      "null_json",
			json:      nil,
			path:      "key",
			value:     "value",
			wantError: true,
			desc:      "Nil JSON should return error",
		},
		{
			name:      "empty_json",
			json:      []byte(``),
			path:      "key",
			value:     "value",
			wantError: true,
			desc:      "Empty JSON should return error",
		},
		{
			name:      "special_characters_in_key",
			json:      []byte(`{}`),
			path:      "key-with-dashes",
			value:     "value",
			wantError: false,
			desc:      "Special characters in key names should work",
		},
		{
			name:      "unicode_characters",
			json:      []byte(`{}`),
			path:      "名前",
			value:     "山田太郎",
			wantError: false,
			desc:      "Unicode characters should work",
		},
		{
			name:      "very_long_path",
			json:      []byte(`{}`),
			path:      strings.Repeat("a.", 100) + "value",
			value:     "deep",
			wantError: false,
			desc:      "Very deep nesting should work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Set(tt.json, tt.path, tt.value)

			if (err != nil) != tt.wantError {
				t.Errorf("Set(%s) error = %v, wantError %v - %s", tt.path, err, tt.wantError, tt.desc)
			}
		})
	}
}

// TestSet_Performance tests performance-critical SET operations
func TestSet_Performance(t *testing.T) {
	// Generate large JSON for testing
	largeJSON := generateLargeJSONForSet(1000)

	tests := []struct {
		name  string
		json  []byte
		path  string
		value interface{}
	}{
		{
			name:  "large_json_simple_set",
			json:  largeJSON,
			path:  "metadata.updated",
			value: "2023-01-01",
		},
		{
			name:  "large_json_array_set",
			json:  largeJSON,
			path:  "items.500.active",
			value: true,
		},
		{
			name:  "large_json_deep_set",
			json:  largeJSON,
			path:  "items.100.properties.tags.0",
			value: "performance-test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Set(tt.json, tt.path, tt.value)
			if err != nil {
				t.Errorf("Performance test failed: %v", err)
			}
			if len(result) == 0 {
				t.Error("Expected non-empty result")
			}
		})
	}
}

// Helper function to generate large JSON for SET performance testing
func generateLargeJSONForSet(itemCount int) []byte {
	result := `{"metadata":{"count":` + string(rune('0'+itemCount%10)) + `},"items":[`

	for i := 0; i < itemCount; i++ {
		if i > 0 {
			result += ","
		}
		result += `{"id":` + string(rune('0'+i%10)) + `,"name":"item` + string(rune('0'+i%10)) + `","active":true,"properties":{"tags":["tag1","tag2"]}}`
	}

	result += `]}`
	return []byte(result)
}

// Split the large missing-coverage optimizations into focused tests to keep
// cyclomatic complexity low per test function.

func TestHandleItemsPattern_Split1(t *testing.T) {
	itemsJSON := `{"items":[`
	for i := 0; i < 1000; i++ {
		if i > 0 {
			itemsJSON += ","
		}
		itemsJSON += `{"name":"item` + strconv.Itoa(i) + `","metadata":{"priority":` + strconv.Itoa(i) + `},"tags":["tag` + strconv.Itoa(i) + `","special"]}`
	}
	itemsJSON += `]}`
	data := []byte(itemsJSON)
	result := Get(data, "items.500.name")
	if result.String() != "item500" {
		t.Errorf("handleItemsPattern failed: expected item500, got %s", result.String())
	}
}

func TestUltraFastArrayAccess_Split1(t *testing.T) {
	hugeArray := `[`
	for i := 0; i < 10000; i++ {
		if i > 0 {
			hugeArray += ","
		}
		hugeArray += `"item` + strconv.Itoa(i) + `"`
	}
	hugeArray += `]`
	result := Get([]byte(hugeArray), "5000")
	if result.String() != "item5000" {
		t.Errorf("ultraFastArrayAccess failed: expected item5000, got %s", result.String())
	}
}

func TestIsDirectArrayIndex_Split1(t *testing.T) {
	result := Get([]byte(`[0,1,2,3,4,5,6,7,8,9]`), "5")
	if result.Int() != 5 {
		t.Errorf("isDirectArrayIndex failed: expected 5, got %d", result.Int())
	}
}

func TestBlazingFastPropertyLookup_Split1(t *testing.T) {
	hugeObj := `{`
	for i := 0; i < 5000; i++ {
		if i > 0 {
			hugeObj += ","
		}
		hugeObj += `"prop` + strconv.Itoa(i) + `":"value` + strconv.Itoa(i) + `"`
	}
	hugeObj += `}`
	result := Get([]byte(hugeObj), "prop2500")
	if result.String() != "value2500" {
		t.Errorf("blazingFastPropertyLookup failed: expected value2500, got %s", result.String())
	}
}

func TestMemoryEfficientLargeIndex(t *testing.T) {
	largeArray := `[`
	for i := 0; i < 50000; i++ {
		if i > 0 {
			largeArray += ","
		}
		largeArray += strconv.Itoa(i)
	}
	largeArray += `]`
	result := Get([]byte(largeArray), "49999")
	if result.Int() != 49999 {
		t.Errorf("memoryEfficientLargeIndex failed: expected 49999, got %d", result.Int())
	}
}

func TestUltraFastLargeDeepAccess(t *testing.T) {
	deepLarge := `{"level1":{"level2":{"level3":{"items":[`
	for i := 0; i < 1000; i++ {
		if i > 0 {
			deepLarge += ","
		}
		deepLarge += `{"id":` + strconv.Itoa(i) + `,"data":"item` + strconv.Itoa(i) + `"}`
	}
	deepLarge += `]}}}}`
	result := Get([]byte(deepLarge), "level1.level2.level3.items.500.id")
	if result.Int() != 500 {
		t.Errorf("ultraFastLargeDeepAccess failed: expected 500, got %d", result.Int())
	}
}

func TestGetComplexPath(t *testing.T) {
	complexData := `{
		"users": [
			{"id": 1, "profile": {"name": "John", "settings": {"theme": "dark"}}},
			{"id": 2, "profile": {"name": "Jane", "settings": {"theme": "light"}}}
		]
	}`
	result := Get([]byte(complexData), "users.0.profile.settings.theme")
	if result.String() != "dark" {
		t.Errorf("getComplexPath failed: expected dark, got %s", result.String())
	}
}

func TestFastGetValue(t *testing.T) {
	result := Get([]byte(`{"a":{"b":{"c":{"d":{"e":"found"}}}}}`), "a.b.c.d.e")
	if result.String() != "found" {
		t.Errorf("fastGetValue failed: expected found, got %s", result.String())
	}
}

func TestGetArrayElement(t *testing.T) {
	result := Get([]byte(`[{"a":1},{"b":2},{"c":3}]`), "1.b")
	if result.Int() != 2 {
		t.Errorf("getArrayElement failed: expected 2, got %d", result.Int())
	}
}

func TestFastWildcardKeyAccess(t *testing.T) {
	result := Get([]byte(`{"users":{"user1":{"name":"John"},"user2":{"name":"Jane"}}}`), "users.user1.name")
	if result.String() != "John" {
		t.Errorf("fastWildcardKeyAccess failed: expected John, got %s", result.String())
	}
}

func TestUltraFastFindPropertySpecific(t *testing.T) {
	result := Get([]byte(`{"items":[1,2,3],"simple":"value","nested":{"prop":"val"}}`), "simple")
	if result.String() != "value" {
		t.Errorf("Expected 'value', got %s", result.String())
	}
}

func TestBlazingFastCommaScanner(t *testing.T) {
	// Create array with nested objects containing commas
	commaArray := `[`
	for i := 0; i < 100; i++ {
		if i > 0 {
			commaArray += ","
		}
		commaArray += `{"id":` + strconv.Itoa(i) + `,"nested":{"a":1,"b":2,"c":3},"name":"item` + strconv.Itoa(i) + `"}`
	}
	commaArray += `]`

	// Test accessing elements that require comma scanning
	for i := 50; i < 60; i++ {
		result := Get([]byte(commaArray), strconv.Itoa(i)+".id")
		if result.Int() != int64(i) {
			t.Errorf("blazingFastCommaScanner failed for index %d", i)
		}
	}
}

func TestHandleItemsPatternSpecific(t *testing.T) {
	// Create JSON that should trigger handleItemsPattern
	largeItemsJSON := `{"items":[`
	for i := 0; i < 1000; i++ {
		if i > 0 {
			largeItemsJSON += ","
		}
		largeItemsJSON += `{"name":"item` + strconv.Itoa(i) + `","metadata":{"priority":` + strconv.Itoa(i%10) + `},"tags":["tag1","tag2"]}`
	}
	largeItemsJSON += `]}`

	data := []byte(largeItemsJSON)

	testCases := []struct {
		path string
		desc string
	}{
		{"items.500.name", "items.500.name should exist"},
		{"items.999.metadata.priority", "items.999.metadata.priority should exist"},
		{"items.250.tags.1", "items.250.tags.1 should exist"},
	}

	for _, tc := range testCases {
		result := Get(data, tc.path)
		if !result.Exists() {
			t.Errorf(tc.desc)
		}
	}

	// Test various indexes to trigger different code paths
	for i := 0; i < 100; i += 10 {
		path := "items." + strconv.Itoa(i) + ".name"
		result := Get(data, path)
		if !result.Exists() {
			t.Errorf("Path %s should exist", path)
		}
	}
}

func TestIsDirectArrayIndexMultiple(t *testing.T) {
	simpleArray := `[0,1,2,3,4,5,6,7,8,9]`
	for i := 0; i < 10; i++ {
		result := Get([]byte(simpleArray), strconv.Itoa(i))
		if result.Int() != int64(i) {
			t.Errorf("isDirectArrayIndex failed for index %d", i)
		}
	}
}

// TestMissingSetCoverageOptimizations tests SET functions with 0% coverage
func TestMissingSetCoverageOptimizations(t *testing.T) {
	tests := []struct {
		name      string
		json      []byte
		path      string
		value     interface{}
		options   *SetOptions
		wantError bool
		validate  func(t *testing.T, result []byte, err error)
	}{
		{
			name:      "ultraFastDirectSet",
			json:      []byte(`{"key":"value"}`),
			path:      "key",
			value:     "newvalue",
			options:   nil,
			wantError: false,
			validate: func(t *testing.T, result []byte, err error) {
				if err != nil {
					t.Errorf("ultraFastDirectSet failed: %v", err)
				}
				if Get(result, "key").String() != "newvalue" {
					t.Error("ultraFastDirectSet didn't set value correctly")
				}
			},
		},
		{
			name:      "marshalJSONAccordingToStyle",
			json:      []byte(`{\n\t\t\t"name": "test",\n\t\t\t"value": 123\n\t\t}`),
			path:      "new",
			value:     "added",
			options:   nil,
			wantError: false,
			validate: func(t *testing.T, result []byte, err error) {
				if err != nil {
					t.Errorf("marshalJSONAccordingToStyle failed: %v", err)
				}
				if !Get(result, "new").Exists() {
					t.Error("marshalJSONAccordingToStyle didn't preserve style")
				}
			},
		},
		{
			name:      "tryOptimisticReplace",
			json:      []byte(`{"count":1,"value":"test"}`),
			path:      "count",
			value:     2,
			options:   &SetOptions{Optimistic: true},
			wantError: false,
			validate: func(t *testing.T, result []byte, err error) {
				if err != nil {
					t.Errorf("tryOptimisticReplace failed: %v", err)
				}
				if Get(result, "count").Int() != 2 {
					t.Error("tryOptimisticReplace didn't update value")
				}
			},
		},
		{
			name:      "fastGetArrayElement",
			json:      []byte(`[{"id":0},{"id":1},{"id":2}]`),
			path:      "1.id",
			value:     99,
			options:   nil,
			wantError: false,
			validate: func(t *testing.T, result []byte, err error) {
				if err != nil {
					t.Errorf("fastGetArrayElement failed: %v", err)
				}
				if Get(result, "1.id").Int() != 99 {
					t.Error("fastGetArrayElement didn't update array element")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []byte
			var err error

			if tt.options != nil {
				result, err = SetWithOptions(tt.json, tt.path, tt.value, tt.options)
			} else {
				result, err = Set(tt.json, tt.path, tt.value)
			}

			if (err != nil) != tt.wantError {
				t.Errorf("%s error = %v, wantError %v", tt.name, err, tt.wantError)
				return
			}

			tt.validate(t, result, err)
		})
	}

	// Special test for fastDelete
	t.Run("fastDelete", func(t *testing.T) {
		deleteData := `{"a":"1","b":"2","c":"3"}`
		result, err := Delete([]byte(deleteData), "b")
		if err != nil {
			t.Errorf("fastDelete failed: %v", err)
		}
		if Get(result, "b").Exists() {
			t.Error("fastDelete didn't delete key")
		}
	})
}

// TestUtilityFunctionsCoverage tests utility functions with low coverage
func TestUtilityFunctionsCoverage(t *testing.T) {
	tests := []struct {
		name      string
		json      []byte
		path      string
		value     interface{}
		wantError bool
		validate  func(t *testing.T, result []byte, err error)
	}{
		{
			name:      "escapeString",
			json:      []byte(`{"test":"value"}`),
			path:      "test",
			value:     "value with \"quotes\" and \\backslashes",
			wantError: false,
			validate: func(t *testing.T, result []byte, err error) {
				if err != nil {
					t.Errorf("escapeString failed: %v", err)
				}
				if !Get(result, "test").Exists() {
					t.Error("escapeString didn't handle escaped characters")
				}
			},
		},
		{
			name:      "unicode_handling",
			json:      []byte(`{"test":"value"}`),
			path:      "unicode",
			value:     "测试 🚀 émojis",
			wantError: false,
			validate: func(t *testing.T, result []byte, err error) {
				if err != nil {
					t.Errorf("unicode_handling failed: %v", err)
				}
				if Get(result, "unicode").String() != "测试 🚀 émojis" {
					t.Error("unicode_handling didn't preserve unicode characters")
				}
			},
		},
		{
			name:      "special_json_characters",
			json:      []byte(`{"test":"value"}`),
			path:      "special",
			value:     "{\"nested\":\"json\",\"array\":[1,2,3]}",
			wantError: false,
			validate: func(t *testing.T, result []byte, err error) {
				if err != nil {
					t.Errorf("special_json_characters failed: %v", err)
				}
				if !Get(result, "special").Exists() {
					t.Error("special_json_characters didn't handle special characters")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Set(tt.json, tt.path, tt.value)

			if (err != nil) != tt.wantError {
				t.Errorf("%s error = %v, wantError %v", tt.name, err, tt.wantError)
				return
			}

			tt.validate(t, result, err)
		})
	}
}
