// Package njson provides high-performance JSON manipulation functions.
// Created by dhawalhost (2025-09-01 13:51:05)
package njson

import (
	"bytes"
	"encoding/base64"
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

// Get retrieves a value from JSON using a path expression.
// This is highly optimized with multiple fast paths for common use cases.
func Get(data []byte, path string) Result {
	// Empty path should return non-existent result according to tests
	if path == "" {
		return Result{Type: TypeUndefined}
	}

	// Root path returns the entire document
	if path == "$" || path == "@" {
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
	// Skip caching for ultra-simple paths - cache overhead is too high for small operations

	// Fast inline search for key pattern: "key":
	keyLen := len(path)
	searchLen := keyLen + 3      // quotes + colon
	if len(data) < searchLen+4 { // minimum space for key + value
		return Result{Type: TypeUndefined}
	}

	// Manual search optimized for small data
	var keyIdx = -1
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
					keyIdx = i
					break
				}
			}
		}
	}

	if keyIdx == -1 {
		return Result{Type: TypeUndefined}
	}

	// Skip to the value (past "key":)
	valueStart := keyIdx + keyLen + 3

	// Skip whitespace
	for valueStart < len(data) && data[valueStart] <= ' ' {
		valueStart++
	}

	if valueStart >= len(data) {
		return Result{Type: TypeUndefined}
	}

	// Parse value based on first character - optimized for common cases
	switch data[valueStart] {
	case '"': // String - most common case first, optimized for ultra-simple path
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
	case 't': // true
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
	case 'f': // false
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
	case 'n': // null
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
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// Number
		return parseNumber(data, valueStart)
	case '{': // Object
		objectEnd := findBlockEnd(data, valueStart, '{', '}')
		if objectEnd == -1 {
			return Result{Type: TypeUndefined}
		}
		return Result{
			Type:  TypeObject,
			Raw:   data[valueStart:objectEnd],
			Index: valueStart,
		}
	case '[': // Array
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

	return Result{Type: TypeUndefined}
}

// getSimplePath handles simple dot notation and basic array access
// This is optimized for paths like "user.name" or "items[0].id" or "items.0.id"
func getSimplePath(data []byte, path string) Result {
	dataStart, dataEnd := 0, len(data)
	p := 0

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
			return Result{Type: TypeUndefined}
		}
		dataStart = start
		dataEnd = end

		// Continue with rest of path if there is more
		if p < len(path) && path[p] == '.' {
			p++ // Skip the dot
		}
	}

	for p < len(path) {
		keyStart := p

		// Find end of key part
		i := p
		for i < len(path) && path[i] != '.' && path[i] != '[' {
			i++
		}

		key := path[keyStart:i]
		p = i

		if key != "" {
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
					return Result{Type: TypeUndefined}
				}
				dataStart += start
				dataEnd = dataStart + (end - start)
			} else {
				// Normal object key access - optimized for medium JSON
				start, end := fastFindObjectValue(data[dataStart:dataEnd], key)
				if start == -1 {
					return Result{Type: TypeUndefined}
				}
				dataStart += start
				dataEnd = dataStart + (end - start)
			}
		}

		// Check for array access with bracket notation
		if p < len(path) && path[p] == '[' {
			p++ // Skip '['

			// Fast manual integer parsing instead of strconv.Atoi
			idx := 0
			for p < len(path) && path[p] >= '0' && path[p] <= '9' {
				idx = idx*10 + int(path[p]-'0')
				p++
			}

			if p >= len(path) || path[p] != ']' {
				return Result{Type: TypeUndefined} // Malformed
			}
			p++ // Skip ']'

			// Use simple array access for bracket notation (typically smaller indices)
			start, end := fastFindArrayElement(data[dataStart:dataEnd], idx)
			if start == -1 {
				return Result{Type: TypeUndefined}
			}
			dataStart += start
			dataEnd = dataStart + (end - start)
		}

		// Move to next part
		if p < len(path) && path[p] == '.' {
			p++
		}
	}

	// Direct parsing of final value
	return fastParseValue(data[dataStart:dataEnd])
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
	// Skip whitespace
	start := 0
	for ; start < len(data); start++ {
		if data[start] > ' ' {
			break
		}
	}
	if start >= len(data) || data[start] != '{' {
		return -1, -1
	}

	pos := start + 1
	keyLen := len(key)

	for pos < len(data) {
		// Skip whitespace
		for ; pos < len(data) && data[pos] <= ' '; pos++ {
		}
		if pos >= len(data) || data[pos] == '}' {
			return -1, -1
		}

		if data[pos] != '"' {
			return -1, -1
		}

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
				// Found our key, skip to colon
				pos += keyLen + 2 // skip "key"
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
		}

		// Skip this key-value pair
		// First, skip the key
		keyEnd := pos + 1
		for keyEnd < len(data) && data[keyEnd] != '"' {
			keyEnd++
		}
		if keyEnd >= len(data) {
			return -1, -1
		}
		keyEnd++ // Skip closing quote

		// Skip to colon
		for keyEnd < len(data) && data[keyEnd] <= ' ' {
			keyEnd++
		}
		if keyEnd >= len(data) || data[keyEnd] != ':' {
			return -1, -1
		}
		keyEnd++ // Skip colon

		// Skip whitespace to get to value
		for keyEnd < len(data) && data[keyEnd] <= ' ' {
			keyEnd++
		}

		// Now call findValueEnd on the actual value position
		pos = findValueEnd(data, keyEnd)
		if pos == -1 {
			return -1, -1
		}

		// Skip to next key or end
		for ; pos < len(data) && data[pos] != ',' && data[pos] != '}'; pos++ {
		}
		if pos >= len(data) || data[pos] == '}' {
			return -1, -1
		}
		pos++ // skip comma
	}

	return -1, -1
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

// fastSkipValue efficiently skips over a JSON value using minimal parsing
func fastSkipValue(data []byte, start int) int {
	pos := start

	if pos >= len(data) {
		return -1
	}

	switch data[pos] {
	case '"': // String
		pos++ // Skip opening quote
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

	case '{': // Object
		pos++ // Skip opening brace
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

	case '[': // Array
		pos++ // Skip opening bracket
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

	case 't': // true
		if pos+3 < len(data) &&
			data[pos+1] == 'r' &&
			data[pos+2] == 'u' &&
			data[pos+3] == 'e' {
			return pos + 4
		}
		return -1

	case 'f': // false
		if pos+4 < len(data) &&
			data[pos+1] == 'a' &&
			data[pos+2] == 'l' &&
			data[pos+3] == 's' &&
			data[pos+4] == 'e' {
			return pos + 5
		}
		return -1

	case 'n': // null
		if pos+3 < len(data) &&
			data[pos+1] == 'u' &&
			data[pos+2] == 'l' &&
			data[pos+3] == 'l' {
			return pos + 4
		}
		return -1

	default: // Number
		for pos < len(data) {
			c := data[pos]
			if !((c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-') {
				break
			}
			pos++
		}
		return pos
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

// parseModifiers extracts and parses modifier tokens from a path
func parseModifiers(path string) ([]pathToken, string) {
	var modifiers []pathToken
	modifierIdx := strings.IndexByte(path, '|')
	if modifierIdx < 0 {
		return modifiers, path
	}

	modifierParts := strings.Split(path[modifierIdx+1:], "|")
	for _, part := range modifierParts {
		parts := strings.SplitN(part, ":", 2)
		mod := pathToken{kind: tokenModifier, str: parts[0]}
		if len(parts) > 1 {
			mod.str = parts[0] + ":" + parts[1] // Keep the full modifier
		}
		modifiers = append(modifiers, mod)
	}

	return modifiers, path[:modifierIdx]
}

// parseArrayAccess parses array access syntax in path parts
func parseArrayAccess(part string) []pathToken {
	var tokens []pathToken

	base := part[:strings.IndexByte(part, '[')]
	if base != "" {
		tokens = append(tokens, pathToken{kind: tokenKey, str: base})
	}

	bracket := part[strings.IndexByte(part, '[')+1 : len(part)-1]

	if bracket == "*" {
		tokens = append(tokens, pathToken{kind: tokenWildcard})
	} else if idx, err := strconv.Atoi(bracket); err == nil {
		tokens = append(tokens, pathToken{kind: tokenIndex, num: idx})
	} else if strings.HasPrefix(bracket, "?") || strings.Contains(bracket, "==") ||
		strings.Contains(bracket, "!=") || strings.Contains(bracket, ">=") ||
		strings.Contains(bracket, "<=") {
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

	// Split the path
	parts := strings.Split(cleanPath, ".")

	for _, part := range parts {
		if part == "" {
			continue
		}

		if part == "*" {
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
	var opIdx int = -1

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
	var modifiers []pathToken
	var pathTokens []pathToken

	for _, token := range tokens {
		if token.kind == tokenModifier {
			modifiers = append(modifiers, token)
		} else {
			pathTokens = append(pathTokens, token)
		}
	}

	// Process each token
	for i, token := range pathTokens {
		switch token.kind {
		case tokenKey:
			if current.Type != TypeObject {
				return Result{Type: TypeUndefined}
			}

			// Use direct object lookup instead of ForEach to avoid allocations
			start, end := fastFindObjectValue(current.Raw, token.str)
			if start == -1 {
				return Result{Type: TypeUndefined}
			}

			current = fastParseValue(current.Raw[start:end])

		case tokenIndex:
			if current.Type != TypeArray {
				return Result{Type: TypeUndefined}
			}

			// Use direct array lookup instead of Array() to avoid allocations
			start, end := fastFindArrayElement(current.Raw, token.num)
			if start == -1 {
				return Result{Type: TypeUndefined}
			}

			current = fastParseValue(current.Raw[start:end])

		case tokenWildcard:
			if current.Type != TypeArray && current.Type != TypeObject {
				return Result{Type: TypeUndefined}
			}

			// Fast path for simple wildcard operations like "phones.*.type"
			if i == len(pathTokens)-2 && len(pathTokens) > 2 {
				nextToken := pathTokens[i+1]
				if nextToken.kind == tokenKey {
					return fastWildcardKeyAccess(current, nextToken.str)
				}
			}

			// Collect all values with minimal allocations
			values := make([]Result, 0, 8) // Pre-allocate for common case
			current.ForEach(func(_, value Result) bool {
				values = append(values, value)
				return true
			})

			if len(values) == 0 {
				return Result{Type: TypeUndefined}
			}

			// If this is the last token, return array of values
			if i == len(pathTokens)-1 {
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

				current = Result{
					Type: TypeArray,
					Raw:  raw,
				}
			} else {
				// Otherwise, need to process each value with remaining tokens
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
					return Result{Type: TypeUndefined}
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

				current = Result{
					Type: TypeArray,
					Raw:  raw.Bytes(),
				}

				// Skip remaining tokens as we've processed them
				return current
			}

		case tokenFilter:
			if current.Type != TypeArray {
				return Result{Type: TypeUndefined}
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
				return Result{Type: TypeUndefined}
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

			current = Result{
				Type: TypeArray,
				Raw:  raw.Bytes(),
			}

		case tokenRecursive:
			if i == len(pathTokens)-1 {
				// This is the last token, which doesn't make sense for recursive descent
				return Result{Type: TypeUndefined}
			}

			// Recursive descent
			current = recursiveSearch(current, pathTokens[i+1:])
			return current // recursiveSearch processes the rest of the tokens
		}

		if !current.Exists() {
			return Result{Type: TypeUndefined}
		}
	}

	// Apply modifiers if any
	if len(modifiers) > 0 {
		for _, mod := range modifiers {
			current = applyModifier(current, mod.str)
		}
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
		return Result{
			Type:     TypeString,
			Str:      result.String(),
			Raw:      []byte(`"` + escapeString(result.String()) + `"`),
			Modified: true,
		}

	case constNumber, "num":
		num := result.Float()
		return Result{
			Type:     TypeNumber,
			Num:      num,
			Raw:      []byte(strconv.FormatFloat(num, 'f', -1, 64)),
			Modified: true,
		}

	case constBool, constBoolean:
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

	case "keys":
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

	case "values":
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

	case "length", "count", "len":
		switch result.Type {
		case TypeArray:
			count := len(result.Array())
			return Result{
				Type:     TypeNumber,
				Num:      float64(count),
				Raw:      []byte(strconv.Itoa(count)),
				Modified: true,
			}
		case TypeObject:
			count := len(result.Map())
			return Result{
				Type:     TypeNumber,
				Num:      float64(count),
				Raw:      []byte(strconv.Itoa(count)),
				Modified: true,
			}
		case TypeString:
			count := len(result.Str)
			return Result{
				Type:     TypeNumber,
				Num:      float64(count),
				Raw:      []byte(strconv.Itoa(count)),
				Modified: true,
			}
		default:
			return Result{Type: TypeUndefined}
		}

	case "type":
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

	case "base64":
		if result.Type == TypeString {
			encoded := base64.StdEncoding.EncodeToString([]byte(result.Str))
			return Result{
				Type:     TypeString,
				Str:      encoded,
				Raw:      []byte(`"` + encoded + `"`),
				Modified: true,
			}
		}

	case "base64decode":
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

	case "lower":
		if result.Type == TypeString {
			lower := strings.ToLower(result.Str)
			return Result{
				Type:     TypeString,
				Str:      lower,
				Raw:      []byte(`"` + escapeString(lower) + `"`),
				Modified: true,
			}
		}

	case "upper":
		if result.Type == TypeString {
			upper := strings.ToUpper(result.Str)
			return Result{
				Type:     TypeString,
				Str:      upper,
				Raw:      []byte(`"` + escapeString(upper) + `"`),
				Modified: true,
			}
		}

	case "join":
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
	}

	// Return original result if no modifier was applied
	return result
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
func getArrayElementRange(data []byte, index int) (int, int) {
	pos, isArray := findArrayElementStart(data)
	if !isArray {
		return -1, -1
	}

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
	case 't': // true
		if start+3 < len(data) &&
			data[start+1] == 'r' &&
			data[start+2] == 'u' &&
			data[start+3] == 'e' {
			return start + 4
		}
		return -1
	case 'f': // false
		if start+4 < len(data) &&
			data[start+1] == 'a' &&
			data[start+2] == 'l' &&
			data[start+3] == 's' &&
			data[start+4] == 'e' {
			return start + 5
		}
		return -1
	case 'n': // null
		if start+3 < len(data) &&
			data[start+1] == 'u' &&
			data[start+2] == 'l' &&
			data[start+3] == 'l' {
			return start + 4
		}
		return -1
	default:
		// Number - scan until non-number character
		if (data[start] >= '0' && data[start] <= '9') ||
			data[start] == '-' || data[start] == '+' ||
			data[start] == '.' || data[start] == 'e' ||
			data[start] == 'E' {
			for i := start + 1; i < len(data); i++ {
				if !((data[i] >= '0' && data[i] <= '9') ||
					data[i] == '.' || data[i] == 'e' ||
					data[i] == 'E' || data[i] == '+' ||
					data[i] == '-') {
					return i
				}
			}
			return len(data)
		}
	}

	return -1
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

	// Fast path for strings without escapes
	if !bytes.ContainsAny(str, "\\") {
		return Result{
			Type:  TypeString,
			Str:   string(str),
			Raw:   raw,
			Index: start,
		}
	}

	// Unescape the string
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
		if !((data[end] >= '0' && data[end] <= '9') ||
			data[end] == '.' || data[end] == 'e' ||
			data[end] == 'E' || data[end] == '+' ||
			data[end] == '-') {
			break
		}
	}

	raw := data[start:end]

	// Fast path for simple integers
	if !bytes.ContainsAny(raw, ".eE+-") {
		// It's a simple integer
		n, err := strconv.ParseInt(string(raw), 10, 64)
		if err == nil {
			return Result{
				Type:  TypeNumber,
				Num:   float64(n),
				Raw:   raw,
				Index: start,
			}
		}
	}

	// Parse as float
	n, err := strconv.ParseFloat(string(raw), 64)
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
		index := 0
		for pos < len(r.Raw) {
			// Skip whitespace
			for ; pos < len(r.Raw) && r.Raw[pos] <= ' '; pos++ {
			}
			if pos >= len(r.Raw) || r.Raw[pos] == ']' {
				break
			}

			valueStart := pos
			valueEnd := findValueEnd(r.Raw, pos)
			if valueEnd == -1 {
				break
			}

			key := Result{Type: TypeNumber, Num: float64(index), Str: strconv.Itoa(index)}
			value := parseAny(r.Raw[valueStart:valueEnd])
			value.Raw = r.Raw[valueStart:valueEnd] // Preserve raw value

			if !iterator(key, value) {
				return
			}

			pos = valueEnd
			// Skip to next element or end of array
			for ; pos < len(r.Raw) && (r.Raw[pos] <= ' ' || r.Raw[pos] == ','); pos++ {
				if r.Raw[pos] == ',' {
					pos++
					break
				}
			}
			index++
		}
	} else { // TypeObject
		for pos < len(r.Raw) {
			// Skip whitespace and find key
			for ; pos < len(r.Raw) && r.Raw[pos] <= ' '; pos++ {
			}
			if pos >= len(r.Raw) || r.Raw[pos] == '}' {
				break
			}
			if r.Raw[pos] != '"' {
				break // Invalid object
			}

			keyStart := pos
			keyRes := parseString(r.Raw, keyStart)
			if !keyRes.Exists() {
				break
			}
			pos = keyStart + len(keyRes.Raw)

			// Find colon
			for ; pos < len(r.Raw) && r.Raw[pos] != ':'; pos++ {
			}

			if pos >= len(r.Raw) {
				break
			}

			// Skip colon and whitespace
			pos++
			for ; pos < len(r.Raw) && r.Raw[pos] <= ' '; pos++ {
			}

			if pos >= len(r.Raw) {
				break
			}

			// Find value
			valueStart := pos
			valueEnd := findValueEnd(r.Raw, pos)

			if valueEnd == -1 {
				break
			}

			// Parse value
			value := parseAny(r.Raw[valueStart:valueEnd])
			value.Raw = r.Raw[valueStart:valueEnd]

			if !iterator(keyRes, value) {
				return
			}

			// Move to next pair
			pos = valueEnd

			// Skip to comma or end
			for ; pos < len(r.Raw) && (r.Raw[pos] <= ' ' || r.Raw[pos] == ','); pos++ {
				if r.Raw[pos] == ',' {
					pos++
					break
				}
			}
		}
	}
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

	// Handle the case where path starts with a number (direct array access)
	if path[p] >= '0' && path[p] <= '9' {
		// Skip the number
		for p < len(path) && path[p] >= '0' && path[p] <= '9' {
			p++
		}
		// If we're at the end or the next char is '.', it's simple
		if p == len(path) || path[p] == '.' {
			if p < len(path) {
				p++ // skip the dot
			}
		} else {
			return false // Not a valid continuation
		}
	}

	// Path can start with a key or an array index
	if p < len(path) && path[p] != '[' {
		// It's a key, scan until separator
		keyStart := p
		for p < len(path) && path[p] != '.' && path[p] != '[' {
			// check for invalid chars in key
			if path[p] == '*' || path[p] == '?' {
				return false
			}
			p++
		}
		if p == keyStart {
			return false
		} // empty key at start
	}

	for p < len(path) {
		switch path[p] {
		case '.':
			p++ // skip dot
			if p == len(path) {
				return false
			} // trailing dot
			// next must be a key
			keyStart := p
			for p < len(path) && path[p] != '.' && path[p] != '[' {
				if path[p] == '*' || path[p] == '?' {
					return false
				}
				p++
			}
			if p == keyStart {
				return false
			} // empty key
		case '[':
			p++ // skip '['
			if p == len(path) {
				return false
			} // dangling '['
			idxStart := p
			for p < len(path) && path[p] >= '0' && path[p] <= '9' {
				p++
			}
			if p == idxStart {
				return false
			} // empty index `[]`
			if p == len(path) || path[p] != ']' {
				return false
			} // not a number or no closing ']'
			p++ // skip ']'
		default:
			// invalid character
			return false
		}
	}
	return true
}

// stringToBytes converts a string to a byte slice without allocation
func stringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(
		&struct {
			string
			Cap int
		}{s, len(s)},
	))
}

// bytesToString converts a byte slice to a string without allocation
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
