package benchmark

import (
	"encoding/json"
	"testing"

	"github.com/itchyny/gojq"
	gjson "github.com/tidwall/gjson"

	"github.com/dhawalhost/nqjson"
)

// Extra large datasets for stress testing
var (
	extraLargeJSON200k []byte
	extraLargeParsed   any
)

func init() {
	// Generate 200,000 users (~88MB)
	extraLargeJSON200k = GenerateLargeJSON(200000)
}

func BenchmarkLarge200k_SimpleField_NQJSON(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(extraLargeJSON200k)))

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(extraLargeJSON200k, "users.100000.name")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkLarge200k_SimpleField_GJSON(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(extraLargeJSON200k)))

	var res gjson.Result
	for i := 0; i < b.N; i++ {
		res = gjson.GetBytes(extraLargeJSON200k, "users.100000.name")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkLarge200k_SimpleField_GojqCompiled(b *testing.B) {
	if extraLargeParsed == nil {
		if err := json.Unmarshal(extraLargeJSON200k, &extraLargeParsed); err != nil {
			b.Fatalf("Failed to parse: %v", err)
		}
	}
	b.ReportAllocs()

	parsed, _ := gojq.Parse(".users[100000].name")
	code, _ := gojq.Compile(parsed)

	for i := 0; i < b.N; i++ {
		iter := code.Run(extraLargeParsed)
		gojqResultSink, _ = iter.Next()
	}
}

func BenchmarkLarge200k_ArrayLast_NQJSON(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(extraLargeJSON200k)))

	var res nqjson.Result
	for i := 0; i < b.N; i++ {
		res = nqjson.Get(extraLargeJSON200k, "users.199999.id")
	}
	nqjsonResultSink = res.String()
}

func BenchmarkLarge200k_ArrayLast_GJSON(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(extraLargeJSON200k)))

	var res gjson.Result
	for i := 0; i < b.N; i++ {
		res = gjson.GetBytes(extraLargeJSON200k, "users.199999.id")
	}
	nqjsonResultSink = res.String()
}
