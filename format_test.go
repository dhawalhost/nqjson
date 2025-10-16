package nqjson

import (
	"bytes"
	"strings"
	"testing"
)

// Test data for formatting operations
var (
	uglyJSON = []byte(`{"name":"John","age":30,"address":{"street":"123 Main St","city":"New York"},"phones":[{"type":"home","number":"555-1234"},{"type":"work","number":"555-5678"}],"active":true,"scores":[95,87,92]}`)

	prettyJSON = []byte(`{
  "name": "John",
  "age": 30,
  "address": {
    "street": "123 Main St",
    "city": "New York"
  },
  "phones": [
    {
      "type": "home",
      "number": "555-1234"
    },
    {
      "type": "work",
      "number": "555-5678"
    }
  ],
  "active": true,
  "scores": [
    95,
    87,
    92
  ]
}`)

	complexJSON = []byte(`{"users":[{"id":1,"profile":{"name":"Alice","settings":{"theme":"dark","notifications":true}}},{"id":2,"profile":{"name":"Bob","settings":{"theme":"light","notifications":false}}}],"metadata":{"count":2,"generated":"2025-09-03"}}`)

	emptyObjects = []byte(`{"empty":{},"emptyArray":[],"nested":{"inner":{}}}`)

	stringWithEscapes = []byte(`{"message":"Hello \"world\"\nNew line\tTab","unicode":"Unicode: \u0048\u0065\u006C\u006C\u006F"}`)

	numbers = []byte(`{"integer":42,"negative":-123,"decimal":3.14159,"scientific":1.23e10,"negativeScientific":-4.56E-7}`)

	literals = []byte(`{"truth":true,"falsehood":false,"nothing":null}`)
)

//------------------------------------------------------------------------------
// PRETTY FORMATTING TESTS
//------------------------------------------------------------------------------

func TestPretty_BasicFormatting(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:  "Simple Object",
			input: []byte(`{"name":"John","age":30}`),
			expected: `{
  "name": "John",
  "age": 30
}`,
		},
		{
			name:  "Simple Array",
			input: []byte(`[1,2,3]`),
			expected: `[
  1,
  2,
  3
]`,
		},
		{
			name:     "Empty Object",
			input:    []byte(`{}`),
			expected: `{}`,
		},
		{
			name:     "Empty Array",
			input:    []byte(`[]`),
			expected: `[]`,
		},
		{
			name:     "Nested Structure",
			input:    uglyJSON,
			expected: string(prettyJSON),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Pretty(tt.input)
			if err != nil {
				t.Fatalf("Pretty() failed: %v", err)
			}

			if string(result) != tt.expected {
				t.Errorf("Pretty() = %q, want %q", string(result), tt.expected)
			}
		})
	}
}

func TestPretty_CustomIndentation(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		indent string
		want   string
	}{
		{
			name:   "Tab Indentation",
			input:  []byte(`{"a":1,"b":2}`),
			indent: "\t",
			want:   "{\n\t\"a\": 1,\n\t\"b\": 2\n}",
		},
		{
			name:   "Four Space Indentation",
			input:  []byte(`{"a":1,"b":2}`),
			indent: "    ",
			want:   "{\n    \"a\": 1,\n    \"b\": 2\n}",
		},
		{
			name:   "No Indentation (Uglify)",
			input:  []byte(`{"a": 1, "b": 2}`),
			indent: "",
			want:   `{"a":1,"b":2}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &FormatOptions{Indent: tt.indent}
			result, err := PrettyWithOptions(tt.input, opts)
			if err != nil {
				t.Fatalf("PrettyWithOptions() failed: %v", err)
			}

			if string(result) != tt.want {
				t.Errorf("PrettyWithOptions() = %q, want %q", string(result), tt.want)
			}
		})
	}
}

func TestPretty_ComplexStructures(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "Complex Nested JSON",
			input: complexJSON,
		},
		{
			name:  "Empty Objects and Arrays",
			input: emptyObjects,
		},
		{
			name:  "Strings with Escapes",
			input: stringWithEscapes,
		},
		{
			name:  "Various Number Formats",
			input: numbers,
		},
		{
			name:  "Boolean and Null Literals",
			input: literals,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Pretty(tt.input)
			if err != nil {
				t.Fatalf("Pretty() failed: %v", err)
			}

			// Verify the result is valid JSON by uglifying it back
			uglified, err := Ugly(result)
			if err != nil {
				t.Fatalf("Failed to uglify prettified JSON: %v", err)
			}

			// Remove all whitespace from original for comparison
			originalUglified, err := Ugly(tt.input)
			if err != nil {
				t.Fatalf("Failed to uglify original JSON: %v", err)
			}

			if !bytes.Equal(uglified, originalUglified) {
				t.Errorf("Prettify->Uglify cycle changed content:\nOriginal: %s\nResult:   %s", originalUglified, uglified)
			}
		})
	}
}

//------------------------------------------------------------------------------
// UGLY FORMATTING TESTS
//------------------------------------------------------------------------------

func TestUgly_BasicMinification(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "Remove Spaces",
			input:    []byte(`{ "name" : "John" , "age" : 30 }`),
			expected: []byte(`{"name":"John","age":30}`),
		},
		{
			name:     "Remove Newlines and Tabs",
			input:    []byte("{\n\t\"name\": \"John\",\n\t\"age\": 30\n}"),
			expected: []byte(`{"name":"John","age":30}`),
		},
		{
			name:     "Preserve String Content",
			input:    []byte(`{"message": "Hello world\nWith newlines\tand tabs"}`),
			expected: []byte(`{"message":"Hello world\nWith newlines\tand tabs"}`),
		},
		{
			name:     "Array Minification",
			input:    []byte(`[ 1 , 2 , 3 , 4 ]`),
			expected: []byte(`[1,2,3,4]`),
		},
		{
			name:     "Complex Nested Structure",
			input:    prettyJSON,
			expected: uglyJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Ugly(tt.input)
			if err != nil {
				t.Fatalf("Ugly() failed: %v", err)
			}

			if !bytes.Equal(result, tt.expected) {
				t.Errorf("Ugly() = %q, want %q", string(result), string(tt.expected))
			}
		})
	}
}

func TestUgly_PreservesStringEscapes(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "Escaped Quotes",
			input: []byte(`{ "message" : "He said \"Hello\"" }`),
		},
		{
			name:  "Escaped Backslashes",
			input: []byte(`{ "path" : "C:\\Users\\Documents" }`),
		},
		{
			name:  "Unicode Escapes",
			input: []byte(`{ "unicode" : "\\u0048\\u0065\\u006C\\u006C\\u006F" }`),
		},
		{
			name:  "Mixed Escapes",
			input: stringWithEscapes,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Ugly(tt.input)
			if err != nil {
				t.Fatalf("Ugly() failed: %v", err)
			}

			// Verify the result is valid JSON by parsing it back
			prettified, err := Pretty(result)
			if err != nil {
				t.Fatalf("Failed to prettify uglified JSON: %v", err)
			}

			// The cycle should preserve semantic content
			roundTrip, err := Ugly(prettified)
			if err != nil {
				t.Fatalf("Failed to uglify round-trip JSON: %v", err)
			}

			if !bytes.Equal(result, roundTrip) {
				t.Errorf("Ugly->Pretty->Ugly cycle changed content:\nFirst:  %s\nSecond: %s", result, roundTrip)
			}
		})
	}
}

//------------------------------------------------------------------------------
// VALIDATION TESTS
//------------------------------------------------------------------------------

func TestValid_CorrectJSON(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  bool
	}{
		{
			name:  "Simple Object",
			input: []byte(`{"name":"John"}`),
			want:  true,
		},
		{
			name:  "Simple Array",
			input: []byte(`[1,2,3]`),
			want:  true,
		},
		{
			name:  "String Value",
			input: []byte(`"hello"`),
			want:  true,
		},
		{
			name:  "Number Value",
			input: []byte(`42`),
			want:  true,
		},
		{
			name:  "Boolean True",
			input: []byte(`true`),
			want:  true,
		},
		{
			name:  "Boolean False",
			input: []byte(`false`),
			want:  true,
		},
		{
			name:  "Null Value",
			input: []byte(`null`),
			want:  true,
		},
		{
			name:  "Complex Nested",
			input: complexJSON,
			want:  true,
		},
		{
			name:  "Empty Object",
			input: []byte(`{}`),
			want:  true,
		},
		{
			name:  "Empty Array",
			input: []byte(`[]`),
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Valid(tt.input)
			if result != tt.want {
				t.Errorf("Valid() = %v, want %v for input: %s", result, tt.want, string(tt.input))
			}
		})
	}
}

func TestValid_InvalidJSON(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  bool
	}{
		{
			name:  "Empty Input",
			input: []byte(``),
			want:  false,
		},
		{
			name:  "Unclosed Object",
			input: []byte(`{"name":"John"`),
			want:  false,
		},
		{
			name:  "Unclosed Array",
			input: []byte(`[1,2,3`),
			want:  false,
		},
		{
			name:  "Unterminated String",
			input: []byte(`{"name":"John`),
			want:  false,
		},
		{
			name:  "Invalid Number",
			input: []byte(`{"age":3.}`),
			want:  false,
		},
		{
			name:  "Missing Colon",
			input: []byte(`{"name""John"}`),
			want:  false,
		},
		{
			name:  "Trailing Comma Object",
			input: []byte(`{"name":"John",}`),
			want:  false,
		},
		{
			name:  "Trailing Comma Array",
			input: []byte(`[1,2,3,]`),
			want:  false,
		},
		{
			name:  "Invalid Literal",
			input: []byte(`{"value":truee}`),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Valid(tt.input)
			if result != tt.want {
				t.Errorf("Valid() = %v, want %v for input: %s", result, tt.want, string(tt.input))
			}
		})
	}
}

//------------------------------------------------------------------------------
// PERFORMANCE TESTS
//------------------------------------------------------------------------------

func TestFormatting_Performance(t *testing.T) {
	// Generate large JSON for performance testing
	largeJSON := generateLargeJSON(1000)

	t.Run("Pretty Performance", func(t *testing.T) {
		result, err := Pretty(largeJSON)
		if err != nil {
			t.Fatalf("Pretty() failed: %v", err)
		}

		// Verify it's still valid
		if !Valid(result) {
			t.Error("Prettified JSON is not valid")
		}
	})

	t.Run("Ugly Performance", func(t *testing.T) {
		result, err := Ugly(largeJSON)
		if err != nil {
			t.Fatalf("Ugly() failed: %v", err)
		}

		// Verify it's still valid
		if !Valid(result) {
			t.Error("Uglified JSON is not valid")
		}
	})

	t.Run("Validation Performance", func(t *testing.T) {
		if !Valid(largeJSON) {
			t.Error("Large JSON should be valid")
		}
	})
}

func TestFormatting_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "Deeply Nested Object",
			input: generateDeeplyNested(20),
		},
		{
			name:  "Large Array",
			input: []byte(`[` + strings.Repeat(`"item",`, 999) + `"item"]`),
		},
		{
			name:  "Many Keys Object",
			input: generateManyKeysObject(100),
		},
		{
			name:  "Long String Values",
			input: []byte(`{"long":"` + strings.Repeat("a", 10000) + `"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Pretty
			pretty, err := Pretty(tt.input)
			if err != nil {
				t.Fatalf("Pretty() failed: %v", err)
			}

			// Test Ugly
			ugly, err := Ugly(pretty)
			if err != nil {
				t.Fatalf("Ugly() failed: %v", err)
			}

			// Test round-trip consistency
			originalUgly, err := Ugly(tt.input)
			if err != nil {
				t.Fatalf("Failed to uglify original: %v", err)
			}

			if !bytes.Equal(ugly, originalUgly) {
				t.Error("Round-trip formatting changed content")
			}
		})
	}
}

//------------------------------------------------------------------------------
// BENCHMARK TESTS
//------------------------------------------------------------------------------

func BenchmarkPretty_Small(b *testing.B) {
	data := []byte(`{"name":"John","age":30}`)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Pretty(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPretty_Medium(b *testing.B) {
	data := uglyJSON
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Pretty(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPretty_Large(b *testing.B) {
	data := generateLargeJSON(1000)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Pretty(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUgly_Small(b *testing.B) {
	data := []byte(`{ "name" : "John" , "age" : 30 }`)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Ugly(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUgly_Medium(b *testing.B) {
	data := prettyJSON
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Ugly(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUgly_Large(b *testing.B) {
	pretty, _ := Pretty(generateLargeJSON(1000))
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := Ugly(pretty)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValid_Small(b *testing.B) {
	data := []byte(`{"name":"John","age":30}`)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		Valid(data)
	}
}

func BenchmarkValid_Large(b *testing.B) {
	data := generateLargeJSON(1000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		Valid(data)
	}
}

//------------------------------------------------------------------------------
// HELPER FUNCTIONS FOR TESTS
//------------------------------------------------------------------------------

func generateLargeJSON(itemCount int) []byte {
	var buf bytes.Buffer
	buf.WriteString(`{"items":[`)

	for i := 0; i < itemCount; i++ {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(`{"id":`)
		buf.WriteString(string(rune(i + '0')))
		buf.WriteString(`,"name":"Item `)
		buf.WriteString(string(rune(i + '0')))
		buf.WriteString(`","active":`)
		if i%2 == 0 {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		buf.WriteString(`,"metadata":{"priority":`)
		buf.WriteString(string(rune((i % 5) + '0')))
		buf.WriteString(`}}`)
	}

	buf.WriteString(`],"count":`)
	buf.WriteString(string(rune(itemCount + '0')))
	buf.WriteByte('}')
	return buf.Bytes()
}

func generateDeeplyNested(depth int) []byte {
	var buf bytes.Buffer

	// Build opening braces
	for i := 0; i < depth; i++ {
		buf.WriteString(`{"level`)
		buf.WriteString(string(rune(i + '0')))
		buf.WriteString(`":`)
	}

	buf.WriteString(`"value"`)

	// Build closing braces
	for i := 0; i < depth; i++ {
		buf.WriteByte('}')
	}

	return buf.Bytes()
}

func generateManyKeysObject(keyCount int) []byte {
	var buf bytes.Buffer
	buf.WriteByte('{')

	for i := 0; i < keyCount; i++ {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(`"key`)
		buf.WriteString(string(rune(i + '0')))
		buf.WriteString(`":"value`)
		buf.WriteString(string(rune(i + '0')))
		buf.WriteByte('"')
	}

	buf.WriteByte('}')
	return buf.Bytes()
}
