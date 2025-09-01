// Package njson provides high-performance JSON manipulation functions.
// Created by dhawalhost (2025-09-01 13:51:05)
package njson

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// Error definitions for query operations
var (
	ErrInvalidQuery   = errors.New("invalid query syntax")
	ErrTypeConversion = errors.New("cannot convert value to requested type")
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

type resultCacheKey struct {
	hash uint64
	path string
}

// Thread-safe caches and pools
var (
	// Shared buffer pools
	smallBufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 0, 512)
			return &buf
		},
	}

	mediumBufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 0, 4096)
			return &buf
		},
	}

	largeBufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 0, 32768)
			return &buf
		},
	}

	// Result cache for hot paths (thread-safe)
	resultCache sync.Map

	// Path cache for compiled paths (thread-safe)
	pathCache sync.Map

	// Statistics for monitoring
	cacheHits   atomic.Int64
	cacheMisses atomic.Int64
)

//------------------------------------------------------------------------------
// CORE GET IMPLEMENTATION
//------------------------------------------------------------------------------

// Get retrieves a value from JSON using a path expression.
// This is highly optimized with multiple fast paths for common use cases.
func Get(data []byte, path string) Result {
	// Empty path or root path returns the entire document
	if path == "" || path == "$" || path == "@" {
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
func Parse(data []byte) Result {
	// Skip leading whitespace
	start := 0
	for ; start < len(data); start++ {
		if data[start] > ' ' {
			break
		}
	}

	if start >= len(data) {
		return Result{Type: TypeUndefined}
	}

	// Parse based on first character
	switch data[start] {
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
	case '"': // String
		return parseString(data, start)
	case 't': // true
		if start+3 < len(data) &&
			data[start+1] == 'r' &&
			data[start+2] == 'u' &&
			data[start+3] == 'e' { // Corrected typo: data[start+3] == 'e'
			return Result{
				Type:    TypeBoolean,
				Boolean: true,
				Raw:     data[start : start+4],
				Index:   start,
			}
		}
	case 'f': // false
		if start+4 < len(data) &&
			data[start+1] == 'a' &&
			data[start+2] == 'l' &&
			data[start+3] == 's' && // Corrected typo: data[start+3] == 's'
			data[start+4] == 'e' {
			return Result{
				Type:    TypeBoolean,
				Boolean: false,
				Raw:     data[start : start+5],
				Index:   start,
			}
		}
	case 'n': // null
		if start+3 < len(data) &&
			data[start+1] == 'u' &&
			data[start+2] == 'l' &&
			data[start+3] == 'l' {
			return Result{
				Type:  TypeNull,
				Raw:   data[start : start+4],
				Index: start,
			}
		}
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// Number
		return parseNumber(data, start)
	}

	return Result{Type: TypeUndefined}
}

// GetMany executes multiple queries against the same JSON data
func GetMany(data []byte, paths ...string) []Result {
	if len(paths) == 0 {
		return nil
	}

	// For small number of paths, sequential is faster
	if len(paths) <= 3 {
		results := make([]Result, len(paths))
		for i, path := range paths {
			results[i] = Get(data, path)
		}
		return results
	}

	// For many paths, use parallel execution
	results := make([]Result, len(paths))

	// Calculate optimal batch size based on CPU count
	numCPU := runtime.NumCPU()
	batchSize := (len(paths) + numCPU - 1) / numCPU
	if batchSize < 1 {
		batchSize = 1
	}

	// Process in parallel batches
	var wg sync.WaitGroup
	for i := 0; i < len(paths); i += batchSize {
		wg.Add(1)

		go func(start int) {
			defer wg.Done()

			end := start + batchSize
			if end > len(paths) {
				end = len(paths)
			}

			for j := start; j < end; j++ {
				results[j] = Get(data, paths[j])
			}
		}(i)
	}

	wg.Wait()
	return results
}

// getUltraSimplePath is an ultra-fast path for very simple JSON with basic paths
// This handles cases like {"name":"John","age":30} with path "name"
func getUltraSimplePath(data []byte, path string) Result {
	// Skip caching for ultra-simple paths - cache overhead is too high for small operations
	
	// Fast inline search for key pattern: "key":
	keyLen := len(path)
	searchLen := keyLen + 3 // quotes + colon
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
// This is optimized for paths like "user.name" or "items[0].id"
func getSimplePath(data []byte, path string) Result {
	dataStart, dataEnd := 0, len(data)
	p := 0
	
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
			// Inline object value finding for performance
			start, end := fastFindObjectValue(data[dataStart:dataEnd], key)
			if start == -1 {
				return Result{Type: TypeUndefined}
			}
			dataStart += start
			dataEnd = dataStart + (end - start)
		}

		// Check for array access
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

			// Inline array element finding for performance
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

	// Direct parsing of final value (same as getUltraSimplePath approach)
	return fastParseValue(data[dataStart:dataEnd])
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
		pos = findValueEnd(data, pos)
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

// fastFindArrayElement finds an element in an array by index, optimized for performance
func fastFindArrayElement(data []byte, index int) (int, int) {
	// For large indices, use ultra-fast scanning
	if index > 100 {
		return fastFindLargeArrayElement(data, index)
	}
	
	// Skip whitespace
	start := 0
	for ; start < len(data); start++ {
		if data[start] > ' ' {
			break
		}
	}
	if start >= len(data) || data[start] != '[' {
		return -1, -1
	}

	pos := start + 1
	currentIndex := 0
	
	// Optimized scanning for smaller arrays
	for pos < len(data) {
		// Skip whitespace
		for ; pos < len(data) && data[pos] <= ' '; pos++ {
		}
		if pos >= len(data) || data[pos] == ']' {
			return -1, -1
		}
		
		if currentIndex == index {
			// Found target element - find its end
			elementEnd := pos
			if data[pos] == '"' {
				// String value - fast scan
				elementEnd++
				for elementEnd < len(data) && data[elementEnd] != '"' {
					if data[elementEnd] == '\\' {
						elementEnd++
					}
					elementEnd++
				}
				elementEnd++ // Skip closing quote
			} else if data[pos] == '{' {
				// Object - bracket matching
				elementEnd++
				depth := 1
				inString := false
				for elementEnd < len(data) && depth > 0 {
					if !inString {
						if data[elementEnd] == '"' {
							inString = true
						} else if data[elementEnd] == '{' {
							depth++
						} else if data[elementEnd] == '}' {
							depth--
						}
					} else {
						if data[elementEnd] == '\\' {
							elementEnd++
						} else if data[elementEnd] == '"' {
							inString = false
						}
					}
					elementEnd++
				}
			} else if data[pos] == '[' {
				// Array - bracket matching  
				elementEnd++
				depth := 1
				inString := false
				for elementEnd < len(data) && depth > 0 {
					if !inString {
						if data[elementEnd] == '"' {
							inString = true
						} else if data[elementEnd] == '[' {
							depth++
						} else if data[elementEnd] == ']' {
							depth--
						}
					} else {
						if data[elementEnd] == '\\' {
							elementEnd++
						} else if data[elementEnd] == '"' {
							inString = false
						}
					}
					elementEnd++
				}
			} else {
				// Number, boolean, null - scan to delimiter
				for elementEnd < len(data) {
					c := data[elementEnd]
					if c == ',' || c == ']' || c == '}' || c <= ' ' {
						break
					}
					elementEnd++
				}
			}
			return pos, elementEnd
		}
		
		// Skip current element to find next comma
		if data[pos] == '"' {
			// String - fast skip
			pos++
			for pos < len(data) && data[pos] != '"' {
				if data[pos] == '\\' {
					pos++
				}
				pos++
			}
			pos++ // Skip closing quote
		} else if data[pos] == '{' || data[pos] == '[' {
			// Object/Array - use bracket counting
			openChar := data[pos]
			var closeChar byte = '}'
			if openChar == '[' {
				closeChar = ']'
			}
			pos++
			depth := 1
			inString := false
			for pos < len(data) && depth > 0 {
				if !inString {
					if data[pos] == '"' {
						inString = true
					} else if data[pos] == openChar {
						depth++
					} else if data[pos] == closeChar {
						depth--
					}
				} else {
					if data[pos] == '\\' {
						pos++
					} else if data[pos] == '"' {
						inString = false
					}
				}
				pos++
			}
		} else {
			// Primitive - scan to delimiter
			for pos < len(data) {
				c := data[pos]
				if c == ',' || c == ']' || c <= ' ' {
					break
				}
				pos++
			}
		}
		
		// Find next comma or end
		for ; pos < len(data) && data[pos] <= ' '; pos++ {
		}
		if pos >= len(data) || data[pos] == ']' {
			return -1, -1
		}
		if data[pos] == ',' {
			pos++
			currentIndex++
		} else {
			return -1, -1
		}
	}
	
	return -1, -1
}

// Ultra-fast array element finder for large indices (>100)
func fastFindLargeArrayElement(data []byte, targetIndex int) (int, int) {
	// Skip whitespace
	start := 0
	for ; start < len(data); start++ {
		if data[start] > ' ' {
			break
		}
	}
	if start >= len(data) || data[start] != '[' {
		return -1, -1
	}

	pos := start + 1
	currentIndex := 0
	
	// Ultra-fast comma counting - just scan for commas at depth 0
	for pos < len(data) && currentIndex < targetIndex {
		c := data[pos]
		
		if c == '"' {
			// Skip string quickly - just find closing quote
			pos++
			for pos < len(data) {
				if data[pos] == '"' {
					// Check if it's escaped
					escapes := 0
					checkPos := pos - 1
					for checkPos >= 0 && data[checkPos] == '\\' {
						escapes++
						checkPos--
					}
					if escapes%2 == 0 { // Even number of escapes means quote is not escaped
						break
					}
				}
				pos++
			}
			pos++
		} else if c == '{' {
			// Skip object by counting braces
			pos++
			depth := 1
			for pos < len(data) && depth > 0 {
				if data[pos] == '"' {
					pos++
					for pos < len(data) {
						if data[pos] == '"' {
							escapes := 0
							checkPos := pos - 1
							for checkPos >= 0 && data[checkPos] == '\\' {
								escapes++
								checkPos--
							}
							if escapes%2 == 0 {
								break
							}
						}
						pos++
					}
				} else if data[pos] == '{' {
					depth++
				} else if data[pos] == '}' {
					depth--
				}
				pos++
			}
		} else if c == '[' {
			// Skip array by counting brackets
			pos++
			depth := 1
			for pos < len(data) && depth > 0 {
				if data[pos] == '"' {
					pos++
					for pos < len(data) {
						if data[pos] == '"' {
							escapes := 0
							checkPos := pos - 1
							for checkPos >= 0 && data[checkPos] == '\\' {
								escapes++
								checkPos--
							}
							if escapes%2 == 0 {
								break
							}
						}
						pos++
					}
				} else if data[pos] == '[' {
					depth++
				} else if data[pos] == ']' {
					depth--
				}
				pos++
			}
		} else if c == ',' {
			// Found element separator at depth 0
			currentIndex++
			pos++
		} else if c == ']' {
			// End of array
			return -1, -1
		} else {
			// Primitive value or whitespace - just advance
			pos++
		}
	}
	
	if currentIndex != targetIndex {
		return -1, -1
	}
	
	// Skip whitespace to element start
	for pos < len(data) && data[pos] <= ' ' {
		pos++
	}
	
	if pos >= len(data) || data[pos] == ']' {
		return -1, -1
	}
	
	// Now extract the element value using same logic as regular function
	elementStart := pos
	
	if data[pos] == '"' {
		// String value
		pos++
		for pos < len(data) && data[pos] != '"' {
			if data[pos] == '\\' {
				pos++
			}
			pos++
		}
		pos++ // Include closing quote
	} else if data[pos] == '{' {
		// Object value
		pos++
		depth := 1
		inString := false
		for pos < len(data) && depth > 0 {
			if !inString {
				if data[pos] == '"' {
					inString = true
				} else if data[pos] == '{' {
					depth++
				} else if data[pos] == '}' {
					depth--
				}
			} else {
				if data[pos] == '\\' {
					pos++
				} else if data[pos] == '"' {
					inString = false
				}
			}
			pos++
		}
	} else if data[pos] == '[' {
		// Array value
		pos++
		depth := 1
		inString := false
		for pos < len(data) && depth > 0 {
			if !inString {
				if data[pos] == '"' {
					inString = true
				} else if data[pos] == '[' {
					depth++
				} else if data[pos] == ']' {
					depth--
				}
			} else {
				if data[pos] == '\\' {
					pos++
				} else if data[pos] == '"' {
					inString = false
				}
			}
			pos++
		}
	} else {
		// Primitive value (number, boolean, null)
		for pos < len(data) {
			c := data[pos]
			if c == ',' || c == ']' || c == '}' || c <= ' ' {
				break
			}
			pos++
		}
	}
	
	return elementStart, pos
}
			// Object/Array - use bracket counting
			openChar := data[pos]
			var closeChar byte = '}'
			if openChar == '[' {
				closeChar = ']'
			}
			pos++
			depth := 1
			inString := false
			for pos < len(data) && depth > 0 {
				if !inString {
					if data[pos] == '"' {
						inString = true
					} else if data[pos] == openChar {
						depth++
					} else if data[pos] == closeChar {
						depth--
					}
				} else {
					if data[pos] == '\\' {
						pos++
					} else if data[pos] == '"' {
						inString = false
					}
				}
				pos++
			}
		} else {
			// Primitive - scan to delimiter
			for pos < len(data) {
				c := data[pos]
				if c == ',' || c == ']' || c <= ' ' {
					break
				}
				pos++
			}
		}
		
		// Find next comma or end
		for ; pos < len(data) && data[pos] <= ' '; pos++ {
		}
		if pos >= len(data) || data[pos] == ']' {
			return -1, -1
		}
		if data[pos] == ',' {
			pos++
			currentIndex++
		} else {
			return -1, -1
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
				if data[pos] == '\\' {
					pos++ // Skip escaped character
				} else if data[pos] == '"' {
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
				if data[pos] == '\\' {
					pos++ // Skip escaped character
				} else if data[pos] == '"' {
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

// tryLargeArrayPath handles patterns like "items.N.field" efficiently for large arrays
func tryLargeArrayPath(data []byte, path string) Result {
	// Check for pattern: arrayName.number.field (e.g., "items.500.name")
	// This is optimized for the common case of accessing elements in large arrays
	
	firstDot := strings.IndexByte(path, '.')
	if firstDot == -1 {
		return Result{Type: TypeUndefined}
	}
	
	secondDot := strings.IndexByte(path[firstDot+1:], '.')
	if secondDot == -1 {
		return Result{Type: TypeUndefined}
	}
	secondDot += firstDot + 1
	
	arrayName := path[:firstDot]
	indexStr := path[firstDot+1 : secondDot]
	fieldName := path[secondDot+1:]
	
	// Check if middle part is a number
	index := 0
	for i, c := range indexStr {
		if c < '0' || c > '9' {
			return Result{Type: TypeUndefined}
		}
		index = index*10 + int(c-'0')
		// Prevent overflow for very large numbers
		if i > 6 || index > 1000000 {
			return Result{Type: TypeUndefined}
		}
	}
	
	// Only optimize for reasonably large indices (where the optimization matters)
	if index < 50 {
		return Result{Type: TypeUndefined}
	}
	
	// Find the array in the root object
	start, end := fastFindObjectValue(data, arrayName)
	if start == -1 {
		return Result{Type: TypeUndefined}
	}
	
	arrayData := data[start:end]
	
	// Use super-fast array element access for large indices
	elementStart, elementEnd := ultraFastArrayAccess(arrayData, index)
	if elementStart == -1 {
		return Result{Type: TypeUndefined}
	}
	
	elementData := arrayData[elementStart:elementEnd]
	
	// Get the field from the element
	fieldStart, fieldEnd := fastFindObjectValue(elementData, fieldName)
	if fieldStart == -1 {
		return Result{Type: TypeUndefined}
	}
	
	return fastParseValue(elementData[fieldStart:fieldEnd])
}

// ultraFastArrayAccess uses aggressive optimizations for large array indices
func ultraFastArrayAccess(data []byte, index int) (int, int) {
	// Skip whitespace and '['
	pos := 0
	for pos < len(data) && data[pos] <= ' ' {
		pos++
	}
	if pos >= len(data) || data[pos] != '[' {
		return -1, -1
	}
	pos++
	
	currentIndex := 0
	
	// Simple linear scan but optimized for speed
	for pos < len(data) {
		// Skip whitespace
		for pos < len(data) && data[pos] <= ' ' {
			pos++
		}
		if pos >= len(data) || data[pos] == ']' {
			return -1, -1
		}
		
		if currentIndex == index {
			elementEnd := ultraFastSkipElement(data, pos)
			if elementEnd == -1 {
				return -1, -1
			}
			return pos, elementEnd
		}
		
		// Skip to next element using ultra-fast skipping
		pos = ultraFastSkipElement(data, pos)
		if pos == -1 {
			return -1, -1
		}
		
		// Skip to comma
		for pos < len(data) && data[pos] <= ' ' {
			pos++
		}
		if pos >= len(data) || data[pos] == ']' {
			return -1, -1
		}
		if data[pos] == ',' {
			pos++
			currentIndex++
		}
	}
	
	return -1, -1
}

// ultraFastSkipElement skips over a JSON element with minimal overhead
func ultraFastSkipElement(data []byte, start int) int {
	pos := start
	if pos >= len(data) {
		return -1
	}
	
	switch data[pos] {
	case '"': // String - most common case, optimize heavily
		pos++
		for pos < len(data) {
			if data[pos] == '"' {
				return pos + 1
			}
			if data[pos] == '\\' {
				pos += 2 // Skip escaped char
				continue
			}
			pos++
		}
		
	case '{': // Object
		pos++
		depth := 1
		for pos < len(data) && depth > 0 {
			c := data[pos]
			if c == '"' {
				pos++
				for pos < len(data) && data[pos] != '"' {
					if data[pos] == '\\' {
						pos++
					}
					pos++
				}
			} else if c == '{' {
				depth++
			} else if c == '}' {
				depth--
			}
			pos++
		}
		return pos
		
	case '[': // Array
		pos++
		depth := 1
		for pos < len(data) && depth > 0 {
			c := data[pos]
			if c == '"' {
				pos++
				for pos < len(data) && data[pos] != '"' {
					if data[pos] == '\\' {
						pos++
					}
					pos++
				}
			} else if c == '[' {
				depth++
			} else if c == ']' {
				depth--
			}
			pos++
		}
		return pos
		
	default: // Numbers, booleans, null
		for pos < len(data) {
			c := data[pos]
			if c == ',' || c == ']' || c == '}' || c <= ' ' {
				break
			}
			pos++
		}
		return pos
	}
	
	return -1
}

// fastParseValue parses a JSON value directly, optimized for performance (zero allocations)
func fastParseValue(data []byte) Result {
	// Skip whitespace
	start := 0
	for ; start < len(data); start++ {
		if data[start] > ' ' {
			break
		}
	}
	
	if start >= len(data) {
		return Result{Type: TypeUndefined}
	}

	switch data[start] {
	case '"': // String - optimized for zero allocations
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
	case 't': // true
		if start+3 < len(data) &&
			data[start+1] == 'r' &&
			data[start+2] == 'u' &&
			data[start+3] == 'e' {
			return Result{
				Type:    TypeBoolean,
				Boolean: true,
				Raw:     data[start : start+4],
				Index:   start,
			}
		}
	case 'f': // false
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
			}
		}
	case 'n': // null
		if start+3 < len(data) &&
			data[start+1] == 'u' &&
			data[start+2] == 'l' &&
			data[start+3] == 'l' {
			return Result{
				Type:  TypeNull,
				Raw:   data[start : start+4],
				Index: start,
			}
		}
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// Number parsing
		return parseNumber(data, start)
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

// tryCommonFilterPath handles common filter patterns efficiently
// Specifically optimized for patterns like: array[?(@.key=="value")].field
func tryCommonFilterPath(data []byte, path string) Result {
	// Check for pattern: array[?(@.key=="value")].field
	// Example: "phones[?(@.type==\"work\")].number"
	
	if !strings.Contains(path, "[?(@.") {
		return Result{Type: TypeUndefined}
	}
	
	// Parse the pattern manually for performance
	dotIdx := strings.Index(path, ".")
	if dotIdx == -1 {
		return Result{Type: TypeUndefined}
	}
	
	arrayPath := path[:dotIdx] // e.g., "phones[?(@.type==\"work\")]"
	field := path[dotIdx+1:]   // e.g., "number"
	
	// Extract array name and filter condition
	bracketIdx := strings.Index(arrayPath, "[")
	if bracketIdx == -1 {
		return Result{Type: TypeUndefined}
	}
	
	arrayName := arrayPath[:bracketIdx] // e.g., "phones"
	filter := arrayPath[bracketIdx+1:]  // e.g., "?(@.type==\"work\")"
	
	if !strings.HasSuffix(filter, "]") {
		return Result{Type: TypeUndefined}
	}
	filter = filter[:len(filter)-1] // Remove trailing ']'
	
	// Parse filter: ?(@.key=="value")
	if !strings.HasPrefix(filter, "?(@.") || !strings.HasSuffix(filter, ")") {
		return Result{Type: TypeUndefined}
	}
	
	filterContent := filter[3 : len(filter)-1] // Remove "?(@" and ")"
	
	// Parse key=="value" or key!="value"
	var filterKey, filterValue string
	var isEquals bool
	
	if strings.Contains(filterContent, "==\"") {
		parts := strings.SplitN(filterContent, "==\"", 2)
		if len(parts) != 2 {
			return Result{Type: TypeUndefined}
		}
		filterKey = parts[0]
		filterValue = strings.Trim(parts[1], "\"")
		isEquals = true
	} else if strings.Contains(filterContent, "!=\"") {
		parts := strings.SplitN(filterContent, "!=\"", 2)
		if len(parts) != 2 {
			return Result{Type: TypeUndefined}
		}
		filterKey = parts[0]
		filterValue = strings.Trim(parts[1], "\"")
		isEquals = false
	} else {
		return Result{Type: TypeUndefined}
	}
	
	// Now execute the optimized filter
	// 1. Get the array
	arrayResult := fastGetValue(data, arrayName)
	if arrayResult.Type != TypeArray {
		return Result{Type: TypeUndefined}
	}
	
	// 2. Find matching elements
	var matchingElements []Result
	pos := 0
	arrayData := arrayResult.Raw
	
	// Skip whitespace and '['
	for pos < len(arrayData) && arrayData[pos] <= ' ' {
		pos++
	}
	if pos >= len(arrayData) || arrayData[pos] != '[' {
		return Result{Type: TypeUndefined}
	}
	pos++
	
	// Iterate through array elements
	for pos < len(arrayData) {
		// Skip whitespace
		for pos < len(arrayData) && arrayData[pos] <= ' ' {
			pos++
		}
		if pos >= len(arrayData) || arrayData[pos] == ']' {
			break
		}
		
		// Find element end
		elementStart := pos
		elementEnd := findValueEnd(arrayData, pos)
		if elementEnd == -1 {
			break
		}
		
		elementData := arrayData[elementStart:elementEnd]
		
		// Check if this element matches the filter
		filterValueResult := fastGetValue(elementData, filterKey)
		if filterValueResult.Type == TypeString {
			matches := (isEquals && filterValueResult.Str == filterValue) ||
					 (!isEquals && filterValueResult.Str != filterValue)
			
			if matches {
				// Get the field from this element
				fieldResult := fastGetValue(elementData, field)
				if fieldResult.Type != TypeUndefined {
					matchingElements = append(matchingElements, fieldResult)
				}
			}
		}
		
		pos = elementEnd
		
		// Skip to next element or end
		for pos < len(arrayData) && arrayData[pos] != ',' && arrayData[pos] != ']' {
			pos++
		}
		if pos >= len(arrayData) || arrayData[pos] == ']' {
			break
		}
		pos++ // Skip comma
	}
	
	// Return result
	if len(matchingElements) == 0 {
		return Result{Type: TypeUndefined}
	}
	
	// For simple cases, return the first match
	// For full JSONPath compliance, should return array, but gjson returns first match for this pattern
	return matchingElements[0]
}

// fastGetValue gets a simple key from an object or array element, optimized for performance
func fastGetValue(data []byte, key string) Result {
	// Skip whitespace
	start := 0
	for start < len(data) && data[start] <= ' ' {
		start++
	}
	if start >= len(data) {
		return Result{Type: TypeUndefined}
	}
	
	switch data[start] {
	case '{': // Object
		return fastGetObjectField(data[start:], key)
	case '[': // Array
		// Parse key as index
		if idx, err := strconv.Atoi(key); err == nil {
			return fastGetArrayElementByIndex(data[start:], idx)
		}
		return Result{Type: TypeUndefined}
	default:
		return Result{Type: TypeUndefined}
	}
}

// fastGetObjectField gets a field from an object
func fastGetObjectField(data []byte, key string) Result {
	start, end := fastFindObjectValue(data, key)
	if start == -1 {
		return Result{Type: TypeUndefined}
	}
	return fastParseValue(data[start:end])
}

// fastGetArrayElement gets an element from an array by index  
func fastGetArrayElementByIndex(data []byte, index int) Result {
	start, end := fastFindArrayElement(data, index)
	if start == -1 {
		return Result{Type: TypeUndefined}
	}
	return fastParseValue(data[start:end])
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

// tokenizePath breaks a path into tokens for efficient execution
func tokenizePath(path string) []pathToken {
	var tokens []pathToken
	var modifiers []pathToken

	// Check for modifiers
	modifierIdx := strings.IndexByte(path, '|')
	if modifierIdx >= 0 {
		modifierParts := strings.Split(path[modifierIdx+1:], "|")
		for _, part := range modifierParts {
			parts := strings.SplitN(part, ":", 2)
			mod := pathToken{kind: tokenModifier, str: parts[0]}
			if len(parts) > 1 {
				mod.str = parts[0] + ":" + parts[1] // Keep the full modifier
			}
			modifiers = append(modifiers, mod)
		}

		path = path[:modifierIdx]
	}

	// Split the path
	parts := strings.Split(path, ".")

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
	if strings.HasPrefix(expr, "?") {
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
	if strings.HasPrefix(path, "@.") {
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

			// Collect all values
			var values []Result
			current.ForEach(func(_, value Result) bool {
				values = append(values, value)
				return true
			})

			if len(values) == 0 {
				return Result{Type: TypeUndefined}
			}

			// If this is the last token, return array of values
			if i == len(pathTokens)-1 {
				// Create a new array result
				var raw bytes.Buffer
				raw.WriteByte('[')
				for i, val := range values {
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
	case "=", "==":
		return compareEqual(filterValue, filter.value)
	case "!=":
		return !compareEqual(filterValue, filter.value)
	case "<":
		return compareLess(filterValue, filter.value)
	case "<=":
		return compareLess(filterValue, filter.value) || compareEqual(filterValue, filter.value)
	case ">":
		return !compareLess(filterValue, filter.value) && !compareEqual(filterValue, filter.value)
	case ">=":
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
		return value == "null"
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
	case TypeObject:
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

	case TypeArray:
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
	case "string", "str":
		return Result{
			Type:     TypeString,
			Str:      result.String(),
			Raw:      []byte(`"` + escapeString(result.String()) + `"`),
			Modified: true,
		}

	case "number", "num":
		num := result.Float()
		return Result{
			Type:     TypeNumber,
			Num:      num,
			Raw:      []byte(strconv.FormatFloat(num, 'f', -1, 64)),
			Modified: true,
		}

	case "bool", "boolean":
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
			typeStr = "string"
		case TypeNumber:
			typeStr = "number"
		case TypeBoolean:
			typeStr = "boolean"
		case TypeObject:
			typeStr = "object"
		case TypeArray:
			typeStr = "array"
		case TypeNull:
			typeStr = "null"
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

// getObjectValue extracts a value from an object by key
func getObjectValue(data []byte, key string) []byte {
	// Skip whitespace
	start := 0
	for ; start < len(data); start++ {
		if data[start] > ' ' {
			break
		}
	}

	// Check if it's an object
	if start >= len(data) || data[start] != '{' {
		return nil
	}

	// Search for the key
	keyStr := `"` + key + `"`
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
		pos++ // Skip opening quote

		// Find end of key
		for ; pos < len(data) && data[pos] != '"'; pos++ {
			if data[pos] == '\\' {
				pos++ // Skip escape char and the escaped char
			}
		}

		if pos >= len(data) {
			return nil
		}

		keyEnd := pos
		currentKey := data[keyStart : keyEnd+1]

		// Skip to colon
		pos++
		for ; pos < len(data) && data[pos] != ':'; pos++ {
		}

		if pos >= len(data) {
			return nil
		}

		// Skip colon and whitespace
		pos++
		for ; pos < len(data) && data[pos] <= ' '; pos++ {
		}

		if pos >= len(data) {
			return nil
		}

		// Found the value, check if it matches our key
		if string(currentKey) == keyStr || string(currentKey) == `"`+key+`"` {
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
		for ; pos < len(data) && data[pos] != ',' && data[pos] != '}'; pos++ {
		}

		if pos >= len(data) || data[pos] == '}' {
			return nil // End of object, key not found
		}

		pos++ // Skip comma
	}

	return nil
}

// getObjectValueRange returns the start and end indices (relative to data) of the value for key within an object.
// Returns (-1, -1) when not found or data is not an object.
func getObjectValueRange(data []byte, key string) (int, int) {
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
	keyStr := `"` + key + `"`
	pos := start + 1
	for pos < len(data) {
		for ; pos < len(data) && data[pos] <= ' '; pos++ {
		}
		if pos >= len(data) || data[pos] != '"' {
			return -1, -1
		}
		keyStart := pos
		pos++
		for ; pos < len(data) && data[pos] != '"'; pos++ {
			if data[pos] == '\\' {
				pos++
			}
		}
		if pos >= len(data) {
			return -1, -1
		}
		keyEnd := pos
		currentKey := data[keyStart : keyEnd+1]
		pos++
		for ; pos < len(data) && data[pos] != ':'; pos++ {
		}
		if pos >= len(data) {
			return -1, -1
		}
		pos++
		for ; pos < len(data) && data[pos] <= ' '; pos++ {
		}
		if pos >= len(data) {
			return -1, -1
		}
		valueStart := pos
		valueEnd := findValueEnd(data, pos)
		if valueEnd == -1 {
			return -1, -1
		}
		if string(currentKey) == keyStr {
			return valueStart, valueEnd
		}
		pos = valueEnd
		for ; pos < len(data) && data[pos] != ',' && data[pos] != '}'; pos++ {
		}
		if pos >= len(data) || data[pos] == '}' {
			return -1, -1
		}
		pos++
	}
	return -1, -1
}

// getArrayElement extracts an element from an array by index
func getArrayElement(data []byte, index int) []byte {
	// Skip whitespace
	start := 0
	for ; start < len(data); start++ {
		if data[start] > ' ' {
			break
		}
	}

	// Check if it's an array
	if start >= len(data) || data[start] != '[' {
		return nil
	}

	// Iterate through array elements
	pos := start + 1
	currentIndex := 0

	for pos < len(data) {
		// Skip whitespace
		for ; pos < len(data) && data[pos] <= ' '; pos++ {
		}

		if pos >= len(data) || data[pos] == ']' {
			return nil // End of array, index out of bounds
		}

		// Found an element
		if currentIndex == index {
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

		// Skip to next element or end of array
		for ; pos < len(data) && data[pos] != ',' && data[pos] != ']'; pos++ {
		}

		if pos >= len(data) || data[pos] == ']' {
			return nil // End of array, index out of bounds
		}

		pos++ // Skip comma
		currentIndex++
	}

	return nil
}

// getArrayElementRange returns the start and end indices (relative to data) of the element at index within an array.
// Returns (-1, -1) when out of bounds or data is not an array.
func getArrayElementRange(data []byte, index int) (int, int) {
	start := 0
	for ; start < len(data); start++ {
		if data[start] > ' ' {
			break
		}
	}
	if start >= len(data) || data[start] != '[' {
		return -1, -1
	}
	pos := start + 1
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
		pos = findValueEnd(data, pos)
		if pos == -1 {
			return -1, -1
		}
		for ; pos < len(data) && data[pos] != ',' && data[pos] != ']'; pos++ {
		}
		if pos >= len(data) || data[pos] == ']' {
			return -1, -1
		}
		pos++
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

// parseString parses a JSON string
func parseString(data []byte, start int) Result {
	if start >= len(data) || data[start] != '"' {
		return Result{Type: TypeUndefined}
	}

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

	// Extract and unescape the string
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
	var sb strings.Builder
	for i := 0; i < len(str); i++ {
		if str[i] == '\\' && i+1 < len(str) {
			i++
			switch str[i] {
			case '"', '\\', '/', '\'':
				sb.WriteByte(str[i])
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
				if i+4 < len(str) {
					// Parse 4 hex digits
					var r rune
					for j := 1; j <= 4; j++ {
						h := str[i+j-1]
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
							break
						}
						r = r*16 + rune(v)
					}
					sb.WriteRune(r)
					i += 3
				}
			}
		} else {
			sb.WriteByte(str[i])
		}
	}

	return Result{
		Type:  TypeString,
		Str:   sb.String(),
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
	// Skip whitespace
	start := 0
	for ; start < len(data); start++ {
		if data[start] > ' ' {
			break
		}
	}

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
		if start+3 < len(data) &&
			data[start+1] == 'r' &&
			data[start+2] == 'u' &&
			data[start+3] == 'e' { // Corrected typo: data[start+3] == 'e'
			return Result{
				Type:    TypeBoolean,
				Boolean: true,
				Raw:     data[start : start+4],
				Index:   start,
			}
		}
	case 'f':
		if start+4 < len(data) &&
			data[start+1] == 'a' &&
			data[start+2] == 'l' &&
			data[start+3] == 's' && // Corrected typo: data[start+3] == 's'
			data[start+4] == 'e' {
			return Result{
				Type:    TypeBoolean,
				Boolean: false,
				Raw:     data[start : start+5],
				Index:   start,
			}
		}
	case 'n':
		if start+3 < len(data) &&
			data[start+1] == 'u' &&
			data[start+2] == 'l' &&
			data[start+3] == 'l' {
			return Result{
				Type:  TypeNull,
				Raw:   data[start : start+4],
				Index: start,
			}
		}
	default:
		if (data[start] >= '0' && data[start] <= '9') ||
			data[start] == '-' || data[start] == '+' {
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
		return "null"
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
	// Path can start with a key or an array index
	if path[p] != '[' {
		// It's a key, scan until separator
		keyStart := p
		for p < len(path) && path[p] != '.' && path[p] != '[' {
			// check for invalid chars in key
			if path[p] == '*' || path[p] == '?' { return false }
			p++
		}
		if p == keyStart { return false } // empty key at start
	}

	for p < len(path) {
		if path[p] == '.' {
			p++ // skip dot
			if p == len(path) { return false } // trailing dot
			// next must be a key
			keyStart := p
			for p < len(path) && path[p] != '.' && path[p] != '[' {
				if path[p] == '*' || path[p] == '?' { return false }
				p++
			}
			if p == keyStart { return false } // empty key
		} else if path[p] == '[' {
			p++ // skip '['
			if p == len(path) { return false } // dangling '['
			idxStart := p
			for p < len(path) && path[p] >= '0' && path[p] <= '9' {
				p++
			}
			if p == idxStart { return false } // empty index `[]`
			if p == len(path) || path[p] != ']' { return false } // not a number or no closing ']'
			p++ // skip ']'
		} else {
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

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// maxInt returns the maximum of two integers
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// fnv1a computes a fast hash of a byte slice
func fnv1a(data []byte) uint64 {
	var hash uint64 = 0xcbf29ce484222325
	for _, b := range data {
		hash ^= uint64(b)
		hash *= 0x100000001b3
	}
	return hash
}
