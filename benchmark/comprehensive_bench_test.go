package benchmark

import (
	"encoding/json"
	"testing"

	"github.com/dhawalhost/nqjson"
	"github.com/itchyny/gojq"
	gjson "github.com/tidwall/gjson"
)

// 100MB+ dataset for comprehensive benchmarks
var (
	hugeJSON300k    []byte
	hugeJSONParsed  any
	arrayTestData   []byte
	arrayTestParsed any
	hugeJSONSize    DataSizeInfo
)

func init() {
	// Generate 300,000 records (~130MB)
	hugeJSON300k = GenerateHugeJSON(300000)
	hugeJSONSize = GetDataSizeInfo(hugeJSON300k)

	// Generate array test data with 100,000 elements
	arrayTestData = GenerateArrayTestData(100000)
}

// ============================================================================
// SIMPLE FIELD ACCESS (100MB+ dataset)
// ============================================================================

func BenchmarkHuge_SimpleField_NQJSON(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(hugeJSON300k)))

	for i := 0; i < b.N; i++ {
		nqjson.Get(hugeJSON300k, "records.150000.name")
	}
}

func BenchmarkHuge_SimpleField_GJSON(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(hugeJSON300k)))

	for i := 0; i < b.N; i++ {
		gjson.GetBytes(hugeJSON300k, "records.150000.name")
	}
}

func BenchmarkHuge_SimpleField_GojqCompiled(b *testing.B) {
	if hugeJSONParsed == nil {
		if err := json.Unmarshal(hugeJSON300k, &hugeJSONParsed); err != nil {
			b.Fatalf("Failed to parse: %v", err)
		}
	}
	b.ReportAllocs()

	parsed, _ := gojq.Parse(".records[150000].name")
	code, _ := gojq.Compile(parsed)

	for i := 0; i < b.N; i++ {
		iter := code.Run(hugeJSONParsed)
		gojqResultSink, _ = iter.Next()
	}
}

// ============================================================================
// NESTED FIELD ACCESS (100MB+ dataset)
// ============================================================================

func BenchmarkHuge_DeepNested_NQJSON(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(hugeJSON300k)))

	for i := 0; i < b.N; i++ {
		nqjson.Get(hugeJSON300k, "records.100000.profile.location.city")
	}
}

func BenchmarkHuge_DeepNested_GJSON(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(hugeJSON300k)))

	for i := 0; i < b.N; i++ {
		gjson.GetBytes(hugeJSON300k, "records.100000.profile.location.city")
	}
}

func BenchmarkHuge_DeepNested_GojqCompiled(b *testing.B) {
	if hugeJSONParsed == nil {
		if err := json.Unmarshal(hugeJSON300k, &hugeJSONParsed); err != nil {
			b.Fatalf("Failed to parse: %v", err)
		}
	}
	b.ReportAllocs()

	parsed, _ := gojq.Parse(".records[100000].profile.location.city")
	code, _ := gojq.Compile(parsed)

	for i := 0; i < b.N; i++ {
		iter := code.Run(hugeJSONParsed)
		gojqResultSink, _ = iter.Next()
	}
}

// ============================================================================
// MODIFIER BENCHMARKS - @slice
// ============================================================================

func BenchmarkModifier_Slice_NQJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		nqjson.Get(arrayTestData, "numbers|@slice:1000:2000")
	}
}

func BenchmarkModifier_Slice_GojqCompiled(b *testing.B) {
	if arrayTestParsed == nil {
		if err := json.Unmarshal(arrayTestData, &arrayTestParsed); err != nil {
			b.Fatalf("Failed to parse: %v", err)
		}
	}
	b.ReportAllocs()

	parsed, _ := gojq.Parse(".numbers[1000:2000]")
	code, _ := gojq.Compile(parsed)

	for i := 0; i < b.N; i++ {
		iter := code.Run(arrayTestParsed)
		gojqResultSink, _ = iter.Next()
	}
}

// ============================================================================
// MODIFIER BENCHMARKS - @has
// ============================================================================

func BenchmarkModifier_Has_NQJSON(b *testing.B) {
	data := []byte(`{"name":"test","email":"test@example.com","active":true}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "@has:email")
	}
}

func BenchmarkModifier_Has_GojqCompiled(b *testing.B) {
	var parsed any
	json.Unmarshal([]byte(`{"name":"test","email":"test@example.com","active":true}`), &parsed)
	b.ReportAllocs()

	q, _ := gojq.Parse("has(\"email\")")
	code, _ := gojq.Compile(q)

	for i := 0; i < b.N; i++ {
		iter := code.Run(parsed)
		gojqResultSink, _ = iter.Next()
	}
}

// ============================================================================
// MODIFIER BENCHMARKS - @contains
// ============================================================================

func BenchmarkModifier_Contains_NQJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		nqjson.Get(arrayTestData, "strings|@contains:item5000")
	}
}

func BenchmarkModifier_Contains_GojqCompiled(b *testing.B) {
	if arrayTestParsed == nil {
		if err := json.Unmarshal(arrayTestData, &arrayTestParsed); err != nil {
			b.Fatalf("Failed to parse: %v", err)
		}
	}
	b.ReportAllocs()

	parsed, _ := gojq.Parse(".strings | contains([\"item5000\"])")
	code, _ := gojq.Compile(parsed)

	for i := 0; i < b.N; i++ {
		iter := code.Run(arrayTestParsed)
		gojqResultSink, _ = iter.Next()
	}
}

// ============================================================================
// MODIFIER BENCHMARKS - @any / @all
// ============================================================================

func BenchmarkModifier_Any_NQJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		nqjson.Get(arrayTestData, "booleans|@any")
	}
}

func BenchmarkModifier_Any_GojqCompiled(b *testing.B) {
	if arrayTestParsed == nil {
		if err := json.Unmarshal(arrayTestData, &arrayTestParsed); err != nil {
			b.Fatalf("Failed to parse: %v", err)
		}
	}
	b.ReportAllocs()

	parsed, _ := gojq.Parse(".booleans | any")
	code, _ := gojq.Compile(parsed)

	for i := 0; i < b.N; i++ {
		iter := code.Run(arrayTestParsed)
		gojqResultSink, _ = iter.Next()
	}
}

func BenchmarkModifier_All_NQJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		nqjson.Get(arrayTestData, "booleans|@all")
	}
}

func BenchmarkModifier_All_GojqCompiled(b *testing.B) {
	if arrayTestParsed == nil {
		if err := json.Unmarshal(arrayTestData, &arrayTestParsed); err != nil {
			b.Fatalf("Failed to parse: %v", err)
		}
	}
	b.ReportAllocs()

	parsed, _ := gojq.Parse(".booleans | all")
	code, _ := gojq.Compile(parsed)

	for i := 0; i < b.N; i++ {
		iter := code.Run(arrayTestParsed)
		gojqResultSink, _ = iter.Next()
	}
}

// ============================================================================
// MODIFIER BENCHMARKS - @entries / @fromentries
// ============================================================================

func BenchmarkModifier_Entries_NQJSON(b *testing.B) {
	data := []byte(`{"a":1,"b":2,"c":3,"d":4,"e":5}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "@entries")
	}
}

func BenchmarkModifier_Entries_GojqCompiled(b *testing.B) {
	var parsed any
	json.Unmarshal([]byte(`{"a":1,"b":2,"c":3,"d":4,"e":5}`), &parsed)
	b.ReportAllocs()

	q, _ := gojq.Parse("to_entries")
	code, _ := gojq.Compile(q)

	for i := 0; i < b.N; i++ {
		iter := code.Run(parsed)
		gojqResultSink, _ = iter.Next()
	}
}

// ============================================================================
// SET HELPER BENCHMARKS - Increment
// ============================================================================

func BenchmarkSetHelper_Increment_NQJSON(b *testing.B) {
	original := []byte(`{"count":100,"name":"test"}`)
	b.ReportAllocs()
	var result []byte
	for i := 0; i < b.N; i++ {
		result, _ = nqjson.Increment(original, "count", 1)
	}
	_ = result
}

// ============================================================================
// SET HELPER BENCHMARKS - SetMany
// ============================================================================

func BenchmarkSetHelper_SetMany_NQJSON(b *testing.B) {
	original := []byte(`{"name":"Alice","age":30}`)
	b.ReportAllocs()
	var result []byte
	for i := 0; i < b.N; i++ {
		result, _ = nqjson.SetMany(original, "name", "Bob", "age", 25, "city", "NYC")
	}
	_ = result
}

// ============================================================================
// SET HELPER BENCHMARKS - DeleteMany
// ============================================================================

func BenchmarkSetHelper_DeleteMany_NQJSON(b *testing.B) {
	original := []byte(`{"name":"Alice","age":30,"temp":"x","debug":true,"cache":"y"}`)
	b.ReportAllocs()
	var result []byte
	for i := 0; i < b.N; i++ {
		result, _ = nqjson.DeleteMany(original, "temp", "debug", "cache")
	}
	_ = result
}

// ============================================================================
// ARRAY LAST ELEMENT ACCESS (100MB+ dataset)
// ============================================================================

func BenchmarkHuge_LastElement_NQJSON(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(hugeJSON300k)))

	for i := 0; i < b.N; i++ {
		nqjson.Get(hugeJSON300k, "records.299999.id")
	}
}

func BenchmarkHuge_LastElement_GJSON(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(hugeJSON300k)))

	for i := 0; i < b.N; i++ {
		gjson.GetBytes(hugeJSON300k, "records.299999.id")
	}
}
