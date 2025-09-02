package njson_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/dhawalhost/njson"
)

var smallJSON = []byte(`{"name":"John","age":30,"city":"New York"}`)

var mediumJSON = []byte(`{
  "name": "John Smith",
  "age": 35,
  "address": {
    "street": "123 Main St",
    "city": "San Francisco",
    "state": "CA",
    "zip": "94103"
  },
  "phones": [
    {"type": "home", "number": "555-1234"},
    {"type": "work", "number": "555-5678"}
  ],
  "email": "john@example.com",
  "active": true,
  "scores": [95, 87, 92, 78, 85]
}`)

var largeJSON []byte
var complexPaths []string
var simplePaths []string

func init() {
	// Generate large JSON with 1000 items
	largeJSON = []byte(`{"items":[`)
	for i := 0; i < 1000; i++ {
		if i > 0 {
			largeJSON = append(largeJSON, ',')
		}
		item := fmt.Sprintf(`{"id":%d,"name":"Item %d","value":%d,"tags":["%s","%s"],"metadata":{"created":"2025-09-01","priority":%d,"active":%v}}`,
			i, i, i*10,
			fmt.Sprintf("tag%d-1", i),
			fmt.Sprintf("tag%d-2", i),
			rand.Intn(5),
			i%3 == 0)
		largeJSON = append(largeJSON, []byte(item)...)
	}
	largeJSON = append(largeJSON, []byte(`],"metadata":{"count":1000,"generated":"2025-09-01"}}`)...)

	// Common test paths
	simplePaths = []string{
		"name",
		"age",
		"address.city",
		"phones.0.number",
		"scores.2",
	}

	complexPaths = []string{
		"phones[0].number",
		"items[0].tags.0",
		"items[500].metadata.priority",
		"items[999].name",
		"metadata.count",
	}
}

//------------------------------------------------------------------------------
// GET BENCHMARKS
//------------------------------------------------------------------------------

// Simple paths with small JSON
func BenchmarkGet_SimpleSmall_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Get(smallJSON, "name")
	}
}

// Simple paths with medium JSON
func BenchmarkGet_SimpleMedium_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, path := range simplePaths {
			njson.Get(mediumJSON, path)
		}
	}
}

// Complex paths with medium JSON
func BenchmarkGet_ComplexMedium_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Get(mediumJSON, "phones[?(@.type==\"work\")].number")
		njson.Get(mediumJSON, "scores[2]")
	}
}

// Large JSON deep access
func BenchmarkGet_LargeDeep_NJSON(b *testing.B) {
	b.ReportAllocs()
	paths := []string{
		"items.500.name",
		"items.999.metadata.priority",
		"items.250.tags.1",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			njson.Get(largeJSON, path)
		}
	}
}

// Multi-path queries
func BenchmarkGet_MultiPath_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.GetMany(mediumJSON, simplePaths...)
	}
}

// Complex filter operation
func BenchmarkGet_Filter_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Get(largeJSON, "items[?(@.metadata.priority>3)].name")
	}
}

// Wildcard operations
func BenchmarkGet_Wildcard_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Get(mediumJSON, "phones.*.type")
	}
}

//------------------------------------------------------------------------------
// SET BENCHMARKS
//------------------------------------------------------------------------------

// Simple set on small JSON
func BenchmarkSet_SimpleSmall_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Set(smallJSON, "name", "Jane")
	}
}

// Add a new field to small JSON
func BenchmarkSet_AddField_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Set(smallJSON, "email", "john@example.com")
	}
}

// Nested set on medium JSON
func BenchmarkSet_NestedMedium_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Set(mediumJSON, "address.city", "New York")
	}
}

// Deep set creating new paths
func BenchmarkSet_DeepCreate_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Set(mediumJSON, "preferences.theme.colors.primary", "#336699")
	}
}

// Array element set
func BenchmarkSet_ArrayElement_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Set(mediumJSON, "phones.1.number", "555-9999")
	}
}

// Append to array
func BenchmarkSet_ArrayAppend_NJSON(b *testing.B) {
	b.ReportAllocs()
	json := []byte(`{"array":[1,2,3]}`)
	for i := 0; i < b.N; i++ {
		njson.Set(json, "array.3", 4)
	}
}

// Delete operations
func BenchmarkDelete_Simple_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Delete(smallJSON, "city")
	}
}

func BenchmarkDelete_Nested_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Delete(mediumJSON, "address.state")
	}
}

func BenchmarkDelete_Array_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Delete(mediumJSON, "phones.0")
	}
}

// Multiple operations in sequence
func BenchmarkMultiOp_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		json := bytes.Clone(smallJSON)
		json, _ = njson.Set(json, "name", "Jane")
		json, _ = njson.Set(json, "age", 25)
		json, _ = njson.Set(json, "email", "jane@example.com")
		_, _ = njson.Delete(json, "city")
	}
}

// Set with merge options
func BenchmarkSet_MergeObjects_NJSON(b *testing.B) {
	b.ReportAllocs()
	opts := njson.SetOptions{MergeObjects: true}

	for i := 0; i < b.N; i++ {
		njson.SetWithOptions(mediumJSON, "address", map[string]interface{}{
			"unit": "4B",
			"zip":  "94105",
		}, &opts)
	}
}

// Realistic scenario: update user profile
func BenchmarkRealistic_UpdateProfile_NJSON(b *testing.B) {
	b.ReportAllocs()
	userJSON := []byte(`{"user":{"id":123,"name":"John","email":"john@example.com","profile":{"age":30,"interests":["sports","music"],"address":{"city":"New York","zip":"10001"}}}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Get current values
		name := njson.Get(userJSON, "user.name").String()
		city := njson.Get(userJSON, "user.profile.address.city").String()

		// Update values
		json := bytes.Clone(userJSON)
		json, _ = njson.Set(json, "user.profile.age", 31)
		json, _ = njson.Set(json, "user.profile.address.zip", "10002")
		json, _ = njson.Set(json, "user.profile.interests.2", "travel")

		// Add new field
		if name == "John" && city == "New York" {
			_, _ = njson.Set(json, "user.profile.lastUpdated", "2025-09-01")
		}
	}
}

// Large document set benchmark
func BenchmarkSet_LargeDocument_NJSON(b *testing.B) {
	// Create document copy - would be too large for stack allocation
	jsonCopy := make([]byte, len(largeJSON))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		copy(jsonCopy, largeJSON)
		index := i % 1000
		njson.Set(jsonCopy, "items."+strconv.Itoa(index)+".metadata.active", true)
	}
}
