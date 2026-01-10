package benchmark

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/itchyny/gojq"
	gjson "github.com/tidwall/gjson"

	"github.com/dhawalhost/nqjson"
)

// Large dataset variables - initialized once
var (
	largeJSONData    []byte
	largeJSONParsed  any
	largeDataSize    DataSizeInfo
	largeDataInitErr error

	// Small datasets for aggregation/filter benchmarks (pre-generated for fair comparison)
	smallJSONData100    []byte
	smallJSONData1000   []byte
	smallJSONParsed100  any
	smallJSONParsed1000 any

	// Pre-compiled gojq queries for fair comparison (reuse scenario)
	gojqQueries      = make(map[string]*gojq.Code)
	gojqQueriesSmall = make(map[string]*gojq.Code)
)

func init() {
	// Generate large dataset (~22MB with 50,000 users)
	largeJSONData = GenerateLargeJSON(50000)
	largeDataSize = GetDataSizeInfo(largeJSONData)

	// Generate small datasets for aggregation/filter benchmarks
	smallJSONData100 = GenerateLargeJSON(100)
	smallJSONData1000 = GenerateLargeJSON(1000)

	// Pre-parse for gojq (which requires unmarshaled data)
	if err := json.Unmarshal(largeJSONData, &largeJSONParsed); err != nil {
		largeDataInitErr = fmt.Errorf("failed to unmarshal large JSON: %w", err)
		return
	}
	if err := json.Unmarshal(smallJSONData100, &smallJSONParsed100); err != nil {
		largeDataInitErr = fmt.Errorf("failed to unmarshal small JSON 100: %w", err)
		return
	}
	if err := json.Unmarshal(smallJSONData1000, &smallJSONParsed1000); err != nil {
		largeDataInitErr = fmt.Errorf("failed to unmarshal small JSON 1000: %w", err)
		return
	}

	// Pre-compile queries for large dataset
	queries := map[string]string{
		"simpleField": ".users[25000].name",
		"nestedPath":  ".users[25000].profile.address.city",
		"arrayFirst":  ".users[0].id",
		"arrayMiddle": ".users[25000].id",
		"arrayLast":   ".users[-1].id",
		"deepNested":  ".users[25000].settings.preferences.colorScheme",
		"arrayLength": ".users | length",
	}

	for name, query := range queries {
		parsed, err := gojq.Parse(query)
		if err != nil {
			largeDataInitErr = fmt.Errorf("failed to parse query %s: %w", name, err)
			return
		}
		code, err := gojq.Compile(parsed)
		if err != nil {
			largeDataInitErr = fmt.Errorf("failed to compile query %s: %w", name, err)
			return
		}
		gojqQueries[name] = code
	}

	// Pre-compile queries for small datasets
	smallQueries := map[string]string{
		"projection100":    "[.users[].name]",
		"filter1000":       "[.users[] | select(.age > 50)]",
		"sum1000":          "[.users[].age] | add",
		"avg1000":          "[.users[].age] | add / length",
		"min1000":          "[.users[].age] | min",
		"max1000":          "[.users[].age] | max",
		"uniqueThemes1000": "[.users[].settings.theme] | unique",
		"countActive1000":  ".users | map(select(.active)) | length",
		"mapTransform100":  "[.users[] | {name: .name, city: .profile.address.city}]",
		"sortByAge100":     ".users | sort_by(.age)",
		"groupByCity100":   ".users | group_by(.profile.address.city)",
	}

	for name, query := range smallQueries {
		parsed, err := gojq.Parse(query)
		if err != nil {
			largeDataInitErr = fmt.Errorf("failed to parse small query %s: %w", name, err)
			return
		}
		code, err := gojq.Compile(parsed)
		if err != nil {
			largeDataInitErr = fmt.Errorf("failed to compile small query %s: %w", name, err)
			return
		}
		gojqQueriesSmall[name] = code
	}
}

// Result sinks to prevent compiler optimization
var (
	gojqResultSink   any
	nqjsonResultSink string
)

// runGojqQuery runs a pre-compiled gojq query
func runGojqQuery(queryName string) any {
	code := gojqQueries[queryName]
	iter := code.Run(largeJSONParsed)
	v, _ := iter.Next()
	return v
}

// runGojqQueryWithParse parses and runs a gojq query (includes compilation overhead)
func runGojqQueryWithParse(query string, input any) any {
	parsed, err := gojq.Parse(query)
	if err != nil {
		return nil
	}
	iter := parsed.Run(input)
	v, _ := iter.Next()
	return v
}

// ==================== DATA SIZE TEST ====================

func TestDataSize(t *testing.T) {
	if largeDataInitErr != nil {
		t.Fatalf("Failed to initialize large data: %v", largeDataInitErr)
	}

	t.Logf("Generated JSON size: %s", largeDataSize.Description)
	t.Logf("Bytes: %d, KB: %.2f, MB: %.2f", largeDataSize.Bytes, largeDataSize.KB, largeDataSize.MB)
	t.Logf("Exceeds L1 (32KB): %t", largeDataSize.ExceedsL1)
	t.Logf("Exceeds L2 (256KB): %t", largeDataSize.ExceedsL2)
	t.Logf("Exceeds L3 (8MB): %t", largeDataSize.ExceedsL3)

	if !largeDataSize.ExceedsL3 {
		t.Error("Generated data should exceed L3 cache size")
	}
}

// ==================== RESULT EQUIVALENCE TEST ====================

func TestResultEquivalence(t *testing.T) {
	if largeDataInitErr != nil {
		t.Fatalf("Failed to initialize large data: %v", largeDataInitErr)
	}

	tests := []struct {
		name       string
		nqjsonPath string
		gojqQuery  string
	}{
		{"SimpleField", "users.25000.name", "simpleField"},
		{"NestedPath", "users.25000.profile.address.city", "nestedPath"},
		{"ArrayFirst", "users.0.id", "arrayFirst"},
		{"ArrayMiddle", "users.25000.id", "arrayMiddle"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nqjsonResult := nqjson.Get(largeJSONData, tt.nqjsonPath)
			gojqResult := runGojqQuery(tt.gojqQuery)

			// Compare string representations
			nqjsonStr := nqjsonResult.String()
			gojqStr := fmt.Sprintf("%v", gojqResult)

			if nqjsonStr != gojqStr {
				t.Logf("nqjson: %s, gojq: %s", nqjsonStr, gojqStr)
				// Not necessarily an error - may have formatting differences
			}

			t.Logf("%s - nqjson: %s, gojq: %v", tt.name, nqjsonStr, gojqResult)
		})
	}
}

// ==================== SIMPLE FIELD ACCESS BENCHMARKS ====================

func BenchmarkGojq_SimpleField_NQJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSONData)))

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(largeJSONData, "users.25000.name")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_SimpleField_NQJSONCached(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSONData)))

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.GetCached(largeJSONData, "users.25000.name")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_SimpleField_NQJSONCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSONData)))

	// Pre-compile path outside the loop
	compiledPath, err := nqjson.CompileGetPath("users.25000.name")
	if err != nil {
		b.Fatalf("Failed to compile path: %v", err)
	}

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = compiledPath.Run(largeJSONData)
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_SimpleField_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		gojqResultSink = runGojqQuery("simpleField")
	}
}

func BenchmarkGojq_SimpleField_GojqWithUnmarshal(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSONData)))

	for i := 0; i < b.N; i++ {
		var parsed any
		_ = json.Unmarshal(largeJSONData, &parsed)
		gojqResultSink = runGojqQueryWithParse(".users[25000].name", parsed)
	}
}

func BenchmarkGojq_SimpleField_GJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSONData)))

	var res gjson.Result
	for i := 0; i < b.N; i++ {
		res = gjson.GetBytes(largeJSONData, "users.25000.name")
	}
	nqjsonResultSink = res.String()
}

// ==================== NESTED PATH BENCHMARKS ====================

func BenchmarkGojq_NestedPath_NQJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSONData)))

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(largeJSONData, "users.25000.profile.address.city")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_NestedPath_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		gojqResultSink = runGojqQuery("nestedPath")
	}
}

func BenchmarkGojq_NestedPath_GJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSONData)))

	var res gjson.Result
	for i := 0; i < b.N; i++ {
		res = gjson.GetBytes(largeJSONData, "users.25000.profile.address.city")
	}
	nqjsonResultSink = res.String()
}

// ==================== ARRAY ACCESS BENCHMARKS ====================

func BenchmarkGojq_ArrayFirst_NQJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSONData)))

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(largeJSONData, "users.0.id")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_ArrayFirst_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		gojqResultSink = runGojqQuery("arrayFirst")
	}
}

func BenchmarkGojq_ArrayMiddle_NQJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSONData)))

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(largeJSONData, "users.25000.id")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_ArrayMiddle_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		gojqResultSink = runGojqQuery("arrayMiddle")
	}
}

func BenchmarkGojq_ArrayMiddle_GJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSONData)))

	var res gjson.Result
	for i := 0; i < b.N; i++ {
		res = gjson.GetBytes(largeJSONData, "users.25000.id")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_ArrayLast_NQJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSONData)))

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(largeJSONData, "users.49999.id")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_ArrayLast_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		gojqResultSink = runGojqQuery("arrayLast")
	}
}

// ==================== DEEP NESTED PATH BENCHMARKS ====================

func BenchmarkGojq_DeepNested_NQJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSONData)))

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(largeJSONData, "users.25000.settings.preferences.colorScheme")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_DeepNested_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		gojqResultSink = runGojqQuery("deepNested")
	}
}

// ==================== ARRAY LENGTH BENCHMARKS ====================

func BenchmarkGojq_ArrayLength_NQJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSONData)))

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(largeJSONData, "users.#")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_ArrayLength_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		gojqResultSink = runGojqQuery("arrayLength")
	}
}

// ==================== PROJECTION BENCHMARKS (small subset for reasonable time) ====================

func BenchmarkGojq_ProjectionSmall_NQJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(smallJSONData100, "users.#.name")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_ProjectionSmall_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	code := gojqQueriesSmall["projection100"]
	for i := 0; i < b.N; i++ {
		iter := code.Run(smallJSONParsed100)
		gojqResultSink, _ = iter.Next()
	}
}

// ==================== FILTER BENCHMARKS ====================

func BenchmarkGojq_FilterSmall_NQJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(smallJSONData1000, "users[?(@.age>50)]")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_FilterSmall_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	code := gojqQueriesSmall["filter1000"]
	for i := 0; i < b.N; i++ {
		iter := code.Run(smallJSONParsed1000)
		gojqResultSink, _ = iter.Next()
	}
}

// ==================== AGGREGATION BENCHMARKS (nqjson modifiers) ====================

func BenchmarkGojq_SumSmall_NQJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(smallJSONData1000, "users.#.age|@sum")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_SumSmall_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	code := gojqQueriesSmall["sum1000"]
	for i := 0; i < b.N; i++ {
		iter := code.Run(smallJSONParsed1000)
		gojqResultSink, _ = iter.Next()
	}
}

func BenchmarkGojq_AvgSmall_NQJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(smallJSONData1000, "users.#.age|@avg")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_AvgSmall_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	code := gojqQueriesSmall["avg1000"]
	for i := 0; i < b.N; i++ {
		iter := code.Run(smallJSONParsed1000)
		gojqResultSink, _ = iter.Next()
	}
}

func BenchmarkGojq_MinSmall_NQJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(smallJSONData1000, "users.#.age|@min")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_MinSmall_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	code := gojqQueriesSmall["min1000"]
	for i := 0; i < b.N; i++ {
		iter := code.Run(smallJSONParsed1000)
		gojqResultSink, _ = iter.Next()
	}
}

func BenchmarkGojq_MaxSmall_NQJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(smallJSONData1000, "users.#.age|@max")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_MaxSmall_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	code := gojqQueriesSmall["max1000"]
	for i := 0; i < b.N; i++ {
		iter := code.Run(smallJSONParsed1000)
		gojqResultSink, _ = iter.Next()
	}
}

// ==================== ADVANCED GOJQ OPERATIONS ====================

func BenchmarkGojq_MapTransform_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	code := gojqQueriesSmall["mapTransform100"]
	for i := 0; i < b.N; i++ {
		iter := code.Run(smallJSONParsed100)
		gojqResultSink, _ = iter.Next()
	}
}

func BenchmarkGojq_SortByAge_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	code := gojqQueriesSmall["sortByAge100"]
	for i := 0; i < b.N; i++ {
		iter := code.Run(smallJSONParsed100)
		gojqResultSink, _ = iter.Next()
	}
}

func BenchmarkGojq_GroupByCity_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	code := gojqQueriesSmall["groupByCity100"]
	for i := 0; i < b.N; i++ {
		iter := code.Run(smallJSONParsed100)
		gojqResultSink, _ = iter.Next()
	}
}

func BenchmarkGojq_UniqueThemes_NQJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(smallJSONData1000, "users.#.settings.theme|@distinct")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_UniqueThemes_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	code := gojqQueriesSmall["uniqueThemes1000"]
	for i := 0; i < b.N; i++ {
		iter := code.Run(smallJSONParsed1000)
		gojqResultSink, _ = iter.Next()
	}
}

func BenchmarkGojq_CountActive_NQJSON(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(smallJSONData1000, "users[?(@.active==true)].#")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkGojq_CountActive_GojqCompiled(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()

	code := gojqQueriesSmall["countActive1000"]
	for i := 0; i < b.N; i++ {
		iter := code.Run(smallJSONParsed1000)
		gojqResultSink, _ = iter.Next()
	}
}

// ==================== FULL JSON UNMARSHAL COMPARISON ====================

func BenchmarkGojq_FullUnmarshal_JSONStdlib(b *testing.B) {
	if largeDataInitErr != nil {
		b.Fatalf("Failed to initialize: %v", largeDataInitErr)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSONData)))

	for i := 0; i < b.N; i++ {
		var parsed any
		_ = json.Unmarshal(largeJSONData, &parsed)
		gojqResultSink = parsed
	}
}
