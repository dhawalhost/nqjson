package njson

import (
	"fmt"
	"strings"
	"testing"
)

// TestUltraFastOptimizati	// Test direct numeric indices
func TestUltraFastOptimizations(t *testing.T) {
	// Test ultraFastFindProperty with simple root-level property
	simpleObj := `{"name":"value","age":30,"active":true}`
	result := Get([]byte([]byte(simpleObj)), "name")
	if result.String() != "value" {
		t.Errorf("Expected 'value', got %s", result.String())
	}

	// Test with multiple properties to trigger ultraFastFindProperty
	multiProp := `{"a":"1","b":"2","c":"3","d":"4","e":"5"}`
	result = Get([]byte([]byte(multiProp)), "c")
	if result.String() != "3" {
		t.Errorf("Expected '3', got %s", result.String())
	}
}

// TestUltraFastArrayAccess creates conditions to trigger ultraFastArrayAccess
func TestUltraFastArrayAccess(t *testing.T) {
	// Large array to trigger ultra-fast array access optimizations
	var elements []string
	for i := 0; i < 1000; i++ {
		elements = append(elements, fmt.Sprintf(`"item%d"`, i))
	}
	largeArray := `[` + strings.Join(elements, ",") + `]`

	// Access elements at various positions to trigger optimization paths
	result := Get([]byte([]byte(largeArray)), "0")
	if result.String() != "item0" {
		t.Errorf("Expected 'item0', got %s", result.String())
	}

	result = Get([]byte([]byte(largeArray)), "500")
	if result.String() != "item500" {
		t.Errorf("Expected 'item500', got %s", result.String())
	}

	result = Get([]byte([]byte(largeArray)), "999")
	if result.String() != "item999" {
		t.Errorf("Expected 'item999', got %s", result.String())
	}
}

// TestUltraFastObjectAccess triggers object access optimizations
func TestUltraFastObjectAccess(t *testing.T) {
	// Create a large object to trigger ultra-fast object access
	var props []string
	for i := 0; i < 100; i++ {
		props = append(props, fmt.Sprintf(`"prop%d":"value%d"`, i, i))
	}
	largeObj := `{` + strings.Join(props, ",") + `}`

	// Access various properties
	result := Get([]byte(largeObj), "prop0")
	if result.String() != "value0" {
		t.Errorf("Expected 'value0', got %s", result.String())
	}

	result = Get([]byte(largeObj), "prop50")
	if result.String() != "value50" {
		t.Errorf("Expected 'value50', got %s", result.String())
	}

	result = Get([]byte(largeObj), "prop99")
	if result.String() != "value99" {
		t.Errorf("Expected 'value99', got %s", result.String())
	}
}

// TestDirectArrayIndex triggers isDirectArrayIndex function
func TestDirectArrayIndex(t *testing.T) {
	// Test various array index patterns
	testData := `[1,2,3,4,5,6,7,8,9,10]`

	// Test direct numeric indices
	for i := 0; i < 10; i++ {
		result := Get([]byte(testData), fmt.Sprintf("%d", i))
		expected := fmt.Sprintf("%d", i+1)
		if result.String() != expected {
			t.Errorf("Expected %s, got %s", expected, result.String())
		}
	}
}

// TestLargeDeepAccess triggers ultraFastLargeDeepAccess
func TestLargeDeepAccess(t *testing.T) {
	// Create deeply nested structure with large arrays
	nested := `{
		"level1": {
			"level2": {
				"level3": {
					"largeArray": [` + strings.Repeat(`"item",`, 999) + `"lastItem"]
				}
			}
		}
	}`

	result := Get([]byte(nested), "level1.level2.level3.largeArray.999")
	if result.String() != "lastItem" {
		t.Errorf("Expected 'lastItem', got %s", result.String())
	}
}

// TestBlazingFastPropertyLookup creates conditions for blazingFastPropertyLookup
func TestBlazingFastPropertyLookup(t *testing.T) {
	// Create object with many properties in alphabetical order
	var props []string
	for i := 0; i < 50; i++ {
		letter := string(rune('a' + i%26))
		props = append(props, fmt.Sprintf(`"%s%d":"val%d"`, letter, i, i))
	}
	sortedObj := `{` + strings.Join(props, ",") + `}`

	// Test property lookup
	result := Get([]byte(sortedObj), "a0")
	if result.String() != "val0" {
		t.Errorf("Expected 'val0', got %s", result.String())
	}
}

// TestStatisticalJumpAccess creates large arrays to trigger statistical jump access
func TestStatisticalJumpAccess(t *testing.T) {
	// Create very large array (>2000 elements) to trigger statistical optimization
	var elements []string
	for i := 0; i < 3000; i++ {
		elements = append(elements, fmt.Sprintf(`{"id":%d,"data":"item%d"}`, i, i))
	}
	hugeArray := `[` + strings.Join(elements, ",") + `]`

	// Access elements that should trigger statistical jump access
	result := Get([]byte(hugeArray), "1500.id")
	if result.Int() != 1500 {
		t.Errorf("Expected 1500, got %d", result.Int())
	}

	result = Get([]byte(hugeArray), "2500.data")
	if result.String() != "item2500" {
		t.Errorf("Expected 'item2500', got %s", result.String())
	}
}

// TestCommaCountingAccess triggers comma counting optimization functions
func TestCommaCountingAccess(t *testing.T) {
	// Create array with many elements to trigger comma counting
	var numbers []string
	for i := 0; i < 500; i++ {
		numbers = append(numbers, fmt.Sprintf("%d", i))
	}
	numberArray := `[` + strings.Join(numbers, ",") + `]`

	// Access elements that require comma counting
	result := Get([]byte(numberArray), "250")
	if result.Int() != 250 {
		t.Errorf("Expected 250, got %d", result.Int())
	}

	result = Get([]byte(numberArray), "400")
	if result.Int() != 400 {
		t.Errorf("Expected 400, got %d", result.Int())
	}
}

// TestHandleItemsPattern triggers handleItemsPattern function
func TestHandleItemsPattern(t *testing.T) {
	// Test .items patterns and similar
	arrayOfArrays := `[
		[1,2,3],
		[4,5,6],
		[7,8,9]
	]`

	// Try to access using different patterns that might trigger handleItemsPattern
	result := Get([]byte(arrayOfArrays), "1.1")
	if result.Int() != 5 {
		t.Errorf("Expected 5, got %d", result.Int())
	}

	// Test with object arrays
	objArray := `[
		{"items":[1,2,3]},
		{"items":[4,5,6]},
		{"items":[7,8,9]}
	]`

	result = Get([]byte(objArray), "1.items.2")
	if result.Int() != 6 {
		t.Errorf("Expected 6, got %d", result.Int())
	}
}

// TestUltraFastSkipElement triggers ultraFastSkipElement
func TestUltraFastSkipElement(t *testing.T) {
	// Create complex nested structures that require element skipping
	complexArray := `[
		{"skip":true,"data":[1,2,3,{"nested":[4,5,6]}]},
		{"skip":false,"data":[7,8,9,{"nested":[10,11,12]}]},
		{"skip":true,"data":[13,14,15,{"nested":[16,17,18]}]}
	]`

	// Access elements that require skipping over complex structures
	result := Get([]byte(complexArray), "1.data.3.nested.1")
	if result.Int() != 11 {
		t.Errorf("Expected 11, got %d", result.Int())
	}
}

// TestTryLargeArrayPath triggers tryLargeArrayPath
func TestTryLargeArrayPath(t *testing.T) {
	// Create very large array with complex path access
	var largeElements []string
	for i := 0; i < 1000; i++ {
		largeElements = append(largeElements, fmt.Sprintf(`{
			"index":%d,
			"metadata":{
				"tags":["tag%d","tag%d"],
				"values":[%d,%d,%d]
			}
		}`, i, i, i+1, i*10, i*20, i*30))
	}
	megaArray := `[` + strings.Join(largeElements, ",") + `]`

	// Access deep paths in large arrays
	result := Get([]byte(megaArray), "500.metadata.values.1")
	if result.Int() != 10000 {
		t.Errorf("Expected 10000, got %d", result.Int())
	}

	result = Get([]byte(megaArray), "750.metadata.tags.0")
	if result.String() != "tag750" {
		t.Errorf("Expected 'tag750', got %s", result.String())
	}
}

// TestAccessMethods triggers accessItemProperty, accessArrayElement, accessObjectProperty
func TestAccessMethods(t *testing.T) {
	// Test different access patterns
	mixedData := `{
		"array": [1,2,3,4,5],
		"object": {"a":1,"b":2,"c":3},
		"nested": {
			"items": [
				{"prop":"value1"},
				{"prop":"value2"},
				{"prop":"value3"}
			]
		}
	}`

	// Test array element access
	result := Get([]byte(mixedData), "array.2")
	if result.Int() != 3 {
		t.Errorf("Expected 3, got %d", result.Int())
	}

	// Test object property access
	result = Get([]byte(mixedData), "object.b")
	if result.Int() != 2 {
		t.Errorf("Expected 2, got %d", result.Int())
	}

	// Test nested access
	result = Get([]byte(mixedData), "nested.items.1.prop")
	if result.String() != "value2" {
		t.Errorf("Expected 'value2', got %s", result.String())
	}
}

// TestUnsafeOperations ensures unsafe operations get coverage
func TestUnsafeOperations(t *testing.T) {
	// Test operations that use unsafe pointers
	data := `[1,2,3,4,5,6,7,8,9,10]`

	// Call functions that might use unsafe operations internally
	for i := 0; i < 10; i++ {
		result := Get([]byte(data), fmt.Sprintf("%d", i))
		expected := int64(i + 1)
		if result.Int() != expected {
			t.Errorf("Index %d: Expected %d, got %d", i, expected, result.Int())
		}
	}
}

// TestLargeIndexAccess triggers ultraFastLargeIndexAccess
func TestLargeIndexAccess(t *testing.T) {
	// Create array with very large indices
	var hugeElements []string
	for i := 0; i < 5000; i++ {
		hugeElements = append(hugeElements, fmt.Sprintf(`"element%d"`, i))
	}
	hugeArray := `[` + strings.Join(hugeElements, ",") + `]`

	// Access large indices
	result := Get([]byte(hugeArray), "4999")
	if result.String() != "element4999" {
		t.Errorf("Expected 'element4999', got %s", result.String())
	}

	result = Get([]byte(hugeArray), "2500")
	if result.String() != "element2500" {
		t.Errorf("Expected 'element2500', got %s", result.String())
	}
}

// TestSimplePropertyLookup triggers ultraFastSimplePropertyLookup
func TestSimplePropertyLookup(t *testing.T) {
	// Test simple property lookups that should trigger optimizations
	simpleObjs := []string{
		`{"a":"1"}`,
		`{"name":"test"}`,
		`{"id":123}`,
		`{"active":true}`,
		`{"value":null}`,
	}

	for i, obj := range simpleObjs {
		var key string
		var expected interface{}
		switch i {
		case 0:
			key, expected = "a", "1"
		case 1:
			key, expected = "name", "test"
		case 2:
			key, expected = "id", 123
		case 3:
			key, expected = "active", true
		case 4:
			key, expected = "value", nil
		}

		result := Get([]byte(obj), key)
		switch exp := expected.(type) {
		case string:
			if result.String() != exp {
				t.Errorf("Expected %s, got %s", exp, result.String())
			}
		case int:
			if result.Int() != int64(exp) {
				t.Errorf("Expected %d, got %d", exp, result.Int())
			}
		case bool:
			if result.Bool() != exp {
				t.Errorf("Expected %t, got %t", exp, result.Bool())
			}
		case nil:
			if result.Type != TypeNull {
				t.Errorf("Expected null, got %v", result.Type)
			}
		}
	}
}
