package benchmark

import (
	"testing"

	sjson "github.com/tidwall/sjson"

	"github.com/dhawalhost/nqjson"
)

var (
	setBaseSimple = []byte(`{"name":"John","age":30,"profile":{"stats":{"score":90}},"items":[{"id":1,"qty":2}]}`)
	setBaseLarge  = []byte(`{
        "users": [
            {"id": 1, "name": "Alice", "profile": {"email": "alice@example.com", "score": 87.5}},
            {"id": 2, "name": "Bob", "profile": {"email": "bob@example.com", "score": 64.2}},
            {"id": 3, "name": "Charlie", "profile": {"email": "charlie@example.com", "score": 92.3}}
        ],
        "metadata": {
            "created": "2025-01-01",
            "updated": "2025-10-16",
            "stats": {
                "total": 3,
                "active": 2
            }
        }
    }`)
	setBaseDeep = []byte(`{
        "level1": {
            "level2": {
                "level3": {
                    "level4": {
                        "level5": {
                            "value": 42
                        }
                    }
                }
            }
        }
    }`)
)

func benchmarkNQJSONSet(b *testing.B, data []byte, path string, value interface{}) {
	b.Helper()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		working := append([]byte(nil), data...)
		result, err := nqjson.Set(working, path, value)
		if err != nil {
			b.Fatalf("nqjson set failed for path %s: %v", path, err)
		}
		resultSink = string(result)
	}
}

func benchmarkSJSONSet(b *testing.B, data []byte, path string, value interface{}) {
	b.Helper()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		working := append([]byte(nil), data...)
		result, err := sjson.SetBytes(working, path, value)
		if err != nil {
			b.Fatalf("sjson set failed for path %s: %v", path, err)
		}
		resultSink = string(result)
	}
}

func BenchmarkSet_SimpleField_NQJSON(b *testing.B) {
	benchmarkNQJSONSet(b, setBaseSimple, "profile.stats.score", 91)
}

func BenchmarkSet_SimpleField_SJSON(b *testing.B) {
	benchmarkSJSONSet(b, setBaseSimple, "profile.stats.score", 91)
}

func BenchmarkSet_DeepCreate_NQJSON(b *testing.B) {
	benchmarkNQJSONSet(b, setBaseSimple, "profile.preferences.ui.theme", "dark")
}

func BenchmarkSet_DeepCreate_SJSON(b *testing.B) {
	benchmarkSJSONSet(b, setBaseSimple, "profile.preferences.ui.theme", "dark")
}

func BenchmarkSet_ArrayAppend_NQJSON(b *testing.B) {
	benchmarkNQJSONSet(b, setBaseSimple, "items.-1", map[string]interface{}{"id": 2, "qty": 5})
}

func BenchmarkSet_ArrayAppend_SJSON(b *testing.B) {
	benchmarkSJSONSet(b, setBaseSimple, "items.-1", map[string]interface{}{"id": 2, "qty": 5})
}

// ==================== NESTED ARRAY BENCHMARKS ====================

func BenchmarkSet_ArrayElementUpdate_NQJSON(b *testing.B) {
	benchmarkNQJSONSet(b, setBaseLarge, "users.0.profile.score", 95.0)
}

func BenchmarkSet_ArrayElementUpdate_SJSON(b *testing.B) {
	benchmarkSJSONSet(b, setBaseLarge, "users.0.profile.score", 95.0)
}

func BenchmarkSet_ArrayMiddleElement_NQJSON(b *testing.B) {
	benchmarkNQJSONSet(b, setBaseLarge, "users.1.name", "Robert")
}

func BenchmarkSet_ArrayMiddleElement_SJSON(b *testing.B) {
	benchmarkSJSONSet(b, setBaseLarge, "users.1.name", "Robert")
}

func BenchmarkSet_ArrayLastElement_NQJSON(b *testing.B) {
	benchmarkNQJSONSet(b, setBaseLarge, "users.2.profile.email", "charlie@newdomain.com")
}

func BenchmarkSet_ArrayLastElement_SJSON(b *testing.B) {
	benchmarkSJSONSet(b, setBaseLarge, "users.2.profile.email", "charlie@newdomain.com")
}

// ==================== DEEP NESTING BENCHMARKS ====================

func BenchmarkSet_DeepNested_NQJSON(b *testing.B) {
	benchmarkNQJSONSet(b, setBaseDeep, "level1.level2.level3.level4.level5.value", 100)
}

func BenchmarkSet_DeepNested_SJSON(b *testing.B) {
	benchmarkSJSONSet(b, setBaseDeep, "level1.level2.level3.level4.level5.value", 100)
}

func BenchmarkSet_DeepNestedCreate_NQJSON(b *testing.B) {
	benchmarkNQJSONSet(b, setBaseDeep, "level1.level2.level3.level4.level5.level6.newval", "created")
}

func BenchmarkSet_DeepNestedCreate_SJSON(b *testing.B) {
	benchmarkSJSONSet(b, setBaseDeep, "level1.level2.level3.level4.level5.level6.newval", "created")
}

// ==================== METADATA UPDATE BENCHMARKS ====================

func BenchmarkSet_MetadataUpdate_NQJSON(b *testing.B) {
	benchmarkNQJSONSet(b, setBaseLarge, "metadata.updated", "2025-10-17")
}

func BenchmarkSet_MetadataUpdate_SJSON(b *testing.B) {
	benchmarkSJSONSet(b, setBaseLarge, "metadata.updated", "2025-10-17")
}

func BenchmarkSet_NestedStats_NQJSON(b *testing.B) {
	benchmarkNQJSONSet(b, setBaseLarge, "metadata.stats.total", 4)
}

func BenchmarkSet_NestedStats_SJSON(b *testing.B) {
	benchmarkSJSONSet(b, setBaseLarge, "metadata.stats.total", 4)
}

// ==================== COMPLEX VALUE BENCHMARKS ====================

func BenchmarkSet_ObjectValue_NQJSON(b *testing.B) {
	value := map[string]interface{}{
		"theme":    "dark",
		"language": "en",
		"timezone": "UTC",
	}
	benchmarkNQJSONSet(b, setBaseSimple, "profile.preferences", value)
}

func BenchmarkSet_ObjectValue_SJSON(b *testing.B) {
	value := map[string]interface{}{
		"theme":    "dark",
		"language": "en",
		"timezone": "UTC",
	}
	benchmarkSJSONSet(b, setBaseSimple, "profile.preferences", value)
}

func BenchmarkSet_ArrayValue_NQJSON(b *testing.B) {
	value := []interface{}{1, 2, 3, 4, 5}
	benchmarkNQJSONSet(b, setBaseSimple, "profile.badges", value)
}

func BenchmarkSet_ArrayValue_SJSON(b *testing.B) {
	value := []interface{}{1, 2, 3, 4, 5}
	benchmarkSJSONSet(b, setBaseSimple, "profile.badges", value)
}

// ==================== DELETE BENCHMARKS ====================

func benchmarkNQJSONDelete(b *testing.B, data []byte, path string) {
	b.Helper()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		working := append([]byte(nil), data...)
		result, err := nqjson.Delete(working, path)
		if err != nil {
			b.Fatalf("nqjson delete failed for path %s: %v", path, err)
		}
		resultSink = string(result)
	}
}

func benchmarkSJSONDelete(b *testing.B, data []byte, path string) {
	b.Helper()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		working := append([]byte(nil), data...)
		result, err := sjson.DeleteBytes(working, path)
		if err != nil {
			b.Fatalf("sjson delete failed for path %s: %v", path, err)
		}
		resultSink = string(result)
	}
}

func BenchmarkDelete_SimpleField_NQJSON(b *testing.B) {
	benchmarkNQJSONDelete(b, setBaseSimple, "age")
}

func BenchmarkDelete_SimpleField_SJSON(b *testing.B) {
	benchmarkSJSONDelete(b, setBaseSimple, "age")
}

func BenchmarkDelete_NestedField_NQJSON(b *testing.B) {
	benchmarkNQJSONDelete(b, setBaseSimple, "profile.stats.score")
}

func BenchmarkDelete_NestedField_SJSON(b *testing.B) {
	benchmarkSJSONDelete(b, setBaseSimple, "profile.stats.score")
}

func BenchmarkDelete_ArrayElement_NQJSON(b *testing.B) {
	benchmarkNQJSONDelete(b, setBaseLarge, "users.1")
}

func BenchmarkDelete_ArrayElement_SJSON(b *testing.B) {
	benchmarkSJSONDelete(b, setBaseLarge, "users.1")
}

func BenchmarkDelete_DeepNested_NQJSON(b *testing.B) {
	benchmarkNQJSONDelete(b, setBaseDeep, "level1.level2.level3.level4.level5.value")
}

func BenchmarkDelete_DeepNested_SJSON(b *testing.B) {
	benchmarkSJSONDelete(b, setBaseDeep, "level1.level2.level3.level4.level5.value")
}

// ==================== MULTIPLE OPERATIONS BENCHMARKS ====================

func BenchmarkSet_MultipleUpdates_NQJSON(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		working := append([]byte(nil), setBaseLarge...)
		var err error
		working, err = nqjson.Set(working, "users.0.name", "Alicia")
		if err != nil {
			b.Fatal(err)
		}
		working, err = nqjson.Set(working, "users.1.profile.score", 70.5)
		if err != nil {
			b.Fatal(err)
		}
		working, err = nqjson.Set(working, "metadata.stats.active", 3)
		if err != nil {
			b.Fatal(err)
		}
		resultSink = string(working)
	}
}

func BenchmarkSet_MultipleUpdates_SJSON(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		working := append([]byte(nil), setBaseLarge...)
		var err error
		working, err = sjson.SetBytes(working, "users.0.name", "Alicia")
		if err != nil {
			b.Fatal(err)
		}
		working, err = sjson.SetBytes(working, "users.1.profile.score", 70.5)
		if err != nil {
			b.Fatal(err)
		}
		working, err = sjson.SetBytes(working, "metadata.stats.active", 3)
		if err != nil {
			b.Fatal(err)
		}
		resultSink = string(working)
	}
}
