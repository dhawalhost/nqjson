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
	constNull = "null"
	// constFalse   = "false"
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

	// Custom modifier registry (thread-safe)
	customModifiers   = make(map[string]ModifierFunc)
	customModifiersMu sync.RWMutex
)

// ModifierFunc is the signature for custom modifier functions.
// It receives the current Result and an optional argument string,
// and returns a transformed Result.
type ModifierFunc func(result Result, arg string) Result

// RegisterModifier registers a custom modifier with the given name.
// The modifier can then be used in queries with @name or @name:arg syntax.
// Thread-safe for concurrent use.
//
// Example:
//
//	nqjson.RegisterModifier("double", func(r nqjson.Result, arg string) nqjson.Result {
//	    if r.Type == nqjson.TypeNumber {
//	        return nqjson.Result{Type: nqjson.TypeNumber, Num: r.Num * 2, Modified: true}
//	    }
//	    return r
//	})
//	result := nqjson.Get(json, "price|@double")
func RegisterModifier(name string, fn ModifierFunc) {
	customModifiersMu.Lock()
	defer customModifiersMu.Unlock()
	customModifiers[name] = fn
}

// UnregisterModifier removes a custom modifier by name.
// Returns true if the modifier was found and removed.
func UnregisterModifier(name string) bool {
	customModifiersMu.Lock()
	defer customModifiersMu.Unlock()
	if _, exists := customModifiers[name]; exists {
		delete(customModifiers, name)
		return true
	}
	return false
}

// ListModifiers returns a list of all registered modifier names,
// including both built-in and custom modifiers.
func ListModifiers() []string {
	builtIn := []string{
		"reverse", "keys", "values", "flatten", "first", "last", "join", "sort",
		"distinct", "unique", "length", "count", "len", "type", "string", "str",
		"number", "num", "bool", "boolean", "base64", "base64decode", "lower", "upper",
		"this", "valid", "pretty", "ugly", "sum", "avg", "average", "mean", "min", "max",
		"group", "groupby", "sortby", "map", "project", "uniqueby", "slice", "has",
		"contains", "split", "startswith", "endswith", "entries", "toentries",
		"fromentries", "any", "all",
	}

	customModifiersMu.RLock()
	defer customModifiersMu.RUnlock()

	for name := range customModifiers {
		builtIn = append(builtIn, name)
	}
	return builtIn
}

// getCustomModifier returns a custom modifier function by name, or nil if not found.
func getCustomModifier(name string) ModifierFunc {
	customModifiersMu.RLock()
	defer customModifiersMu.RUnlock()
	return customModifiers[name]
}

//------------------------------------------------------------------------------
// CORE GET IMPLEMENTATION
//------------------------------------------------------------------------------

type getOptions struct {
	allowMultipath bool
	allowJSONLines bool
}

// Compiled path structure for cached execution
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

// GetCached - Optimized version that caches compiled paths
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

// GetPath represents a pre-compiled path for fast repeated GET operations.
// Use CompileGetPath to create a GetPath, then call Run() to execute.
// This avoids path parsing overhead when executing the same query multiple times.
type GetPath struct {
	compiled *compiledPath
}

// CompileGetPath compiles a path expression for fast repeated execution.
// Returns a GetPath that can be reused across multiple Get calls.
// This is faster than GetCached for hot paths as it avoids sync.Map overhead.
//
// Example:
//
//	path, err := nqjson.CompileGetPath("users.0.name")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	result := path.Run(jsonData)
func CompileGetPath(path string) (*GetPath, error) {
	if path == "" {
		return nil, ErrInvalidQuery
	}

	cp := compilePath(path)
	return &GetPath{compiled: cp}, nil
}

// Run executes the compiled path against the provided JSON data.
// This is optimized for repeated execution with zero sync overhead.
func (p *GetPath) Run(data []byte) Result {
	if p == nil || p.compiled == nil {
		return Result{Type: TypeUndefined}
	}

	result := executeCompiledPath(data, p.compiled)
	if result.Exists() {
		return result
	}

	// Fallback to full Get for complex paths that compiled execution can't handle
	return Get(data, p.compiled.original)
}

// String returns the original path string.
func (p *GetPath) String() string {
	if p == nil || p.compiled == nil {
		return ""
	}
	return p.compiled.original
}

// unescapePathGet unescapes special characters in a path segment for GET operations
// Supports: \\ . : | @ * ? # , ( ) = ! < > ~
func unescapePathGet(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}

	var result strings.Builder
	result.Grow(len(s))

	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			next := s[i+1]
			switch next {
			case '.', ':', '\\', '|', '@', '*', '?', '#', ',', '(', ')', '=', '!', '<', '>', '~':
				result.WriteByte(next)
				i++
				continue
			}
		}
		result.WriteByte(s[i])
	}

	return result.String()
}

// hasColonPrefixGet checks if a path segment starts with : to force object key interpretation
func hasColonPrefixGet(s string) bool {
	return len(s) > 1 && s[0] == ':'
}

// stripColonPrefixGet removes the leading : from a path segment
func stripColonPrefixGet(s string) string {
	if hasColonPrefixGet(s) {
		return s[1:]
	}
	return s
}

// splitPathGet splits a path by dots while respecting escape sequences for GET operations
func splitPathGet(path string) []string {
	if !strings.Contains(path, "\\") {
		return strings.Split(path, ".")
	}

	var parts []string
	var current strings.Builder

	for i := 0; i < len(path); i++ {
		if path[i] == '\\' && i+1 < len(path) {
			current.WriteByte(path[i])
			i++
			if i < len(path) {
				current.WriteByte(path[i])
			}
		} else if path[i] == '.' {
			parts = append(parts, current.String())
			current.Reset()
		} else {
			current.WriteByte(path[i])
		}
	}

	if current.Len() > 0 || len(parts) > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// compilePath - Parse and compile a path for fast repeated execution
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

// parsePathSegments - Break path into executable segments
//
//nolint:gocyclo
func parsePathSegments(path string) []pathSegment {
	if path == "" {
		return nil
	}

	var segments []pathSegment

	// Split by dots, respecting escape sequences
	parts := splitPathGet(path)

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Unescape the part
		unescaped := unescapePathGet(part)

		// Check for colon prefix
		forceObjectKey := hasColonPrefixGet(unescaped)
		if forceObjectKey {
			unescaped = stripColonPrefixGet(unescaped)
		}

		// Handle bracket notation
		if strings.Contains(unescaped, "[") {
			// Parse base and brackets
			bracketIdx := strings.Index(unescaped, "[")
			if bracketIdx > 0 {
				// Has a base key before bracket
				segments = append(segments, pathSegment{key: unescaped[:bracketIdx], index: -1, isArray: false})
			}

			// Parse bracket indices
			remaining := unescaped[bracketIdx:]
			for len(remaining) > 0 && remaining[0] == '[' {
				closeIdx := strings.Index(remaining, "]")
				if closeIdx == -1 {
					break
				}

				idxStr := remaining[1:closeIdx]
				if idx, err := strconv.Atoi(idxStr); err == nil {
					segments = append(segments, pathSegment{key: "", index: idx, isArray: true})
				}

				remaining = remaining[closeIdx+1:]
			}
			continue
		}

		// Check if it's a numeric key (array access via dot notation)
		// But NOT if colon prefix was used
		if !forceObjectKey && isAllDigitsGet(unescaped) {
			idx, _ := strconv.Atoi(unescaped)
			segments = append(segments, pathSegment{key: "", index: idx, isArray: true})
		} else {
			segments = append(segments, pathSegment{key: unescaped, index: -1, isArray: false})
		}
	}

	return segments
}

// isAllDigitsGet checks if a string contains only digits
func isAllDigitsGet(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// executeCompiledPath - Fast execution of pre-parsed path
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

// getWithOptions is the core Get implementation with options for multipath and JSON Lines support.
//
// getWithOptions allows internal customization of behavior
//
//nolint:gocyclo
func getWithOptions(data []byte, path string, opts getOptions) Result {
	// Empty path should return non-existent result according to tests
	if path == "" {
		return Result{Type: TypeUndefined}
	}

	// This avoids multipath detection overhead for the most common case
	if shouldHandleMultipath(path, opts) {
		// Multipath detection (only when enabled and path contains comma/pipe)
		if multi, handled := getMultiPathResult(data, path, opts); handled {
			return multi
		}
	}

	return getSinglePathResult(data, path, opts)
}

func shouldHandleMultipath(path string, opts getOptions) bool {
	return opts.allowMultipath && strings.ContainsAny(path, ",|")
}

func getSinglePathResult(data []byte, path string, opts getOptions) Result {
	// JSON Lines support: treat leading ".." prefix as newline-delimited documents when applicable.
	if opts.allowJSONLines && len(path) >= 2 && path[0] == '.' && path[1] == '.' {
		if jsonLinesResult, handled := getJSONLinesResult(data, path); handled {
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

// splitMultiPath splits a path string into multiple segments based on commas, pipes, and whitespace.
//
//go:inline
func splitMultiPath(path string) []string {
	// Fast detection for single-path (no split needed)
	if !checkForMultiPathSplit(path) {
		return []string{path}
	}
	return performMultiPathSplit(path)
}

// checkForMultiPathSplit checks if the path needs to be split
func checkForMultiPathSplit(path string) bool {
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
				return true
			}
		}
	}
	return false
}

// performMultiPathSplit actually splits the path
// performMultiPathSplit actually splits the path
func performMultiPathSplit(path string) []string {
	segments := make([]string, 0, 4)
	start := 0
	depth := 0
	inString := false
	var stringQuote byte
	escaped := false

	for i := 0; i < len(path); i++ {
		c := path[i]

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' {
			escaped = true
			continue
		}

		if inString {
			if c == stringQuote {
				inString = false
			}
			continue
		}

		if c == '\'' || c == '"' {
			inString = true
			stringQuote = c
			continue
		}

		if c == ',' && depth == 0 {
			// Found split point
			segment := strings.TrimSpace(path[start:i])
			if len(segment) > 0 {
				segments = append(segments, segment)
			}
			start = i + 1
			continue
		}

		depth = updateGenericDepth(c, depth)
	}

	// Add final segment
	if start < len(path) {
		segment := strings.TrimSpace(path[start:])
		if len(segment) > 0 {
			segments = append(segments, segment)
		}
	}

	return segments
}

// updateGenericDepth updates depth based on brackets, braces, and parentheses
func updateGenericDepth(c byte, depth int) int {
	switch c {
	case '[', '{', '(':
		return depth + 1
	case ']', '}', ')':
		if depth > 0 {
			return depth - 1
		}
	}
	return depth
}

func buildNullResult() Result {
	return Result{Type: TypeNull, Raw: []byte("null"), Modified: true}
}

func getJSONLinesResult(data []byte, path string) (Result, bool) {
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
//
// getUltraSimplePath is an ultra-fast path for very simple JSON with basic paths
// This handles cases like {"name":"John","age":30} with path "name"
//
//nolint:gocyclo
//go:inline
func getUltraSimplePath(data []byte, path string) Result {
	// Target: 20-30ns for single-key lookups
	keyLen := len(path)
	if keyLen == 0 || len(data) < keyLen+6 {
		return Result{Type: TypeUndefined}
	}

	// Skip leading whitespace and find opening brace
	i := locateObjectStart(data)
	if i == -1 {
		return Result{Type: TypeUndefined}
	}

	// Scan through object keys
	for i < len(data) {
		i = skipWhitespaceInline(data, i)

		if i >= len(data) || data[i] == '}' {
			return Result{Type: TypeUndefined}
		}

		if data[i] != '"' {
			return Result{Type: TypeUndefined}
		}

		// Check if key matches
		match, keyEnd := compareSimpleKey(data, i+1, path)
		if match {
			// Found the key! Skip to value
			i = keyEnd + 1 // After closing quote

			// Skip colon
			i = skipToSimpleValue(data, i)
			if i == -1 {
				return Result{Type: TypeUndefined}
			}

			// Inline value parsing
			return parseValueAtPosition(data, i)
		}

		// Skip this key-value pair and move to next
		i = skipSimpleKeyValuePair(data, i)
		if i == -1 {
			return Result{Type: TypeUndefined}
		}
	}

	return Result{Type: TypeUndefined}
}

// locateObjectStart skips whitespace and checks for '{'
func locateObjectStart(data []byte) int {
	i := skipWhitespaceInline(data, 0)
	if i >= len(data) || data[i] != '{' {
		return -1
	}
	return i + 1
}

// compareSimpleKey compares key at pos with path. Returns match and keyEnd position.
func compareSimpleKey(data []byte, keyStart int, path string) (bool, int) {
	keyLen := len(path)

	// Fast key comparison without allocation
	if keyStart+keyLen+1 < len(data) && data[keyStart+keyLen] == '"' {
		match := true
		for j := 0; j < keyLen; j++ {
			if data[keyStart+j] != path[j] {
				match = false
				break
			}
		}
		if match {
			return true, keyStart + keyLen
		}
	}
	return false, 0
}

// skipToSimpleValue skips whitespace and colon to find value start
func skipToSimpleValue(data []byte, i int) int {
	i = skipWhitespaceInline(data, i)
	if i >= len(data) || data[i] != ':' {
		return -1
	}
	i++ // Skip ':'
	i = skipWhitespaceInline(data, i)
	if i >= len(data) {
		return -1
	}
	return i
}

// skipSimpleKeyValuePair skips key, colon, value, and optional comma
func skipSimpleKeyValuePair(data []byte, i int) int {
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
	i = skipToSimpleValue(data, i)
	if i == -1 {
		return -1
	}

	// Skip value
	i = skipValue(data, i)

	// Skip comma
	i = skipWhitespaceInline(data, i)
	if i < len(data) && data[i] == ',' {
		i++
	}
	return i
}

/*
// DEAD CODE - NEVER CALLED: findKeyInqJSON searches for a key in JSON data and returns its index
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
*/

/*
// DEAD CODE - REPLACED BY FAST VARIANTS: parseStringValue parses a JSON string value
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

// DEAD CODE - REPLACED BY FAST VARIANTS: parseTrueValue parses a JSON true value
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

// DEAD CODE - REPLACED BY FAST VARIANTS: parseFalseValue parses a JSON false value
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

// DEAD CODE - REPLACED BY FAST VARIANTS: parseNullValue parses a JSON null value
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

// DEAD CODE - REPLACED BY FAST VARIANTS: parseObjectValue parses a JSON object value
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

// DEAD CODE - REPLACED BY FAST VARIANTS: parseArrayValue parses a JSON array value
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
*/

// getSimplePath handles simple dot notation and basic array access
// This is optimized for paths like "user.name" or "items[0].id" or "items.0.id"
// Recursive one-pass path processing
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
	switch data[start] {
	case '{':
		return parseObjectRecursive(data, start+1, path)
	case '[':
		return parseArrayRecursive(data, start+1, path)
	}

	return Result{Type: TypeUndefined}
}

// Recursive object parser - processes path segments on the fly
//
// parsePathSegmentForObject parses the first segment of a path and returns segment, remaining path, and force object key flag
func parsePathSegmentForObject(path string) (segment string, remainingPath string, forceObjectKey bool) {
	segEnd := 0
	for segEnd < len(path) {
		if path[segEnd] == '\\' && segEnd+1 < len(path) {
			segEnd += 2
			continue
		}
		if path[segEnd] == '.' || path[segEnd] == '[' {
			break
		}
		segEnd++
	}
	if segEnd == 0 {
		return "", "", false
	}

	rawSegment := path[:segEnd]
	if segEnd < len(path) {
		if path[segEnd] == '.' {
			remainingPath = path[segEnd+1:]
		} else {
			remainingPath = path[segEnd:]
		}
	}

	segment = unescapePathGet(rawSegment)
	forceObjectKey = hasColonPrefixGet(segment)
	if forceObjectKey {
		segment = stripColonPrefixGet(segment)
	}
	return segment, remainingPath, forceObjectKey
}

// scanKeyInObject scans an object key starting after the opening quote
// Returns keyStart, keyEnd, newPos
func scanKeyInObject(data []byte, pos int) (keyStart, keyEnd, newPos int) {
	keyStart = pos

	// ULTRA-FAST KEY SCAN: Process 8 bytes at once
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

	return keyStart, pos, pos + 1
}

// matchKeyBytes checks if key in data matches segment
func matchKeyBytes(data []byte, keyStart, keyEnd int, segment string) bool {
	segLen := len(segment)
	if keyEnd-keyStart != segLen {
		return false
	}
	for i := 0; i < segLen; i++ {
		if data[keyStart+i] != segment[i] {
			return false
		}
	}
	return true
}

// skipToObjectValue skips whitespace and colon to get to value position
// Returns new position or -1 on error
func skipToObjectValue(data []byte, pos int) int {
	for pos < len(data) && data[pos] <= ' ' {
		pos++
	}
	if pos >= len(data) || data[pos] != ':' {
		return -1
	}
	pos++
	for pos < len(data) && data[pos] <= ' ' {
		pos++
	}
	if pos >= len(data) {
		return -1
	}
	return pos
}

// parseObjectRecursive parses a JSON object recursively to find a key
//
//go:inline
func parseObjectRecursive(data []byte, pos int, path string) Result {
	segment, remainingPath, _ := parsePathSegmentForObject(path)
	if segment == "" {
		return Result{Type: TypeUndefined}
	}

	hasRemainingPath := len(remainingPath) > 0

	for pos < len(data) {
		pos = skipWhitespaceInline(data, pos)
		if pos >= len(data) || data[pos] == '}' {
			break
		}

		if data[pos] != '"' {
			return Result{Type: TypeUndefined}
		}
		pos++

		keyStart, keyEnd, newPos := scanKeyInObject(data, pos)
		keyMatches := matchKeyBytes(data, keyStart, keyEnd, segment)
		pos = newPos

		pos = skipToObjectValue(data, pos)
		if pos == -1 {
			return Result{Type: TypeUndefined}
		}

		if keyMatches {
			if !hasRemainingPath {
				return parseValueAtPosition(data, pos)
			}
			c := data[pos]
			if c == '{' {
				return parseObjectRecursive(data, pos+1, remainingPath)
			}
			if c == '[' {
				return parseArrayRecursive(data, pos+1, remainingPath)
			}
			return Result{Type: TypeUndefined}
		}

		pos = vectorizedSkipValue(data, pos, len(data))
		if pos == -1 {
			return Result{Type: TypeUndefined}
		}

		pos = skipWhitespaceInline(data, pos)
		if pos < len(data) && data[pos] == ',' {
			pos++
		}
	}

	return Result{Type: TypeUndefined}
}

// skipWhitespaceInline efficiently skips whitespace
func skipWhitespaceInline(data []byte, pos int) int {
	for pos < len(data) && data[pos] <= ' ' {
		pos++
	}
	return pos
}

// Recursive array parser
//
// recursive array parser
//
//nolint:gocyclo
//go:inline
//go:inline
func parseArrayRecursive(data []byte, pos int, path string) Result {
	idx, remainingPath, ok := parseArrayIndexFromPath(path)
	if !ok {
		return Result{Type: TypeUndefined}
	}

	// Skip to target index
	pos = skipToArrayIndex(data, pos, idx)
	if pos == -1 {
		return Result{Type: TypeUndefined}
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

	// Recursive step
	if data[pos] == '{' {
		return parseObjectRecursive(data, pos+1, remainingPath)
	}
	if data[pos] == '[' {
		return parseArrayRecursive(data, pos+1, remainingPath)
	}

	return Result{Type: TypeUndefined}
}

// parseArrayIndexFromPath extracts the numeric index from the start of the path
func parseArrayIndexFromPath(path string) (int, string, bool) {
	// Check if path starts with array index
	if len(path) == 0 || (path[0] < '0' || path[0] > '9') {
		return 0, "", false
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
		switch path[i] {
		case '.':
			remainingPath = path[i+1:]
		case '[':
			remainingPath = path[i:]
		}
	}
	return idx, remainingPath, true
}

// skipToArrayIndex skips array elements until reaching targetIdx
func skipToArrayIndex(data []byte, pos, targetIdx int) int {
	currentIdx := 0
	for pos < len(data) && currentIdx < targetIdx {
		// Skip whitespace
		for pos < len(data) && data[pos] <= ' ' {
			pos++
		}
		if pos >= len(data) || data[pos] == ']' {
			return -1
		}

		// Skip value
		pos = vectorizedSkipValue(data, pos, len(data))
		if pos == -1 {
			return -1
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
	return pos
}

// Parse value at current position
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

// Vectorized value skipper with 8-byte scanning optimization
//
// vectorizedSkipValue skips a JSON value using fast byte scanning
//
//go:inline
func vectorizedSkipValue(data []byte, pos, end int) int {
	if pos >= end {
		return -1
	}

	c := data[pos]
	switch c {
	case '"':
		return vectorizedSkipString(data, pos, end)
	case '{':
		return vectorizedSkipObjectContent(data, pos+1, end)
	case '[':
		return vectorizedSkipArrayContent(data, pos+1, end)
	default:
		return skipPrimitiveToken(data, pos, end)
	}
}

// vectorizedSkipString skips a JSON string using 8-byte vectorized scanning
func vectorizedSkipString(data []byte, pos, end int) int {
	pos++
	// ULTRA-FAST: Process 8 bytes at once - characters > '\\' are safe to skip
	for pos+7 < end {
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
}

// vectorizedSkipObjectContent skips object content (after opening brace)
func vectorizedSkipObjectContent(data []byte, pos, end int) int {
	depth := 1
	for pos < end && depth > 0 {
		c := data[pos]
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
		} else if c == '"' {
			pos = skipStringInContainer(data, pos+1, end)
			continue
		}
		pos++
	}
	return pos
}

// vectorizedSkipArrayContent skips array content (after opening bracket)
func vectorizedSkipArrayContent(data []byte, pos, end int) int {
	depth := 1
	for pos < end && depth > 0 {
		c := data[pos]
		if c == '[' {
			depth++
		} else if c == ']' {
			depth--
		} else if c == '"' {
			pos = skipStringInContainer(data, pos+1, end)
			continue
		}
		pos++
	}
	return pos
}

// skipStringInContainer skips a string within an object/array (starts after opening quote)
func skipStringInContainer(data []byte, pos, end int) int {
	for pos < end {
		if data[pos] == '\\' {
			pos++
		} else if data[pos] == '"' {
			return pos + 1
		}
		pos++
	}
	return pos
}

// skipPrimitiveToken skips a primitive value (number, true, false, null)
func skipPrimitiveToken(data []byte, pos, end int) int {
	for pos < end {
		c := data[pos]
		if c <= ' ' || c == ',' || c == ']' || c == '}' {
			return pos
		}
		pos++
	}
	return pos
}

/*
// DEAD CODE - NEVER CALLED: handleGetDirectArrayIndex handles paths that start with a numeric array index
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
*/

/*
// DEAD CODE - NEVER CALLED: processGetPathSegment processes a single segment of the path
// This function has no callers in the entire codebase
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

// DEAD CODE - ONLY CALLED BY DEAD CODE: processGetKeyAccess handles object key access or numeric array index access
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

// DEAD CODE - ONLY CALLED BY DEAD CODE: processGetBracketAccess handles bracket notation array access like [0]
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

// DEAD CODE - ONLY CALLED BY DEAD CODE: isNumericKey checks if a string is entirely numeric
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
*/

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
// func isNumericKey(key string) bool {
// 	if len(key) == 0 {
// 		return false
// 	}
// 	for i := 0; i < len(key); i++ {
// 		if key[i] < '0' || key[i] > '9' {
// 			return false
// 		}
// 	}
// 	return true
// }

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
	tokenArrayLength // # for array length (when used alone)
	tokenQueryFirst  // #(condition) for first match
	tokenQueryAll    // #(condition)# for all matches
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
//
// parseModifiers extracts and parses modifier tokens from a path.
// Supports both legacy '|' and JSONPath-like '@' suffix modifiers.
// For '@', only treat as a modifier separator when not inside strings/brackets/parentheses
// and not immediately following a '.'. This preserves cases like "users.@length" as invalid
// path segments and avoids interference with filter "@.field" usage.
//
//nolint:gocyclo
//go:inline
func parseModifiers(path string) ([]pathToken, string, string) {
	// Fast path: if neither '|' nor '@' present, return early
	if !strings.ContainsAny(path, "|@") {
		return nil, path, ""
	}

	// Scan for the first valid modifier separator position
	sepIdx := findModifierSeparator(path)
	if sepIdx < 0 {
		return nil, path, ""
	}

	// Extract suffix with modifiers and the main path
	suffix := path[sepIdx+1:]
	cleanPath := path[:sepIdx]

	// Parse modifiers and remaining path from suffix
	parts := splitModifierParts(suffix)
	modifiers, remainingPath := parseModifierParts(parts)

	return modifiers, cleanPath, remainingPath
}

// findModifierSeparator finds the index of the first valid modifier separator
func findModifierSeparator(path string) int {
	bracketDepth := 0
	parenDepth := 0
	inString := false
	var stringQuote byte
	escaped := false

	for i := 0; i < len(path); i++ {
		c := path[i]

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' {
			escaped = true
			continue
		}

		if inString {
			if c == stringQuote {
				inString = false
			}
			continue
		}

		if c == '\'' || c == '"' {
			inString = true
			stringQuote = c
			continue
		}

		if isModifierSeparator(path, i, bracketDepth, parenDepth) {
			return i
		}

		bracketDepth, parenDepth = updatePathDepths(c, bracketDepth, parenDepth)
	}
	return -1
}

// isModifierSeparator checks if the current character is a valid modifier separator
func isModifierSeparator(path string, i int, bracketDepth, parenDepth int) bool {
	c := path[i]
	if bracketDepth > 0 || parenDepth > 0 {
		return false
	}

	if c == '|' {
		return true
	}

	if c == '@' {
		// Treat @ as modifier start if at beginning of path or not immediately after a dot
		if i == 0 || (i > 0 && path[i-1] != '.') {
			return true
		}
	}
	return false
}

// parseModifierParts parses split parts into modifiers and remaining path
func parseModifierParts(parts []string) ([]pathToken, string) {
	var modifiers []pathToken
	var remainingPath string

	for i, part := range parts {
		if part == "" {
			continue
		}
		// Check if this looks like a modifier (starts with letter, known modifier names)
		if isModifierName(part) {
			modParts := strings.SplitN(part, ":", 2)
			mod := pathToken{kind: tokenModifier, str: modParts[0]}
			if len(modParts) > 1 {
				mod.str = modParts[0] + ":" + modParts[1] // Keep the full modifier
			}
			modifiers = append(modifiers, mod)
		} else {
			// This is a path to apply after modifiers
			// Join remaining parts as the path
			remainingPath = strings.Join(parts[i:], "|")
			break
		}
	}
	return modifiers, remainingPath
}

// splitModifierParts splits a string by | and @ at depth 0
func splitModifierParts(s string) []string {
	var parts []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '|' || c == '@' {
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		} else {
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

// isEscapedAt reports whether the character at position pos is escaped by a preceding backslash.
func isEscapedAt(s string, pos int) bool {
	if pos <= 0 || pos >= len(s) {
		return false
	}
	count := 0
	for i := pos - 1; i >= 0 && s[i] == '\\'; i-- {
		count++
	}
	return count%2 == 1
}

// hasUnescapedChar reports if the target character appears unescaped in the string.
func hasUnescapedChar(s string, target byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == target && !isEscapedAt(s, i) {
			return true
		}
	}
	return false
}

// isModifierName checks if a string is a known modifier name
func isModifierName(s string) bool {
	// Extract the base name before any ':'
	name := s
	if idx := strings.Index(s, ":"); idx >= 0 {
		name = s[:idx]
	}

	knownModifiers := map[string]bool{
		"reverse": true, "keys": true, "values": true, "flatten": true,
		"first": true, "last": true, "join": true, "sort": true,
		"distinct": true, "unique": true, "length": true, "count": true, "len": true,
		"type": true, "string": true, "str": true, "number": true, "num": true,
		"bool": true, "boolean": true, "base64": true, "base64decode": true,
		"lower": true, "upper": true, "this": true, "valid": true,
		"pretty": true, "ugly": true,
		// Aggregate modifiers
		"sum": true, "avg": true, "average": true, "mean": true, "min": true, "max": true,
		// Advanced transformation modifiers
		"group": true, "groupby": true, "sortby": true, "map": true, "project": true, "uniqueby": true,
		// Additional jq-style modifiers
		"slice": true, "has": true, "contains": true, "split": true,
		"startswith": true, "endswith": true, "entries": true, "toentries": true,
		"fromentries": true, "any": true, "all": true,
	}

	// Check built-in modifiers first
	if knownModifiers[name] {
		return true
	}

	// Check custom modifiers registry
	return getCustomModifier(name) != nil
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
//
//go:inline
func tokenizePath(path string) []pathToken {
	var tokens []pathToken

	// Check for modifiers (returns modifiers, clean path, and remaining path after modifiers)
	modifiers, cleanPath, remainingPath := parseModifiers(path)

	// Split the path on dots, but respect brackets and parentheses
	parts := splitPathSegments(cleanPath)

	// Convert parts to tokens
	tokens = convertPartsToTokens(parts)

	// Add modifiers as tokens
	tokens = appendModifiersToTokens(tokens, modifiers)

	// Recursively parse remaining path (for piped modifiers)
	if remainingPath != "" {
		remainingTokens := tokenizePath(remainingPath)
		tokens = append(tokens, remainingTokens...)
	}

	return tokens
}

func splitPathSegments(path string) []string {
	var parts []string
	var cur strings.Builder
	bracketDepth := 0
	parenDepth := 0
	inString := false
	var stringQuote byte
	escaped := false

	for i := 0; i < len(path); i++ {
		c := path[i]

		if escaped {
			cur.WriteByte(c)
			escaped = false
			continue
		}

		if c == '\\' {
			escaped = true
			cur.WriteByte(c)
			continue
		}

		if inString {
			cur.WriteByte(c)
			if c == stringQuote {
				inString = false
			}
			continue
		}

		if c == '\'' || c == '"' {
			inString = true
			stringQuote = c
			cur.WriteByte(c)
			continue
		}

		if shouldSplitAtDot(c, bracketDepth, parenDepth) {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}

		bracketDepth, parenDepth = updatePathDepths(c, bracketDepth, parenDepth)
		cur.WriteByte(c)
	}

	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

func updatePathDepths(c byte, bracketDepth, parenDepth int) (int, int) {
	switch c {
	case '[':
		return bracketDepth + 1, parenDepth
	case ']':
		if bracketDepth > 0 {
			return bracketDepth - 1, parenDepth
		}
	case '(':
		return bracketDepth, parenDepth + 1
	case ')':
		if parenDepth > 0 {
			return bracketDepth, parenDepth - 1
		}
	}
	return bracketDepth, parenDepth
}

// shouldSplitAtDot checks if we should split at the current character (dot)
func shouldSplitAtDot(c byte, bracketDepth, parenDepth int) bool {
	return c == '.' && bracketDepth == 0 && parenDepth == 0
}

// convertPartsToTokens converts string parts into pathTokens
func convertPartsToTokens(parts []string) []pathToken {
	var tokens []pathToken
	for _, part := range parts {
		if part == "" {
			continue
		}

		unescaped := unescapePathGet(part)
		if unescaped == "*" && part == "*" {
			tokens = append(tokens, pathToken{kind: tokenWildcard})
			continue
		}

		if unescaped == "#" && part == "#" {
			tokens = append(tokens, pathToken{kind: tokenArrayLength})
			continue
		}

		// Check for query syntax: #(condition) or #(condition)#
		if strings.HasPrefix(unescaped, "#(") {
			queryTokens := parseQueryExpression(unescaped)
			tokens = append(tokens, queryTokens...)
			continue
		}

		if unescaped == ".." {
			tokens = append(tokens, pathToken{kind: tokenRecursive})
			continue
		}

		// Check for bracket notation like [1] or ["key"] or key[index]
		if strings.Contains(unescaped, "[") && strings.HasSuffix(unescaped, "]") {
			arrayTokens := parseArrayAccess(unescaped)
			tokens = append(tokens, arrayTokens...)
		} else if idx, err := strconv.Atoi(unescaped); err == nil && idx >= 0 {
			// Pure numeric token - treat as array index
			tokens = append(tokens, pathToken{kind: tokenIndex, num: idx})
		} else {
			// Standard dot property
			tokens = append(tokens, pathToken{kind: tokenKey, str: unescaped})
		}
	}
	return tokens
}

// appendModifiersToTokens adds modifiers to the token list
func appendModifiersToTokens(tokens []pathToken, modifiers []pathToken) []pathToken {
	// The modifiers are already pathToken objects from parseModifiers
	// We just need to append them.
	return append(tokens, modifiers...)
}

// parseQueryExpression parses query expressions: #(condition) or #(condition)#
// Returns tokens for the query
func parseQueryExpression(part string) []pathToken {
	// Determine if this is first match (#(condition)) or all matches (#(condition)#)
	isAllMatches := strings.HasSuffix(part, ")#")

	// Extract the condition
	var condition string
	if isAllMatches {
		// #(condition)# - strip leading #( and trailing )#
		condition = part[2 : len(part)-2]
	} else {
		// #(condition) - strip leading #( and trailing )
		condition = part[2 : len(part)-1]
	}

	// Parse the condition into a filter expression
	filter := parseQueryCondition(condition)

	if isAllMatches {
		return []pathToken{{kind: tokenQueryAll, filter: filter}}
	}
	return []pathToken{{kind: tokenQueryFirst, filter: filter}}
}

// parseQueryCondition parses a query condition expression
func parseQueryCondition(condition string) *filterExpr {
	// For nested queries like "nets.#(==\"fb\")", the condition is the entire path
	// We need to find operators that are NOT inside parentheses

	// Find the operator at depth 0 (not inside nested queries)
	op, opIdx := findQueryOperator(condition)

	if opIdx != -1 {
		left := strings.TrimSpace(condition[:opIdx])
		value := strings.TrimSpace(condition[opIdx+len(op):])

		// Remove quotes from value if present
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		return &filterExpr{path: left, op: op, value: value}
	}

	// No operator found, assume it's just a path existence check or simple value
	// For now treating as existence check (value present)
	return &filterExpr{path: condition, op: ""}
}

// findQueryOperator finds the query operator in the condition string
func findQueryOperator(condition string) (string, int) {
	// Track parentheses depth
	depth := 0
	inString := false
	var stringChar byte

	possibleOps := []string{"==", "!=", ">=", "<=", ">", "<", "=~", "!~", "%", "!%"}

	for i := 0; i < len(condition); i++ {
		c := condition[i]

		// Handle strings
		if !inString && (c == '"' || c == '\'') {
			inString = true
			stringChar = c
			continue
		}
		if inString {
			if c == '\\' {
				i++ // skip escaped char
				continue
			}
			if c == stringChar {
				inString = false
			}
			continue
		}

		// Track parenthesis depth
		if c == '(' {
			depth++
			continue
		}
		if c == ')' {
			if depth > 0 {
				depth--
			}
			continue
		}

		// Check for operators at depth 0
		if depth == 0 {
			if op, found := checkQueryOperator(condition, i, c, possibleOps); found {
				return op, i
			}
		}
	}
	return "", -1
}

// checkQueryOperator checks if an operator starts at current position
func checkQueryOperator(condition string, i int, c byte, possibleOps []string) (string, bool) {
	// Check longest operators first to avoid partial matches
	// Optimization: only check if character matches start of an operator
	if c == '=' || c == '!' || c == '>' || c == '<' || c == '%' {
		for _, op := range possibleOps {
			if strings.HasPrefix(condition[i:], op) {
				return op, true
			}
		}
	}
	return "", false
}

// cleanQueryValue removes quotes from query values
func cleanQueryValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}
	return value
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

	// Split tokens into before/modifiers/after blocks
	before, modifiers, after := separateModifierTokens(tokens)
	hasModifiers := len(modifiers) > 0

	// Process tokens before modifiers
	for i, token := range before {
		result, shouldReturn := processPathToken(current, token, before, i, hasModifiers)
		if shouldReturn {
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
		if !current.Exists() {
			return Result{Type: TypeUndefined}
		}
	}

	// Process tokens after modifiers (if any)
	if len(after) > 0 {
		for i, token := range after {
			result, shouldReturn := processPathToken(current, token, after, i, false)
			if shouldReturn {
				current = result
				break
			}
			current = result
			if !current.Exists() {
				return Result{Type: TypeUndefined}
			}
		}
	}

	return current
}

// separateModifierTokens splits tokens into before modifiers, modifier tokens, and after modifiers.
func separateModifierTokens(tokens []pathToken) ([]pathToken, []pathToken, []pathToken) {
	var before []pathToken
	var modifiers []pathToken
	var after []pathToken

	foundModifier := false
	for _, token := range tokens {
		if token.kind == tokenModifier {
			foundModifier = true
			modifiers = append(modifiers, token)
			continue
		}
		if foundModifier {
			after = append(after, token)
		} else {
			before = append(before, token)
		}
	}

	return before, modifiers, after
}

// processPathToken processes a single path token and returns the result and whether to return early
func processPathToken(current Result, token pathToken, pathTokens []pathToken, i int, hasModifiers bool) (Result, bool) {
	switch token.kind {
	case tokenKey:
		// If current is an array due to wildcard/filter/query, project key over elements
		if current.Type == TypeArray && i > 0 {
			prev := pathTokens[i-1]
			if prev.kind == tokenWildcard || prev.kind == tokenFilter || prev.kind == tokenArrayLength ||
				prev.kind == tokenQueryAll {
				return processArrayProjection(current, pathTokens, i)
			}
		}
		return processKeyToken(current, token)
	case tokenIndex:
		return processIndexToken(current, token)
	case tokenWildcard:
		return processWildcardToken(current, pathTokens, i)
	case tokenArrayLength:
		return processArrayLengthToken(current, pathTokens, i, hasModifiers)
	case tokenFilter:
		return processFilterToken(current, token)
	case tokenQueryFirst:
		return processQueryFirstToken(current, token)
	case tokenQueryAll:
		return processQueryAllToken(current, token)
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

	key := token.str

	// Check if key contains pattern characters (* or ?)
	if strings.ContainsAny(key, "*?") {
		// Pattern matching on keys
		return processKeyPattern(current, key)
	}

	// Use direct object lookup instead of ForEach to avoid allocations
	start, end := fastFindObjectValue(current.Raw, key)
	if start == -1 {
		return Result{Type: TypeUndefined}, true
	}

	return fastParseValue(current.Raw[start:end]), false
}

// processKeyPattern handles pattern matching on object keys (e.g., child*, c?ildren)
func processKeyPattern(current Result, pattern string) (Result, bool) {
	var matchedValue Result
	found := false

	current.ForEach(func(key, value Result) bool {
		if matchPattern(key.Str, pattern) {
			matchedValue = value
			found = true
			return false // Stop after first match
		}
		return true
	})

	if !found {
		return Result{Type: TypeUndefined}, true
	}
	return matchedValue, false
}

// matchPattern matches a string against a glob pattern with * (any chars) and ? (single char)
func matchPattern(s, pattern string) bool {
	return matchPatternHelper(s, pattern, 0, 0)
}

func matchPatternHelper(s, pattern string, si, pi int) bool {
	for pi < len(pattern) {
		if pattern[pi] == '*' {
			// * matches zero or more characters
			// Try matching zero characters first, then more
			for si <= len(s) {
				if matchPatternHelper(s, pattern, si, pi+1) {
					return true
				}
				si++
			}
			return false
		} else if pattern[pi] == '?' {
			// ? matches exactly one character
			if si >= len(s) {
				return false
			}
			si++
			pi++
		} else {
			// Regular character - must match exactly
			if si >= len(s) || s[si] != pattern[pi] {
				return false
			}
			si++
			pi++
		}
	}

	// Pattern exhausted, string must also be exhausted
	return si == len(s)
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
func processWildcardToken(current Result, pathTokens []pathToken, i int) (Result, bool) {
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

// processArrayLengthToken handles # token for array length or array wildcard
// array.# returns the count, array.#.field returns all field values
// Also acts as wildcard when followed by modifiers (items.#@reverse)
func processArrayLengthToken(current Result, pathTokens []pathToken, i int, hasModifiers bool) (Result, bool) {
	// Check if this is truly the last token OR if there are more path tokens after #
	// Note: modifiers are separated out, so we also check hasModifiers
	isLast := i == len(pathTokens)-1

	// If this is the last path token AND there are no modifiers, return the count
	// If there are modifiers (like @reverse), we need to return all elements
	if isLast && !hasModifiers {
		switch current.Type {
		case TypeArray:
			count := fastCountArrayElements(current.Raw)
			return buildCountResult(count), true
		case TypeObject:
			count := len(current.Map())
			return buildCountResult(count), true
		default:
			return Result{Type: TypeUndefined}, true
		}
	}

	// If there are more tokens after # OR there are modifiers, act as wildcard (get all elements)
	// This handles cases like array.#.field and array.#@reverse
	if current.Type != TypeArray && current.Type != TypeObject {
		return Result{Type: TypeUndefined}, true
	}

	return processWildcardCollection(current, pathTokens, i)
}

// fastCountArrayElements counts array elements without parsing them
func fastCountArrayElements(data []byte) int {
	if len(data) < 2 || data[0] != '[' {
		return 0
	}

	// Skip opening bracket and whitespace
	pos := skipWhitespaceSimple(data, 1)

	// Check for empty array
	if pos < len(data) && data[pos] == ']' {
		return 0
	}

	count := 0
	depth := 0

	for pos < len(data) {
		c := data[pos]

		if c == '"' {
			if depth == 0 && count == 0 {
				count = 1
			}
			pos = skipQuotedString(data, pos)
			continue
		}

		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			pos++
			continue
		}

		// Update count and depth based on character
		count, depth, pos = processArrayCountChar(c, count, depth, pos)

		// If returned from top level array
		if depth < 0 {
			// Check if we just closed top level array
			// depth starts at 0 inside the array scanning loop because we skipped the opening [
			// Wait, the logic below handles depth relative to inside.
			// Original logic: initial depth 0 (inside array). ] at depth 0 returns count.
			// Let's match that.
			return count
		}
	}

	return count
}

// processArrayCountChar updates state based on character
func processArrayCountChar(c byte, count, depth, pos int) (int, int, int) {
	switch c {
	case '[', '{':
		if depth == 0 && count == 0 {
			count = 1
		}
		return count, depth + 1, pos + 1
	case ']':
		if depth == 0 {
			return count, -1, pos + 1 // Signal return
		}
		return count, depth - 1, pos + 1
	case '}':
		if depth > 0 {
			return count, depth - 1, pos + 1
		}
		return count, depth, pos + 1
	case ',':
		if depth == 0 {
			return count + 1, depth, pos + 1
		}
		return count, depth, pos + 1
	default:
		if depth == 0 && count == 0 {
			count = 1
		}
		return count, depth, pos + 1
	}
}

// skipQuotedString skips a quoted string starting at pos (including the opening quote)
func skipQuotedString(data []byte, pos int) int {
	pos++ // Skip opening quote
	for pos < len(data) {
		if data[pos] == '\\' {
			pos += 2
			continue
		}
		if data[pos] == '"' {
			return pos + 1
		}
		pos++
	}
	return pos
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

// processQueryFirstToken handles #(condition) - returns first matching element
func processQueryFirstToken(current Result, token pathToken) (Result, bool) {
	if current.Type != TypeArray {
		return Result{Type: TypeUndefined}, true
	}

	// Find first match
	var match Result
	found := false
	current.ForEach(func(_, value Result) bool {
		if matchesQueryCondition(value, token.filter) {
			match = value
			found = true
			return false // Stop after first match
		}
		return true
	})

	if !found {
		return Result{Type: TypeUndefined}, true
	}

	return match, false
}

// processQueryAllToken handles #(condition)# - returns all matching elements
func processQueryAllToken(current Result, token pathToken) (Result, bool) {
	if current.Type != TypeArray {
		return Result{Type: TypeUndefined}, true
	}

	// Find all matches
	var matches []Result
	current.ForEach(func(_, value Result) bool {
		if matchesQueryCondition(value, token.filter) {
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

// matchesQueryCondition checks if a value matches a query condition
func matchesQueryCondition(value Result, filter *filterExpr) bool {
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
	case "!=":
		return !compareEqual(filterValue, filter.value)
	case ">":
		return compareGreater(filterValue, filter.value)
	case "<":
		return compareLess(filterValue, filter.value)
	case ">=":
		return compareGreaterEqual(filterValue, filter.value)
	case "<=":
		return compareLessEqual(filterValue, filter.value)
	case "%":
		// Pattern matching
		return matchPattern(filterValue.String(), filter.value)
	case "!%":
		// Negative pattern matching
		return !matchPattern(filterValue.String(), filter.value)
	}

	return false
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

// compareGreater compares if a result is greater than a string value
func compareGreater(result Result, value string) bool {
	switch result.Type {
	case TypeString:
		return result.Str > value
	case TypeNumber:
		valueNum, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return false
		}
		return result.Num > valueNum
	default:
		return false
	}
}

// compareGreaterEqual compares if a result is greater than or equal to a string value
func compareGreaterEqual(result Result, value string) bool {
	switch result.Type {
	case TypeString:
		return result.Str >= value
	case TypeNumber:
		valueNum, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return false
		}
		return result.Num >= valueNum
	default:
		return false
	}
}

// compareLessEqual compares if a result is less than or equal to a string value
func compareLessEqual(result Result, value string) bool {
	switch result.Type {
	case TypeString:
		return result.Str <= value
	case TypeNumber:
		valueNum, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return false
		}
		return result.Num <= valueNum
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

	// Try each category of modifiers
	if r, ok := applyTypeConversionModifier(result, name); ok {
		return r
	}
	if r, ok := applyCollectionModifier(result, name, arg); ok {
		return r
	}
	if r, ok := applyAggregateModifier(result, name); ok {
		return r
	}
	if r, ok := applyFormattingModifier(result, name, arg); ok {
		return r
	}
	if r, ok := applyAdvancedModifier(result, name, arg); ok {
		return r
	}
	if r, ok := applyJQStyleModifier(result, name, arg); ok {
		return r
	}

	// Check for custom modifiers
	if customFn := getCustomModifier(name); customFn != nil {
		return customFn(result, arg)
	}

	return result
}

// applyTypeConversionModifier handles type conversion modifiers
func applyTypeConversionModifier(result Result, name string) (Result, bool) {
	switch name {
	case constString, "str":
		return applyStringModifier(result), true
	case constNumber, "num":
		return applyNumberModifier(result), true
	case constBool, constBoolean:
		return applyBooleanModifier(result), true
	case "type":
		return applyTypeModifier(result), true
	}
	return Result{}, false
}

// applyCollectionModifier handles collection/array modifiers
func applyCollectionModifier(result Result, name, arg string) (Result, bool) {
	switch name {
	case "keys":
		return applyKeysModifier(result), true
	case "values":
		return applyValuesModifier(result), true
	case "length", "count", "len":
		return applyLengthModifier(result), true
	case "reverse":
		return applyReverseModifier(result), true
	case "flatten":
		return applyFlattenModifier(result), true
	case "distinct", "unique":
		return applyDistinctModifier(result), true
	case "sort":
		return applySortModifier(result, arg), true
	case "first":
		return applyFirstModifier(result), true
	case "last":
		return applyLastModifier(result), true
	case "join":
		return applyJoinModifier(result, arg), true
	}
	return Result{}, false
}

// applyAggregateModifier handles aggregate modifiers
func applyAggregateModifier(result Result, name string) (Result, bool) {
	switch name {
	case "sum":
		return applySumModifier(result), true
	case "avg", "average", "mean":
		return applyAverageModifier(result), true
	case "min":
		return applyMinModifier(result), true
	case "max":
		return applyMaxModifier(result), true
	}
	return Result{}, false
}

// applyFormattingModifier handles formatting modifiers
func applyFormattingModifier(result Result, name, arg string) (Result, bool) {
	switch name {
	case "base64":
		return applyBase64Modifier(result), true
	case "base64decode":
		return applyBase64DecodeModifier(result), true
	case "lower":
		return applyLowerModifier(result), true
	case "upper":
		return applyUpperModifier(result), true
	case "this":
		return applyThisModifier(result), true
	case "valid":
		return applyValidModifier(result), true
	case "pretty":
		return applyPrettyModifier(result, arg), true
	case "ugly":
		return applyUglyModifier(result), true
	}
	return Result{}, false
}

// applyAdvancedModifier handles advanced transformation modifiers
func applyAdvancedModifier(result Result, name, arg string) (Result, bool) {
	switch name {
	case "group", "groupby":
		return applyGroupModifier(result, arg), true
	case "sortby":
		return applySortByModifier(result, arg), true
	case "map", "project":
		return applyMapModifier(result, arg), true
	case "uniqueby":
		return applyUniqueByModifier(result, arg), true
	}
	return Result{}, false
}

// applyJQStyleModifier handles jq-style modifiers
func applyJQStyleModifier(result Result, name, arg string) (Result, bool) {
	switch name {
	case "slice":
		return applySliceModifier(result, arg), true
	case "has":
		return applyHasModifier(result, arg), true
	case "contains":
		return applyContainsModifier(result, arg), true
	case "split":
		return applySplitModifier(result, arg), true
	case "startswith":
		return applyStartsWithModifier(result, arg), true
	case "endswith":
		return applyEndsWithModifier(result, arg), true
	case "entries", "toentries":
		return applyEntriesToModifier(result), true
	case "fromentries":
		return applyFromEntriesModifier(result), true
	case "any":
		return applyAnyModifier(result), true
	case "all":
		return applyAllModifier(result), true
	}
	return Result{}, false
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

	// Fast path: scan raw bytes directly for simple numeric arrays
	if len(result.Raw) > 0 {
		sum, count := scanArrayNumbersFast(result.Raw)
		if count > 0 {
			return buildNumberResult(sum)
		}
	}

	// Fallback to original implementation for complex cases
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

	// Fast path for simple numeric arrays
	if len(result.Raw) > 0 {
		sum, count := scanArrayNumbersFast(result.Raw)
		if count > 0 {
			return buildNumberResult(sum / float64(count))
		}
	}

	// Fallback for complex arrays
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

	// Fast path for simple numeric arrays
	if len(result.Raw) > 0 {
		if value, found := scanArrayMinMaxFast(result.Raw, true); found {
			return buildNumberResult(value)
		}
	}

	// Fallback for complex arrays
	var m float64
	hasValue := false
	result.ForEach(func(_, value Result) bool {
		if num, ok := numericValue(value); ok {
			if !hasValue || num < m {
				m = num
				hasValue = true
			}
		}
		return true
	})

	if !hasValue {
		return Result{Type: TypeUndefined}
	}

	return buildNumberResult(m)
}

func applyMaxModifier(result Result) Result {
	if result.Type != TypeArray {
		return Result{Type: TypeUndefined}
	}

	// Fast path for simple numeric arrays
	if len(result.Raw) > 0 {
		if value, found := scanArrayMinMaxFast(result.Raw, false); found {
			return buildNumberResult(value)
		}
	}

	// Fallback for complex arrays
	var m float64
	hasValue := false
	result.ForEach(func(_, value Result) bool {
		if num, ok := numericValue(value); ok {
			if !hasValue || num > m {
				m = num
				hasValue = true
			}
		}
		return true
	})

	if !hasValue {
		return Result{Type: TypeUndefined}
	}

	return buildNumberResult(m)
}

// ==================== ADVANCED TRANSFORMATION MODIFIERS ====================

// applyGroupModifier groups array elements by a field value
// Example: users|@group:city returns {"NYC": [...], "Boston": [...]}
func applyGroupModifier(result Result, field string) Result {
	if result.Type != TypeArray || field == "" {
		return Result{Type: TypeUndefined}
	}

	items := result.Array()
	if len(items) == 0 {
		return Result{Type: TypeObject, Raw: []byte("{}"), Modified: true}
	}

	// Group elements by field value
	groups := make(map[string][]Result)
	var order []string // Maintain insertion order

	for _, value := range items {
		if value.Type != TypeObject {
			continue
		}
		// Get the field value to group by
		keyResult := Get(value.Raw, field)
		key := keyResult.String()
		if key == "" {
			key = "null"
		}

		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], value)
	}

	if len(groups) == 0 {
		return Result{Type: TypeObject, Raw: []byte("{}"), Modified: true}
	}

	// Build result object
	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true
	for _, key := range order {
		items := groups[key]
		if !first {
			buf.WriteByte(',')
		}
		first = false

		// Write key
		buf.WriteByte('"')
		buf.WriteString(escapeString(key))
		buf.WriteString(`":[`)

		// Write array of items
		for i, item := range items {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.Write(item.Raw)
		}
		buf.WriteByte(']')
	}
	buf.WriteByte('}')

	return Result{Type: TypeObject, Raw: buf.Bytes(), Modified: true}
}

// applySortByModifier sorts array of objects by a field
// Example: users|@sortby:age
func applySortByModifier(result Result, field string) Result {
	if result.Type != TypeArray || field == "" {
		return Result{Type: TypeUndefined}
	}

	items := result.Array()
	if len(items) == 0 {
		return Result{Type: TypeArray, Raw: []byte("[]"), Modified: true}
	}

	sorted := make([]Result, len(items))
	copy(sorted, items)

	// Sort by field value (supports string and numeric comparison)
	sort.SliceStable(sorted, func(i, j int) bool {
		vi := Get(sorted[i].Raw, field)
		vj := Get(sorted[j].Raw, field)

		// Try numeric comparison first
		if vi.Type == TypeNumber && vj.Type == TypeNumber {
			return vi.Num < vj.Num
		}
		// Fall back to string comparison
		return vi.String() < vj.String()
	})

	sortedResult := buildWildcardResult(sorted)
	if sortedResult.Type == TypeUndefined {
		return Result{Type: TypeArray, Raw: []byte("[]"), Modified: true}
	}
	sortedResult.Modified = true
	return sortedResult
}

// applyMapModifier projects specific fields from array of objects
// Example: users|@map:name;age returns [{"name":"Alice","age":30}, ...]
// Note: Use semicolon (;) to separate multiple fields, not comma
func applyMapModifier(result Result, fields string) Result {
	if result.Type != TypeArray || fields == "" {
		return Result{Type: TypeUndefined}
	}

	items := result.Array()
	if len(items) == 0 {
		return Result{Type: TypeArray, Raw: []byte("[]"), Modified: true}
	}

	// Parse field list (semicolon-separated to avoid conflict with path parser)
	fieldList := strings.Split(fields, ";")
	for i := range fieldList {
		fieldList[i] = strings.TrimSpace(fieldList[i])
	}

	// Build projected array
	var buf bytes.Buffer
	buf.WriteByte('[')
	first := true

	for _, value := range items {
		if value.Type != TypeObject {
			continue
		}

		if !first {
			buf.WriteByte(',')
		}
		first = false

		// Build projected object
		buf.WriteByte('{')
		fieldFirst := true
		for _, f := range fieldList {
			fieldVal := Get(value.Raw, f)
			if !fieldVal.Exists() {
				continue
			}

			if !fieldFirst {
				buf.WriteByte(',')
			}
			fieldFirst = false

			buf.WriteByte('"')
			buf.WriteString(escapeString(f))
			buf.WriteString(`":`)
			buf.Write(fieldVal.Raw)
		}
		buf.WriteByte('}')
	}
	buf.WriteByte(']')

	return Result{Type: TypeArray, Raw: buf.Bytes(), Modified: true}
}

// applyUniqueByModifier returns unique elements by field
// Example: users|@uniqueby:city
func applyUniqueByModifier(result Result, field string) Result {
	if result.Type != TypeArray || field == "" {
		return Result{Type: TypeUndefined}
	}

	items := result.Array()
	if len(items) == 0 {
		return Result{Type: TypeArray, Raw: []byte("[]"), Modified: true}
	}

	// Track seen field values
	seen := make(map[string]bool)
	var unique []Result

	for _, value := range items {
		keyResult := Get(value.Raw, field)
		key := keyResult.String()

		if !seen[key] {
			seen[key] = true
			unique = append(unique, value)
		}
	}

	if len(unique) == 0 {
		return Result{Type: TypeArray, Raw: []byte("[]"), Modified: true}
	}

	uniqueResult := buildWildcardResult(unique)
	if uniqueResult.Type == TypeUndefined {
		return Result{Type: TypeArray, Raw: []byte("[]"), Modified: true}
	}
	uniqueResult.Modified = true
	return uniqueResult
}

// ==================== ADDITIONAL JQ-STYLE MODIFIERS ====================

// applySliceModifier returns a slice of an array
// Example: items|@slice:2:5 returns elements from index 2 to 4 (exclusive)
func applySliceModifier(result Result, arg string) Result {
	if result.Type != TypeArray {
		return Result{Type: TypeUndefined}
	}

	items := result.Array()
	n := len(items)
	if n == 0 {
		return Result{Type: TypeArray, Raw: []byte("[]"), Modified: true}
	}

	start, end := parseSliceIndices(arg, n)

	if start >= end || start >= n {
		return Result{Type: TypeArray, Raw: []byte("[]"), Modified: true}
	}

	sliced := items[start:end]
	slicedResult := buildWildcardResult(sliced)
	if slicedResult.Type == TypeUndefined {
		return Result{Type: TypeArray, Raw: []byte("[]"), Modified: true}
	}
	slicedResult.Modified = true
	return slicedResult
}

// parseSliceIndices parses slice arguments and clamps to bounds
func parseSliceIndices(arg string, n int) (int, int) {
	// Parse start:end from arg
	start, end := 0, n
	if arg != "" {
		parts := strings.Split(arg, ":")
		if len(parts) >= 1 && parts[0] != "" {
			if s, err := strconv.Atoi(parts[0]); err == nil {
				start = s
			}
		}
		if len(parts) >= 2 && parts[1] != "" {
			if e, err := strconv.Atoi(parts[1]); err == nil {
				end = e
			}
		}
	}

	// Handle negative indices
	if start < 0 {
		start = n + start
	}
	if end < 0 {
		end = n + end
	}

	// Clamp to bounds
	if start < 0 {
		start = 0
	}
	if end > n {
		end = n
	}
	return start, end
}

// applyHasModifier checks if an object has a field or array has index
// Example: user|@has:name returns true/false
func applyHasModifier(result Result, field string) Result {
	if field == "" {
		return Result{Type: TypeBoolean, Boolean: false, Raw: []byte("false"), Modified: true}
	}

	hasField := false
	if result.Type == TypeObject {
		fieldResult := Get(result.Raw, field)
		hasField = fieldResult.Exists()
	} else if result.Type == TypeArray {
		// Check if index exists
		if idx, err := strconv.Atoi(field); err == nil {
			items := result.Array()
			if idx >= 0 && idx < len(items) {
				hasField = true
			}
		}
	}

	if hasField {
		return Result{Type: TypeBoolean, Boolean: true, Raw: []byte("true"), Modified: true}
	}
	return Result{Type: TypeBoolean, Boolean: false, Raw: []byte("false"), Modified: true}
}

// applyContainsModifier checks if array contains value or string contains substring
// Example: tags|@contains:featured returns true/false
func applyContainsModifier(result Result, value string) Result {
	if value == "" {
		return Result{Type: TypeBoolean, Boolean: false, Raw: []byte("false"), Modified: true}
	}

	contains := false
	switch result.Type {
	case TypeString:
		contains = strings.Contains(result.String(), value)
	case TypeArray:
		items := result.Array()
		for _, item := range items {
			if item.String() == value {
				contains = true
				break
			}
		}
	}

	if contains {
		return Result{Type: TypeBoolean, Boolean: true, Raw: []byte("true"), Modified: true}
	}
	return Result{Type: TypeBoolean, Boolean: false, Raw: []byte("false"), Modified: true}
}

// applySplitModifier splits a string by delimiter
// Example: "a,b,c"|@split:, returns ["a","b","c"]
func applySplitModifier(result Result, delim string) Result {
	if result.Type != TypeString {
		return Result{Type: TypeUndefined}
	}

	if delim == "" {
		delim = ","
	}

	str := result.String()
	parts := strings.Split(str, delim)

	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, part := range parts {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteByte('"')
		buf.WriteString(escapeString(part))
		buf.WriteByte('"')
	}
	buf.WriteByte(']')

	return Result{Type: TypeArray, Raw: buf.Bytes(), Modified: true}
}

// applyStartsWithModifier checks if string starts with prefix
// Example: name|@startswith:John returns true/false
func applyStartsWithModifier(result Result, prefix string) Result {
	if result.Type != TypeString || prefix == "" {
		return Result{Type: TypeBoolean, Boolean: false, Raw: []byte("false"), Modified: true}
	}

	if strings.HasPrefix(result.String(), prefix) {
		return Result{Type: TypeBoolean, Boolean: true, Raw: []byte("true"), Modified: true}
	}
	return Result{Type: TypeBoolean, Boolean: false, Raw: []byte("false"), Modified: true}
}

// applyEndsWithModifier checks if string ends with suffix
// Example: email|@endswith:.com returns true/false
func applyEndsWithModifier(result Result, suffix string) Result {
	if result.Type != TypeString || suffix == "" {
		return Result{Type: TypeBoolean, Boolean: false, Raw: []byte("false"), Modified: true}
	}

	if strings.HasSuffix(result.String(), suffix) {
		return Result{Type: TypeBoolean, Boolean: true, Raw: []byte("true"), Modified: true}
	}
	return Result{Type: TypeBoolean, Boolean: false, Raw: []byte("false"), Modified: true}
}

// applyEntriesToModifier converts object to array of {key, value} entries
// Example: {a:1,b:2}|@entries returns [{"key":"a","value":1},{"key":"b","value":2}]
func applyEntriesToModifier(result Result) Result {
	if result.Type != TypeObject {
		return Result{Type: TypeUndefined}
	}

	var buf bytes.Buffer
	buf.WriteByte('[')
	first := true

	result.ForEach(func(key, value Result) bool {
		if !first {
			buf.WriteByte(',')
		}
		first = false

		buf.WriteString(`{"key":`)
		buf.Write(key.Raw)
		buf.WriteString(`,"value":`)
		buf.Write(value.Raw)
		buf.WriteByte('}')
		return true
	})

	buf.WriteByte(']')
	return Result{Type: TypeArray, Raw: buf.Bytes(), Modified: true}
}

// applyFromEntriesModifier converts array of {key, value} entries to object
// Example: [{"key":"a","value":1}]|@fromentries returns {"a":1}
func applyFromEntriesModifier(result Result) Result {
	if result.Type != TypeArray {
		return Result{Type: TypeUndefined}
	}

	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true

	result.ForEach(func(_, entry Result) bool {
		if entry.Type != TypeObject {
			return true
		}

		keyResult := Get(entry.Raw, "key")
		valueResult := Get(entry.Raw, "value")

		// Also support "k" and "v" or "name" and "value"
		if !keyResult.Exists() {
			keyResult = Get(entry.Raw, "k")
		}
		if !keyResult.Exists() {
			keyResult = Get(entry.Raw, "name")
		}
		if !valueResult.Exists() {
			valueResult = Get(entry.Raw, "v")
		}

		if !keyResult.Exists() || !valueResult.Exists() {
			return true
		}

		if !first {
			buf.WriteByte(',')
		}
		first = false

		// Ensure key is a quoted string
		keyStr := keyResult.String()
		buf.WriteByte('"')
		buf.WriteString(escapeString(keyStr))
		buf.WriteString(`":`)
		buf.Write(valueResult.Raw)
		return true
	})

	buf.WriteByte('}')
	return Result{Type: TypeObject, Raw: buf.Bytes(), Modified: true}
}

// applyAnyModifier checks if any element in array is truthy
// Example: [false,true,false]|@any returns true
func applyAnyModifier(result Result) Result {
	if result.Type != TypeArray {
		return Result{Type: TypeBoolean, Boolean: false, Raw: []byte("false"), Modified: true}
	}

	items := result.Array()
	for _, item := range items {
		if item.Bool() {
			return Result{Type: TypeBoolean, Boolean: true, Raw: []byte("true"), Modified: true}
		}
	}
	return Result{Type: TypeBoolean, Boolean: false, Raw: []byte("false"), Modified: true}
}

// applyAllModifier checks if all elements in array are truthy
// Example: [true,true,true]|@all returns true
func applyAllModifier(result Result) Result {
	if result.Type != TypeArray {
		return Result{Type: TypeBoolean, Boolean: false, Raw: []byte("false"), Modified: true}
	}

	items := result.Array()
	if len(items) == 0 {
		return Result{Type: TypeBoolean, Boolean: true, Raw: []byte("true"), Modified: true} // Vacuous truth
	}

	for _, item := range items {
		if !item.Bool() {
			return Result{Type: TypeBoolean, Boolean: false, Raw: []byte("false"), Modified: true}
		}
	}
	return Result{Type: TypeBoolean, Boolean: true, Raw: []byte("true"), Modified: true}
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

// skipWhitespaceSimple skips whitespace and returns the new position
func skipWhitespaceSimple(data []byte, i int) int {
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	return i
}

// isComplexValueStart checks if a byte starts a complex JSON value
func isComplexValueStart(b byte) bool {
	return b == '{' || b == '[' || b == '"' || b == 't' || b == 'f' || b == 'n'
}

// scanNumber scans a number from data starting at position i
// Returns the parsed number, new position, and success flag
func scanNumber(data []byte, i int) (float64, int, bool) {
	start := i
	for i < len(data) && isNumberChar(data[i]) {
		i++
	}
	num, err := strconv.ParseFloat(string(data[start:i]), 64)
	return num, i, err == nil
}

// scanArrayNumbersFast scans raw JSON array bytes and extracts numbers directly
// without creating Result objects. Returns sum and count.
// Returns (0, 0) if the array contains non-numeric or complex values.
func scanArrayNumbersFast(data []byte) (sum float64, count int) {
	if len(data) < 2 || data[0] != '[' {
		return 0, 0
	}

	i := 1 // Skip opening '['
	for i < len(data) {
		i = skipWhitespaceSimple(data, i)
		if i >= len(data) || data[i] == ']' {
			break
		}

		// Check for number start
		if isNumberStart(data[i]) {
			num, newPos, ok := scanNumber(data, i)
			if !ok {
				return 0, 0
			}
			sum += num
			count++
			i = newPos
		} else if isComplexValueStart(data[i]) {
			return 0, 0 // Complex value, bail
		}

		// Skip whitespace and comma
		i = skipWhitespaceSimple(data, i)
		if i < len(data) && data[i] == ',' {
			i++
		}
	}

	return sum, count
}

func scanArrayMinMaxFast(data []byte, findMin bool) (value float64, found bool) {
	if len(data) < 2 || data[0] != '[' {
		return 0, false
	}

	i := 1
	for i < len(data) {
		i = skipWhitespaceSimple(data, i)
		if i >= len(data) || data[i] == ']' {
			break
		}

		// Check for number
		if isNumberStart(data[i]) {
			var ok bool
			value, found, i, ok = processMinMaxNumber(data, i, value, found, findMin)
			if !ok {
				return 0, false
			}
		} else if isComplexValueStart(data[i]) {
			return 0, false // Complex value, bail
		}

		// Skip comma
		i = skipWhitespaceSimple(data, i)
		if i < len(data) && data[i] == ',' {
			i++
		}
	}

	return value, found
}

// processMinMaxNumber scans a number and updates min/max value
func processMinMaxNumber(data []byte, i int, currentVal float64, found, findMin bool) (float64, bool, int, bool) {
	num, newPos, ok := scanNumber(data, i)
	if !ok {
		return 0, false, i, false
	}
	if !found {
		return num, true, newPos, true
	}
	if findMin {
		if num < currentVal {
			return num, true, newPos, true
		}
	} else {
		if num > currentVal {
			return num, true, newPos, true
		}
	}
	return currentVal, true, newPos, true
}

// applyThisModifier returns the result unchanged (@this)
func applyThisModifier(result Result) Result {
	result.Modified = true
	return result
}

// applyValidModifier validates JSON and returns it if valid (@valid)
func applyValidModifier(result Result) Result {
	// If the result has valid Raw JSON, return it
	if len(result.Raw) > 0 && result.Exists() {
		result.Modified = true
		return result
	}
	return Result{Type: TypeUndefined}
}

// applyPrettyModifier formats JSON with indentation (@pretty)
func applyPrettyModifier(result Result, arg string) Result {
	if len(result.Raw) == 0 {
		return result
	}

	// Default indent options
	indent := "  "
	prefix := ""

	// Parse argument if provided (e.g., @pretty:{"indent":"\t"})
	if arg != "" {
		argResult := Parse([]byte(arg))
		if indentVal := argResult.Get("indent"); indentVal.Exists() {
			indent = indentVal.String()
		}
		if prefixVal := argResult.Get("prefix"); prefixVal.Exists() {
			prefix = prefixVal.String()
		}
	}

	// Use standard library to pretty print
	var out bytes.Buffer
	if err := json.Indent(&out, result.Raw, prefix, indent); err != nil {
		return result // Return unchanged on error
	}

	return Result{
		Type:     result.Type,
		Str:      result.Str,
		Num:      result.Num,
		Boolean:  result.Boolean,
		Raw:      out.Bytes(),
		Modified: true,
	}
}

// applyUglyModifier minifies JSON by removing whitespace (@ugly)
func applyUglyModifier(result Result) Result {
	if len(result.Raw) == 0 {
		return result
	}

	// Use compact from encoding/json
	var out bytes.Buffer
	if err := json.Compact(&out, result.Raw); err != nil {
		return result // Return unchanged on error
	}

	return Result{
		Type:     result.Type,
		Str:      result.Str,
		Num:      result.Num,
		Boolean:  result.Boolean,
		Raw:      out.Bytes(),
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
//
// getArrayElementRange returns the start and end position of the element at index
//
//go:inline
func getArrayElementRange(data []byte, index int) (int, int) {
	pos, isArray := findArrayElementStart(data)
	if !isArray {
		return -1, -1
	}

	// Statistical jump for large indices
	if index > 100 && len(data) > 10000 {
		pos, index = performOptimizedJump(data, pos, index)
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

// performOptimizedJump attempts to jump closer to the target index using statistical estimation
func performOptimizedJump(data []byte, pos, index int) (int, int) {
	// Estimate element size by sampling first few elements
	avgSize, elementsScanned := calculateAverageElementSize(data, pos)

	if elementsScanned > 0 {
		// Jump to estimated position
		jumpPos := calculateJumpPosition(pos, avgSize, index, elementsScanned)

		if jumpPos < len(data) && jumpPos > pos {
			return refineJumpPosition(data, pos, jumpPos, index)
		}
	}
	return pos, index
}

// calculateAverageElementSize samples the first few elements to estimate size
func calculateAverageElementSize(data []byte, pos int) (int, int) {
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
		return totalSize / elementsScanned, elementsScanned
	}
	return 0, 0
}

// calculateJumpPosition determines the byte offset to jump to
func calculateJumpPosition(pos, avgSize, index, elementsScanned int) int {
	return pos + (avgSize+2)*(index-elementsScanned) // +2 for comma and space
}

// refineJumpPosition adjusts position based on actual comma count from start
func refineJumpPosition(data []byte, startPos, jumpPos, targetIndex int) (int, int) {
	// Count actual elements from jump position backward to refine
	commaCount := 0
	for i := startPos; i < jumpPos && i < len(data); i++ {
		if data[i] == ',' {
			commaCount++
		}
	}

	// Adjust position based on comma count
	if commaCount > targetIndex {
		// Overshot, go back - use original pos
		return startPos, targetIndex
	}

	// Use comma count as starting point
	newPos := jumpPos
	// Back up to last known comma
	for newPos > 0 && data[newPos] != ',' {
		newPos--
	}
	if newPos > 0 {
		newPos++ // After comma
	}
	return newPos, targetIndex - commaCount
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
// skipValue - Fast value skipping without parsing
// Used for efficiently skipping unwanted key-value pairs
// findStringEnd finds the end of a JSON string, handling escapes
// skipValue - Fast value skipping without parsing
// Used for efficiently skipping unwanted key-value pairs
//
//go:inline
func skipValue(data []byte, i int) int {
	// Skip leading whitespace
	for ; i < len(data) && data[i] <= ' '; i++ {
	}

	if i >= len(data) {
		return i
	}

	switch data[i] {
	case '"':
		return skipStringValue(data, i)
	case '{':
		return skipObjectValue(data, i)
	case '[':
		return skipArrayValue(data, i)
	default:
		return skipPrimitiveValue(data, i)
	}
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

	// Zero-copy string conversion for strings without escapes
	// Fast path for strings without escapes (most common case)
	//nolint:gosec //G103
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

	// Zero-copy number parsing
	// Fast path for simple integers using unsafe.String
	//nolint:gosec //G103
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
	//nolint:gosec //G103
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

// Uint returns the result as a uint64
func (r Result) Uint() uint64 {
	switch r.Type {
	case TypeNumber:
		if len(r.Raw) > 0 {
			if n, err := strconv.ParseUint(strings.TrimSpace(string(r.Raw)), 10, 64); err == nil {
				return n
			}
		}
		return uint64(r.Num)
	case TypeString:
		n, _ := strconv.ParseUint(r.Str, 10, 64)
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

// Value returns the result as a native Go type (interface{}).
// Returns:
//   - nil for TypeNull or non-existent values
//   - bool for TypeBoolean
//   - float64 for TypeNumber
//   - string for TypeString
//   - []interface{} for TypeArray
//   - map[string]interface{} for TypeObject
func (r Result) Value() interface{} {
	switch r.Type {
	case TypeNull, TypeUndefined:
		return nil
	case TypeBoolean:
		return r.Boolean
	case TypeNumber:
		return r.Num
	case TypeString:
		return r.Str
	case TypeArray:
		arr := r.Array()
		result := make([]interface{}, len(arr))
		for i, v := range arr {
			result[i] = v.Value()
		}
		return result
	case TypeObject:
		m := r.Map()
		result := make(map[string]interface{}, len(m))
		for k, v := range m {
			result[k] = v.Value()
		}
		return result
	default:
		return nil
	}
}

// Less compares two Result values and returns true if r is less than token.
// The comparison rules are:
//   - Null < Boolean < Number < String < Array/Object
//   - For booleans: false < true
//   - For numbers: numeric comparison
//   - For strings: lexicographic comparison (case-sensitive or insensitive based on caseSensitive parameter)
func (r Result) Less(token Result, caseSensitive bool) bool {
	// Define type priority: Null=0, Boolean=1, Number=2, String=3, Array/Object=4
	typePriority := func(t ValueType) int {
		switch t {
		case TypeNull, TypeUndefined:
			return 0
		case TypeBoolean:
			return 1
		case TypeNumber:
			return 2
		case TypeString:
			return 3
		default:
			return 4
		}
	}

	rPriority := typePriority(r.Type)
	tokenPriority := typePriority(token.Type)

	// Different types: compare by priority
	if rPriority != tokenPriority {
		return rPriority < tokenPriority
	}

	// Same type: compare by value
	switch r.Type {
	case TypeNull, TypeUndefined:
		return false // Both are null/undefined, equal
	case TypeBoolean:
		// false < true
		if r.Boolean == token.Boolean {
			return false
		}
		return !r.Boolean && token.Boolean
	case TypeNumber:
		return r.Num < token.Num
	case TypeString:
		if caseSensitive {
			return r.Str < token.Str
		}
		return strings.ToLower(r.Str) < strings.ToLower(token.Str)
	default:
		// For arrays/objects, compare raw JSON string representation
		return string(r.Raw) < string(token.Raw)
	}
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
