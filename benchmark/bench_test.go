package njson_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/Jeffail/gabs/v2"
	"github.com/akshaybharambe14/ijson"
	"github.com/dhawalhost/njson"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/valyala/fastjson"
)

var smallJSON = []byte(`{"name":"John","age":30,"city":"New York"}`)
var smallJSONParsed interface{}

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
var mediumJSONParsed interface{}

var largeJSON []byte
var largeJSONParsed interface{}
var complexPaths []string
var simplePaths []string

func init() {
	// Parse JSON objects for ijson
	json.Unmarshal(smallJSON, &smallJSONParsed)
	json.Unmarshal(mediumJSON, &mediumJSONParsed)
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

	// Parse large JSON for ijson
	json.Unmarshal(largeJSON, &largeJSONParsed)

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

func BenchmarkGet_SimpleSmall_GJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		gjson.GetBytes(smallJSON, "name")
	}
}

func BenchmarkGet_SimpleSmall_GABS(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		parsed, _ := gabs.ParseJSON(smallJSON)
		parsed.Path("name")
	}
}

func BenchmarkGet_SimpleSmall_FASTJSON(b *testing.B) {
	b.ReportAllocs()
	var p fastjson.Parser
	for i := 0; i < b.N; i++ {
		v, _ := p.ParseBytes(smallJSON)
		v.GetStringBytes("name")
	}
}

func BenchmarkGet_SimpleSmall_IJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ijson.Get(smallJSONParsed, "name")
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

func BenchmarkGet_SimpleMedium_GJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, path := range simplePaths {
			gjson.GetBytes(mediumJSON, path)
		}
	}
}

func BenchmarkGet_SimpleMedium_GABS(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		parsed, _ := gabs.ParseJSON(mediumJSON)
		for _, path := range simplePaths {
			parsed.Path(path)
		}
	}
}

func BenchmarkGet_SimpleMedium_FASTJSON(b *testing.B) {
	b.ReportAllocs()
	var p fastjson.Parser
	for i := 0; i < b.N; i++ {
		v, _ := p.ParseBytes(mediumJSON)
		for _, path := range simplePaths {
			// fastjson requires manual path navigation
			switch path {
			case "name":
				v.GetStringBytes("name")
			case "age":
				v.GetInt("age")
			case "address.city":
				v.Get("address", "city")
			case "phones.0.number":
				v.Get("phones", "0", "number")
			case "scores.2":
				v.Get("scores", "2")
			}
		}
	}
}

func BenchmarkGet_SimpleMedium_IJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, path := range simplePaths {
			ijson.Get(mediumJSONParsed, path)
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

func BenchmarkGet_ComplexMedium_GJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		gjson.GetBytes(mediumJSON, "phones[?(@.type==\"work\")].number")
		gjson.GetBytes(mediumJSON, "scores[2]")
	}
}

func BenchmarkGet_ComplexMedium_GABS(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		parsed, _ := gabs.ParseJSON(mediumJSON)
		// GABS doesn't support complex filters directly, use simple path
		parsed.Path("phones.1.number") // Approximate work phone
		parsed.Path("scores.2")
	}
}

func BenchmarkGet_ComplexMedium_FASTJSON(b *testing.B) {
	b.ReportAllocs()
	var p fastjson.Parser
	for i := 0; i < b.N; i++ {
		v, _ := p.ParseBytes(mediumJSON)
		// Manual filter simulation for work phone
		phones := v.Get("phones")
		if phones != nil {
			for j := 0; j < 2; j++ {
				phone := phones.Get(strconv.Itoa(j))
				if phone != nil && string(phone.GetStringBytes("type")) == "work" {
					phone.GetStringBytes("number")
					break
				}
			}
		}
		v.Get("scores", "2")
	}
}

func BenchmarkGet_ComplexMedium_IJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// ijson may not support complex filters, use simple path
		ijson.Get(mediumJSONParsed, "phones.1.number")
		ijson.Get(mediumJSONParsed, "scores.2")
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

func BenchmarkGet_LargeDeep_GJSON(b *testing.B) {
	b.ReportAllocs()
	paths := []string{
		"items.500.name",
		"items.999.metadata.priority",
		"items.250.tags.1",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			gjson.GetBytes(largeJSON, path)
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

func BenchmarkGet_MultiPath_GJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		gjson.GetManyBytes(mediumJSON, simplePaths...)
	}
}

func BenchmarkGet_MultiPath_GABS(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		parsed, _ := gabs.ParseJSON(mediumJSON)
		for _, path := range simplePaths {
			parsed.Path(path)
		}
	}
}

func BenchmarkGet_MultiPath_FASTJSON(b *testing.B) {
	b.ReportAllocs()
	var p fastjson.Parser
	for i := 0; i < b.N; i++ {
		v, _ := p.ParseBytes(mediumJSON)
		for _, path := range simplePaths {
			switch path {
			case "name":
				v.GetStringBytes("name")
			case "age":
				v.GetInt("age")
			case "address.city":
				v.Get("address", "city")
			case "phones.0.number":
				v.Get("phones", "0", "number")
			case "scores.2":
				v.Get("scores", "2")
			}
		}
	}
}

func BenchmarkGet_MultiPath_IJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, path := range simplePaths {
			ijson.Get(mediumJSONParsed, path)
		}
	}
}

// Complex filter operation
func BenchmarkGet_Filter_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Get(largeJSON, "items[?(@.metadata.priority>3)].name")
	}
}

func BenchmarkGet_Filter_GJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		gjson.GetBytes(largeJSON, "items[?(@.metadata.priority>3)].name")
	}
}

func BenchmarkGet_Filter_GABS(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		parsed, _ := gabs.ParseJSON(largeJSON)
		// GABS doesn't support JSONPath filters, manual iteration
		items := parsed.S("items").Children()
		for _, item := range items {
			if priority, ok := item.Path("metadata.priority").Data().(float64); ok && priority > 3 {
				item.Path("name")
			}
		}
	}
}

func BenchmarkGet_Filter_FASTJSON(b *testing.B) {
	b.ReportAllocs()
	var p fastjson.Parser
	for i := 0; i < b.N; i++ {
		v, _ := p.ParseBytes(largeJSON)
		items := v.Get("items")
		if items != nil {
			// Manual filtering
			items.GetArray() // This forces parsing the array
			for j := 0; j < 1000; j++ {
				item := items.Get(strconv.Itoa(j))
				if item != nil {
					priority := item.Get("metadata", "priority")
					if priority != nil && priority.GetInt() > 3 {
						item.GetStringBytes("name")
					}
				}
			}
		}
	}
}

func BenchmarkGet_Filter_IJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// ijson likely doesn't support complex filters, manual approach
		for j := 0; j < 1000; j++ {
			priorityPath := fmt.Sprintf("items.%d.metadata.priority", j)
			namePath := fmt.Sprintf("items.%d.name", j)

			priority, _ := ijson.Get(largeJSONParsed, priorityPath)
			if p, ok := priority.(float64); ok && p > 3 {
				ijson.Get(largeJSONParsed, namePath)
			}
		}
	}
}

// Wildcard operations
func BenchmarkGet_Wildcard_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Get(mediumJSON, "phones.*.type")
	}
}

func BenchmarkGet_Wildcard_GJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		gjson.GetBytes(mediumJSON, "phones.*.type")
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

func BenchmarkSet_SimpleSmall_SJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sjson.SetBytes(smallJSON, "name", "Jane")
	}
}

func BenchmarkSet_SimpleSmall_GABS(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		parsed, _ := gabs.ParseJSON(smallJSON)
		parsed.Set("Jane", "name")
		parsed.Bytes()
	}
}

// Add a new field to small JSON
func BenchmarkSet_AddField_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Set(smallJSON, "email", "john@example.com")
	}
}

func BenchmarkSet_AddField_SJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sjson.SetBytes(smallJSON, "email", "john@example.com")
	}
}

func BenchmarkSet_AddField_GABS(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		parsed, _ := gabs.ParseJSON(smallJSON)
		parsed.Set("john@example.com", "email")
		parsed.Bytes()
	}
}

// Nested set on medium JSON
func BenchmarkSet_NestedMedium_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Set(mediumJSON, "address.city", "New York")
	}
}

func BenchmarkSet_NestedMedium_SJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sjson.SetBytes(mediumJSON, "address.city", "New York")
	}
}

// Deep set creating new paths
func BenchmarkSet_DeepCreate_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Set(mediumJSON, "preferences.theme.colors.primary", "#336699")
	}
}

func BenchmarkSet_DeepCreate_SJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sjson.SetBytes(mediumJSON, "preferences.theme.colors.primary", "#336699")
	}
}

// Array element set
func BenchmarkSet_ArrayElement_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Set(mediumJSON, "phones.1.number", "555-9999")
	}
}

func BenchmarkSet_ArrayElement_SJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sjson.SetBytes(mediumJSON, "phones.1.number", "555-9999")
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

func BenchmarkSet_ArrayAppend_SJSON(b *testing.B) {
	b.ReportAllocs()
	json := []byte(`{"array":[1,2,3]}`)
	for i := 0; i < b.N; i++ {
		sjson.SetBytes(json, "array.3", 4)
	}
}

// Delete operations
func BenchmarkDelete_Simple_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Delete(smallJSON, "city")
	}
}

func BenchmarkDelete_Simple_SJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sjson.DeleteBytes(smallJSON, "city")
	}
}

func BenchmarkDelete_Nested_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Delete(mediumJSON, "address.state")
	}
}

func BenchmarkDelete_Nested_SJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sjson.DeleteBytes(mediumJSON, "address.state")
	}
}

func BenchmarkDelete_Array_NJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		njson.Delete(mediumJSON, "phones.0")
	}
}

func BenchmarkDelete_Array_SJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sjson.DeleteBytes(mediumJSON, "phones.0")
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

func BenchmarkMultiOp_SJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		json := bytes.Clone(smallJSON)
		json, _ = sjson.SetBytes(json, "name", "Jane")
		json, _ = sjson.SetBytes(json, "age", 25)
		json, _ = sjson.SetBytes(json, "email", "jane@example.com")
		_, _ = sjson.DeleteBytes(json, "city")
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

func BenchmarkRealistic_UpdateProfile_SJSON(b *testing.B) {
	b.ReportAllocs()
	userJSON := []byte(`{"user":{"id":123,"name":"John","email":"john@example.com","profile":{"age":30,"interests":["sports","music"],"address":{"city":"New York","zip":"10001"}}}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Get current values
		name := gjson.GetBytes(userJSON, "user.name").String()
		city := gjson.GetBytes(userJSON, "user.profile.address.city").String()

		// Update values
		json := bytes.Clone(userJSON)
		json, _ = sjson.SetBytes(json, "user.profile.age", 31)
		json, _ = sjson.SetBytes(json, "user.profile.address.zip", "10002")
		json, _ = sjson.SetBytes(json, "user.profile.interests.2", "travel")

		// Add new field
		if name == "John" && city == "New York" {
			_, _ = sjson.SetBytes(json, "user.profile.lastUpdated", "2025-09-01")
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

func BenchmarkSet_LargeDocument_SJSON(b *testing.B) {
	// Create document copy - would be too large for stack allocation
	jsonCopy := make([]byte, len(largeJSON))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		copy(jsonCopy, largeJSON)
		index := i % 1000
		sjson.SetBytes(jsonCopy, "items."+strconv.Itoa(index)+".metadata.active", true)
	}
}
