package benchmark

import (
	"fmt"
	"math/rand"
	"strings"
)

// Schema defines the structure for generating JSON documents
type Schema struct {
	Name   string  // Schema name
	Fields []Field // Top-level fields
}

// Field defines a field in the schema
type Field struct {
	Name     string  // Field name
	Type     string  // string, number, bool, object, array
	Children []Field // For object type
	ItemType string  // For array type: string, number, object
	Items    []Field // For array of objects
}

// Fuzzer generates JSON documents based on a schema
type Fuzzer struct {
	rng    *rand.Rand
	schema Schema
}

// NewFuzzer creates a new fuzzer with a seed for reproducibility
func NewFuzzer(seed int64, schema Schema) *Fuzzer {
	return &Fuzzer{
		rng:    rand.New(rand.NewSource(seed)),
		schema: schema,
	}
}

// DefaultSchema returns a schema for realistic user/config data
func DefaultSchema() Schema {
	return Schema{
		Name: "users",
		Fields: []Field{
			{Name: "metadata", Type: "object", Children: []Field{
				{Name: "version", Type: "string"},
				{Name: "generated", Type: "string"},
				{Name: "count", Type: "number"},
			}},
			{Name: "records", Type: "array", ItemType: "object", Items: []Field{
				{Name: "id", Type: "number"},
				{Name: "uuid", Type: "string"},
				{Name: "name", Type: "string"},
				{Name: "email", Type: "string"},
				{Name: "age", Type: "number"},
				{Name: "salary", Type: "number"},
				{Name: "active", Type: "bool"},
				{Name: "tags", Type: "array", ItemType: "string"},
				{Name: "profile", Type: "object", Children: []Field{
					{Name: "bio", Type: "string"},
					{Name: "avatar", Type: "string"},
					{Name: "location", Type: "object", Children: []Field{
						{Name: "city", Type: "string"},
						{Name: "country", Type: "string"},
						{Name: "lat", Type: "number"},
						{Name: "lng", Type: "number"},
					}},
				}},
				{Name: "settings", Type: "object", Children: []Field{
					{Name: "theme", Type: "string"},
					{Name: "notifications", Type: "bool"},
					{Name: "language", Type: "string"},
				}},
			}},
		},
	}
}

// GenerateToSize generates a JSON document of approximately the target size in bytes
func (f *Fuzzer) GenerateToSize(targetBytes int) []byte {
	// Estimate bytes per record (approximately 500 bytes per record with this schema)
	bytesPerRecord := 500
	estimatedRecords := targetBytes / bytesPerRecord

	return f.Generate(estimatedRecords)
}

// Generate creates a JSON document with the specified number of records
func (f *Fuzzer) Generate(recordCount int) []byte {
	var sb strings.Builder
	sb.Grow(recordCount * 500) // Pre-allocate

	sb.WriteString("{")

	for i, field := range f.schema.Fields {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`"%s":`, field.Name))

		if field.Name == "records" && field.Type == "array" {
			f.writeRecordArray(&sb, field.Items, recordCount)
		} else {
			f.writeField(&sb, field, 0)
		}
	}

	sb.WriteString("}")
	return []byte(sb.String())
}

func (f *Fuzzer) writeRecordArray(sb *strings.Builder, itemFields []Field, count int) {
	sb.WriteString("[")
	for i := 0; i < count; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		f.writeObject(sb, itemFields, i)
	}
	sb.WriteString("]")
}

func (f *Fuzzer) writeField(sb *strings.Builder, field Field, index int) {
	switch field.Type {
	case "string":
		sb.WriteString(fmt.Sprintf(`"%s"`, f.randomString(field.Name, index)))
	case "number":
		sb.WriteString(fmt.Sprintf("%v", f.randomNumber(field.Name, index)))
	case "bool":
		sb.WriteString(fmt.Sprintf("%t", f.rng.Intn(2) == 1))
	case "object":
		f.writeObject(sb, field.Children, index)
	case "array":
		f.writeArray(sb, field, index)
	}
}

func (f *Fuzzer) writeObject(sb *strings.Builder, fields []Field, index int) {
	sb.WriteString("{")
	for i, field := range fields {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`"%s":`, field.Name))
		f.writeField(sb, field, index)
	}
	sb.WriteString("}")
}

func (f *Fuzzer) writeArray(sb *strings.Builder, field Field, index int) {
	sb.WriteString("[")
	count := 3 + f.rng.Intn(3) // 3-5 items
	for i := 0; i < count; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		switch field.ItemType {
		case "string":
			sb.WriteString(fmt.Sprintf(`"%s%d"`, f.randomTag(), i))
		case "number":
			sb.WriteString(fmt.Sprintf("%d", f.rng.Intn(1000)))
		}
	}
	sb.WriteString("]")
}

// Random data generators
var (
	firstNames = []string{"Alice", "Bob", "Charlie", "Diana", "Eve", "Frank", "Grace", "Henry", "Ivy", "Jack",
		"Kate", "Leo", "Mia", "Noah", "Olivia", "Paul", "Quinn", "Rose", "Sam", "Tina"}
	lastNames = []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez",
		"Anderson", "Taylor", "Thomas", "Moore", "Jackson", "Martin", "Lee", "Thompson", "White", "Harris"}
	cities = []string{"New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "Philadelphia", "San Antonio", "San Diego",
		"Dallas", "Austin", "London", "Paris", "Tokyo", "Sydney", "Berlin", "Toronto", "Mumbai", "Singapore"}
	countries = []string{"USA", "Canada", "UK", "Germany", "France", "Australia", "Japan", "Brazil", "India", "Mexico",
		"Italy", "Spain", "Netherlands", "Sweden", "Norway", "Denmark", "Finland", "Ireland", "Poland", "Austria"}
	themes = []string{"light", "dark", "system", "custom", "blue", "green", "purple", "orange"}
	tags   = []string{"premium", "verified", "active", "new", "featured", "trending", "popular", "recommended", "vip", "beta"}
)

func (f *Fuzzer) randomString(fieldName string, index int) string {
	switch fieldName {
	case "name":
		return firstNames[index%len(firstNames)] + " " + lastNames[(index*7)%len(lastNames)]
	case "email":
		return fmt.Sprintf("user%d@example.com", index)
	case "uuid":
		return fmt.Sprintf("uuid-%d-%d-%d-%d", index, index*7, index*13, index*17)
	case "bio":
		return fmt.Sprintf("User %d biography with detailed professional information", index)
	case "avatar":
		return fmt.Sprintf("https://cdn.example.com/avatars/%d.jpg", index)
	case "city":
		return cities[index%len(cities)]
	case "country":
		return countries[index%len(countries)]
	case "theme":
		return themes[index%len(themes)]
	case "language":
		return "en"
	case "version":
		return "2.0.0"
	case "generated":
		return "2026-01-10T00:00:00Z"
	default:
		return fmt.Sprintf("value_%d", index)
	}
}

func (f *Fuzzer) randomNumber(fieldName string, index int) interface{} {
	switch fieldName {
	case "id":
		return index
	case "age":
		return 18 + (index % 62)
	case "salary":
		return 30000 + (index % 70000)
	case "count":
		return index
	case "lat":
		return 25.0 + float64(index%40) + float64(index%100)/100.0
	case "lng":
		return -120.0 + float64(index%240) + float64(index%100)/100.0
	default:
		return f.rng.Intn(10000)
	}
}

func (f *Fuzzer) randomTag() string {
	return tags[f.rng.Intn(len(tags))]
}

// TargetSizes returns the benchmark target sizes in bytes
func TargetSizes() map[string]int {
	return map[string]int{
		"32MiB":  32 * 1024 * 1024,
		"64MiB":  64 * 1024 * 1024,
		"128MiB": 128 * 1024 * 1024,
		"256MiB": 256 * 1024 * 1024,
		"512MiB": 512 * 1024 * 1024,
		"1GiB":   1024 * 1024 * 1024,
	}
}

// SizeOrder returns sizes in ascending order for consistent iteration
func SizeOrder() []string {
	return []string{"32MiB", "64MiB", "128MiB", "256MiB", "512MiB", "1GiB"}
}

// ==================== COMPLEX NESTED DATA GENERATION ====================

// GenerateComplexJSON creates deeply nested JSON with multiple levels of arrays of objects
// Structure: organizations[] → departments[] → teams[] → projects[] → tasks[] → subtasks[] → comments[]
// This creates 7+ levels of nesting with arrays of objects at each level
func GenerateComplexJSON(orgCount, deptPerOrg, teamsPerDept, projectsPerTeam, tasksPerProject, subtasksPerTask int) []byte {
	var sb strings.Builder
	sb.Grow(orgCount * deptPerOrg * teamsPerDept * projectsPerTeam * 500)

	sb.WriteString(`{"version":"1.0","timestamp":"2026-01-10T00:00:00Z","organizations":[`)

	idx := 0
	for o := 0; o < orgCount; o++ {
		if o > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`{"orgId":%d,"orgName":"Organization %d","country":"%s","departments":[`,
			o, o, countries[o%len(countries)]))

		for d := 0; d < deptPerOrg; d++ {
			if d > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf(`{"deptId":%d,"deptName":"Department %d","budget":%d,"teams":[`,
				d, d, 100000+(d*10000)))

			for t := 0; t < teamsPerDept; t++ {
				if t > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(fmt.Sprintf(`{"teamId":%d,"teamName":"Team %d","lead":"%s","projects":[`,
					t, t, firstNames[t%len(firstNames)]))

				for p := 0; p < projectsPerTeam; p++ {
					if p > 0 {
						sb.WriteString(",")
					}
					sb.WriteString(fmt.Sprintf(`{"projectId":%d,"projectName":"Project %d","status":"%s","priority":%d,"tasks":[`,
						p, p, []string{"active", "pending", "complete"}[p%3], 1+(p%5)))

					for tk := 0; tk < tasksPerProject; tk++ {
						if tk > 0 {
							sb.WriteString(",")
						}
						sb.WriteString(fmt.Sprintf(`{"taskId":%d,"title":"Task %d","assignee":"%s %s","hours":%d,"subtasks":[`,
							tk, tk, firstNames[tk%len(firstNames)], lastNames[tk%len(lastNames)], 4+(tk%20)))

						for st := 0; st < subtasksPerTask; st++ {
							if st > 0 {
								sb.WriteString(",")
							}
							sb.WriteString(fmt.Sprintf(`{"subtaskId":%d,"description":"Subtask %d item %d","complete":%t,"comments":[`,
								st, idx, st, st%3 == 0))

							// Add 2-4 comments per subtask
							for c := 0; c < 2+(idx%3); c++ {
								if c > 0 {
									sb.WriteString(",")
								}
								sb.WriteString(fmt.Sprintf(`{"commentId":%d,"author":"%s","text":"Comment %d on subtask","timestamp":"2026-01-0%dT1%d:00:00Z","reactions":[{"type":"like","count":%d},{"type":"helpful","count":%d}]}`,
									c, firstNames[c%len(firstNames)], c, 1+(c%9), c%12, idx%50, idx%20))
							}
							sb.WriteString("]}")
							idx++
						}
						sb.WriteString("]}")
					}
					sb.WriteString("]}")
				}
				sb.WriteString("]}")
			}
			sb.WriteString("]}")
		}
		sb.WriteString("]}")
	}

	sb.WriteString("]}")
	return []byte(sb.String())
}

// GenerateOrdersJSON creates complex e-commerce order data with nested line items
// Structure: orders[] → lineItems[] → variants[] → modifiers[]
func GenerateOrdersJSON(orderCount, itemsPerOrder int) []byte {
	var sb strings.Builder
	sb.Grow(orderCount * itemsPerOrder * 300)

	products := []string{"Laptop", "Phone", "Tablet", "Monitor", "Keyboard", "Mouse", "Headphones", "Camera", "Speaker", "Watch"}

	sb.WriteString(`{"store":{"id":"store-1","name":"TechStore","location":{"city":"San Francisco","country":"USA"}},"orders":[`)

	for o := 0; o < orderCount; o++ {
		if o > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf(`{"orderId":"ORD-%d","customer":{"id":%d,"name":"%s %s","email":"user%d@example.com","addresses":[{"type":"billing","street":"%d Main St","city":"%s","zip":"%05d"},{"type":"shipping","street":"%d Oak Ave","city":"%s","zip":"%05d"}]},"status":"%s","lineItems":[`,
			o, o, firstNames[o%len(firstNames)], lastNames[o%len(lastNames)], o,
			100+o, cities[o%len(cities)], 10000+(o%90000),
			200+o, cities[(o+5)%len(cities)], 20000+(o%80000),
			[]string{"pending", "processing", "shipped", "delivered"}[o%4]))

		for i := 0; i < itemsPerOrder; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			productIdx := (o + i) % len(products)
			sb.WriteString(fmt.Sprintf(`{"itemId":%d,"product":{"sku":"SKU-%d","name":"%s","category":{"id":%d,"name":"%s","parent":{"id":%d,"name":"Electronics"}}},"quantity":%d,"price":%.2f,"variants":[`,
				i, i*100+o, products[productIdx], productIdx, products[productIdx]+"s", productIdx/3,
				1+(i%5), float64(99+(productIdx*100))+float64(i)/100.0))

			// 2-3 variants per item
			for v := 0; v < 2+(i%2); v++ {
				if v > 0 {
					sb.WriteString(",")
				}
				sb.WriteString(fmt.Sprintf(`{"variantId":%d,"name":"%s","value":"%s","priceAdjust":%.2f,"modifiers":[`,
					v, []string{"Color", "Size", "Material"}[v%3], []string{"Black", "Large", "Metal", "White", "Small", "Plastic"}[v],
					float64(v*10)+0.99))

				// 1-2 modifiers per variant
				for m := 0; m < 1+(v%2); m++ {
					if m > 0 {
						sb.WriteString(",")
					}
					sb.WriteString(fmt.Sprintf(`{"modId":%d,"type":"%s","applied":%t,"fee":%.2f}`,
						m, []string{"warranty", "gift-wrap", "express", "insurance"}[m%4], m%2 == 0, float64(m*5)+0.99))
				}
				sb.WriteString("]}")
			}
			sb.WriteString("]}")
		}

		sb.WriteString(fmt.Sprintf(`],"totals":{"subtotal":%.2f,"tax":%.2f,"shipping":%.2f,"total":%.2f}}`,
			float64(o*100)+99.99, float64(o*10)+9.99, 9.99, float64(o*120)+119.97))
	}

	sb.WriteString("]}")
	return []byte(sb.String())
}

// GenerateMixedComplexJSON creates a document with multiple types of complex nesting
func GenerateMixedComplexJSON(size int) []byte {
	// Estimate record counts based on size
	// A typical complex record is ~2KB
	recordsPerSize := size / 2000

	// Split between different complex structures
	orgCount := 1 + (recordsPerSize / 1000)
	orderCount := recordsPerSize / 10

	var sb strings.Builder
	sb.Grow(size)

	sb.WriteString(`{"meta":{"generated":"2026-01-10","format":"mixed-complex"},"data":{`)

	// Add organization hierarchy
	orgData := GenerateComplexJSON(orgCount, 3, 2, 2, 3, 2)
	sb.WriteString(`"hierarchy":`)
	sb.Write(orgData)

	sb.WriteString(`,"commerce":`)
	orderData := GenerateOrdersJSON(orderCount, 5)
	sb.Write(orderData)

	sb.WriteString("}}")
	return []byte(sb.String())
}
