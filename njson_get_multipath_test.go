package njson

import (
	"testing"
)

func TestGetMultiPath(t *testing.T) {
	data := []byte(`{"user":{"name":"Alice","age":30},"meta":{"active":true,"score":2.5}}`)
	path := "user.name,meta.active,meta.score,missing"

	res := Get(data, path)
	if !res.Exists() || res.Type != TypeArray {
		t.Fatalf("expected array result for multipath, got %#v", res)
	}

	values := res.Array()
	if len(values) != 4 {
		t.Fatalf("expected 4 results, got %d", len(values))
	}

	if got := values[0].String(); got != "Alice" {
		t.Fatalf("expected first value Alice, got %s", got)
	}
	if !values[1].Bool() {
		t.Fatalf("expected second value true, got %#v", values[1])
	}
	if got := values[2].Float(); got != 2.5 {
		t.Fatalf("expected third value 2.5, got %f", got)
	}
	if !values[3].IsNull() {
		t.Fatalf("expected null for missing path, got %#v", values[3])
	}

	t.Logf("Multipath query successful: returned %d results", len(values))
}

func TestExtendedModifiers(t *testing.T) {
	data := []byte(`{"nums":[1,4,2,3],"nested":[[1,2],[3],[]],"dups":["a","b","a"],"words":["b","c","a"],"mixedNums":["1","2","2"]}`)

	// Test flatten modifier
	flat := Get(data, "nested|@flatten")
	if !flat.Exists() || flat.Type != TypeArray {
		t.Fatalf("flatten modifier failed, got %#v", flat)
	}
	flatVals := flat.Array()
	if len(flatVals) != 3 || flatVals[0].Int() != 1 || flatVals[1].Int() != 2 || flatVals[2].Int() != 3 {
		t.Fatalf("flatten results unexpected: %v", flatVals)
	}

	// Test distinct + sort modifiers
	distinct := Get(data, "dups|@distinct|@sort")
	if !distinct.Exists() || distinct.Type != TypeArray {
		t.Fatalf("distinct modifier failed, got %#v", distinct)
	}
	dVals := distinct.Array()
	if len(dVals) != 2 || dVals[0].String() != "a" || dVals[1].String() != "b" {
		t.Fatalf("expected distinct sorted values [a b], got %v", dVals)
	}

	// Test first/last modifiers
	first := Get(data, "nums|@first")
	if !first.Exists() || first.Int() != 1 {
		t.Fatalf("first modifier expected 1, got %#v", first)
	}

	last := Get(data, "nums|@last")
	if !last.Exists() || last.Int() != 3 {
		t.Fatalf("last modifier expected 3, got %#v", last)
	}

	// Test aggregate modifiers
	sum := Get(data, "nums|@sum")
	if !sum.Exists() || sum.Float() != 10 {
		t.Fatalf("sum modifier expected 10, got %#v", sum)
	}

	avg := Get(data, "nums|@avg")
	if !avg.Exists() || avg.Float() != 2.5 {
		t.Fatalf("avg modifier expected 2.5, got %#v", avg)
	}

	min := Get(data, "nums|@min")
	if !min.Exists() || min.Int() != 1 {
		t.Fatalf("min modifier expected 1, got %#v", min)
	}

	max := Get(data, "nums|@max")
	if !max.Exists() || max.Int() != 4 {
		t.Fatalf("max modifier expected 4, got %#v", max)
	}

	// Test sort with argument
	sortedDesc := Get(data, "nums|@sort:desc")
	if !sortedDesc.Exists() || sortedDesc.Type != TypeArray {
		t.Fatalf("sort modifier (desc) failed, got %#v", sortedDesc)
	}
	sdVals := sortedDesc.Array()
	if len(sdVals) != 4 || sdVals[0].Int() != 4 || sdVals[3].Int() != 1 {
		t.Fatalf("sort desc produced unexpected values: %v", sdVals)
	}

	// Test string sorting
	wordSort := Get(data, "words|@sort:desc")
	if !wordSort.Exists() || wordSort.Type != TypeArray {
		t.Fatalf("string sort modifier failed, got %#v", wordSort)
	}
	wsVals := wordSort.Array()
	if len(wsVals) != 3 || wsVals[0].String() != "c" || wsVals[2].String() != "a" {
		t.Fatalf("string sort expected [c b a], got %v", wsVals)
	}

	// Test sum with string numbers
	mixedSum := Get(data, "mixedNums|@sum")
	if !mixedSum.Exists() || mixedSum.Float() != 5 {
		t.Fatalf("mixed numeric sum expected 5, got %#v", mixedSum)
	}
}
