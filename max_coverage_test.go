package njson

import (
	"testing"
)

// TestEdgeCasesForMaxCoverage tests edge cases to push coverage higher
func TestEdgeCasesForMaxCoverage(t *testing.T) {
	// Test Get with various edge cases to reach 100%

	// Empty JSON
	emptyResult := Get([]byte(""), "key")
	if emptyResult.Exists() {
		t.Errorf("Empty JSON should not have key")
	}

	// Single character JSON
	singleChar := Get([]byte("{"), "key")
	if singleChar.Exists() {
		t.Errorf("Invalid JSON should not have key")
	}

	// Test with whitespace-only JSON
	whitespaceResult := Get([]byte("   "), "key")
	if whitespaceResult.Exists() {
		t.Errorf("Whitespace JSON should not have key")
	}

	// Test JSON with null value
	nullJson := `{"nullkey":null,"validkey":"value"}`
	nullResult := Get([]byte(nullJson), "nullkey")
	if !nullResult.IsNull() {
		t.Errorf("Should be null")
	}

	// Test array with out-of-bounds access
	arrJson := `[1,2,3]`
	outOfBounds := Get([]byte(arrJson), "10")
	if outOfBounds.Exists() {
		t.Errorf("Out of bounds should not exist")
	}

	// Test deeply nested access
	deepJson := `{"a":{"b":{"c":{"d":"value"}}}}`
	deepResult := Get([]byte(deepJson), "a.b.c.d")
	if deepResult.String() != "value" {
		t.Errorf("Deep access failed")
	}

	// Test array in object in array
	complexNested := `[{"items":[1,2,3]},{"items":[4,5,6]}]`
	nestedResult := Get([]byte(complexNested), "1.items.2")
	if nestedResult.Int() != 6 {
		t.Errorf("Complex nested access failed")
	}
}

// TestJSONWithDifferentFormats tests different JSON formatting to trigger different code paths
func TestJSONWithDifferentFormats(t *testing.T) {
	// Compact JSON
	compact := `{"name":"John","age":30,"active":true}`
	compactResult := Get([]byte(compact), "name")
	if compactResult.String() != "John" {
		t.Errorf("Compact JSON failed")
	}

	// Pretty JSON with lots of whitespace
	pretty := `{
		"name" : "John" ,
		"age"  : 30,
		"active" : true
	}`
	prettyResult := Get([]byte(pretty), "name")
	if prettyResult.String() != "John" {
		t.Errorf("Pretty JSON failed")
	}

	// JSON with escaped characters
	escaped := `{"message":"He said \"Hello, World!\"","path":"C:\\Users\\test"}`
	escapedResult := Get([]byte(escaped), "message")
	if !escapedResult.Exists() {
		t.Errorf("Escaped JSON failed")
	}

	// JSON with unicode
	unicode := `{"emoji":"🚀","chinese":"中文","math":"∑"}`
	unicodeResult := Get([]byte(unicode), "emoji")
	if unicodeResult.String() != "🚀" {
		t.Errorf("Unicode JSON failed")
	}
}

// TestLargeNumbersAndComplexValues tests handling of large numbers and complex values
func TestLargeNumbersAndComplexValues(t *testing.T) {
	// Large numbers
	largeNumbers := `{
		"bigint": 9223372036854775807,
		"float": 3.141592653589793,
		"scientific": 1.23e-10,
		"negative": -999999999
	}`

	bigintResult := Get([]byte(largeNumbers), "bigint")
	if bigintResult.Int() != 9223372036854775807 {
		t.Errorf("Large int failed")
	}

	floatResult := Get([]byte(largeNumbers), "float")
	if floatResult.Float() == 0 {
		t.Errorf("Float parsing failed")
	}

	// Complex nested structures
	complexStruct := `{
		"users": [
			{
				"id": 1,
				"name": "Alice",
				"permissions": ["read", "write"],
				"metadata": {
					"created": "2023-01-01",
					"tags": {
						"department": "engineering",
						"level": "senior"
					}
				}
			}
		]
	}`

	permResult := Get([]byte(complexStruct), "users.0.permissions.1")
	if permResult.String() != "write" {
		t.Errorf("Complex structure access failed")
	}

	tagResult := Get([]byte(complexStruct), "users.0.metadata.tags.level")
	if tagResult.String() != "senior" {
		t.Errorf("Deep nested access failed")
	}
}

// TestArrayOperationsForCoverage tests various array operations
func TestArrayOperationsForCoverage(t *testing.T) {
	// Empty array
	emptyArr := `[]`
	emptyResult := Get([]byte(emptyArr), "0")
	if emptyResult.Exists() {
		t.Errorf("Empty array should not have elements")
	}

	// Single element array
	singleArr := `["only"]`
	singleResult := Get([]byte(singleArr), "0")
	if singleResult.String() != "only" {
		t.Errorf("Single element access failed")
	}

	// Mixed type array
	mixedArr := `[1, "string", true, null, {"key":"value"}, [1,2,3]]`

	numResult := Get([]byte(mixedArr), "0")
	if numResult.Int() != 1 {
		t.Errorf("Mixed array number failed")
	}

	strResult := Get([]byte(mixedArr), "1")
	if strResult.String() != "string" {
		t.Errorf("Mixed array string failed")
	}

	boolResult := Get([]byte(mixedArr), "2")
	if !boolResult.Bool() {
		t.Errorf("Mixed array bool failed")
	}

	nullResult := Get([]byte(mixedArr), "3")
	if !nullResult.IsNull() {
		t.Errorf("Mixed array null failed")
	}

	objResult := Get([]byte(mixedArr), "4.key")
	if objResult.String() != "value" {
		t.Errorf("Mixed array object failed")
	}

	arrResult := Get([]byte(mixedArr), "5.1")
	if arrResult.Int() != 2 {
		t.Errorf("Mixed array nested array failed")
	}
}

// TestStringParsingEdgeCases tests string parsing edge cases
func TestStringParsingEdgeCases(t *testing.T) {
	// Empty string
	emptyStr := `{"empty":""}`
	emptyResult := Get([]byte(emptyStr), "empty")
	if emptyResult.String() != "" {
		t.Errorf("Empty string failed")
	}

	// String with only spaces
	spaceStr := `{"spaces":"   "}`
	spaceResult := Get([]byte(spaceStr), "spaces")
	if spaceResult.String() != "   " {
		t.Errorf("Space string failed")
	}

	// String with escape sequences
	escapeStr := `{"escaped":"\\n\\t\\r\\\"\\\\\\/"}`
	escapeResult := Get([]byte(escapeStr), "escaped")
	if !escapeResult.Exists() {
		t.Errorf("Escape string failed")
	}

	// Very long string
	longStr := `{"long":"` + generateTestString(1000) + `"}`
	longResult := Get([]byte(longStr), "long")
	if len(longResult.String()) != 1000 {
		t.Errorf("Long string failed, got length %d", len(longResult.String()))
	}
}

// TestBooleanAndNullHandling tests boolean and null value handling
func TestBooleanAndNullHandling(t *testing.T) {
	boolNullJson := `{
		"true_val": true,
		"false_val": false,
		"null_val": null,
		"str_true": "true",
		"str_false": "false",
		"str_null": "null"
	}`

	// Test actual boolean values
	trueResult := Get([]byte(boolNullJson), "true_val")
	if !trueResult.Bool() || trueResult.Type != TypeBoolean {
		t.Errorf("True boolean failed")
	}

	falseResult := Get([]byte(boolNullJson), "false_val")
	if falseResult.Bool() || falseResult.Type != TypeBoolean {
		t.Errorf("False boolean failed")
	}

	// Test null value
	nullResult := Get([]byte(boolNullJson), "null_val")
	if !nullResult.IsNull() || nullResult.Type != TypeNull {
		t.Errorf("Null value failed")
	}

	// Test string representations (should be strings, not booleans/null)
	strTrueResult := Get([]byte(boolNullJson), "str_true")
	if strTrueResult.String() != "true" || strTrueResult.Type != TypeString {
		t.Errorf("String 'true' failed")
	}

	strFalseResult := Get([]byte(boolNullJson), "str_false")
	if strFalseResult.String() != "false" || strFalseResult.Type != TypeString {
		t.Errorf("String 'false' failed")
	}

	strNullResult := Get([]byte(boolNullJson), "str_null")
	if strNullResult.String() != "null" || strNullResult.Type != TypeString {
		t.Errorf("String 'null' failed")
	}
}

// generateTestString creates a test string of specified length
func generateTestString(length int) string {
	result := make([]byte, length)
	for i := range result {
		result[i] = byte('a' + (i % 26))
	}
	return string(result)
}
