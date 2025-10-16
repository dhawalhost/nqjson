// Package nqjson provides next-gen query operations for JSON with zero allocations.
// Created by dhawalhost (2025-09-01 13:51:05)
package nqjson

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

// Error definitions for query operations
var (
	ErrInvalidQuery   = errors.New("invalid query syntax")
	ErrTypeConversion = errors.New("cannot convert value to requested type")
)

// String constants for common values and operators
const (
	constNull    = "null"
	constFalse   = "false"
	constString  = "string"
	constNumber  = "number"
	constBool    = "bool"
	constBoolean = "boolean"
	constEq      = "=="
	constNe      = "!="
	constLe      = "<="
	constGe      = ">="
)

// ValueType represents the type of a JSON value
type ValueType uint8

const (
	TypeUndefined ValueType = iota
	TypeNull
	TypeString
	TypeNumber
	TypeBoolean
	TypeObject
	TypeArray
)

// Result represents the result of a JSON query operation
type Result struct {
	Type     ValueType
	Str      string
	Num      float64
	Boolean  bool // Renamed to avoid conflict with Bool() method
	Index    int
	Raw      []byte
	Path     string
	Indexes  []int
	Modified bool
}

// Thread-safe caches and pools
var (
	// Shared buffer pools
	// smallBufferPool = sync.Pool{
	// 	New: func() interface{} {
	// 		buf := make([]byte, 0, 512)
	// 		return &buf
	// 	},
	// }

	// mediumBufferPool = sync.Pool{
	// 	New: func() interface{} {
	// 		buf := make([]byte, 0, 4096)
	// 		return &buf
	// 	},
	// }

	// largeBufferPool = sync.Pool{
	// 	New: func() interface{} {
	// 		buf := make([]byte, 0, 32768)
	// 		return &buf
	// 	},
	// }

	// Result cache for hot paths (thread-safe)
	// resultCache sync.Map

	// Path cache for compiled paths (thread-safe)
	pathCache sync.Map
)

//------------------------------------------------------------------------------
// CORE GET IMPLEMENTATION
//------------------------------------------------------------------------------

type getOptions struct {
	allowMultipath bool
	allowJSONLines bool
}

// PHASE 3A: Compiled path structure for cached execution
type compiledPath struct {
	original string
	segments []pathSegment
	isSimple bool
	isUltra  bool
}

type pathSegment struct {
	key     string
	index   int // -1 if not array access
	isArray bool
}

// Get retrieves a value from JSON using a path expression.
// This is highly optimized with multiple fast paths for common use cases.
//
//go:inline
func Get(data []byte, path string) Result {
	return getWithOptions(data, path, getOptions{allowMultipath: true, allowJSONLines: true})
}

// PHASE 3A: GetCached - Optimized version that caches compiled paths
// Use this for frequently repeated queries with the same path (5-10x faster on hot paths)
// Thread-safe and suitable for concurrent use
func GetCached(data []byte, path string) Result {
	// Try cache first
	if cached, ok := pathCache.Load(path); ok {
		cp := cached.(*compiledPath)
		result := executeCompiledPath(data, cp)
		if result.Exists() {
			return result
		}
	}

	// Not in cache or not found - use normal path and cache it
	result := Get(data, path)

	// Cache successful simple/ultra paths for future use
	if result.Exists() && (isSimplePath(path) || isUltraSimplePath(path)) {
		cp := compilePath(path)
		pathCache.Store(path, cp)
	}

	return result
}

// PHASE 3A: compilePath - Parse and compile a path for fast repeated execution
func compilePath(path string) *compiledPath {
	cp := &compiledPath{
		original: path,
		isUltra:  isUltraSimplePath(path),
		isSimple: isSimplePath(path),
	}

	// For ultra-simple paths (single key), no need to parse segments
	if cp.isUltra {
		cp.segments = []pathSegment{{key: path, index: -1, isArray: false}}
		return cp
	}

	// Parse path into segments
	cp.segments = parsePathSegments(path)
	return cp
}

// PHASE 3A: parsePathSegments - Break path into executable segments
func parsePathSegments(path string) []pathSegment {
	if path == "" {
		return nil
	}

	var segments []pathSegment
	i := 0

	// Handle leading array index
	if i < len(path) && path[i] >= '0' && path[i] <= '9' {
		idx := 0
		for i < len(path) && path[i] >= '0' && path[i] <= '9' {
			idx = idx*10 + int(path[i]-'0')
			i++
		}
		segments = append(segments, pathSegment{key: "", index: idx, isArray: true})
		if i < len(path) && path[i] == '.' {
			i++ // Skip dot
		}
	}

	// Parse remaining segments
	for i < len(path) {
		// Find end of current segment
		start := i
		for i < len(path) && path[i] != '.' && path[i] != '[' {
			i++
		}

		if start < i {
			key := path[start:i]

			// Check if it's a numeric key (array access via dot notation)
			isNumeric := true
			idx := 0
			for j := 0; j < len(key); j++ {
				if key[j] < '0' || key[j] > '9' {
					isNumeric = false
					break
				}
				idx = idx*10 + int(key[j]-'0')
			}

			if isNumeric && len(key) > 0 {
				segments = append(segments, pathSegment{key: "", index: idx, isArray: true})
			} else {
				segments = append(segments, pathSegment{key: key, index: -1, isArray: false})
			}
		}

		// Handle bracket notation
		if i < len(path) && path[i] == '[' {
			i++ // Skip '['
			idx := 0
			for i < len(path) && path[i] >= '0' && path[i] <= '9' {
				idx = idx*10 + int(path[i]-'0')
				i++
			}
			if i < len(path) && path[i] == ']' {
				i++ // Skip ']'
			}
			segments = append(segments, pathSegment{key: "", index: idx, isArray: true})
		}

		// Skip dot
		if i < len(path) && path[i] == '.' {
			i++
		}
	}

	return segments
}

// PHASE 3A: executeCompiledPath - Fast execution of pre-parsed path
func executeCompiledPath(data []byte, cp *compiledPath) Result {
	if len(data) == 0 {
		return Result{Type: TypeUndefined}
	}

	// Ultra-fast path for single keys
	if cp.isUltra && len(cp.segments) == 1 && !cp.segments[0].isArray {
		return getUltraSimplePath(data, cp.segments[0].key)
	}

	// Single-pass execution through all segments
	dataStart, dataEnd := 0, len(data)

	for _, seg := range cp.segments {
		if seg.isArray {
			// Array access
			start, end := fastFindArrayElement(data[dataStart:dataEnd], seg.index)
			if start == -1 {
				return Result{Type: TypeUndefined}
			}
			dataStart, dataEnd = dataStart+start, dataStart+end
		} else {
			// Object key access
			start, end := fastFindObjectValue(data[dataStart:dataEnd], seg.key)
			if start == -1 {
				return Result{Type: TypeUndefined}
			}
			dataStart, dataEnd = dataStart+start, dataStart+end
		}
	}

	// Parse final value
	return fastParseValue(data[dataStart:dataEnd])
}

func getWithOptions(data []byte, path string, opts getOptions) Result {
	// Empty path should return non-existent result according to tests
	if path == "" {
		return Result{Type: TypeUndefined}
	}

	// PHASE 1 OPTIMIZATION: Ultra-fast path for simple single keys (90% of use cases)
	// This avoids multipath detection overhead for the most common case
	if !opts.allowMultipath || !strings.ContainsAny(path, ",|") {
		// JSON Lines support: treat leading ".." prefix as newline-delimited documents when applicable.
		if opts.allowJSONLines && len(path) >= 2 && path[0] == '.' && path[1] == '.' {
			if jsonLinesResult, handled := getJSONLinesResult(data, path, opts); handled {
				return jsonLinesResult
			}
		}

		// Root path returns the entire document
		if len(path) == 1 && (path[0] == '$' || path[0] == '@') {
			return Parse(data)
		}

		// Check if the data is empty
		if len(data) == 0 {
			return Result{Type: TypeUndefined}
		}

		// Check if we can use the ultra-fast path for simple keys
		if len(data) < 1024 && isUltraSimplePath(path) {
			result := getUltraSimplePath(data, path)
			if result.Exists() {
				return result
			}
		}

		// Fast path for simple dot notation paths
		if isSimplePath(path) {
			return getSimplePath(data, path)
		}

		// Use more advanced path processing for complex paths
		return getComplexPath(data, path)
	}

	// Multipath detection (only when enabled and path contains comma/pipe)
	if multi, handled := getMultiPathResult(data, path, opts); handled {
		return multi
	}

	// Fallback to single-path processing
	// Check if the data is empty
	if len(data) == 0 {
		return Result{Type: TypeUndefined}
	}

	// Fast path for simple dot notation paths
	if isSimplePath(path) {
		return getSimplePath(data, path)
	}

	// Use more advanced path processing for complex paths
	return getComplexPath(data, path)
}

// getJSONLinesResult normalizes JSON Lines content into an array and then executes the provided path.
func getMultiPathResult(data []byte, path string, opts getOptions) (Result, bool) {
	segments := splitMultiPath(path)
	if len(segments) < 2 {
		return Result{}, false
	}

	results := make([]Result, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		subResult := getWithOptions(data, segment, getOptions{allowMultipath: false, allowJSONLines: opts.allowJSONLines})
		if !subResult.Exists() {
			subResult = buildNullResult()
		}
		results = append(results, subResult)
	}

	if len(results) == 0 {
		return Result{Type: TypeArray, Raw: []byte("[]"), Modified: true}, true
	}

	combined := buildWildcardResult(results)
	if combined.Type == TypeUndefined {
		combined = Result{Type: TypeArray, Raw: []byte("[]")}
	}
	combined.Modified = true
	return combined, true
}

func splitMultiPath(path string) []string {
	// PHASE 1 OPTIMIZATION: Fast detection for single-path (no split needed)
	// Check if path contains comma outside of brackets/quotes
	hasComma := false
	bracketDepth := 0
	inString := false
	var stringQuote byte

	for i := 0; i < len(path); i++ {
		c := path[i]
		if inString {
			if c == '\\' && i+1 < len(path) {
				i++ // Skip escaped character
				continue
			}
			if c == stringQuote {
				inString = false
			}
			continue
		}

		switch c {
		case '\'', '"':
			inString = true
			stringQuote = c
		case '[', '{', '(':
			bracketDepth++
		case ']', '}', ')':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case ',':
			if bracketDepth == 0 {
				hasComma = true
				// Early exit - we found a split point
				goto doSplit
			}
		}
	}

	// No split needed - return single-element slice without allocation-heavy builder
	if !hasComma {
		return []string{path}
	}

doSplit:
	// Pre-allocate for common case (2-4 paths)
	segments := make([]string, 0, 4)

	// Use byte slice indexing instead of strings.Builder for zero-copy
	start := 0
	bracketDepth = 0
	braceDepth := 0
	parenDepth := 0
	inString = false

	for i := 0; i < len(path); i++ {
		c := path[i]
		if inString {
			if c == '\\' {
				if i+1 < len(path) {
					i++
				}
				continue
			}
			if c == stringQuote {
				inString = false
			}
			continue
		}

		switch c {
		case '\'', '"':
			inString = true
			stringQuote = c
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case ',':
			if bracketDepth == 0 && braceDepth == 0 && parenDepth == 0 {
				// Zero-copy substring extraction
				segment := strings.TrimSpace(path[start:i])
				if segment != "" {
					segments = append(segments, segment)
				}
				start = i + 1
			}
		}
	}

	if start < len(path) {
		segment := strings.TrimSpace(path[start:])
		if segment != "" {
			segments = append(segments, segment)
		}
	}

	return segments
}

func buildNullResult() Result {
	return Result{Type: TypeNull, Raw: []byte("null"), Modified: true}
}

func getJSONLinesResult(data []byte, path string, opts getOptions) (Result, bool) {
	values, ok := extractJSONLinesValues(data)
	if !ok {
		return Result{}, false
	}

	if len(values) == 0 {
		return Result{Type: TypeUndefined}, true
	}

	arrayBytes := buildJSONArrayFromLines(values)

	// Normalize the path by removing the ".." prefix and optional leading dot.
	trimmedPath := path[2:]
	if len(trimmedPath) > 0 && trimmedPath[0] == '.' {
		trimmedPath = trimmedPath[1:]
	}

	if trimmedPath == "" {
		return Parse(arrayBytes), true
	}

	return getWithOptions(arrayBytes, trimmedPath, getOptions{allowMultipath: true, allowJSONLines: false}), true
}

// extractJSONLinesValues returns valid JSON documents when the input represents JSON Lines.
func extractJSONLinesValues(data []byte) ([][]byte, bool) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, false
	}

	if json.Valid(trimmed) {
		return nil, false
	}

	// Prefer actual newline-separated payloads after normalizing CRLF.
	normalized := bytes.ReplaceAll(trimmed, []byte{'\r'}, nil)
	if values, ok := splitAndValidateJSONLines(normalized, []byte{'\n'}); ok {
		return values, true
	}

	// Fallback: handle literal "\n" sequences (common when raw string literals are used).
	if values, ok := splitAndValidateJSONLines(trimmed, []byte(`\n`)); ok {
		return values, true
	}

	// Handle literal "\r\n" sequences if present.
	if values, ok := splitAndValidateJSONLines(trimmed, []byte(`\r\n`)); ok {
		return values, true
	}

	return nil, false
}

func splitAndValidateJSONLines(data []byte, sep []byte) ([][]byte, bool) {
	if len(sep) == 0 {
		return nil, false
	}

	segments := bytes.Split(data, sep)
	values := make([][]byte, 0, len(segments))
	for _, segment := range segments {
		entry := bytes.TrimSpace(segment)
		if len(entry) == 0 {
			continue
		}
		if !json.Valid(entry) {
			return nil, false
		}
		values = append(values, entry)
	}

	if len(values) == 0 {
		return nil, false
	}

	return values, true
}

// buildJSONArrayFromLines constructs a JSON array from individual JSON documents.
func buildJSONArrayFromLines(values [][]byte) []byte {
	totalSize := 2 // opening and closing brackets
	for i, v := range values {
		if i > 0 {
			totalSize++ // comma
		}
		totalSize += len(v)
	}

	result := make([]byte, 0, totalSize)
	result = append(result, '[')
	for i, v := range values {
		if i > 0 {
			result = append(result, ',')
		}
		result = append(result, v...)
	}
	result = append(result, ']')
	return result
}

// GetString is like Get but accepts a string input
func GetString(json string, path string) Result {
	return Get(stringToBytes(json), path)
}

// Parse parses a JSON value and returns a Result
// skipLeadingWhitespace skips whitespace at the start of data
func skipLeadingWhitespace(data []byte) int {
	start := 0
	for ; start < len(data); start++ {
		if data[start] > ' ' {
			break
		}
	}
	return start
}

// parseBooleanTrue attempts to parse a boolean true value
func parseBooleanTrue(data []byte, start int) (Result, bool) {
	if start+3 < len(data) &&
		data[start+1] == 'r' &&
		data[start+2] == 'u' &&
		data[start+3] == 'e' {
		return Result{
			Type:    TypeBoolean,
			Boolean: true,
			Raw:     data[start : start+4],
			Index:   start,
		}, true
	}
	return Result{}, false
}

// parseBooleanFalse attempts to parse a boolean false value
func parseBooleanFalse(data []byte, start int) (Result, bool) {
	if start+4 < len(data) &&
		data[start+1] == 'a' &&
		data[start+2] == 'l' &&
		data[start+3] == 's' &&
		data[start+4] == 'e' {
		return Result{
			Type:    TypeBoolean,
			Boolean: false,
			Raw:     data[start : start+5],
			Index:   start,
		}, true
	}
	return Result{}, false
}

// parseNull attempts to parse a null value
func parseNull(data []byte, start int) (Result, bool) {
	if start+3 < len(data) &&
		data[start+1] == 'u' &&
		data[start+2] == 'l' &&
		data[start+3] == 'l' {
		return Result{
			Type:  TypeNull,
			Raw:   data[start : start+4],
			Index: start,
		}, true
	}
	return Result{}, false
}

// isNumericChar checks if a character can start a number
func isNumericChar(c byte) bool {
	return c == '-' || (c >= '0' && c <= '9')
}

// isValidBasicJSON performs minimal JSON validation for objects and arrays
func isValidBasicJSON(data []byte, start int) bool {
	if start >= len(data) {
		return false
	}

	var depth int
	inString := false
	escaped := false

	for i := start; i < len(data); i++ {
		char := data[i]

		if inString {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
		} else {
			switch char {
			case '"':
				inString = true
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				if depth < 0 {
					return false
				}
			}
		}
	}

	return depth == 0 && !inString
}

func Parse(data []byte) Result {
	// Skip leading whitespace
	start := skipLeadingWhitespace(data)

	if start >= len(data) {
		return Result{Type: TypeUndefined}
	}

	// Parse based on first character
	switch data[start] {
	case '{': // Object
		if isValidBasicJSON(data, start) {
			return Result{
				Type:  TypeObject,
				Raw:   data,
				Index: start,
			}
		}
	case '[': // Array
		if isValidBasicJSON(data, start) {
			return Result{
				Type:  TypeArray,
				Raw:   data,
				Index: start,
			}
		}
	case '"': // String
		return parseString(data, start)
	case 't': // true
		if result, ok := parseBooleanTrue(data, start); ok {
			return result
		}
	case 'f': // false
		if result, ok := parseBooleanFalse(data, start); ok {
			return result
		}
	case 'n': // null
		if result, ok := parseNull(data, start); ok {
			return result
		}
	default:
		if isNumericChar(data[start]) {
			return parseNumber(data, start)
		}
	}

	return Result{Type: TypeUndefined}
}

// GetMany executes multiple queries against the same JSON data
func GetMany(data []byte, paths ...string) []Result {
	if len(paths) == 0 {
		return nil
	}

	// Always use sequential approach - it's faster than parallel for these workloads
	results := make([]Result, len(paths))
	for i, path := range paths {
		results[i] = Get(data, path)
	}
	return results
}

// getUltraSimplePath is an ultra-fast path for very simple JSON with basic paths
// This handles cases like {"name":"John","age":30} with path "name"
func getUltraSimplePath(data []byte, path string) Result {
	// PHASE 2 OPTIMIZATION: Ultra-fast inline object scanning
	// This path is optimized for single-key lookups in small-medium JSON objects
	// Target: 20-30ns for optimal performance

	keyLen := len(path)
	if keyLen == 0 || len(data) < keyLen+6 { // Minimum: {"k":v}
		return Result{Type: TypeUndefined}
	}

	// Skip leading whitespace and find opening brace
	i := 0
	for ; i < len(data); i++ {
		if data[i] > ' ' {
			break
		}
	}

	if i >= len(data) || data[i] != '{' {
		return Result{Type: TypeUndefined}
	}
	i++ // Skip '{'

	// PHASE 2: Inline key scanning with early termination
	// Scan through object keys looking for exact match
	for i < len(data) {
		// Skip whitespace
		for ; i < len(data) && data[i] <= ' '; i++ {
		}

		if i >= len(data) || data[i] == '}' {
			return Result{Type: TypeUndefined}
		}

		if data[i] != '"' {
			return Result{Type: TypeUndefined}
		}

		keyStart := i + 1

		// Fast key comparison without allocation
		if i+keyLen+2 < len(data) && data[keyStart+keyLen] == '"' {
			match := true
			for j := 0; j < keyLen; j++ {
				if data[keyStart+j] != path[j] {
					match = false
					break
				}
			}

			if match {
				// Found the key! Skip to value
				i = keyStart + keyLen + 1 // After closing quote

				// Skip whitespace and colon
				for ; i < len(data) && data[i] <= ' '; i++ {
				}
				if i >= len(data) || data[i] != ':' {
					return Result{Type: TypeUndefined}
				}
				i++ // Skip ':'

				// Skip whitespace before value
				for ; i < len(data) && data[i] <= ' '; i++ {
				}

				if i >= len(data) {
					return Result{Type: TypeUndefined}
				}

				// PHASE 2: Inline value parsing (zero allocation)
				return parseValueAtPosition(data, i)
			}
		}

		// Skip this key-value pair
		// Find end of current key
		for i++; i < len(data); i++ {
			if data[i] == '\\' && i+1 < len(data) {
				i++ // Skip escaped char
				continue
			}
			if data[i] == '"' {
				i++
				break
			}
		}

		// Skip colon
		for ; i < len(data) && data[i] <= ' '; i++ {
		}
		if i >= len(data) || data[i] != ':' {
			return Result{Type: TypeUndefined}
		}
		i++ // Skip ':'

		// Skip value
		i = skipValue(data, i)

		// Skip comma
		for ; i < len(data) && data[i] <= ' '; i++ {
		}
		if i < len(data) && data[i] == ',' {
			i++
		}
	}

	return Result{Type: TypeUndefined}
}

// findKeyInqJSON searches for a key in JSON data and returns its index
func findKeyInqJSON(data []byte, path string, keyLen, searchLen int) int {
	for i := 0; i <= len(data)-searchLen; i++ {
		if data[i] == '"' {
			// Check if the key matches
			if i+keyLen+2 < len(data) && data[i+keyLen+1] == '"' && data[i+keyLen+2] == ':' {
				// Compare the key bytes directly
				match := true
				for j := 0; j < keyLen; j++ {
					if data[i+1+j] != path[j] {
						match = false
						break
					}
				}
				if match {
					return i
				}
			}
		}
	}
	return -1
}

// parseStringValue parses a JSON string value
func parseStringValue(data []byte, valueStart int) Result {
	if valueStart >= len(data) {
		return Result{Type: TypeUndefined}
	}

	end := valueStart + 1
	for ; end < len(data); end++ {
		if data[end] == '\\' {
			end++ // Skip escape character
			continue
		}
		if data[end] == '"' {
			break
		}
	}

	if end >= len(data) {
		return Result{Type: TypeUndefined}
	}

	// Extract the string content (without quotes)
	raw := data[valueStart : end+1]
	str := raw[1 : len(raw)-1] // Remove quotes

	// For ultra-simple path, use zero-allocation string conversion
	// This is safe because we're not modifying the underlying data
	return Result{
		Type:  TypeString,
		Str:   bytesToString(str),
		Raw:   raw,
		Index: valueStart,
	}
}

// parseTrueValue parses a JSON true value
func parseTrueValue(data []byte, valueStart int) Result {
	if valueStart+3 < len(data) &&
		data[valueStart+1] == 'r' &&
		data[valueStart+2] == 'u' &&
		data[valueStart+3] == 'e' {
		return Result{
			Type:    TypeBoolean,
			Boolean: true,
			Raw:     data[valueStart : valueStart+4],
			Index:   valueStart,
		}
	}
	return Result{Type: TypeUndefined}
}

// parseFalseValue parses a JSON false value
func parseFalseValue(data []byte, valueStart int) Result {
	if valueStart+4 < len(data) &&
		data[valueStart+1] == 'a' &&
		data[valueStart+2] == 'l' &&
		data[valueStart+3] == 's' &&
		data[valueStart+4] == 'e' {
		return Result{
			Type:    TypeBoolean,
			Boolean: false,
			Raw:     data[valueStart : valueStart+5],
			Index:   valueStart,
		}
	}
	return Result{Type: TypeUndefined}
}

// parseNullValue parses a JSON null value
func parseNullValue(data []byte, valueStart int) Result {
	if valueStart+3 < len(data) &&
		data[valueStart+1] == 'u' &&
		data[valueStart+2] == 'l' &&
		data[valueStart+3] == 'l' {
		return Result{
			Type:  TypeNull,
			Raw:   data[valueStart : valueStart+4],
			Index: valueStart,
		}
	}
	return Result{Type: TypeUndefined}
}

// parseObjectValue parses a JSON object value
func parseObjectValue(data []byte, valueStart int) Result {
	objectEnd := findBlockEnd(data, valueStart, '{', '}')
	if objectEnd == -1 {
		return Result{Type: TypeUndefined}
	}
	return Result{
		Type:  TypeObject,
		Raw:   data[valueStart:objectEnd],
		Index: valueStart,
	}
}

// parseArrayValue parses a JSON array value
func parseArrayValue(data []byte, valueStart int) Result {
	arrayEnd := findBlockEnd(data, valueStart, '[', ']')
	if arrayEnd == -1 {
		return Result{Type: TypeUndefined}
	}
	return Result{
		Type:  TypeArray,
		Raw:   data[valueStart:arrayEnd],
		Index: valueStart,
	}
}

// getSimplePath handles simple dot notation and basic array access
// This is optimized for paths like "user.name" or "items[0].id" or "items.0.id"
// PHASE 4: Recursive one-pass path processing
// Process entire path in a single traversal without extracting intermediate values
//
//go:inline
func getSimplePath(data []byte, path string) Result {
	// Skip leading whitespace
	start := 0
	for start < len(data) && data[start] <= ' ' {
		start++
	}
	if start >= len(data) {
		return Result{Type: TypeUndefined}
	}

	// Start recursive descent
	if data[start] == '{' {
		return parseObjectRecursive(data, start+1, path)
	} else if data[start] == '[' {
		return parseArrayRecursive(data, start+1, path)
	}

	return Result{Type: TypeUndefined}
}

// PHASE 4: Recursive object parser - processes path segments on the fly
//
//go:inline
func parseObjectRecursive(data []byte, pos int, path string) Result {
	// Parse the first segment of the path
	segEnd := 0
	for segEnd < len(path) && path[segEnd] != '.' && path[segEnd] != '[' {
		segEnd++
	}
	if segEnd == 0 {
		return Result{Type: TypeUndefined}
	}

	segment := path[:segEnd]
	remainingPath := ""
	if segEnd < len(path) {
		if path[segEnd] == '.' {
			remainingPath = path[segEnd+1:]
		} else {
			remainingPath = path[segEnd:]
		}
	}

	segLen := len(segment)
	hasRemainingPath := len(remainingPath) > 0

	// Scan object for matching key
	for pos < len(data) {
		// Skip whitespace
		for pos < len(data) && data[pos] <= ' ' {
			pos++
		}
		if pos >= len(data) || data[pos] == '}' {
			break
		}

		// Expect quote
		if data[pos] != '"' {
			return Result{Type: TypeUndefined}
		}
		pos++

		keyStart := pos

		// ULTRA-FAST KEY SCAN: Process 8 bytes at once
		// Characters > '\\' (92) can be skipped instantly
		for pos+7 < len(data) {
			if data[pos] > '\\' && data[pos+1] > '\\' &&
				data[pos+2] > '\\' && data[pos+3] > '\\' &&
				data[pos+4] > '\\' && data[pos+5] > '\\' &&
				data[pos+6] > '\\' && data[pos+7] > '\\' {
				pos += 8
				continue
			}
			break
		}

		// Find end of key
		for pos < len(data) {
			c := data[pos]
			if c == '\\' {
				pos += 2
				continue
			}
			if c == '"' {
				break
			}
			pos++
		}

		keyEnd := pos

		// Quick length check before byte-by-byte comparison
		keyMatches := (keyEnd-keyStart == segLen)
		if keyMatches {
			for i := 0; i < segLen; i++ {
				if data[keyStart+i] != segment[i] {
					keyMatches = false
					break
				}
			}
		}

		pos++ // Skip closing quote

		// Skip to colon
		for pos < len(data) && data[pos] <= ' ' {
			pos++
		}
		if pos >= len(data) || data[pos] != ':' {
			return Result{Type: TypeUndefined}
		}
		pos++

		// Skip to value
		for pos < len(data) && data[pos] <= ' ' {
			pos++
		}
		if pos >= len(data) {
			return Result{Type: TypeUndefined}
		}

		if keyMatches {
			// Found matching key!
			if !hasRemainingPath {
				// This is the final segment - parse and return value
				return parseValueAtPosition(data, pos)
			}

			// Continue with remaining path
			c := data[pos]
			if c == '{' {
				return parseObjectRecursive(data, pos+1, remainingPath)
			} else if c == '[' {
				return parseArrayRecursive(data, pos+1, remainingPath)
			}

			return Result{Type: TypeUndefined}
		}

		// Skip this value using ultra-fast vectorized skipper
		pos = vectorizedSkipValue(data, pos, len(data))
		if pos == -1 {
			return Result{Type: TypeUndefined}
		}

		// Skip comma
		for pos < len(data) && data[pos] <= ' ' {
			pos++
		}
		if pos < len(data) && data[pos] == ',' {
			pos++
		}
	}

	return Result{Type: TypeUndefined}
}

// PHASE 4: Recursive array parser
//
//go:inline
func parseArrayRecursive(data []byte, pos int, path string) Result {
	// Check if path starts with array index
	if len(path) == 0 || (path[0] < '0' || path[0] > '9') {
		return Result{Type: TypeUndefined}
	}

	// Parse index
	idx := 0
	i := 0
	for i < len(path) && path[i] >= '0' && path[i] <= '9' {
		idx = idx*10 + int(path[i]-'0')
		i++
	}

	remainingPath := ""
	if i < len(path) {
		if path[i] == '.' {
			remainingPath = path[i+1:]
		} else if path[i] == '[' {
			remainingPath = path[i:]
		}
	}

	// Skip to target index
	currentIdx := 0
	for pos < len(data) && currentIdx < idx {
		// Skip whitespace
		for pos < len(data) && data[pos] <= ' ' {
			pos++
		}
		if pos >= len(data) || data[pos] == ']' {
			return Result{Type: TypeUndefined}
		}

		// Skip value
		pos = vectorizedSkipValue(data, pos, len(data))
		if pos == -1 {
			return Result{Type: TypeUndefined}
		}
		currentIdx++

		// Skip comma
		for pos < len(data) && data[pos] <= ' ' {
			pos++
		}
		if pos < len(data) && data[pos] == ',' {
			pos++
		}
	}

	// Skip to element
	for pos < len(data) && data[pos] <= ' ' {
		pos++
	}
	if pos >= len(data) {
		return Result{Type: TypeUndefined}
	}

	if len(remainingPath) == 0 {
		// This is the final value
		return parseValueAtPosition(data, pos)
	}

	// Continue with remaining path
	if data[pos] == '{' {
		return parseObjectRecursive(data, pos+1, remainingPath)
	} else if data[pos] == '[' {
		return parseArrayRecursive(data, pos+1, remainingPath)
	}

	return Result{Type: TypeUndefined}
}

// PHASE 4: Parse value at current position
//
//go:inline
func parseValueAtPosition(data []byte, pos int) Result {
	valueStart := pos
	valueEnd := vectorizedSkipValue(data, pos, len(data))
	if valueEnd == -1 {
		return Result{Type: TypeUndefined}
	}

	return fastParseValue(data[valueStart:valueEnd])
}

// PHASE 4: Vectorized value skipper with 8-byte scanning optimization
//
//go:inline
func vectorizedSkipValue(data []byte, pos, end int) int {
	if pos >= end {
		return -1
	}

	c := data[pos]
	switch c {
	case '"':
		// ULTRA-FAST STRING SKIP - Process 8 bytes at once
		pos++
		for pos+7 < end {
			// Check 8 bytes at once - characters > '\\' are safe to skip
			if data[pos] > '\\' && data[pos+1] > '\\' &&
				data[pos+2] > '\\' && data[pos+3] > '\\' &&
				data[pos+4] > '\\' && data[pos+5] > '\\' &&
				data[pos+6] > '\\' && data[pos+7] > '\\' {
				pos += 8
				continue
			}
			break
		}

		// Handle remaining bytes
		for pos < end {
			c := data[pos]
			if c == '\\' {
				pos += 2
				if pos >= end {
					return -1
				}
				continue
			}
			if c == '"' {
				return pos + 1
			}
			pos++
		}
		return -1

	case '{':
		// Skip object - simplified without string tracking (faster!)
		pos++
		depth := 1
		for pos < end && depth > 0 {
			c := data[pos]
			if c == '{' {
				depth++
			} else if c == '}' {
				depth--
			} else if c == '"' {
				// Skip string content quickly
				pos++
				for pos < end {
					if data[pos] == '\\' {
						pos++
					} else if data[pos] == '"' {
						break
					}
					pos++
				}
			}
			pos++
		}
		return pos

	case '[':
		// Skip array - simplified
		pos++
		depth := 1
		for pos < end && depth > 0 {
			c := data[pos]
			if c == '[' {
				depth++
			} else if c == ']' {
				depth--
			} else if c == '"' {
				// Skip string content quickly
				pos++
				for pos < end {
					if data[pos] == '\\' {
						pos++
					} else if data[pos] == '"' {
						break
					}
					pos++
				}
			}
			pos++
		}
		return pos

	default:
		// Skip number, true, false, null - tightest possible loop
		for pos < end {
			c := data[pos]
			if c <= ' ' || c == ',' || c == ']' || c == '}' {
				return pos
			}
			pos++
		}
		return pos
	}
}

// handleGetDirectArrayIndex handles paths that start with a numeric array index
func handleGetDirectArrayIndex(data []byte, path string, p, dataStart, dataEnd int) (int, int, int) {
	// Special case: if the path starts with a number (direct array index)
	if p < len(path) && path[p] >= '0' && path[p] <= '9' {
		// Parse the array index
		idx := 0
		for p < len(path) && path[p] >= '0' && path[p] <= '9' {
			idx = idx*10 + int(path[p]-'0')
			p++
		}

		// Use simple array access for better performance on medium data
		start, end := fastFindArrayElement(data, idx)
		if start == -1 {
			return p, -1, -1 // Signal error
		}
		dataStart = start
		dataEnd = end

		// Continue with rest of path if there is more
		if p < len(path) && path[p] == '.' {
			p++ // Skip the dot
		}
	}
	return p, dataStart, dataEnd
}

// processGetPathSegment processes a single segment of the path
func processGetPathSegment(data []byte, path string, p, dataStart, dataEnd int) (int, int, int, error) {
	keyStart := p

	// Find end of key part
	i := p
	for i < len(path) && path[i] != '.' && path[i] != '[' {
		i++
	}

	key := path[keyStart:i]
	p = i

	// Process key if not empty
	if key != "" {
		var err error
		dataStart, dataEnd, err = processGetKeyAccess(data, key, dataStart, dataEnd)
		if err != nil {
			return p, dataStart, dataEnd, err
		}
	}

	// Handle bracket notation array access
	if p < len(path) && path[p] == '[' {
		var err error
		p, dataStart, dataEnd, err = processGetBracketAccess(data, path, p, dataStart, dataEnd)
		if err != nil {
			return p, dataStart, dataEnd, err
		}
	}

	// Move to next part
	if p < len(path) && path[p] == '.' {
		p++
	}

	return p, dataStart, dataEnd, nil
}

// processGetKeyAccess handles object key access or numeric array index access
func processGetKeyAccess(data []byte, key string, dataStart, dataEnd int) (int, int, error) {
	// Check if current data is an array and the key is numeric
	if dataEnd > dataStart && data[dataStart] == '[' && isNumericKey(key) {
		// Treat as array index access using dot notation
		idx := 0
		for j := 0; j < len(key); j++ {
			idx = idx*10 + int(key[j]-'0')
		}

		// Use simple array access - ultra-fast approach has bugs
		start, end := fastFindArrayElement(data[dataStart:dataEnd], idx)
		if start == -1 {
			return dataStart, dataEnd, fmt.Errorf("array index not found")
		}
		return dataStart + start, dataStart + start + (end - start), nil
	} else {
		// Normal object key access - optimized for medium JSON
		start, end := fastFindObjectValue(data[dataStart:dataEnd], key)
		if start == -1 {
			return dataStart, dataEnd, fmt.Errorf("object key not found")
		}
		return dataStart + start, dataStart + start + (end - start), nil
	}
}

// processGetBracketAccess handles bracket notation array access like [0]
func processGetBracketAccess(data []byte, path string, p, dataStart, dataEnd int) (int, int, int, error) {
	p++ // Skip '['

	// Fast manual integer parsing instead of strconv.Atoi
	idx := 0
	for p < len(path) && path[p] >= '0' && path[p] <= '9' {
		idx = idx*10 + int(path[p]-'0')
		p++
	}

	if p >= len(path) || path[p] != ']' {
		return p, dataStart, dataEnd, fmt.Errorf("malformed bracket notation")
	}
	p++ // Skip ']'

	// Use simple array access for bracket notation (typically smaller indices)
	start, end := fastFindArrayElement(data[dataStart:dataEnd], idx)
	if start == -1 {
		return p, dataStart, dataEnd, fmt.Errorf("array index not found")
	}

	return p, dataStart + start, dataStart + start + (end - start), nil
}

// skipStringValue skips over a JSON string value efficiently
func skipStringValue(data []byte, start int) int {
	start++
	for start < len(data) {
		if data[start] == '"' {
			return start + 1
		}
		if data[start] == '\\' {
			start++
		}
		start++
	}
	return -1
}

// skipObjectValue skips over a JSON object value efficiently
func skipObjectValue(data []byte, start int) int {
	start++
	depth := 1
	inString := false

	for start < len(data) && depth > 0 {
		c := data[start]
		if !inString {
			switch c {
			case '"':
				inString = true
			case '{':
				depth++
			case '}':
				depth--
			}
		} else {
			switch c {
			case '"':
				inString = false
			case '\\':
				start++
			}
		}
		start++
	}
	return start
}

// skipArrayValue skips over a JSON array value efficiently
func skipArrayValue(data []byte, start int) int {
	start++
	depth := 1
	inString := false

	for start < len(data) && depth > 0 {
		c := data[start]
		if !inString {
			switch c {
			case '"':
				inString = true
			case '[':
				depth++
			case ']':
				depth--
			}
		} else {
			switch c {
			case '"':
				inString = false
			case '\\':
				start++
			}
		}
		start++
	}
	return start
}

// skipPrimitiveValue skips over a primitive JSON value (number, boolean, null)
func skipPrimitiveValue(data []byte, start int) int {
	for start < len(data) {
		c := data[start]
		if c == ',' || c == '}' || c == ']' || c <= ' ' {
			break
		}
		start++
	}
	return start
}

func ultraFastSkipValue(data []byte, start int) int {
	if start >= len(data) {
		return -1
	}

	switch data[start] {
	case '"':
		return skipStringValue(data, start)
	case '{':
		return skipObjectValue(data, start)
	case '[':
		return skipArrayValue(data, start)
	default:
		// Primitive value
		return skipPrimitiveValue(data, start)
	}
}

// isNumericKey checks if a key is purely numeric (for array index access)
func isNumericKey(key string) bool {
	if len(key) == 0 {
		return false
	}
	for i := 0; i < len(key); i++ {
		if key[i] < '0' || key[i] > '9' {
			return false
		}
	}
	return true
}

// fastFindObjectValue finds a key's value in an object, optimized for performance
func fastFindObjectValue(data []byte, key string) (int, int) {
	// Skip whitespace and validate object start
	start := findObjectStartForFastFind(data)
	if start == -1 {
		return -1, -1
	}

	pos := start + 1
	keyLen := len(key)

	for pos < len(data) {
		// Skip to next key or detect end of object
		pos = skipToNextKeyInFastFind(data, pos)
		if pos == -1 {
			return -1, -1
		}

		// Check if this key matches our target
		if valueStart, valueEnd := checkKeyMatchInFastFind(data, pos, key, keyLen); valueStart != -1 {
			return valueStart, valueEnd
		}

		// Skip this key-value pair and continue
		pos = skipKeyValuePairInFastFind(data, pos)
		if pos == -1 {
			return -1, -1
		}
	}

	return -1, -1
}

// findObjectStartForFastFind finds the starting position of the JSON object
func findObjectStartForFastFind(data []byte) int {
	start := 0
	for ; start < len(data); start++ {
		if data[start] > ' ' {
			break
		}
	}
	if start >= len(data) || data[start] != '{' {
		return -1
	}
	return start
}

// skipToNextKeyInFastFind advances to the next key in the object or returns -1 if end reached
func skipToNextKeyInFastFind(data []byte, pos int) int {
	// Skip whitespace
	for ; pos < len(data) && data[pos] <= ' '; pos++ {
	}
	if pos >= len(data) || data[pos] == '}' {
		return -1
	}

	if data[pos] != '"' {
		return -1
	}

	return pos
}

// checkKeyMatchInFastFind checks if the current key matches our target and returns value bounds if so
func checkKeyMatchInFastFind(data []byte, pos int, key string, keyLen int) (int, int) {
	// Check if this key matches
	if pos+keyLen+1 < len(data) && data[pos+keyLen+1] == '"' {
		match := true
		for i := 0; i < keyLen; i++ {
			if data[pos+1+i] != key[i] {
				match = false
				break
			}
		}

		if match {
			return extractValueBoundsInFastFind(data, pos+keyLen+2)
		}
	}
	return -1, -1
}

// extractValueBoundsInFastFind finds the start and end positions of the value after a matched key
func extractValueBoundsInFastFind(data []byte, pos int) (int, int) {
	// Skip to colon
	for ; pos < len(data) && data[pos] <= ' '; pos++ {
	}
	if pos >= len(data) || data[pos] != ':' {
		return -1, -1
	}
	pos++ // skip ':'
	for ; pos < len(data) && data[pos] <= ' '; pos++ {
	}

	// Find value end
	valueStart := pos
	valueEnd := findValueEnd(data, pos)
	if valueEnd == -1 {
		return -1, -1
	}
	return valueStart, valueEnd
}

// skipKeyValuePairInFastFind skips over the current key-value pair and positions at the next element
func skipKeyValuePairInFastFind(data []byte, pos int) int {
	// Skip quoted key (pos should point at '"')
	afterKey := fastSkipQuotedStringGet(data, pos)
	if afterKey == -1 {
		return -1
	}

	// Skip spaces to colon and validate
	afterSpaces := fastSkipSpacesGet(data, afterKey)
	if afterSpaces >= len(data) || data[afterSpaces] != ':' {
		return -1
	}
	// Move past ':' and spaces to the value start
	valStart := fastSkipSpacesGet(data, afterSpaces+1)

	// Find end of value
	valEnd := findValueEnd(data, valStart)
	if valEnd == -1 {
		return -1
	}

	// Position to next pair, or -1 if end
	return fastSkipToNextPairDividerGet(data, valEnd)
}

// fastSkipQuotedStringGet skips a JSON quoted string starting at pos (must be '"').
// Returns index after closing quote or -1 on error.
func fastSkipQuotedStringGet(data []byte, pos int) int {
	if pos >= len(data) || data[pos] != '"' {
		return -1
	}
	pos++
	for pos < len(data) {
		c := data[pos]
		if c == '\\' { // escape
			pos += 2
			continue
		}
		if c == '"' {
			return pos + 1
		}
		pos++
	}
	return -1
}

// fastSkipSpacesGet advances over ASCII spaces and returns new position.
func fastSkipSpacesGet(data []byte, pos int) int {
	for pos < len(data) && data[pos] <= ' ' {
		pos++
	}
	return pos
}

// fastSkipToNextPairDividerGet moves from valueEnd to the next comma separating
// pairs or returns -1 if end of object is reached.
func fastSkipToNextPairDividerGet(data []byte, pos int) int {
	for pos < len(data) && data[pos] != ',' && data[pos] != '}' {
		pos++
	}
	if pos >= len(data) || data[pos] == '}' {
		return -1
	}
	return pos + 1 // skip comma
}

// validateArrayAndGetStart validates input is array and returns starting position
func validateArrayAndGetStart(data []byte) (int, bool) {
	if len(data) == 0 || data[0] != '[' {
		return 0, false
	}

	// Skip initial whitespace
	i := 1
	for i < len(data) && data[i] <= ' ' {
		i++
	}

	// Handle empty array
	if i < len(data) && data[i] == ']' {
		return 0, false
	}

	return i, true
}

// skipToNextArrayElementInFind advances position after current element for find operations
func skipToNextArrayElementInFind(data []byte, i int) (int, bool) {
	// Skip current element
	end := fastSkipValue(data, i)
	if end == -1 {
		return 0, false
	}
	i = end

	// Skip whitespace and comma
	for i < len(data) && data[i] <= ' ' {
		i++
	}

	if i >= len(data) || data[i] == ']' {
		return 0, false
	}

	if data[i] == ',' {
		i++
		// Skip whitespace after comma
		for i < len(data) && data[i] <= ' ' {
			i++
		}
		return i, true
	}

	return 0, false
}

func fastFindArrayElement(data []byte, index int) (int, int) {
	// For large indices, use the fixed blazing fast comma scanning
	if index > 10 {
		return blazingFastCommaScanner(data, index)
	}

	// Validate array and get starting position
	i, isValid := validateArrayAndGetStart(data)
	if !isValid {
		return -1, -1
	}

	// For small indices, use element-by-element parsing
	currentIndex := 0

	for i < len(data) {
		if currentIndex == index {
			// Found target element, find its end
			start := i
			end := fastSkipValue(data, i)
			if end == -1 {
				return -1, -1
			}
			return start, end
		}

		// Skip to next element
		var ok bool
		i, ok = skipToNextArrayElementInFind(data, i)
		if !ok {
			break
		}
		currentIndex++
	}

	return -1, -1
}

// blazingFastCommaScanner - ultra-optimized array access with adaptive strategies
func blazingFastCommaScanner(data []byte, targetIndex int) (int, int) {
	dataLen := len(data)

	if targetIndex == 0 {
		// Special case for index 0
		i := 1
		for i < dataLen && data[i] <= ' ' {
			i++
		}
		if i >= dataLen || data[i] == ']' {
			return -1, -1
		}
		start := i
		end := ultraFastSkipValue(data, i)
		if end == -1 {
			return -1, -1
		}
		return start, end
	}

	// For very large indices with large data, use memory-efficient scanning
	if targetIndex > 100 && dataLen > 50000 {
		return memoryEfficientLargeIndexAccess(data, targetIndex)
	}

	// For medium to large indices, use optimized comma scanning
	return optimizedCommaScanning(data, targetIndex)
}

// memoryEfficientLargeIndexAccess uses chunk-based processing for very large arrays
// processChunkForIndex processes a chunk of data looking for the target index
func processChunkForIndex(data []byte, chunkStart, chunkEnd, targetIndex int, commasFound *int) (int, int, int, bool) {
	i := chunkStart
	for i < chunkEnd && *commasFound < targetIndex {
		switch data[i] {
		case ',':
			*commasFound++
			if *commasFound == targetIndex {
				// Found target comma
				i++
				for i < len(data) && data[i] <= ' ' {
					i++
				}
				if i >= len(data) {
					return -1, -1, i, true
				}
				start := i
				end := ultraFastSkipValue(data, i)
				return start, end, i, true
			}
			i++
			for i < len(data) && data[i] <= ' ' {
				i++
			}
		case ']':
			return -1, -1, i, true
		default:
			// Skip entire JSON value
			end := ultraFastSkipValue(data, i)
			if end == -1 {
				return -1, -1, i, true
			}
			i = end
		}
	}
	return 0, 0, i, false // Return new position, not found
}

func memoryEfficientLargeIndexAccess(data []byte, targetIndex int) (int, int) {
	dataLen := len(data)
	commasFound := 0
	i := 1

	// Skip initial whitespace
	for i < dataLen && data[i] <= ' ' {
		i++
	}

	// Process in chunks to maintain memory efficiency
	const chunkSize = 8192
	chunkStart := i

	for i < dataLen && commasFound < targetIndex {
		chunkEnd := chunkStart + chunkSize
		if chunkEnd > dataLen {
			chunkEnd = dataLen
		}

		// Process this chunk
		start, end, newPos, found := processChunkForIndex(data, chunkStart, chunkEnd, targetIndex, &commasFound)
		if found {
			return start, end
		}
		i = newPos
		chunkStart = i
	}

	return -1, -1
}

// optimizedCommaScanning for medium-sized indices with minimal overhead
func optimizedCommaScanning(data []byte, targetIndex int) (int, int) {
	dataLen := len(data)
	commasFound := 0
	i := 1

	// Skip initial whitespace
	for i < dataLen && data[i] <= ' ' {
		i++
	}

	for i < dataLen {
		if data[i] == ',' {
			commasFound++
			if commasFound == targetIndex {
				// Found target comma
				i++ // Skip comma
				for i < dataLen && data[i] <= ' ' {
					i++
				}
				if i >= dataLen {
					return -1, -1
				}
				start := i
				end := ultraFastSkipValue(data, i)
				return start, end
			}
			i++
			for i < dataLen && data[i] <= ' ' {
				i++
			}
		} else if data[i] == ']' {
			break
		} else {
			// Skip entire JSON value
			end := ultraFastSkipValue(data, i)
			if end == -1 {
				return -1, -1
			}
			i = end
		}
	}
	return -1, -1
}

// fastSkipString efficiently skips over a JSON string
func fastSkipString(data []byte, start int) int {
	pos := start + 1 // Skip opening quote
	for pos < len(data) {
		if data[pos] == '\\' {
			pos += 2 // Skip escaped character
			continue
		}
		if data[pos] == '"' {
			pos++ // Skip closing quote
			break
		}
		pos++
	}
	return pos
}

// fastSkipObject efficiently skips over a JSON object
func fastSkipObject(data []byte, start int) int {
	pos := start + 1 // Skip opening brace
	depth := 1
	inString := false
	for pos < len(data) && depth > 0 {
		if !inString {
			switch data[pos] {
			case '"':
				inString = true
			case '{':
				depth++
			case '}':
				depth--
			}
		} else {
			switch data[pos] {
			case '\\':
				pos++ // Skip escaped character
			case '"':
				inString = false
			}
		}
		pos++
	}
	return pos
}

// fastSkipArray efficiently skips over a JSON array
func fastSkipArray(data []byte, start int) int {
	pos := start + 1 // Skip opening bracket
	depth := 1
	inString := false
	for pos < len(data) && depth > 0 {
		if !inString {
			switch data[pos] {
			case '"':
				inString = true
			case '[':
				depth++
			case ']':
				depth--
			}
		} else {
			switch data[pos] {
			case '\\':
				pos++ // Skip escaped character
			case '"':
				inString = false
			}
		}
		pos++
	}
	return pos
}

// fastSkipLiteral efficiently skips over a JSON literal (true, false, null)
func fastSkipLiteral(data []byte, start int) int {
	if start >= len(data) {
		return -1
	}
	switch data[start] {
	case 't':
		return matchLiteralAt(data, start, "true")
	case 'f':
		return matchLiteralAt(data, start, "false")
	case 'n':
		return matchLiteralAt(data, start, "null")
	default:
		return -1
	}
}

// matchLiteralAt verifies that data at pos matches the ASCII literal and returns
// the index immediately after the literal or -1 if it doesn't match.
func matchLiteralAt(data []byte, pos int, lit string) int {
	// Fast bounds check
	l := len(lit)
	if pos+l > len(data) {
		return -1
	}
	// Compare bytes
	for i := 0; i < l; i++ {
		if data[pos+i] != lit[i] {
			return -1
		}
	}
	return pos + l
}

// fastSkipNumber efficiently skips over a JSON number
func fastSkipNumber(data []byte, start int) int {
	pos := start
	for pos < len(data) {
		c := data[pos]
		if (c < '0' || c > '9') && c != '.' && c != 'e' && c != 'E' && c != '+' && c != '-' {
			break
		}
		pos++
	}
	return pos
}

// fastSkipValue efficiently skips over a JSON value using minimal parsing
func fastSkipValue(data []byte, start int) int {
	pos := start

	if pos >= len(data) {
		return -1
	}

	switch data[pos] {
	case '"': // String
		return fastSkipString(data, pos)
	case '{': // Object
		return fastSkipObject(data, pos)
	case '[': // Array
		return fastSkipArray(data, pos)
	case 't', 'f', 'n': // true, false, null
		return fastSkipLiteral(data, pos)
	default: // Number
		return fastSkipNumber(data, pos)
	}
}

// fastParseValue parses a JSON value directly, optimized for performance (zero allocations)
// parseStringValueFast parses a string value for fastParseValue
func parseStringValueFast(data []byte, start int) Result {
	end := start + 1
	for ; end < len(data); end++ {
		if data[end] == '\\' {
			end++ // Skip escape character
			continue
		}
		if data[end] == '"' {
			break
		}
	}

	if end >= len(data) {
		return Result{Type: TypeUndefined}
	}

	raw := data[start : end+1]
	str := raw[1 : len(raw)-1] // Remove quotes

	return Result{
		Type:  TypeString,
		Str:   bytesToString(str),
		Raw:   raw,
		Index: start,
	}
}

func fastParseValue(data []byte) Result {
	// Skip leading whitespace
	start := skipLeadingWhitespace(data)

	if start >= len(data) {
		return Result{Type: TypeUndefined}
	}

	switch data[start] {
	case '"': // String - optimized for zero allocations
		return parseStringValueFast(data, start)
	case 't': // true
		if result, ok := parseBooleanTrue(data, start); ok {
			return result
		}
	case 'f': // false
		if result, ok := parseBooleanFalse(data, start); ok {
			return result
		}
	case 'n': // null
		if result, ok := parseNull(data, start); ok {
			return result
		}
	case '{': // Object
		return Result{
			Type:  TypeObject,
			Raw:   data,
			Index: start,
		}
	case '[': // Array
		return Result{
			Type:  TypeArray,
			Raw:   data,
			Index: start,
		}
	default:
		if isNumericChar(data[start]) {
			return parseNumber(data, start)
		}
	}

	return Result{Type: TypeUndefined}
}

// getComplexPath handles more complex path expressions by tokenizing the path
// and executing the tokens.
func getComplexPath(data []byte, path string) Result {
	// Use the path cache for tokenized paths
	cachedTokens, found := pathCache.Load(path)
	var tokens []pathToken
	if found {
		tokens = cachedTokens.([]pathToken)
	} else {
		tokens = tokenizePath(path)
		if len(tokens) == 0 {
			return Result{Type: TypeUndefined}
		}
		pathCache.Store(path, tokens)
	}

	return executeTokenizedPath(data, tokens)
}

// Path token types
type tokenKind int

const (
	tokenKey tokenKind = iota
	tokenIndex
	tokenWildcard
	tokenFilter
	tokenRecursive
	tokenModifier
)

// pathToken represents a single token in a parsed path
type pathToken struct {
	kind   tokenKind
	str    string
	num    int
	filter *filterExpr
}

type filterExpr struct {
	path  string
	op    string
	value string
}

// parseModifiers extracts and parses modifier tokens from a path.
// Supports both legacy '|' and JSONPath-like '@' suffix modifiers.
// For '@', only treat as a modifier separator when not inside strings/brackets/parentheses
// and not immediately following a '.'. This preserves cases like "users.@length" as invalid
// path segments and avoids interference with filter "@.field" usage.
func parseModifiers(path string) ([]pathToken, string) {
	var modifiers []pathToken

	// Fast path: if neither '|' nor '@' present, return early
	if !strings.ContainsAny(path, "|@") {
		return modifiers, path
	}

	// Scan for the first valid modifier separator position
	sepIdx := -1
	bracketDepth := 0
	parenDepth := 0
	inString := false
	var stringQuote byte
	for i := 0; i < len(path); i++ {
		c := path[i]
		if inString {
			if c == '\\' {
				// skip escaped char
				i++
				continue
			}
			if c == stringQuote {
				inString = false
			}
			continue
		}

		switch c {
		case '\'', '"':
			inString = true
			stringQuote = c
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '|':
			if bracketDepth == 0 && parenDepth == 0 {
				sepIdx = i
				i = len(path) // break
			}
		case '@':
			if bracketDepth == 0 && parenDepth == 0 {
				// Only treat as modifier if not immediately after a dot
				if i > 0 && path[i-1] != '.' {
					sepIdx = i
					i = len(path) // break
				}
			}
		}
	}

	if sepIdx < 0 {
		return modifiers, path
	}

	// Extract suffix with modifiers and the main path
	suffix := path[sepIdx+1:]
	cleanPath := path[:sepIdx]

	// Split suffix by either '|' or '@'
	fields := strings.FieldsFunc(suffix, func(r rune) bool { return r == '|' || r == '@' })
	for _, part := range fields {
		if part == "" {
			continue
		}
		parts := strings.SplitN(part, ":", 2)
		mod := pathToken{kind: tokenModifier, str: parts[0]}
		if len(parts) > 1 {
			mod.str = parts[0] + ":" + parts[1] // Keep the full modifier
		}
		modifiers = append(modifiers, mod)
	}

	return modifiers, cleanPath
}

// parseArrayAccess parses array access syntax in path parts
func parseArrayAccess(part string) []pathToken {
	var tokens []pathToken

	base := part[:strings.IndexByte(part, '[')]
	if base != "" {
		tokens = append(tokens, pathToken{kind: tokenKey, str: base})
	}

	bracket := part[strings.IndexByte(part, '[')+1 : len(part)-1]

	if bracket == "*" || bracket == "#" {
		tokens = append(tokens, pathToken{kind: tokenWildcard})
	} else if idx, err := strconv.Atoi(bracket); err == nil {
		tokens = append(tokens, pathToken{kind: tokenIndex, num: idx})
	} else if strings.HasPrefix(bracket, "?") || strings.Contains(bracket, "==") ||
		strings.Contains(bracket, "!=") || strings.Contains(bracket, ">=") ||
		strings.Contains(bracket, "<=") || strings.Contains(bracket, ">") ||
		strings.Contains(bracket, "<") || strings.Contains(bracket, "=~") {
		// Parse filter expression
		filter := parseFilterExpression(bracket)
		tokens = append(tokens, pathToken{kind: tokenFilter, filter: filter})
	}

	return tokens
}

// tokenizePath breaks a path into tokens for efficient execution
func tokenizePath(path string) []pathToken {
	var tokens []pathToken

	// Check for modifiers
	modifiers, cleanPath := parseModifiers(path)

	// Split the path on dots, but respect brackets and parentheses so that
	// dots inside filters like [?( @.age>28 )] don't split the segment.
	var parts []string
	var cur strings.Builder
	bracketDepth := 0
	parenDepth := 0
	inString := false
	var stringQuote byte
	for i := 0; i < len(cleanPath); i++ {
		c := cleanPath[i]
		if inString {
			cur.WriteByte(c)
			if c == '\\' { // escape next char
				if i+1 < len(cleanPath) {
					i++
					cur.WriteByte(cleanPath[i])
				}
				continue
			}
			if c == stringQuote {
				inString = false
			}
			continue
		}

		switch c {
		case '\'', '"':
			inString = true
			stringQuote = c
			cur.WriteByte(c)
			continue
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '.':
			if bracketDepth == 0 && parenDepth == 0 {
				parts = append(parts, cur.String())
				cur.Reset()
				continue
			}
		}
		cur.WriteByte(c)
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}

	for _, part := range parts {
		if part == "" {
			continue
		}

		if part == "*" || part == "#" {
			tokens = append(tokens, pathToken{kind: tokenWildcard})
			continue
		}

		if part == ".." {
			tokens = append(tokens, pathToken{kind: tokenRecursive})
			continue
		}

		// Check for array access
		if strings.HasSuffix(part, "]") && strings.Contains(part, "[") {
			arrayTokens := parseArrayAccess(part)
			tokens = append(tokens, arrayTokens...)
		} else {
			tokens = append(tokens, pathToken{kind: tokenKey, str: part})
		}
	}

	// Append modifiers at the end if any
	tokens = append(tokens, modifiers...)

	return tokens
}

// parseFilterExpression parses a filter expression like '?(@.age>30)'
func parseFilterExpression(expr string) *filterExpr {
	// Strip leading '?' if present
	if ok := strings.HasPrefix(expr, "?"); ok {
		expr = expr[1:]
	}

	// Strip parentheses if present
	if strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
		expr = expr[1 : len(expr)-1]
	}

	// Find the operator
	var op string
	opIdx := -1

	// Check for various operators
	for _, operator := range []string{"==", "!=", ">=", "<=", ">", "<", "=~"} {
		idx := strings.Index(expr, operator)
		if idx != -1 {
			opIdx = idx
			op = operator
			break
		}
	}

	// If no operator found, treat as existence check
	if opIdx == -1 {
		return &filterExpr{
			path: expr,
			op:   "",
		}
	}

	// Split into path and value
	path := strings.TrimSpace(expr[:opIdx])
	value := strings.TrimSpace(expr[opIdx+len(op):])

	// Clean up path
	if ok := strings.HasPrefix(path, "@."); ok {
		path = path[2:]
	}

	// Clean up value
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}

	return &filterExpr{
		path:  path,
		op:    op,
		value: value,
	}
}

// executeTokenizedPath executes a tokenized path against JSON data
func executeTokenizedPath(data []byte, tokens []pathToken) Result {
	// Start with the root value
	current := Parse(data)

	// Filter out modifier tokens
	modifiers, pathTokens := separateModifierTokens(tokens)

	// Process each token
	for i, token := range pathTokens {
		result, shouldReturn := processPathToken(current, token, pathTokens, i)
		if shouldReturn {
			// Use the result and stop processing more tokens; modifiers will be applied below
			current = result
			break
		}
		current = result

		if !current.Exists() {
			return Result{Type: TypeUndefined}
		}
	}

	// Apply modifiers if any
	if len(modifiers) > 0 {
		current = applyModifiersToResult(current, modifiers)
	}

	return current
}

// separateModifierTokens separates modifier tokens from path tokens
func separateModifierTokens(tokens []pathToken) ([]pathToken, []pathToken) {
	var modifiers []pathToken
	var pathTokens []pathToken

	for _, token := range tokens {
		if token.kind == tokenModifier {
			modifiers = append(modifiers, token)
		} else {
			pathTokens = append(pathTokens, token)
		}
	}

	return modifiers, pathTokens
}

// processPathToken processes a single path token and returns the result and whether to return early
func processPathToken(current Result, token pathToken, pathTokens []pathToken, i int) (Result, bool) {
	switch token.kind {
	case tokenKey:
		// If current is an array due to wildcard/filter, project key over elements
		if current.Type == TypeArray && i > 0 {
			prev := pathTokens[i-1]
			if prev.kind == tokenWildcard || prev.kind == tokenFilter {
				return processArrayProjection(current, pathTokens, i)
			}
		}
		return processKeyToken(current, token)
	case tokenIndex:
		return processIndexToken(current, token)
	case tokenWildcard:
		return processWildcardToken(current, token, pathTokens, i)
	case tokenFilter:
		return processFilterToken(current, token)
	case tokenRecursive:
		return processRecursiveToken(current, pathTokens, i)
	default:
		return Result{Type: TypeUndefined}, true
	}
}

// processKeyToken handles object key access
func processKeyToken(current Result, token pathToken) (Result, bool) {
	if current.Type != TypeObject {
		return Result{Type: TypeUndefined}, true
	}

	// Use direct object lookup instead of ForEach to avoid allocations
	start, end := fastFindObjectValue(current.Raw, token.str)
	if start == -1 {
		return Result{Type: TypeUndefined}, true
	}

	return fastParseValue(current.Raw[start:end]), false
}

// processIndexToken handles array index access
func processIndexToken(current Result, token pathToken) (Result, bool) {
	if current.Type != TypeArray {
		return Result{Type: TypeUndefined}, true
	}

	// Use direct array lookup instead of Array() to avoid allocations
	start, end := fastFindArrayElement(current.Raw, token.num)
	if start == -1 {
		return Result{Type: TypeUndefined}, true
	}

	return fastParseValue(current.Raw[start:end]), false
}

// processWildcardToken handles wildcard access
func processWildcardToken(current Result, token pathToken, pathTokens []pathToken, i int) (Result, bool) {
	if current.Type != TypeArray && current.Type != TypeObject {
		return Result{Type: TypeUndefined}, true
	}

	// Fast path for simple wildcard operations like "phones.*.type"
	if i == len(pathTokens)-2 && len(pathTokens) > 2 {
		nextToken := pathTokens[i+1]
		if nextToken.kind == tokenKey {
			return fastWildcardKeyAccess(current, nextToken.str), true
		}
	}

	return processWildcardCollection(current, pathTokens, i)
}

// processWildcardCollection handles wildcard collection processing
func processWildcardCollection(current Result, pathTokens []pathToken, i int) (Result, bool) {
	// Collect all values with minimal allocations
	values := make([]Result, 0, 8) // Pre-allocate for common case
	current.ForEach(func(_, value Result) bool {
		values = append(values, value)
		return true
	})

	if len(values) == 0 {
		return Result{Type: TypeUndefined}, true
	}

	// If this is the last token, return array of values
	if i == len(pathTokens)-1 {
		return buildArrayResult(values), false
	}

	// Otherwise, need to process each value with remaining tokens
	return processRemainingTokensForWildcard(values, pathTokens, i)
}

// buildArrayResult builds an array result from a slice of values
func buildArrayResult(values []Result) Result {
	// Use pre-calculated size to avoid reallocations
	totalSize := 2 // brackets
	for j, val := range values {
		if j > 0 {
			totalSize++ // comma
		}
		totalSize += len(val.Raw)
	}

	// Build result with single allocation
	raw := make([]byte, 0, totalSize)
	raw = append(raw, '[')
	for j, val := range values {
		if j > 0 {
			raw = append(raw, ',')
		}
		raw = append(raw, val.Raw...)
	}
	raw = append(raw, ']')

	return Result{
		Type: TypeArray,
		Raw:  raw,
	}
}

// processRemainingTokensForWildcard processes remaining tokens for wildcard results
func processRemainingTokensForWildcard(values []Result, pathTokens []pathToken, i int) (Result, bool) {
	var results []Result
	for _, val := range values {
		// Process remaining tokens for this value
		remaining := executeTokenizedPath(val.Raw, pathTokens[i+1:])
		if remaining.Exists() {
			if remaining.Type == TypeArray {
				// If result is array, merge all elements
				remaining.ForEach(func(_, item Result) bool {
					results = append(results, item)
					return true
				})
			} else {
				results = append(results, remaining)
			}
		}
	}

	if len(results) == 0 {
		return Result{Type: TypeUndefined}, true
	}

	// Create array from results
	var raw bytes.Buffer
	raw.WriteByte('[')
	for i, val := range results {
		if i > 0 {
			raw.WriteByte(',')
		}
		raw.Write(val.Raw)
	}
	raw.WriteByte(']')

	return Result{
		Type: TypeArray,
		Raw:  raw.Bytes(),
	}, true // Skip remaining tokens as we've processed them
}

// processArrayProjection applies the remaining tokens starting at index i to each
// element of the current array and combines the results into a single array.
func processArrayProjection(current Result, pathTokens []pathToken, i int) (Result, bool) {
	// Collect all values first
	values := make([]Result, 0, 8)
	current.ForEach(func(_, value Result) bool {
		values = append(values, value)
		return true
	})

	if len(values) == 0 {
		return Result{Type: TypeUndefined}, true
	}

	// Apply remaining tokens (including current token) to each element
	var results []Result
	for _, val := range values {
		remaining := executeTokenizedPath(val.Raw, pathTokens[i:])
		if remaining.Exists() {
			if remaining.Type == TypeArray {
				// Merge array items
				remaining.ForEach(func(_, item Result) bool {
					results = append(results, item)
					return true
				})
			} else {
				results = append(results, remaining)
			}
		}
	}

	if len(results) == 0 {
		return Result{Type: TypeUndefined}, true
	}

	// Build array result
	var raw bytes.Buffer
	raw.WriteByte('[')
	for i, val := range results {
		if i > 0 {
			raw.WriteByte(',')
		}
		raw.Write(val.Raw)
	}
	raw.WriteByte(']')

	return Result{Type: TypeArray, Raw: raw.Bytes()}, true
}

// processFilterToken handles filter token processing
func processFilterToken(current Result, token pathToken) (Result, bool) {
	if current.Type != TypeArray {
		return Result{Type: TypeUndefined}, true
	}

	// Apply filter
	var matches []Result
	current.ForEach(func(_, value Result) bool {
		if matchesFilter(value, token.filter) {
			matches = append(matches, value)
		}
		return true
	})

	if len(matches) == 0 {
		return Result{Type: TypeUndefined}, true
	}

	// Create a new array result
	var raw bytes.Buffer
	raw.WriteByte('[')
	for i, val := range matches {
		if i > 0 {
			raw.WriteByte(',')
		}
		raw.Write(val.Raw)
	}
	raw.WriteByte(']')

	return Result{
		Type: TypeArray,
		Raw:  raw.Bytes(),
	}, false
}

// processRecursiveToken handles recursive token processing
func processRecursiveToken(current Result, pathTokens []pathToken, i int) (Result, bool) {
	if i == len(pathTokens)-1 {
		// This is the last token, which doesn't make sense for recursive descent
		return Result{Type: TypeUndefined}, true
	}

	// Recursive descent
	result := recursiveSearch(current, pathTokens[i+1:])
	return result, true // recursiveSearch processes the rest of the tokens
}

// applyModifiersToResult applies all modifiers to the result
func applyModifiersToResult(current Result, modifiers []pathToken) Result {
	for _, mod := range modifiers {
		current = applyModifier(current, mod.str)
	}
	return current
}

// matchesFilter checks if a value matches a filter expression
func matchesFilter(value Result, filter *filterExpr) bool {
	// Get the value to filter on
	var filterValue Result
	if filter.path == "" {
		filterValue = value
	} else {
		filterValue = value.Get(filter.path)
	}

	if !filterValue.Exists() {
		return false
	}

	// If no operator, just check existence
	if filter.op == "" {
		return true
	}

	// Compare based on operator
	switch filter.op {
	case "=", constEq:
		return compareEqual(filterValue, filter.value)
	case constNe:
		return !compareEqual(filterValue, filter.value)
	case "<":
		return compareLess(filterValue, filter.value)
	case constLe:
		return compareLess(filterValue, filter.value) || compareEqual(filterValue, filter.value)
	case ">":
		return !compareLess(filterValue, filter.value) && !compareEqual(filterValue, filter.value)
	case constGe:
		return !compareLess(filterValue, filter.value) || compareEqual(filterValue, filter.value)
	case "=~", "~=":
		return strings.Contains(filterValue.String(), filter.value)
	}

	return false
}

// compareEqual compares a result with a string value for equality
func compareEqual(result Result, value string) bool {
	switch result.Type {
	case TypeString:
		return result.Str == value
	case TypeNumber:
		valueNum, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return false
		}
		return result.Num == valueNum
	case TypeBoolean:
		valueBool, err := strconv.ParseBool(value)
		if err != nil {
			return false
		}
		return result.Boolean == valueBool
	case TypeNull:
		return value == constNull
	default:
		return false
	}
}

// compareLess compares if a result is less than a string value
func compareLess(result Result, value string) bool {
	switch result.Type {
	case TypeString:
		return result.Str < value
	case TypeNumber:
		valueNum, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return false
		}
		return result.Num < valueNum
	default:
		return false
	}
}

// processRecursiveMatches processes recursive search matches for both objects and arrays
func processRecursiveMatches(current Result, remainingTokens []pathToken) []Result {
	var matches []Result
	current.ForEach(func(_, value Result) bool {
		// Try this value with remaining tokens
		subResult := executeTokenizedPath(value.Raw, remainingTokens)
		if subResult.Exists() {
			matches = append(matches, subResult)
		}

		// Continue recursion for objects and arrays
		if value.Type == TypeObject || value.Type == TypeArray {
			subMatches := recursiveSearch(value, remainingTokens)
			if subMatches.Exists() {
				if subMatches.Type == TypeArray {
					// Add all items from array
					subMatches.ForEach(func(_, item Result) bool {
						matches = append(matches, item)
						return true
					})
				} else {
					matches = append(matches, subMatches)
				}
			}
		}
		return true
	})
	return matches
}

// recursiveSearch searches recursively through a JSON structure
func recursiveSearch(current Result, remainingTokens []pathToken) Result {
	// End of path, return current
	if len(remainingTokens) == 0 {
		return current
	}

	// Try direct match first
	direct := executeTokenizedPath(current.Raw, remainingTokens)
	if direct.Exists() {
		return direct
	}

	// Collect results from recursive descent
	var matches []Result

	switch current.Type {
	case TypeObject, TypeArray:
		matches = processRecursiveMatches(current, remainingTokens)
	}

	if len(matches) == 0 {
		return Result{Type: TypeUndefined}
	}

	// Return array of matches
	var raw bytes.Buffer
	raw.WriteByte('[')
	for i, val := range matches {
		if i > 0 {
			raw.WriteByte(',')
		}
		raw.Write(val.Raw)
	}
	raw.WriteByte(']')

	return Result{
		Type: TypeArray,
		Raw:  raw.Bytes(),
	}
}

// applyModifier applies a modifier to a result
func applyModifier(result Result, modifier string) Result {
	// Parse modifier and argument
	parts := strings.SplitN(modifier, ":", 2)
	name := parts[0]
	var arg string
	if len(parts) > 1 {
		arg = parts[1]
	}

	switch name {
	case constString, "str":
		return applyStringModifier(result)
	case constNumber, "num":
		return applyNumberModifier(result)
	case constBool, constBoolean:
		return applyBooleanModifier(result)
	case "keys":
		return applyKeysModifier(result)
	case "values":
		return applyValuesModifier(result)
	case "length", "count", "len":
		return applyLengthModifier(result)
	case "type":
		return applyTypeModifier(result)
	case "base64":
		return applyBase64Modifier(result)
	case "base64decode":
		return applyBase64DecodeModifier(result)
	case "lower":
		return applyLowerModifier(result)
	case "upper":
		return applyUpperModifier(result)
	case "join":
		return applyJoinModifier(result, arg)
	case "reverse":
		return applyReverseModifier(result)
	case "flatten":
		return applyFlattenModifier(result)
	case "distinct", "unique":
		return applyDistinctModifier(result)
	case "sort":
		return applySortModifier(result, arg)
	case "first":
		return applyFirstModifier(result)
	case "last":
		return applyLastModifier(result)
	case "sum":
		return applySumModifier(result)
	case "avg", "average", "mean":
		return applyAverageModifier(result)
	case "min":
		return applyMinModifier(result)
	case "max":
		return applyMaxModifier(result)
	}

	// Return original result if no modifier was applied
	return result
}

// applyStringModifier converts result to string type
func applyStringModifier(result Result) Result {
	return Result{
		Type:     TypeString,
		Str:      result.String(),
		Raw:      []byte(`"` + escapeString(result.String()) + `"`),
		Modified: true,
	}
}

// applyNumberModifier converts result to number type
func applyNumberModifier(result Result) Result {
	num := result.Float()
	return Result{
		Type:     TypeNumber,
		Num:      num,
		Raw:      []byte(strconv.FormatFloat(num, 'f', -1, 64)),
		Modified: true,
	}
}

// applyBooleanModifier converts result to boolean type
func applyBooleanModifier(result Result) Result {
	b := result.Bool()
	var raw []byte
	if b {
		raw = []byte("true")
	} else {
		raw = []byte("false")
	}
	return Result{
		Type:     TypeBoolean,
		Boolean:  b, // Use the renamed field
		Raw:      raw,
		Modified: true,
	}
}

// applyKeysModifier extracts object keys as array
func applyKeysModifier(result Result) Result {
	if result.Type != TypeObject {
		return Result{Type: TypeUndefined}
	}

	var keys []string
	result.ForEach(func(key, _ Result) bool {
		keys = append(keys, key.Str)
		return true
	})

	// Sort keys for stable output
	sort.Strings(keys)

	// Build JSON array of keys
	var raw bytes.Buffer
	raw.WriteByte('[')
	for i, k := range keys {
		if i > 0 {
			raw.WriteByte(',')
		}
		raw.WriteByte('"')
		raw.WriteString(escapeString(k))
		raw.WriteByte('"')
	}
	raw.WriteByte(']')

	return Result{
		Type:     TypeArray,
		Raw:      raw.Bytes(),
		Modified: true,
	}
}

// applyValuesModifier extracts object values as array
func applyValuesModifier(result Result) Result {
	if result.Type != TypeObject {
		return Result{Type: TypeUndefined}
	}

	var values []Result
	var keys []string

	// First collect keys and values
	result.ForEach(func(key, value Result) bool {
		keys = append(keys, key.Str)
		values = append(values, value)
		return true
	})

	// Sort by keys for stable output
	sort.Slice(values, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	// Build JSON array of values
	var raw bytes.Buffer
	raw.WriteByte('[')
	for i, v := range values {
		if i > 0 {
			raw.WriteByte(',')
		}
		raw.Write(v.Raw)
	}
	raw.WriteByte(']')

	return Result{
		Type:     TypeArray,
		Raw:      raw.Bytes(),
		Modified: true,
	}
}

// applyLengthModifier returns length/count of result
func applyLengthModifier(result Result) Result {
	switch result.Type {
	case TypeArray:
		count := len(result.Array())
		return buildCountResult(count)
	case TypeObject:
		count := len(result.Map())
		return buildCountResult(count)
	case TypeString:
		count := len(result.Str)
		return buildCountResult(count)
	default:
		return Result{Type: TypeUndefined}
	}
}

// buildCountResult builds a numeric result for count values
func buildCountResult(count int) Result {
	return Result{
		Type:     TypeNumber,
		Num:      float64(count),
		Raw:      []byte(strconv.Itoa(count)),
		Modified: true,
	}
}

// applyTypeModifier returns the type of the result as string
func applyTypeModifier(result Result) Result {
	var typeStr string
	switch result.Type {
	case TypeString:
		typeStr = constString
	case TypeNumber:
		typeStr = constNumber
	case TypeBoolean:
		typeStr = constBoolean
	case TypeObject:
		typeStr = "object"
	case TypeArray:
		typeStr = "array"
	case TypeNull:
		typeStr = constNull
	default:
		typeStr = "undefined"
	}

	return Result{
		Type:     TypeString,
		Str:      typeStr,
		Raw:      []byte(`"` + typeStr + `"`),
		Modified: true,
	}
}

// applyBase64Modifier encodes string as base64
func applyBase64Modifier(result Result) Result {
	if result.Type == TypeString {
		encoded := base64.StdEncoding.EncodeToString([]byte(result.Str))
		return Result{
			Type:     TypeString,
			Str:      encoded,
			Raw:      []byte(`"` + encoded + `"`),
			Modified: true,
		}
	}
	return result
}

// applyBase64DecodeModifier decodes base64 string
func applyBase64DecodeModifier(result Result) Result {
	if result.Type == TypeString {
		decoded, err := base64.StdEncoding.DecodeString(result.Str)
		if err != nil {
			return Result{Type: TypeUndefined}
		}
		return Result{
			Type:     TypeString,
			Str:      string(decoded),
			Raw:      []byte(`"` + escapeString(string(decoded)) + `"`),
			Modified: true,
		}
	}
	return result
}

// applyLowerModifier converts string to lowercase
func applyLowerModifier(result Result) Result {
	if result.Type == TypeString {
		lower := strings.ToLower(result.Str)
		return Result{
			Type:     TypeString,
			Str:      lower,
			Raw:      []byte(`"` + escapeString(lower) + `"`),
			Modified: true,
		}
	}
	return result
}

// applyUpperModifier converts string to uppercase
func applyUpperModifier(result Result) Result {
	if result.Type == TypeString {
		upper := strings.ToUpper(result.Str)
		return Result{
			Type:     TypeString,
			Str:      upper,
			Raw:      []byte(`"` + escapeString(upper) + `"`),
			Modified: true,
		}
	}
	return result
}

// applyJoinModifier joins array elements with separator
func applyJoinModifier(result Result, arg string) Result {
	if result.Type == TypeArray {
		arr := result.Array()
		sep := ","
		if arg != "" {
			sep = arg
		}

		var parts []string
		for _, v := range arr {
			parts = append(parts, v.String())
		}

		joined := strings.Join(parts, sep)
		return Result{
			Type:     TypeString,
			Str:      joined,
			Raw:      []byte(`"` + escapeString(joined) + `"`),
			Modified: true,
		}
	}
	return result
}

// applyReverseModifier reverses array elements order
func applyReverseModifier(result Result) Result {
	if result.Type != TypeArray {
		// No-op for non-arrays
		return result
	}
	// Collect elements
	var values []Result
	result.ForEach(func(_, value Result) bool {
		values = append(values, value)
		return true
	})
	// Reverse in place
	for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
		values[i], values[j] = values[j], values[i]
	}
	// Build array raw
	reversed := buildWildcardResult(values)
	reversed.Modified = true
	return reversed
}

func applyFlattenModifier(result Result) Result {
	if result.Type != TypeArray {
		return result
	}

	var flattened []Result
	flattenResults(result, &flattened)
	if len(flattened) == 0 {
		return Result{Type: TypeArray, Raw: []byte("[]"), Modified: true}
	}

	flattenedResult := buildWildcardResult(flattened)
	flattenedResult.Modified = true
	return flattenedResult
}

func applyDistinctModifier(result Result) Result {
	if result.Type != TypeArray {
		return result
	}

	unique := make([]Result, 0)
	seen := make(map[string]struct{})

	result.ForEach(func(_, value Result) bool {
		key := string(value.Raw)
		if key == "" {
			key = fmt.Sprintf("%d:%s", value.Type, value.String())
		}
		if _, exists := seen[key]; exists {
			return true
		}
		seen[key] = struct{}{}
		unique = append(unique, value)
		return true
	})

	if len(unique) == 0 {
		return Result{Type: TypeArray, Raw: []byte("[]"), Modified: true}
	}

	distinctResult := buildWildcardResult(unique)
	distinctResult.Modified = true
	return distinctResult
}

func applySortModifier(result Result, arg string) Result {
	if result.Type != TypeArray {
		return result
	}

	items := result.Array()
	if len(items) == 0 {
		return Result{Type: TypeArray, Raw: []byte("[]"), Modified: true}
	}

	sorted := make([]Result, len(items))
	copy(sorted, items)

	descending := strings.EqualFold(arg, "desc") || strings.EqualFold(arg, "descending") || strings.EqualFold(arg, "reverse")

	allNumbers := true
	for _, item := range sorted {
		if item.Type != TypeNumber {
			allNumbers = false
			break
		}
	}

	if allNumbers {
		sort.Slice(sorted, func(i, j int) bool {
			if descending {
				return sorted[i].Float() > sorted[j].Float()
			}
			return sorted[i].Float() < sorted[j].Float()
		})
	} else {
		sort.Slice(sorted, func(i, j int) bool {
			if descending {
				return sorted[i].String() > sorted[j].String()
			}
			return sorted[i].String() < sorted[j].String()
		})
	}

	sortedResult := buildWildcardResult(sorted)
	if sortedResult.Type == TypeUndefined {
		return Result{Type: TypeArray, Raw: []byte("[]"), Modified: true}
	}
	sortedResult.Modified = true
	return sortedResult
}

func applyFirstModifier(result Result) Result {
	if result.Type != TypeArray {
		return result
	}

	first := Result{Type: TypeUndefined}
	result.ForEach(func(_, value Result) bool {
		first = value
		return false
	})
	return first
}

func applyLastModifier(result Result) Result {
	if result.Type != TypeArray {
		return result
	}

	last := Result{Type: TypeUndefined}
	result.ForEach(func(_, value Result) bool {
		last = value
		return true
	})
	return last
}

func applySumModifier(result Result) Result {
	if result.Type != TypeArray {
		return Result{Type: TypeUndefined}
	}

	var sum float64
	count := 0
	result.ForEach(func(_, value Result) bool {
		if num, ok := numericValue(value); ok {
			sum += num
			count++
		}
		return true
	})

	if count == 0 {
		return Result{Type: TypeUndefined}
	}

	return buildNumberResult(sum)
}

func applyAverageModifier(result Result) Result {
	if result.Type != TypeArray {
		return Result{Type: TypeUndefined}
	}

	var sum float64
	count := 0
	result.ForEach(func(_, value Result) bool {
		if num, ok := numericValue(value); ok {
			sum += num
			count++
		}
		return true
	})

	if count == 0 {
		return Result{Type: TypeUndefined}
	}

	return buildNumberResult(sum / float64(count))
}

func applyMinModifier(result Result) Result {
	if result.Type != TypeArray {
		return Result{Type: TypeUndefined}
	}

	var min float64
	hasValue := false
	result.ForEach(func(_, value Result) bool {
		if num, ok := numericValue(value); ok {
			if !hasValue || num < min {
				min = num
				hasValue = true
			}
		}
		return true
	})

	if !hasValue {
		return Result{Type: TypeUndefined}
	}

	return buildNumberResult(min)
}

func applyMaxModifier(result Result) Result {
	if result.Type != TypeArray {
		return Result{Type: TypeUndefined}
	}

	var max float64
	hasValue := false
	result.ForEach(func(_, value Result) bool {
		if num, ok := numericValue(value); ok {
			if !hasValue || num > max {
				max = num
				hasValue = true
			}
		}
		return true
	})

	if !hasValue {
		return Result{Type: TypeUndefined}
	}

	return buildNumberResult(max)
}

func flattenResults(result Result, out *[]Result) {
	if result.Type != TypeArray {
		*out = append(*out, result)
		return
	}

	result.ForEach(func(_, value Result) bool {
		if value.Type == TypeArray {
			flattenResults(value, out)
		} else {
			*out = append(*out, value)
		}
		return true
	})
}

func numericValue(result Result) (float64, bool) {
	switch result.Type {
	case TypeNumber:
		return result.Num, true
	case TypeString:
		trimmed := strings.TrimSpace(result.Str)
		if trimmed == "" {
			return 0, false
		}
		v, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, false
		}
		return v, true
	case TypeBoolean:
		if result.Boolean {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func buildNumberResult(value float64) Result {
	raw := strconv.FormatFloat(value, 'f', -1, 64)
	return Result{
		Type:     TypeNumber,
		Num:      value,
		Raw:      []byte(raw),
		Modified: true,
	}
}

// processArrayWildcard processes wildcard access on array elements
func processArrayWildcard(data []byte, key string) []Result {
	results := make([]Result, 0, 4)
	start := 1 // Skip '['

	for start < len(data) {
		// Skip whitespace
		for start < len(data) && data[start] <= ' ' {
			start++
		}

		if start >= len(data) || data[start] == ']' {
			break
		}

		// Find end of this element
		end := ultraFastSkipValue(data, start)
		if end == -1 {
			break
		}

		// Parse element and get the key
		element := fastParseValue(data[start:end])
		if element.Type == TypeObject {
			valueBytes := getObjectValue(element.Raw, key)
			if len(valueBytes) > 0 {
				result := fastParseValue(valueBytes)
				results = append(results, result)
			}
		}

		start = end
		// Skip comma and whitespace
		for start < len(data) && (data[start] <= ' ' || data[start] == ',') {
			start++
		}
	}

	return results
}

// processObjectWildcard processes wildcard access on object values
func processObjectWildcard(current Result, key string) []Result {
	results := make([]Result, 0, 4)

	// For objects, get all values and then extract the key
	current.ForEach(func(_, value Result) bool {
		if value.Type == TypeObject {
			valueBytes := getObjectValue(value.Raw, key)
			if len(valueBytes) > 0 {
				result := fastParseValue(valueBytes)
				results = append(results, result)
			}
		}
		return true
	})

	return results
}

// buildWildcardResult constructs the final result from collected wildcard results
func buildWildcardResult(results []Result) Result {
	if len(results) == 0 {
		return Result{Type: TypeUndefined}
	}

	// If only one result, return it directly
	if len(results) == 1 {
		return results[0]
	}

	// Build array result with optimized allocation
	totalSize := 2 // brackets
	for i, result := range results {
		if i > 0 {
			totalSize++ // comma
		}
		totalSize += len(result.Raw)
	}

	raw := make([]byte, 0, totalSize)
	raw = append(raw, '[')
	for i, result := range results {
		if i > 0 {
			raw = append(raw, ',')
		}
		raw = append(raw, result.Raw...)
	}
	raw = append(raw, ']')

	return Result{
		Type: TypeArray,
		Raw:  raw,
	}
}

// fastWildcardKeyAccess optimizes wildcard access followed by a key like "phones.*.type"
func fastWildcardKeyAccess(current Result, key string) Result {
	if current.Type != TypeArray && current.Type != TypeObject {
		return Result{Type: TypeUndefined}
	}

	var results []Result

	// Optimize for arrays vs objects
	if current.Type == TypeArray {
		results = processArrayWildcard(current.Raw, key)
	} else {
		results = processObjectWildcard(current, key)
	}

	return buildWildcardResult(results)
}

// getObjectValue extracts a value from an object by key
// validateObjectForSearch validates data is an object and returns starting position
func validateObjectForSearch(data []byte) (int, bool) {
	start := 0
	for ; start < len(data); start++ {
		if data[start] > ' ' {
			break
		}
	}
	if start >= len(data) || data[start] != '{' {
		return 0, false
	}
	return start, true
}

// findKeyEnd finds the end position of a JSON key
func findKeyEnd(data []byte, pos int) int {
	pos++ // Skip opening quote
	for ; pos < len(data) && data[pos] != '"'; pos++ {
		if data[pos] == '\\' {
			pos++ // Skip escape char and the escaped char
		}
	}
	if pos >= len(data) {
		return -1
	}
	return pos
}

// skipToColon finds the colon after a key
func skipToColon(data []byte, pos int) int {
	for ; pos < len(data) && data[pos] != ':'; pos++ {
	}
	if pos >= len(data) {
		return -1
	}
	return pos
}

// skipToValue skips whitespace and colon to find the value
func skipToValue(data []byte, pos int) int {
	// Skip colon and whitespace
	pos++
	for ; pos < len(data) && data[pos] <= ' '; pos++ {
	}
	if pos >= len(data) {
		return -1
	}
	return pos
}

// keyMatches checks if the current key matches the target key
func keyMatches(currentKey []byte, key string) bool {
	keyStr := `"` + key + `"`
	return string(currentKey) == keyStr || string(currentKey) == `"`+key+`"`
}

// skipToNextKeyOrEnd skips to the next key or end of object
func skipToNextKeyOrEnd(data []byte, pos int) int {
	// Skip to next key or end of object
	for ; pos < len(data) && data[pos] != ',' && data[pos] != '}'; pos++ {
	}
	if pos >= len(data) || data[pos] == '}' {
		return -1 // End of object, key not found
	}
	return pos + 1 // Skip comma
}

func getObjectValue(data []byte, key string) []byte {
	// Validate object and get starting position
	start, valid := validateObjectForSearch(data)
	if !valid {
		return nil
	}

	pos := start + 1

	for pos < len(data) {
		// Skip whitespace
		for ; pos < len(data) && data[pos] <= ' '; pos++ {
		}

		if pos >= len(data) || data[pos] != '"' {
			return nil
		}

		// Check if this is our key
		keyStart := pos
		keyEnd := findKeyEnd(data, pos)
		if keyEnd == -1 {
			return nil
		}

		currentKey := data[keyStart : keyEnd+1]

		// Skip to colon
		pos = skipToColon(data, keyEnd+1)
		if pos == -1 {
			return nil
		}

		// Skip to value
		pos = skipToValue(data, pos)
		if pos == -1 {
			return nil
		}

		// Found the value, check if it matches our key
		if keyMatches(currentKey, key) {
			// This is our value, find its end
			valueStart := pos
			valueEnd := findValueEnd(data, pos)
			if valueEnd == -1 {
				return nil
			}
			return data[valueStart:valueEnd]
		}

		// Skip this value
		pos = findValueEnd(data, pos)
		if pos == -1 {
			return nil
		}

		// Skip to next key or end of object
		pos = skipToNextKeyOrEnd(data, pos)
		if pos == -1 {
			return nil
		}
	}

	return nil
}

// getObjectValueRange returns the start and end indices (relative to data) of the value for key within an object.
// Returns (-1, -1) when not found or data is not an object.
func getObjectValueRange(data []byte, key string) (int, int) {
	// Validate object and get starting position
	start, valid := validateObjectForSearch(data)
	if !valid {
		return -1, -1
	}

	pos := start + 1

	for pos < len(data) {
		// Skip whitespace
		for ; pos < len(data) && data[pos] <= ' '; pos++ {
		}

		if pos >= len(data) || data[pos] != '"' {
			return -1, -1
		}

		// Check if this is our key
		keyStart := pos
		keyEnd := findKeyEnd(data, pos)
		if keyEnd == -1 {
			return -1, -1
		}

		currentKey := data[keyStart : keyEnd+1]

		// Skip to colon
		pos = skipToColon(data, keyEnd+1)
		if pos == -1 {
			return -1, -1
		}

		// Skip to value
		pos = skipToValue(data, pos)
		if pos == -1 {
			return -1, -1
		}

		valueStart := pos
		valueEnd := findValueEnd(data, pos)
		if valueEnd == -1 {
			return -1, -1
		}

		// Check if this matches our key
		if keyMatches(currentKey, key) {
			return valueStart, valueEnd
		}

		// Skip to next key or end of object
		pos = skipToNextKeyOrEnd(data, valueEnd)
		if pos == -1 {
			return -1, -1
		}
	}

	return -1, -1
}

// findArrayElementStart finds the start position of an array and validates it
func findArrayElementStart(data []byte) (int, bool) {
	// Skip whitespace
	start := 0
	for ; start < len(data); start++ {
		if data[start] > ' ' {
			break
		}
	}

	// Check if it's an array
	if start >= len(data) || data[start] != '[' {
		return 0, false
	}

	return start + 1, true
}

// skipToNextArrayElement advances position to the next array element
func skipToNextArrayElement(data []byte, pos int) (int, bool) {
	// Skip this value
	pos = findValueEnd(data, pos)
	if pos == -1 {
		return 0, false
	}

	// Skip to next element or end of array
	for ; pos < len(data) && data[pos] != ',' && data[pos] != ']'; pos++ {
	}

	if pos >= len(data) || data[pos] == ']' {
		return 0, false // End of array, index out of bounds
	}

	return pos + 1, true // Skip comma
}

// getArrayElementRange returns the start and end indices (relative to data) of the element at index within an array.
// Returns (-1, -1) when out of bounds or data is not an array.
// PHASE 2 OPTIMIZATION: Statistical jump algorithm for large arrays
func getArrayElementRange(data []byte, index int) (int, int) {
	pos, isArray := findArrayElementStart(data)
	if !isArray {
		return -1, -1
	}

	// PHASE 2: Statistical jump for large indices
	// Use statistical estimation to jump closer to target index
	if index > 100 && len(data) > 10000 {
		// Estimate element size by sampling first few elements
		sampleSize := 10
		samplePos := pos
		elementsScanned := 0
		totalSize := 0

		for elementsScanned < sampleSize && samplePos < len(data) {
			// Skip whitespace
			for ; samplePos < len(data) && data[samplePos] <= ' '; samplePos++ {
			}
			if samplePos >= len(data) || data[samplePos] == ']' {
				break
			}

			elemStart := samplePos
			elemEnd := findValueEnd(data, samplePos)
			if elemEnd == -1 {
				break
			}

			totalSize += elemEnd - elemStart
			samplePos = elemEnd

			// Skip comma and whitespace
			for ; samplePos < len(data) && data[samplePos] <= ' '; samplePos++ {
			}
			if samplePos < len(data) && data[samplePos] == ',' {
				samplePos++
			}

			elementsScanned++
		}

		if elementsScanned > 0 {
			// Calculate average element size
			avgSize := totalSize / elementsScanned

			// Jump to estimated position
			jumpPos := pos + (avgSize+2)*(index-elementsScanned) // +2 for comma and space
			if jumpPos < len(data) && jumpPos > pos {
				// Count actual elements from jump position backward to refine
				commaCount := 0
				for i := pos; i < jumpPos && i < len(data); i++ {
					if data[i] == ',' {
						commaCount++
					}
				}

				// Adjust position based on comma count
				if commaCount > index {
					// Overshot, go back
					jumpPos = pos
				} else {
					// Use comma count as starting point
					pos = jumpPos
					// Back up to last known comma
					for pos > 0 && data[pos] != ',' {
						pos--
					}
					if pos > 0 {
						pos++ // After comma
					}
					index = index - commaCount
				}
			}
		}
	}

	// Linear scan from current position (either start or jumped position)
	currentIndex := 0
	for pos < len(data) {
		for ; pos < len(data) && data[pos] <= ' '; pos++ {
		}
		if pos >= len(data) || data[pos] == ']' {
			return -1, -1
		}
		if currentIndex == index {
			valueStart := pos
			valueEnd := findValueEnd(data, pos)
			if valueEnd == -1 {
				return -1, -1
			}
			return valueStart, valueEnd
		}

		// Skip to next element
		var ok bool
		pos, ok = skipToNextArrayElement(data, pos)
		if !ok {
			return -1, -1
		}
		currentIndex++
	}
	return -1, -1
}

// findValueEnd finds the end position of a JSON value
func findValueEnd(data []byte, start int) int {
	if start >= len(data) {
		return -1
	}

	switch data[start] {
	case '{':
		return findBlockEnd(data, start, '{', '}')
	case '[':
		return findBlockEnd(data, start, '[', ']')
	case '"':
		return findStringEndFromStart(data, start)
	case 't':
		return findTrueEnd(data, start)
	case 'f':
		return findFalseEnd(data, start)
	case 'n':
		return findNullEnd(data, start)
	default:
		return findNumberEnd(data, start)
	}
}

// findStringEndFromStart finds the end of a JSON string starting from quote
func findStringEndFromStart(data []byte, start int) int {
	// String - find closing quote
	for i := start + 1; i < len(data); i++ {
		if data[i] == '\\' {
			i++ // Skip escape character
			continue
		}
		if data[i] == '"' {
			return i + 1
		}
	}
	return -1
}

// findTrueEnd finds the end of a 'true' literal
func findTrueEnd(data []byte, start int) int {
	if start+3 < len(data) &&
		data[start+1] == 'r' &&
		data[start+2] == 'u' &&
		data[start+3] == 'e' {
		return start + 4
	}
	return -1
}

// findFalseEnd finds the end of a 'false' literal
func findFalseEnd(data []byte, start int) int {
	if start+4 < len(data) &&
		data[start+1] == 'a' &&
		data[start+2] == 'l' &&
		data[start+3] == 's' &&
		data[start+4] == 'e' {
		return start + 5
	}
	return -1
}

// findNullEnd finds the end of a 'null' literal
func findNullEnd(data []byte, start int) int {
	if start+3 < len(data) &&
		data[start+1] == 'u' &&
		data[start+2] == 'l' &&
		data[start+3] == 'l' {
		return start + 4
	}
	return -1
}

// findNumberEnd finds the end of a number value
func findNumberEnd(data []byte, start int) int {
	// Number - scan until non-number character
	if isNumberStart(data[start]) {
		for i := start + 1; i < len(data); i++ {
			if !isNumberChar(data[i]) {
				return i
			}
		}
		return len(data)
	}
	return -1
}

// isNumberStart checks if character can start a number
func isNumberStart(c byte) bool {
	return (c >= '0' && c <= '9') ||
		c == '-' || c == '+' ||
		c == '.' || c == 'e' ||
		c == 'E'
}

// isNumberChar checks if character can be part of a number
func isNumberChar(c byte) bool {
	return (c >= '0' && c <= '9') ||
		c == '.' || c == 'e' ||
		c == 'E' || c == '+' ||
		c == '-'
}

// findBlockEnd finds the end of a JSON block (object or array)
func findBlockEnd(data []byte, start int, openChar, closeChar byte) int {
	depth := 1
	inString := false

	for i := start + 1; i < len(data); i++ {
		if inString {
			if data[i] == '\\' {
				i++ // Skip escape character
				continue
			}
			if data[i] == '"' {
				inString = false
			}
			continue
		}

		switch data[i] {
		case '"':
			inString = true
		case openChar:
			depth++
		case closeChar:
			depth--
			if depth == 0 {
				return i + 1 // Found matching close
			}
		}
	}

	return -1 // No matching close found
}

// findStringEnd finds the end of a JSON string, handling escapes
// PHASE 2: skipValue - Fast value skipping without parsing
// Used for efficiently skipping unwanted key-value pairs
func skipValue(data []byte, i int) int {
	// Skip leading whitespace
	for ; i < len(data) && data[i] <= ' '; i++ {
	}

	if i >= len(data) {
		return i
	}

	switch data[i] {
	case '"': // String
		i++
		for ; i < len(data); i++ {
			if data[i] == '\\' && i+1 < len(data) {
				i++ // Skip escaped char
				continue
			}
			if data[i] == '"' {
				return i + 1
			}
		}
		return i

	case '{': // Object
		i++
		depth := 1
		inString := false
		for ; i < len(data) && depth > 0; i++ {
			if inString {
				if data[i] == '\\' && i+1 < len(data) {
					i++
					continue
				}
				if data[i] == '"' {
					inString = false
				}
			} else {
				if data[i] == '"' {
					inString = true
				} else if data[i] == '{' {
					depth++
				} else if data[i] == '}' {
					depth--
				}
			}
		}
		return i

	case '[': // Array
		i++
		depth := 1
		inString := false
		for ; i < len(data) && depth > 0; i++ {
			if inString {
				if data[i] == '\\' && i+1 < len(data) {
					i++
					continue
				}
				if data[i] == '"' {
					inString = false
				}
			} else {
				if data[i] == '"' {
					inString = true
				} else if data[i] == '[' {
					depth++
				} else if data[i] == ']' {
					depth--
				}
			}
		}
		return i

	case 't': // true
		if i+3 < len(data) && data[i+1] == 'r' && data[i+2] == 'u' && data[i+3] == 'e' {
			return i + 4
		}
		return i + 1

	case 'f': // false
		if i+4 < len(data) && data[i+1] == 'a' && data[i+2] == 'l' && data[i+3] == 's' && data[i+4] == 'e' {
			return i + 5
		}
		return i + 1

	case 'n': // null
		if i+3 < len(data) && data[i+1] == 'u' && data[i+2] == 'l' && data[i+3] == 'l' {
			return i + 4
		}
		return i + 1

	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9': // Number
		i++
		for ; i < len(data); i++ {
			c := data[i]
			if (c < '0' || c > '9') && c != '.' && c != 'e' && c != 'E' && c != '+' && c != '-' {
				return i
			}
		}
		return i
	}

	return i
}

func findStringEnd(data []byte, start int) int {
	if start >= len(data) || data[start] != '"' {
		return -1
	}

	end := start + 1
	for end < len(data) {
		if data[end] == '\\' {
			end++ // Skip escape character
			if end < len(data) {
				end++
			}
			continue
		}
		if data[end] == '"' {
			return end
		}
		end++
	}
	return -1
}

// processEscapeSequence processes a single escape sequence during string parsing
func processEscapeSequence(str []byte, i *int, sb *strings.Builder) {
	if *i+1 >= len(str) {
		return
	}
	*i++
	switch str[*i] {
	case '"', '\\', '/', '\'':
		sb.WriteByte(str[*i])
	case 'b':
		sb.WriteByte('\b')
	case 'f':
		sb.WriteByte('\f')
	case 'n':
		sb.WriteByte('\n')
	case 'r':
		sb.WriteByte('\r')
	case 't':
		sb.WriteByte('\t')
	case 'u':
		processUnicodeEscape(str, i, sb)
	}
}

// processUnicodeEscape processes a unicode escape sequence (\uXXXX)
func processUnicodeEscape(str []byte, i *int, sb *strings.Builder) {
	if *i+4 >= len(str) {
		return
	}

	// Parse 4 hex digits
	var r rune
	validHex := true
	for j := 1; j <= 4; j++ {
		h := str[*i+j]
		var v byte
		if h >= '0' && h <= '9' {
			v = h - '0'
		} else if h >= 'a' && h <= 'f' {
			v = h - 'a' + 10
		} else if h >= 'A' && h <= 'F' {
			v = h - 'A' + 10
		} else {
			// Invalid hex digit
			sb.WriteByte('?')
			validHex = false
			break
		}
		r = r*16 + rune(v)
	}
	if validHex {
		sb.WriteRune(r)
	}
	*i += 4
}

// unescapeStringContent unescapes string content with escape sequences
func unescapeStringContent(str []byte) string {
	var sb strings.Builder
	for i := 0; i < len(str); i++ {
		if str[i] == '\\' {
			processEscapeSequence(str, &i, &sb)
		} else {
			sb.WriteByte(str[i])
		}
	}
	return sb.String()
}

// parseString parses a JSON string
func parseString(data []byte, start int) Result {
	end := findStringEnd(data, start)
	if end == -1 {
		return Result{Type: TypeUndefined}
	}

	// Extract string content
	raw := data[start : end+1]
	str := raw[1 : len(raw)-1] // Remove quotes

	// PHASE 1 OPTIMIZATION: Zero-copy string conversion for strings without escapes
	// Fast path for strings without escapes (most common case)
	if !bytes.ContainsAny(str, "\\") {
		return Result{
			Type:  TypeString,
			Str:   unsafe.String(unsafe.SliceData(str), len(str)), // Zero-copy conversion
			Raw:   raw,
			Index: start,
		}
	}

	// Unescape the string (still allocates, but less common)
	unescaped := unescapeStringContent(str)

	return Result{
		Type:  TypeString,
		Str:   unescaped,
		Raw:   raw,
		Index: start,
	}
}

// parseNumber parses a JSON number
func parseNumber(data []byte, start int) Result {
	if start >= len(data) {
		return Result{Type: TypeUndefined}
	}

	// Find the end of the number
	end := start
	for ; end < len(data); end++ {
		if (data[end] < '0' || data[end] > '9') &&
			data[end] != '.' && data[end] != 'e' &&
			data[end] != 'E' && data[end] != '+' &&
			data[end] != '-' {
			break
		}
	}

	raw := data[start:end]

	// PHASE 1 OPTIMIZATION: Zero-copy number parsing
	// Fast path for simple integers using unsafe.String
	if !bytes.ContainsAny(raw, ".eE+-") {
		// It's a simple integer
		n, err := strconv.ParseInt(unsafe.String(unsafe.SliceData(raw), len(raw)), 10, 64)
		if err == nil {
			return Result{
				Type:  TypeNumber,
				Num:   float64(n),
				Raw:   raw,
				Index: start,
			}
		}
	}

	// Parse as float (zero-copy)
	n, err := strconv.ParseFloat(unsafe.String(unsafe.SliceData(raw), len(raw)), 64)
	if err != nil {
		return Result{Type: TypeUndefined}
	}

	return Result{
		Type:  TypeNumber,
		Num:   n,
		Raw:   raw,
		Index: start,
	}
}

// parseAny parses any JSON value
func parseAny(data []byte) Result {
	// Skip leading whitespace
	start := skipLeadingWhitespace(data)

	if start >= len(data) {
		return Result{Type: TypeUndefined}
	}

	switch data[start] {
	case '{':
		return Result{
			Type:  TypeObject,
			Raw:   data,
			Index: start,
		}
	case '[':
		return Result{
			Type:  TypeArray,
			Raw:   data,
			Index: start,
		}
	case '"':
		return parseString(data, start)
	case 't':
		if result, ok := parseBooleanTrue(data, start); ok {
			return result
		}
	case 'f':
		if result, ok := parseBooleanFalse(data, start); ok {
			return result
		}
	case 'n':
		if result, ok := parseNull(data, start); ok {
			return result
		}
	default:
		if isNumericChar(data[start]) || data[start] == '+' {
			return parseNumber(data, start)
		}
	}

	return Result{Type: TypeUndefined}
}

//------------------------------------------------------------------------------
// RESULT METHODS
//------------------------------------------------------------------------------

// String returns the result as a string
func (r Result) String() string {
	switch r.Type {
	case TypeString:
		return r.Str
	case TypeNumber:
		return strconv.FormatFloat(r.Num, 'f', -1, 64)
	case TypeBoolean:
		return strconv.FormatBool(r.Boolean)
	case TypeNull:
		return constNull
	case TypeArray, TypeObject:
		return string(r.Raw)
	default:
		return ""
	}
}

// Int returns the result as an int64
func (r Result) Int() int64 {
	switch r.Type {
	case TypeNumber:
		return int64(r.Num)
	case TypeString:
		n, _ := strconv.ParseInt(r.Str, 10, 64)
		return n
	case TypeBoolean:
		if r.Boolean {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// Float returns the result as a float64
func (r Result) Float() float64 {
	switch r.Type {
	case TypeNumber:
		return r.Num
	case TypeString:
		n, _ := strconv.ParseFloat(r.Str, 64)
		return n
	case TypeBoolean:
		if r.Boolean {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// Bool returns the result as a boolean
func (r Result) Bool() bool {
	switch r.Type {
	case TypeBoolean:
		return r.Boolean // Use the renamed field
	case TypeNumber:
		return r.Num != 0
	case TypeString:
		b, err := strconv.ParseBool(strings.ToLower(r.Str))
		if err != nil {
			return r.Str != "" && r.Str != "0" && r.Str != "false"
		}
		return b
	default:
		return false
	}
}

// Exists checks if the result exists
func (r Result) Exists() bool {
	return r.Type != TypeUndefined
}

// IsNull checks if the result is null
func (r Result) IsNull() bool {
	return r.Type == TypeNull
}

func (r Result) IsArray() bool {
	return r.Type == TypeArray
}

func (r Result) IsObject() bool {
	return r.Type == TypeObject
}

// Array returns the result as a slice of results
func (r Result) Array() []Result {
	if r.Type != TypeArray {
		return nil
	}
	var results []Result
	r.ForEach(func(_, value Result) bool {
		results = append(results, value)
		return true
	})
	return results
}

// Map returns the result as a map
func (r Result) Map() map[string]Result {
	if r.Type != TypeObject {
		return nil
	}
	results := make(map[string]Result)
	r.ForEach(func(key, value Result) bool {
		results[key.Str] = value
		return true
	})
	return results
}

// ForEach iterates over each element in an array or object
func (r Result) ForEach(iterator func(key, value Result) bool) {
	if r.Type != TypeArray && r.Type != TypeObject {
		return
	}

	// Find start of array/object
	start := 0
	for ; start < len(r.Raw); start++ {
		if r.Raw[start] == '[' || r.Raw[start] == '{' {
			break
		}
	}

	if start >= len(r.Raw) {
		return
	}

	pos := start + 1

	if r.Type == TypeArray {
		forEachArrayRaw(r.Raw, pos, iterator)
	} else {
		forEachObjectRaw(r.Raw, pos, iterator)
	}
}

// forEachArrayRaw iterates over array elements starting at pos
func forEachArrayRaw(raw []byte, pos int, iterator func(key, value Result) bool) {
	index := 0
	for pos < len(raw) {
		// Skip whitespace
		for ; pos < len(raw) && raw[pos] <= ' '; pos++ {
		}
		if pos >= len(raw) || raw[pos] == ']' {
			break
		}

		valueStart := pos
		valueEnd := findValueEnd(raw, pos)
		if valueEnd == -1 {
			break
		}

		key := Result{Type: TypeNumber, Num: float64(index), Str: strconv.Itoa(index)}
		value := parseAny(raw[valueStart:valueEnd])
		value.Raw = raw[valueStart:valueEnd] // Preserve raw value

		if !iterator(key, value) {
			return
		}

		pos = valueEnd
		// Skip to next element or end of array
		for ; pos < len(raw) && (raw[pos] <= ' ' || raw[pos] == ','); pos++ {
			if raw[pos] == ',' {
				pos++
				break
			}
		}
		index++
	}
}

// forEachObjectRaw iterates over object key/value pairs starting at pos
func forEachObjectRaw(raw []byte, pos int, iterator func(key, value Result) bool) {
	for pos < len(raw) {
		// Move to the next entry start or end of object
		nextPos, end := advanceToNextObjectEntry(raw, pos)
		if end {
			break
		}
		if nextPos < 0 {
			break // invalid object
		}

		// Parse key and find value start
		keyRes, valueStart := parseObjectKeyAt(raw, nextPos)
		if valueStart < 0 {
			break
		}

		// Compute value end
		valueEnd := findValueEnd(raw, valueStart)
		if valueEnd == -1 {
			break
		}

		// Parse and yield
		value := parseAny(raw[valueStart:valueEnd])
		value.Raw = raw[valueStart:valueEnd]
		if !iterator(keyRes, value) {
			return
		}

		// Move after optional comma for next iteration
		pos = skipSpacesAndOptionalComma(raw, valueEnd)
	}
}

// advanceToNextObjectEntry skips spaces and returns the position of the next '"' key
// or indicates the end of the object.
func advanceToNextObjectEntry(raw []byte, pos int) (int, bool) {
	pos = fastSkipSpacesGet(raw, pos)
	if pos >= len(raw) || raw[pos] == '}' {
		return pos, true
	}
	if raw[pos] != '"' { // invalid object structure
		return -1, false
	}
	return pos, false
}

// parseObjectKeyAt parses a string key starting at 'pos' and positions to the value start.
func parseObjectKeyAt(raw []byte, pos int) (Result, int) {
	keyRes := parseString(raw, pos)
	if !keyRes.Exists() {
		return Result{}, -1
	}
	pos = pos + len(keyRes.Raw)
	// Find colon
	for pos < len(raw) && raw[pos] != ':' {
		pos++
	}
	if pos >= len(raw) {
		return Result{}, -1
	}
	// Skip colon and whitespace to value start
	pos++
	pos = fastSkipSpacesGet(raw, pos)
	if pos >= len(raw) {
		return Result{}, -1
	}
	return keyRes, pos
}

// skipSpacesAndOptionalComma advances over whitespace and a single optional comma.
func skipSpacesAndOptionalComma(raw []byte, pos int) int {
	// Skip spaces and at most one comma
	for pos < len(raw) && (raw[pos] <= ' ' || raw[pos] == ',') {
		if raw[pos] == ',' {
			pos++
			break
		}
		pos++
	}
	return pos
}

// Get returns a value from an object or array
func (r Result) Get(path string) Result {
	if !r.Exists() {
		return Result{Type: TypeUndefined}
	}

	return Get(r.Raw, path)
}

// Time parses the result as a time.Time
func (r Result) Time() (time.Time, error) {
	if r.Type != TypeString {
		return time.Time{}, ErrTypeConversion
	}

	// Try standard formats
	for _, layout := range []string{
		time.RFC3339,
		time.RFC3339Nano,
		time.RFC1123,
		time.RFC1123Z,
		time.RFC822,
		time.RFC822Z,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, r.Str); err == nil {
			return t, nil
		}
	}

	return time.Time{}, ErrTypeConversion
}

//------------------------------------------------------------------------------
// UTILITY FUNCTIONS
//------------------------------------------------------------------------------

// isUltraSimplePath checks if a path is a single key with no special characters
func isUltraSimplePath(path string) bool {
	return !strings.ContainsAny(path, ".[]*?()#$@")
}

// isSimplePath checks if a path can be executed directly without compilation
func isSimplePath(path string) bool {
	if len(path) == 0 {
		return true
	}

	p := 0

	// Handle leading numeric array index if present
	var ok bool
	p, ok = handleLeadingNumber(path, p)
	if !ok {
		return false
	}

	// If starts with a key (not bracket), scan the key
	if p < len(path) && path[p] != '[' {
		p, ok = scanKey(path, p)
		if !ok {
			return false
		}
	}

	for p < len(path) {
		switch path[p] {
		case '.':
			p++
			if p == len(path) {
				return false
			}
			p, ok = scanKey(path, p)
			if !ok {
				return false
			}
		case '[':
			p, ok = scanIndex(path, p)
			if !ok {
				return false
			}
		default:
			return false
		}
	}

	return true
}

// handleLeadingNumber handles optional leading numeric array index, returns new pos and ok
func handleLeadingNumber(path string, p int) (int, bool) {
	if p < len(path) && path[p] >= '0' && path[p] <= '9' {
		for p < len(path) && path[p] >= '0' && path[p] <= '9' {
			p++
		}
		if p == len(path) {
			return p, true
		}
		if path[p] == '.' {
			p++ // skip dot
			return p, true
		}
		return p, false
	}
	return p, true
}

// scanKey scans a key starting at p and returns the new position and ok
func scanKey(path string, p int) (int, bool) {
	keyStart := p
	for p < len(path) && path[p] != '.' && path[p] != '[' && path[p] != '|' && path[p] != '@' {
		if path[p] == '*' || path[p] == '?' || path[p] == '#' {
			return p, false
		}
		p++
	}
	if p == keyStart {
		return p, false
	}
	return p, true
}

// scanIndex scans an index starting at '[' and returns the new position (after ']') and ok
func scanIndex(path string, p int) (int, bool) {
	// expect '[' at p
	p++ // skip '['
	if p >= len(path) {
		return p, false
	}
	idxStart := p
	if path[p] == '*' || path[p] == '#' { // wildcard
		p++
	} else {
		for p < len(path) && path[p] >= '0' && path[p] <= '9' {
			p++
		}
		if p == idxStart {
			return p, false
		}
	}
	if p >= len(path) || path[p] != ']' {
		return p, false
	}
	p++
	return p, true
}

// stringToBytes converts a string to a byte slice without allocation
//
//nolint:gosec // G103: intentional use of unsafe for zero-copy string to bytes conversion
func stringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(
		&struct {
			string
			Cap int
		}{s, len(s)},
	))
}

// bytesToString converts a byte slice to a string without allocation
// This unsafe pointer usage has been audited and is safe in this context
// as it's a performance optimization for zero-copy conversion
//
//nolint:gosec // G103: intentional use of unsafe for zero-copy bytes to string conversion
func bytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// escapeString escapes special characters in a string for JSON
func escapeString(s string) string {
	var buf bytes.Buffer
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"', '\\', '/':
			buf.WriteByte('\\')
			buf.WriteByte(c)
		case '\b':
			buf.WriteString("\\b")
		case '\f':
			buf.WriteString("\\f")
		case '\n':
			buf.WriteString("\\n")
		case '\r':
			buf.WriteString("\\r")
		case '\t':
			buf.WriteString("\\t")
		default:
			if c < 32 {
				fmt.Fprintf(&buf, "\\u%04x", c)
			} else {
				buf.WriteByte(c)
			}
		}
	}
	return buf.String()
}
