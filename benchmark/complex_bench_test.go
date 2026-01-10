package benchmark

import (
	"encoding/json"
	"testing"

	"github.com/dhawalhost/nqjson"
	"github.com/itchyny/gojq"
)

// ==================== COMPLEX NESTED DATA BENCHMARKS ====================

// Pre-generated complex test data
var (
	complexHierarchyData []byte // organizations → departments → teams → projects → tasks → subtasks → comments
	complexOrderData     []byte // orders → lineItems → variants → modifiers
	complexMixedData     []byte // Mixed complex structures

	parsedComplexHierarchy any
	parsedComplexOrder     any
	parsedComplexMixed     any
)

func getComplexHierarchyData() []byte {
	if complexHierarchyData == nil {
		// 5 orgs × 4 depts × 3 teams × 4 projects × 5 tasks × 3 subtasks = 3600 subtasks
		// Each subtask has 2-4 comments → ~10,000 comments total
		// 7 levels of nesting
		complexHierarchyData = GenerateComplexJSON(5, 4, 3, 4, 5, 3)
	}
	return complexHierarchyData
}

func getComplexOrderData() []byte {
	if complexOrderData == nil {
		// 10,000 orders × 5 items = 50,000 line items
		// Each item has 2-3 variants × 1-2 modifiers = ~100,000+ nested elements
		complexOrderData = GenerateOrdersJSON(10000, 5)
	}
	return complexOrderData
}

func getComplexMixedData() []byte {
	if complexMixedData == nil {
		complexMixedData = GenerateMixedComplexJSON(50 * 1024 * 1024) // 50MB mixed
	}
	return complexMixedData
}

// ==================== HIERARCHY BENCHMARKS (7 LEVELS DEEP) ====================

// Path: organizations[2].departments[1].teams[0].projects[2].tasks[3].subtasks[1].comments[0].author
func BenchmarkComplexHierarchy_7LevelDeep_NQJSON(b *testing.B) {
	data := getComplexHierarchyData()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "organizations.2.departments.1.teams.0.projects.2.tasks.3.subtasks.1.comments.0.author")
	}
}

func BenchmarkComplexHierarchy_7LevelDeep_Gojq(b *testing.B) {
	data := getComplexHierarchyData()
	var parsed any
	json.Unmarshal(data, &parsed)
	b.ReportAllocs()
	b.ResetTimer()

	query, _ := gojq.Parse(".organizations[2].departments[1].teams[0].projects[2].tasks[3].subtasks[1].comments[0].author")
	code, _ := gojq.Compile(query)

	for i := 0; i < b.N; i++ {
		iter := code.Run(parsed)
		gojqResultSink, _ = iter.Next()
	}
}

// Access deepest nested array element: reactions inside comments
func BenchmarkComplexHierarchy_8LevelReactions_NQJSON(b *testing.B) {
	data := getComplexHierarchyData()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "organizations.2.departments.1.teams.0.projects.2.tasks.3.subtasks.1.comments.0.reactions.0.count")
	}
}

func BenchmarkComplexHierarchy_8LevelReactions_Gojq(b *testing.B) {
	data := getComplexHierarchyData()
	var parsed any
	json.Unmarshal(data, &parsed)
	b.ReportAllocs()
	b.ResetTimer()

	query, _ := gojq.Parse(".organizations[2].departments[1].teams[0].projects[2].tasks[3].subtasks[1].comments[0].reactions[0].count")
	code, _ := gojq.Compile(query)

	for i := 0; i < b.N; i++ {
		iter := code.Run(parsed)
		gojqResultSink, _ = iter.Next()
	}
}

// Mid-level access: just to projects level
func BenchmarkComplexHierarchy_4LevelMid_NQJSON(b *testing.B) {
	data := getComplexHierarchyData()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "organizations.3.departments.2.teams.1.projects.0.projectName")
	}
}

func BenchmarkComplexHierarchy_4LevelMid_Gojq(b *testing.B) {
	data := getComplexHierarchyData()
	var parsed any
	json.Unmarshal(data, &parsed)
	b.ReportAllocs()
	b.ResetTimer()

	query, _ := gojq.Parse(".organizations[3].departments[2].teams[1].projects[0].projectName")
	code, _ := gojq.Compile(query)

	for i := 0; i < b.N; i++ {
		iter := code.Run(parsed)
		gojqResultSink, _ = iter.Next()
	}
}

// ==================== E-COMMERCE ORDER BENCHMARKS ====================

// Path: orders[5000].lineItems[2].variants[1].modifiers[0].type
func BenchmarkComplexOrder_DeepVariant_NQJSON(b *testing.B) {
	data := getComplexOrderData()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "orders.5000.lineItems.2.variants.1.modifiers.0.type")
	}
}

func BenchmarkComplexOrder_DeepVariant_Gojq(b *testing.B) {
	data := getComplexOrderData()
	var parsed any
	json.Unmarshal(data, &parsed)
	b.ReportAllocs()
	b.ResetTimer()

	query, _ := gojq.Parse(".orders[5000].lineItems[2].variants[1].modifiers[0].type")
	code, _ := gojq.Compile(query)

	for i := 0; i < b.N; i++ {
		iter := code.Run(parsed)
		gojqResultSink, _ = iter.Next()
	}
}

// Access product category parent (nested objects inside arrays)
func BenchmarkComplexOrder_NestedCategory_NQJSON(b *testing.B) {
	data := getComplexOrderData()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "orders.8000.lineItems.3.product.category.parent.name")
	}
}

func BenchmarkComplexOrder_NestedCategory_Gojq(b *testing.B) {
	data := getComplexOrderData()
	var parsed any
	json.Unmarshal(data, &parsed)
	b.ReportAllocs()
	b.ResetTimer()

	query, _ := gojq.Parse(".orders[8000].lineItems[3].product.category.parent.name")
	code, _ := gojq.Compile(query)

	for i := 0; i < b.N; i++ {
		iter := code.Run(parsed)
		gojqResultSink, _ = iter.Next()
	}
}

// Access customer address (nested within order)
func BenchmarkComplexOrder_CustomerAddress_NQJSON(b *testing.B) {
	data := getComplexOrderData()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		nqjson.Get(data, "orders.9500.customer.addresses.1.city")
	}
}

func BenchmarkComplexOrder_CustomerAddress_Gojq(b *testing.B) {
	data := getComplexOrderData()
	var parsed any
	json.Unmarshal(data, &parsed)
	b.ReportAllocs()
	b.ResetTimer()

	query, _ := gojq.Parse(".orders[9500].customer.addresses[1].city")
	code, _ := gojq.Compile(query)

	for i := 0; i < b.N; i++ {
		iter := code.Run(parsed)
		gojqResultSink, _ = iter.Next()
	}
}

// ==================== DATA SIZE VERIFICATION ====================

func TestComplexDataSizes(t *testing.T) {
	hierarchy := getComplexHierarchyData()
	orders := getComplexOrderData()

	t.Logf("Complex Hierarchy Data: %.2f MB", float64(len(hierarchy))/1024/1024)
	t.Logf("Complex Order Data: %.2f MB", float64(len(orders))/1024/1024)

	// Verify data is valid
	r := nqjson.Get(hierarchy, "organizations.0.orgName")
	if r.Str == "" {
		t.Error("Expected non-empty orgName")
	}
	t.Logf("Sample org name: %s", r.Str)

	r = nqjson.Get(hierarchy, "organizations.2.departments.1.teams.0.projects.2.tasks.3.subtasks.1.comments.0.author")
	if r.Str == "" {
		t.Error("Expected non-empty 7-level deep author")
	}
	t.Logf("7-level deep author: %s", r.Str)

	r = nqjson.Get(orders, "orders.100.lineItems.2.variants.0.modifiers.0.type")
	if r.Str == "" {
		t.Error("Expected non-empty modifier type")
	}
	t.Logf("Order variant modifier type: %s", r.Str)
}
