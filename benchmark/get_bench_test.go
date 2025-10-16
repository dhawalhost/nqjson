package benchmark

import (
	"fmt"
	"testing"

	gjson "github.com/tidwall/gjson"

	"github.com/dhawalhost/njson"
)

var (
	simpleSmallJSON  = []byte(`{"name":"John","age":30,"active":true}`)
	simpleMediumJSON = []byte(`{
        "user": {
            "profile": {
                "address": {
                    "city": "New York",
                    "zip": "10001"
                },
                "stats": {
                    "followers": 1280,
                    "following": 523
                }
            }
        }
    }`)
	complexMediumJSON = []byte(`{
        "metrics": {
            "readings": [
                {"ts": 1700000001, "value": 31.5},
                {"ts": 1700000002, "value": 30.2},
                {"ts": 1700000003, "value": 29.9},
                {"ts": 1700000004, "value": 31.1}
            ]
        },
        "status": {
            "healthy": true,
            "retries": 2
        }
    }`)
	wildcardJSON = []byte(`{
        "teams": {
            "alpha": {"lead": "Alice", "members": ["Tom", "Eve"]},
            "beta": {"lead": "Bob", "members": ["Raj", "Ivy"]},
            "gamma": {"lead": "Carol", "members": ["Uma", "Noel"]}
        }
    }`)
	largeDeepJSON = []byte(`{
        "root": {
            "level1": {
                "level2": {
                    "level3": {
                        "level4": {
                            "level5": {
                                "value": 42,
                                "metadata": {
                                    "source": "sensor",
                                    "updated": "2025-01-01T00:00:00Z"
                                }
                            }
                        }
                    }
                }
            }
        }
    }`)
	projectedJSON = []byte(`{
        "systems": [
            {
                "name": "alpha",
                "services": [
                    {"name": "auth", "version": "1.0.0"},
                    {"name": "billing", "version": "1.1.3"}
                ]
            },
            {
                "name": "beta",
                "services": [
                    {"name": "search", "version": "2.2.0"},
                    {"name": "cache", "version": "2.1.0"}
                ]
            }
        ]
    }`)
	jsonLines = []byte(`{"name":"Gilbert","age":61}
{"name":"Alexa","age":34}
{"name":"May","age":57}
{"name":"Deloise","age":44}
`)
	modifierJSON = []byte(`{
        "nums": [1, 5, 3, 8, 2, 9, 4],
        "items": [
            {"id": 1, "name": "apple", "price": 1.5},
            {"id": 2, "name": "banana", "price": 0.8},
            {"id": 3, "name": "cherry", "price": 2.2},
            {"id": 1, "name": "apple", "price": 1.5},
            {"id": 2, "name": "banana", "price": 0.8}
        ],
        "nested": {
            "data": [
                {"values": [1, 2, 3]},
                {"values": [4, 5, 6]},
                {"values": [7, 8, 9]}
            ]
        },
        "scores": [95.5, 87.3, 92.1, 88.7, 91.2]
    }`)
	multipathJSON = []byte(`{
        "user": {
            "id": 123,
            "name": "Alice",
            "email": "alice@example.com",
            "age": 28,
            "active": true
        },
        "metadata": {
            "created": "2025-01-01",
            "updated": "2025-10-16"
        }
    }`)
	largeArrayJSON = func() []byte {
		// Generate array with 1,000 elements for stress testing (reduced from 10k for reasonable benchmarks)
		json := `{"items":[`
		for i := 0; i < 1000; i++ {
			if i > 0 {
				json += ","
			}
			// Use string formatting for proper JSON numbers
			json += fmt.Sprintf(`{"id":%d,"val":%d}`, i, i*2)
		}
		json += `]}`
		return []byte(json)
	}()
)

var resultSink string

func benchmarkNJSONGet(b *testing.B, data []byte, path string) {
	b.Helper()
	b.ReportAllocs()

	var res njson.Result
	for i := 0; i < b.N; i++ {
		res = njson.Get(data, path)
	}
	if !res.Exists() {
		b.Fatalf("njson result missing for path %s", path)
	}
	resultSink = res.String()
}

func benchmarkGJSONGet(b *testing.B, data []byte, path string) {
	b.Helper()
	b.ReportAllocs()

	var res gjson.Result
	for i := 0; i < b.N; i++ {
		res = gjson.GetBytes(data, path)
	}
	if !res.Exists() {
		b.Fatalf("gjson result missing for path %s", path)
	}
	resultSink = res.String()
}

func BenchmarkGet_SimpleSmall_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, simpleSmallJSON, "name")
}

func BenchmarkGet_SimpleSmall_GJSON(b *testing.B) {
	benchmarkGJSONGet(b, simpleSmallJSON, "name")
}

func BenchmarkGet_SimpleMedium_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, simpleMediumJSON, "user.profile.address.city")
}

func BenchmarkGet_SimpleMedium_GJSON(b *testing.B) {
	benchmarkGJSONGet(b, simpleMediumJSON, "user.profile.address.city")
}

func BenchmarkGet_ComplexMedium_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, complexMediumJSON, "metrics.readings.2.value")
}

func BenchmarkGet_ComplexMedium_GJSON(b *testing.B) {
	benchmarkGJSONGet(b, complexMediumJSON, "metrics.readings.2.value")
}

func BenchmarkGet_WildcardLeads_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, wildcardJSON, "teams.*.lead")
}

func BenchmarkGet_WildcardLeads_GJSON(b *testing.B) {
	benchmarkGJSONGet(b, wildcardJSON, "teams.*.lead")
}

func BenchmarkGet_LargeDeep_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, largeDeepJSON, "root.level1.level2.level3.level4.level5.value")
}

func BenchmarkGet_LargeDeep_GJSON(b *testing.B) {
	benchmarkGJSONGet(b, largeDeepJSON, "root.level1.level2.level3.level4.level5.value")
}

func BenchmarkGet_ProjectServices_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, projectedJSON, "systems.#.services.#.name")
}

func BenchmarkGet_ProjectServices_GJSON(b *testing.B) {
	benchmarkGJSONGet(b, projectedJSON, "systems.#.services.#.name")
}

func BenchmarkGet_JSONLinesName_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, jsonLines, "..#.name")
}

func BenchmarkGet_JSONLinesName_GJSON(b *testing.B) {
	benchmarkGJSONGet(b, jsonLines, "..#.name")
}

// ==================== MULTIPATH BENCHMARKS ====================
// Note: Multipath is an njson-specific feature, not supported by gjson

func BenchmarkGet_MultiPath_TwoFields_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, multipathJSON, "user.name,user.email")
}

func BenchmarkGet_MultiPath_FiveFields_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, multipathJSON, "user.id,user.name,user.email,user.age,user.active")
}

func BenchmarkGet_MultiPath_Mixed_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, multipathJSON, "user.name,metadata.created,user.active")
}

// ==================== EXTENDED MODIFIER BENCHMARKS ====================
// gjson supports: @reverse, @flatten
// njson adds: @distinct, @sort, @first, @last, @sum, @avg, @min, @max

func BenchmarkGet_Modifier_Reverse_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, modifierJSON, "nums|@reverse")
}

func BenchmarkGet_Modifier_Reverse_GJSON(b *testing.B) {
	benchmarkGJSONGet(b, modifierJSON, "nums|@reverse")
}

func BenchmarkGet_Modifier_Flatten_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, modifierJSON, "nested.data.#.values|@flatten")
}

func BenchmarkGet_Modifier_Flatten_GJSON(b *testing.B) {
	benchmarkGJSONGet(b, modifierJSON, "nested.data.#.values|@flatten")
}

func BenchmarkGet_Modifier_Distinct_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, modifierJSON, "items.#.id|@distinct")
}

func BenchmarkGet_Modifier_Sort_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, modifierJSON, "nums|@sort")
}

func BenchmarkGet_Modifier_First_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, modifierJSON, "nums|@first")
}

func BenchmarkGet_Modifier_Last_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, modifierJSON, "nums|@last")
}

func BenchmarkGet_Modifier_Sum_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, modifierJSON, "nums|@sum")
}

func BenchmarkGet_Modifier_Avg_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, modifierJSON, "scores|@avg")
}

func BenchmarkGet_Modifier_Min_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, modifierJSON, "nums|@min")
}

func BenchmarkGet_Modifier_Max_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, modifierJSON, "nums|@max")
}

// ==================== COMPLEX COMBINED BENCHMARKS ====================

func BenchmarkGet_MultiPath_WithModifier_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, modifierJSON, "nums|@reverse,scores|@avg")
}

func BenchmarkGet_JSONLines_WithProjection_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, jsonLines, "..#.name")
}

func BenchmarkGet_JSONLines_WithProjection_GJSON(b *testing.B) {
	benchmarkGJSONGet(b, jsonLines, "..#.name")
}

func BenchmarkGet_JSONLines_Indexed_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, jsonLines, "..2.age")
}

func BenchmarkGet_JSONLines_Indexed_GJSON(b *testing.B) {
	benchmarkGJSONGet(b, jsonLines, "..2.age")
}

// ==================== LARGE DATASET BENCHMARKS ====================

func BenchmarkGet_LargeArray_FirstElement_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, largeArrayJSON, "items.0.id")
}

func BenchmarkGet_LargeArray_FirstElement_GJSON(b *testing.B) {
	benchmarkGJSONGet(b, largeArrayJSON, "items.0.id")
}

func BenchmarkGet_LargeArray_MiddleElement_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, largeArrayJSON, "items.500.id")
}

func BenchmarkGet_LargeArray_MiddleElement_GJSON(b *testing.B) {
	benchmarkGJSONGet(b, largeArrayJSON, "items.500.id")
}

func BenchmarkGet_LargeArray_LastElement_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, largeArrayJSON, "items.999.val")
}

func BenchmarkGet_LargeArray_LastElement_GJSON(b *testing.B) {
	benchmarkGJSONGet(b, largeArrayJSON, "items.999.val")
}

func BenchmarkGet_LargeArray_Count_NJSON(b *testing.B) {
	benchmarkNJSONGet(b, largeArrayJSON, "items.#")
}

func BenchmarkGet_LargeArray_Count_GJSON(b *testing.B) {
	benchmarkGJSONGet(b, largeArrayJSON, "items.#")
}
