package benchmark

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/dhawalhost/nqjson"
	"github.com/itchyny/gojq"
)

// Pre-generated test data at various sizes
var (
	testData32MiB  []byte
	testData64MiB  []byte
	testData128MiB []byte
	testData256MiB []byte
	testData512MiB []byte
	testData1GiB   []byte

	// Parsed versions for gojq
	parsed32MiB  any
	parsed64MiB  any
	parsed128MiB any
	parsed256MiB any
	parsed512MiB any
	parsed1GiB   any

	// Fuzzer instance
	benchFuzzer *Fuzzer
)

func init() {
	// Initialize fuzzer with reproducible seed
	benchFuzzer = NewFuzzer(42, DefaultSchema())
}

// Helper to get or generate test data
func getTestData(sizeName string) []byte {
	sizes := TargetSizes()
	targetSize := sizes[sizeName]

	switch sizeName {
	case "32MiB":
		if testData32MiB == nil {
			testData32MiB = benchFuzzer.GenerateToSize(targetSize)
		}
		return testData32MiB
	case "64MiB":
		if testData64MiB == nil {
			testData64MiB = benchFuzzer.GenerateToSize(targetSize)
		}
		return testData64MiB
	case "128MiB":
		if testData128MiB == nil {
			testData128MiB = benchFuzzer.GenerateToSize(targetSize)
		}
		return testData128MiB
	case "256MiB":
		if testData256MiB == nil {
			testData256MiB = benchFuzzer.GenerateToSize(targetSize)
		}
		return testData256MiB
	case "512MiB":
		if testData512MiB == nil {
			testData512MiB = benchFuzzer.GenerateToSize(targetSize)
		}
		return testData512MiB
	case "1GiB":
		if testData1GiB == nil {
			testData1GiB = benchFuzzer.GenerateToSize(targetSize)
		}
		return testData1GiB
	}
	return nil
}

func getParsedData(sizeName string, data []byte) any {
	switch sizeName {
	case "32MiB":
		if parsed32MiB == nil {
			json.Unmarshal(data, &parsed32MiB)
		}
		return parsed32MiB
	case "64MiB":
		if parsed64MiB == nil {
			json.Unmarshal(data, &parsed64MiB)
		}
		return parsed64MiB
	case "128MiB":
		if parsed128MiB == nil {
			json.Unmarshal(data, &parsed128MiB)
		}
		return parsed128MiB
	case "256MiB":
		if parsed256MiB == nil {
			json.Unmarshal(data, &parsed256MiB)
		}
		return parsed256MiB
	case "512MiB":
		if parsed512MiB == nil {
			json.Unmarshal(data, &parsed512MiB)
		}
		return parsed512MiB
	case "1GiB":
		if parsed1GiB == nil {
			json.Unmarshal(data, &parsed1GiB)
		}
		return parsed1GiB
	}
	return nil
}

// ==================== 32 MiB BENCHMARKS ====================

func BenchmarkSize32MiB_SimpleField_NQJSON(b *testing.B) {
	data := getTestData("32MiB")
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "records.10000.name")
	}
}

func BenchmarkSize32MiB_SimpleField_Gojq(b *testing.B) {
	data := getTestData("32MiB")
	parsed := getParsedData("32MiB", data)
	b.ReportAllocs()
	b.ResetTimer()

	query, _ := gojq.Parse(".records[10000].name")
	code, _ := gojq.Compile(query)

	for i := 0; i < b.N; i++ {
		iter := code.Run(parsed)
		gojqResultSink, _ = iter.Next()
	}
}

func BenchmarkSize32MiB_DeepNested_NQJSON(b *testing.B) {
	data := getTestData("32MiB")
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "records.10000.profile.location.city")
	}
}

func BenchmarkSize32MiB_LastElement_NQJSON(b *testing.B) {
	data := getTestData("32MiB")
	// 32MiB / 500 bytes per record ≈ 64000 records
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "records.60000.id")
	}
}

// ==================== 64 MiB BENCHMARKS ====================

func BenchmarkSize64MiB_SimpleField_NQJSON(b *testing.B) {
	data := getTestData("64MiB")
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "records.50000.name")
	}
}

func BenchmarkSize64MiB_SimpleField_Gojq(b *testing.B) {
	data := getTestData("64MiB")
	parsed := getParsedData("64MiB", data)
	b.ReportAllocs()
	b.ResetTimer()

	query, _ := gojq.Parse(".records[50000].name")
	code, _ := gojq.Compile(query)

	for i := 0; i < b.N; i++ {
		iter := code.Run(parsed)
		gojqResultSink, _ = iter.Next()
	}
}

func BenchmarkSize64MiB_DeepNested_NQJSON(b *testing.B) {
	data := getTestData("64MiB")
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "records.50000.profile.location.city")
	}
}

func BenchmarkSize64MiB_LastElement_NQJSON(b *testing.B) {
	data := getTestData("64MiB")
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "records.120000.id")
	}
}

// ==================== 128 MiB BENCHMARKS ====================

func BenchmarkSize128MiB_SimpleField_NQJSON(b *testing.B) {
	data := getTestData("128MiB")
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "records.100000.name")
	}
}

func BenchmarkSize128MiB_SimpleField_Gojq(b *testing.B) {
	data := getTestData("128MiB")
	parsed := getParsedData("128MiB", data)
	b.ReportAllocs()
	b.ResetTimer()

	query, _ := gojq.Parse(".records[100000].name")
	code, _ := gojq.Compile(query)

	for i := 0; i < b.N; i++ {
		iter := code.Run(parsed)
		gojqResultSink, _ = iter.Next()
	}
}

func BenchmarkSize128MiB_DeepNested_NQJSON(b *testing.B) {
	data := getTestData("128MiB")
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "records.100000.profile.location.city")
	}
}

// ==================== 256 MiB BENCHMARKS ====================

func BenchmarkSize256MiB_SimpleField_NQJSON(b *testing.B) {
	data := getTestData("256MiB")
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "records.200000.name")
	}
}

func BenchmarkSize256MiB_SimpleField_Gojq(b *testing.B) {
	data := getTestData("256MiB")
	parsed := getParsedData("256MiB", data)
	b.ReportAllocs()
	b.ResetTimer()

	query, _ := gojq.Parse(".records[200000].name")
	code, _ := gojq.Compile(query)

	for i := 0; i < b.N; i++ {
		iter := code.Run(parsed)
		gojqResultSink, _ = iter.Next()
	}
}

func BenchmarkSize256MiB_DeepNested_NQJSON(b *testing.B) {
	data := getTestData("256MiB")
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "records.200000.profile.location.city")
	}
}

// ==================== 512 MiB BENCHMARKS ====================

func BenchmarkSize512MiB_SimpleField_NQJSON(b *testing.B) {
	data := getTestData("512MiB")
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "records.500000.name")
	}
}

func BenchmarkSize512MiB_SimpleField_Gojq(b *testing.B) {
	data := getTestData("512MiB")
	parsed := getParsedData("512MiB", data)
	b.ReportAllocs()
	b.ResetTimer()

	query, _ := gojq.Parse(".records[500000].name")
	code, _ := gojq.Compile(query)

	for i := 0; i < b.N; i++ {
		iter := code.Run(parsed)
		gojqResultSink, _ = iter.Next()
	}
}

func BenchmarkSize512MiB_DeepNested_NQJSON(b *testing.B) {
	data := getTestData("512MiB")
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "records.500000.profile.location.city")
	}
}

// ==================== 1 GiB BENCHMARKS ====================

func BenchmarkSize1GiB_SimpleField_NQJSON(b *testing.B) {
	data := getTestData("1GiB")
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "records.1000000.name")
	}
}

func BenchmarkSize1GiB_SimpleField_Gojq(b *testing.B) {
	data := getTestData("1GiB")
	parsed := getParsedData("1GiB", data)
	b.ReportAllocs()
	b.ResetTimer()

	query, _ := gojq.Parse(".records[1000000].name")
	code, _ := gojq.Compile(query)

	for i := 0; i < b.N; i++ {
		iter := code.Run(parsed)
		gojqResultSink, _ = iter.Next()
	}
}

func BenchmarkSize1GiB_DeepNested_NQJSON(b *testing.B) {
	data := getTestData("1GiB")
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "records.1000000.profile.location.city")
	}
}

// ==================== SET BENCHMARKS ====================

func BenchmarkSize32MiB_Set_NQJSON(b *testing.B) {
	data := getTestData("32MiB")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Set(data, "records.10000.name", "Updated Name")
	}
}

func BenchmarkSize64MiB_Set_NQJSON(b *testing.B) {
	data := getTestData("64MiB")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Set(data, "records.50000.name", "Updated Name")
	}
}

// ==================== SIZE INFO TEST ====================

func TestFuzzerSizes(t *testing.T) {
	for _, sizeName := range []string{"32MiB", "64MiB"} {
		data := getTestData(sizeName)
		info := GetDataSizeInfo(data)
		t.Logf("%s: %s (actual: %d bytes)", sizeName, info.Description, len(data))
	}
}

// TestFuzzerDataQuality verifies that generated data is valid for queries
func TestFuzzerDataQuality(t *testing.T) {
	data := benchFuzzer.GenerateToSize(1024 * 1024) // 1MB test

	// Test basic queries work
	result := nqjson.Get(data, "metadata.version")
	if result.Str != "2.0.0" {
		t.Errorf("Expected version 2.0.0, got %s", result.Str)
	}

	result = nqjson.Get(data, "records.0.id")
	if result.Num != 0 {
		t.Errorf("Expected id 0, got %.0f", result.Num)
	}

	result = nqjson.Get(data, "records.100.name")
	if result.Str == "" {
		t.Error("Expected non-empty name")
	}

	result = nqjson.Get(data, "records.50.profile.location.city")
	if result.Str == "" {
		t.Error("Expected non-empty city")
	}

	t.Logf("Generated %s of valid JSON", fmt.Sprintf("%.2f MB", float64(len(data))/1024/1024))
}
