// Package njson provides high-performance JSON manipulation functions.
// Created by dhawalhost (2025-09-01 06:41:07)
package njson

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Common errors for set operations
var (
	ErrInvalidPath     = errors.New("invalid path syntax")
	ErrPathNotFound    = errors.New("path not found in document")
	ErrInvalidJSON     = errors.New("invalid json document")
	ErrNoChange        = errors.New("no change detected")
	ErrTypeMismatch    = errors.New("type mismatch between value and destination")
	ErrArrayIndex      = errors.New("array index out of bounds")
	ErrOperationFailed = errors.New("operation failed")
)

// SetOptions represents additional options for set operations
type SetOptions struct {
	// Optimistic indicates the path likely exists for faster operation
	Optimistic bool

	// ReplaceInPlace attempts to modify the byte slice directly instead of allocating
	// a new one. The input JSON will be modified and should not be used afterwards.
	ReplaceInPlace bool

	// MergeArrays causes array values to be merged rather than replaced
	MergeArrays bool

	// MergeObjects causes object values to be merged rather than replaced
	MergeObjects bool

	// Context for cancelable operations
	Context context.Context
}

// DefaultSetOptions provides default settings for set operations
var DefaultSetOptions = SetOptions{
	Optimistic:     false,
	ReplaceInPlace: false,
	MergeArrays:    false,
	MergeObjects:   false,
	Context:        context.Background(),
}

// SetPath represents a pre-compiled path for setting values
type SetPath struct {
	segments []setPathSegment
	original string
	hash     uint64
}

type setPathSegment struct {
	key   string
	index int  // -1 for non-numeric
	last  bool // true if this is the last segment
}

// LRU cache implementation for path compilation
type lruCache struct {
	capacity int
	items    map[string]interface{}
	order    []string
	mutex    sync.RWMutex
}

func newLRUCache(capacity int) *lruCache {
	return &lruCache{
		capacity: capacity,
		items:    make(map[string]interface{}),
		order:    make([]string, 0, capacity),
	}
}

func (c *lruCache) Get(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if val, ok := c.items[key]; ok {
		return val, true
	}
	return nil, false
}

func (c *lruCache) Set(key string, value interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exists := c.items[key]; !exists {
		if len(c.items) >= c.capacity {
			// Evict oldest item
			delete(c.items, c.order[0])
			c.order = c.order[1:]
		}
		c.order = append(c.order, key)
	}
	c.items[key] = value
}

// hashString creates a simple hash of a string
func hashString(s string) uint64 {
	var h uint64 = 14695981039346656037 // FNV offset basis
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211 // FNV prime
	}
	return h
}

// Thread-safe caches for set operations
var (
	setPathCache = newLRUCache(512)
)

// Set sets a value at the specified path in the JSON document.
// This is the main entry point for most use cases.
func Set(json []byte, path string, value interface{}) ([]byte, error) {
	return SetWithOptions(json, path, value, nil)
}

// SetWithOptions sets a value with the specified options
func SetWithOptions(json []byte, path string, value interface{}, options *SetOptions) ([]byte, error) {
	// Handle nil options
	opts := DefaultSetOptions
	if options != nil {
		opts = *options
	}

	// Handle empty path - can't set root
	if path == "" {
		return json, ErrInvalidPath
	}

	// Fast path: byte-level replacement/insert for simple paths (no full unmarshal)
	if isSimpleSetPath(path) && !opts.ReplaceInPlace {
		// Fast delete (compact JSON preferred to keep commas simple)
		if value == nil && !opts.MergeObjects && !opts.MergeArrays {
			if !isLikelyPretty(json) {
				if fast, ok, err := fastDelete(json, path); err == nil && ok {
					return fast, nil
				}
			}
		} else if !opts.MergeObjects && !opts.MergeArrays {
			// Replacement of existing value
			if fast, ok, err := setFastReplace(json, path, value); err == nil && ok {
				return fast, nil
			}
			// Insert new key or append/extend array (compact JSON only)
			if !isLikelyPretty(json) {
				if fast, ok, err := setFastInsertOrAppend(json, path, value); err == nil && ok {
					return fast, nil
				}
				// Deep create nested objects quickly for dot-only object paths
				if fast, ok, err := setFastDeepCreateObjects(json, path, value); err == nil && ok {
					return fast, nil
				}
			}
		}
		// Fallback to generic simple setter which handles creation/merge/deletion with pretty output
		return setSimplePath(json, path, value, opts)
	}

	// Use compiled path for complex paths or when optimizations are requested
	compiledPath, err := CompileSetPath(path)
	if err != nil {
		return json, err
	}

	return SetWithCompiledPath(json, compiledPath, value, &opts)
}

// SetString sets a value in a JSON string and returns the modified string
func SetString(json string, path string, value interface{}) (string, error) {
	result, err := Set([]byte(json), path, value)
	if err != nil {
		return json, err
	}
	return string(result), nil
}

// CompileSetPath compiles a path for repeated set operations
func CompileSetPath(path string) (*SetPath, error) {
	// Check cache first
	if cached, found := setPathCache.Get(path); found {
		return cached.(*SetPath), nil
	}

	segments, err := parseSetPath(path)
	if err != nil {
		return nil, err
	}

	compiled := &SetPath{
		segments: segments,
		original: path,
		hash:     hashString(path),
	}

	// Cache the compiled path
	setPathCache.Set(path, compiled)

	return compiled, nil
}

// SetWithCompiledPath sets a value using a pre-compiled path
func SetWithCompiledPath(json []byte, path *SetPath, value interface{}, options *SetOptions) ([]byte, error) {
	if options == nil {
		options = &DefaultSetOptions
	}

	// Check context cancellation
	if options.Context != nil {
		select {
		case <-options.Context.Done():
			return json, options.Context.Err()
		default:
		}
	}

	// Handle special case of optimistic in-place replacement
	if options.Optimistic && options.ReplaceInPlace {
		result, changed, err := tryOptimisticReplace(json, path, value)
		if err == nil && changed {
			return result, nil
		}
		// Fall through to standard path if optimistic replace fails
	}

	// Process the set operation and return the new bytes
	result, modified, err := setValueWithPath(json, path, value, options)
	if err != nil {
		return json, err
	}
	if !modified {
		return json, nil
	}
	return result, nil
}

// Delete removes a value at the specified path
func Delete(json []byte, path string) ([]byte, error) {
	return DeleteWithOptions(json, path, nil)
}

// DeleteWithOptions removes a value with the specified options
func DeleteWithOptions(json []byte, path string, options *SetOptions) ([]byte, error) {
	// Use nil as the "deletion marker"
	return SetWithOptions(json, path, nil, options)
}

// DeleteString removes a value at the specified path from a JSON string
func DeleteString(json string, path string) (string, error) {
	result, err := Delete([]byte(json), path)
	if err != nil {
		return json, err
	}
	return string(result), nil
}

// isSimpleSetPath checks if a path can be processed without compilation
func isSimpleSetPath(path string) bool {
	// Path shouldn't be empty
	if path == "" {
		return false
	}

	// Check for invalid characters that would indicate complex paths
	for _, c := range path {
		switch c {
		case '|', '*', '?', '#', '(', ')', '=', '!', '<', '>', '~':
			return false
		}
	}

	// Should only contain dots, letters, numbers, and brackets with numbers
	parts := strings.Split(path, ".")
	for _, part := range parts {
		if part == "" {
			continue
		}

		// Check for array notation
		if strings.Contains(part, "[") {
			base := part[:strings.Index(part, "[")]
			// Verify base name is valid
			for _, c := range base {
				if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
					(c >= '0' && c <= '9') || c == '_' || c == '-') {
					return false
				}
			}

			// Extract indexes and verify they're numeric
			start := strings.Index(part, "[")
			for start != -1 && start < len(part) {
				end := strings.Index(part[start:], "]")
				if end == -1 {
					return false
				}
				end += start

				// Check if the index is numeric
				idx := part[start+1 : end]
				for _, c := range idx {
					if c < '0' || c > '9' {
						return false
					}
				}

				// Move to next bracket
				if end+1 < len(part) {
					start = strings.Index(part[end+1:], "[")
					if start != -1 {
						start += end + 1
					}
				} else {
					start = -1
				}
			}
		} else {
			// Simple key - verify it's valid
			for _, c := range part {
				if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
					(c >= '0' && c <= '9') || c == '_' || c == '-') {
					return false
				}
			}
		}
	}

	return true
}

// setFastReplace performs a fast, in-place style replacement by scanning bytes for simple existing paths.
// It does not create missing structure; it only replaces values that already exist.
// Returns (result, ok, err). If ok=false with err=nil, caller should fall back to slower path.
func setFastReplace(data []byte, path string, value interface{}) ([]byte, bool, error) {
	// Limit to reasonably sized docs to keep scans cheap
	if len(data) == 0 {
		return nil, false, nil
	}

	// Split path parts
	parts := strings.Split(path, ".")

	// Track current window of data that contains the target value and its base offset in original data
	window := data
	baseOffset := 0
	// Maintain indexes for reconstructing
	var valueStart, valueEnd int

	// Navigate through keys/indices
	for i, part := range parts {
		if part == "" {
			return nil, false, nil
		}
		isLast := i == len(parts)-1

		// Handle bracket form inside part first: key[index][index]...
		if strings.Contains(part, "[") {
			base := part[:strings.Index(part, "[")]
			if base != "" {
				// find object key value
				s, e := getObjectValueRange(window, base)
				if s < 0 {
					return nil, false, nil
				}
				baseOffset += s
				window = window[s:e]
			}
			// Process each [n]
			idxStart := strings.Index(part, "[")
			for idxStart != -1 {
				idxEnd := strings.Index(part[idxStart+1:], "]")
				if idxEnd == -1 {
					return nil, false, ErrInvalidPath
				}
				idxEnd += idxStart + 1
				idxStr := part[idxStart+1 : idxEnd]
				idx, err := strconv.Atoi(idxStr)
				if err != nil {
					return nil, false, ErrInvalidPath
				}
				s, e := getArrayElementRange(window, idx)
				if s < 0 {
					return nil, false, nil
				}
				baseOffset += s
				window = window[s:e]
				if idxEnd+1 >= len(part) { // no more brackets
					break
				}
				next := strings.Index(part[idxEnd+1:], "[")
				if next == -1 {
					return nil, false, ErrInvalidPath
				}
				idxStart = idxEnd + 1 + next
			}

			if isLast {
				// record value range inside original data
				valueStart = baseOffset
				valueEnd = valueStart + len(window)
				goto replace
			}
			continue
		}

		// Dot numeric segment means array index
		if isAllDigits(part) {
			idx, _ := strconv.Atoi(part)
			s, e := getArrayElementRange(window, idx)
			if s < 0 {
				return nil, false, nil
			}
			baseOffset += s
			window = window[s:e]
			if isLast {
				valueStart = baseOffset
				valueEnd = valueStart + len(window)
			}
			continue
		}

		// Simple key
	s, e := getObjectValueRange(window, part)
	if s < 0 {
			return nil, false, nil
		}
	baseOffset += s
	window = window[s:e]
		if isLast {
			valueStart = baseOffset
			valueEnd = valueStart + len(window)
		}
	}

replace:
	if valueStart <= 0 && valueEnd <= 0 {
		return nil, false, nil
	}

	// Encode new value quickly
	enc, err := fastEncodeJSONValue(value)
	if err != nil {
		return nil, false, err
	}

	// If the new bytes are identical, skip
	if len(enc) == len(window) && bytes.Equal(enc, window) {
		return data, true, nil
	}

	// Build result with minimal allocations
	out := make([]byte, 0, len(data)-len(window)+len(enc))
	out = append(out, data[:valueStart]...)
	out = append(out, enc...)
	out = append(out, data[valueEnd:]...)

	return out, true, nil
}

// isLikelyPretty returns true if the JSON appears to be pretty-printed (contains newlines/indentation)
func isLikelyPretty(data []byte) bool {
	// Heuristic: presence of '\n' or two-space indentation pattern suggests pretty
	if bytes.IndexByte(data, '\n') >= 0 {
		return true
	}
	// Also check for sequences of space after colon that exceed one space
	if bytes.Contains(data, []byte(":  ")) {
		return true
	}
	return false
}

// setFastInsertOrAppend can add a new object field or append/extend an array element when parent exists.
// Returns (result, ok, err). Only supports simple dot paths and compact JSON. No merges or deletions.
func setFastInsertOrAppend(data []byte, path string, value interface{}) ([]byte, bool, error) {
	if len(data) == 0 || value == nil {
		return nil, false, nil
	}
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, false, nil
	}

	// Walk to parent container window
	window := data
	baseOffset := 0
	for i, part := range parts[:len(parts)-1] {
		if part == "" {
			return nil, false, nil
		}
		// bracket form inside part
		if strings.Contains(part, "[") {
			base := part[:strings.Index(part, "[")]
			if base != "" {
				s, e := getObjectValueRange(window, base)
				if s < 0 {
					return nil, false, nil
				}
				baseOffset += s
				window = window[s:e]
			}
			// iterate indices
			idxStart := strings.Index(part, "[")
			for idxStart != -1 {
				idxEnd := strings.Index(part[idxStart+1:], "]")
				if idxEnd == -1 {
					return nil, false, ErrInvalidPath
				}
				idxEnd += idxStart + 1
				idxStr := part[idxStart+1 : idxEnd]
				idx, err := strconv.Atoi(idxStr)
				if err != nil {
					return nil, false, ErrInvalidPath
				}
				s, e := getArrayElementRange(window, idx)
				if s < 0 {
					return nil, false, nil
				}
				baseOffset += s
				window = window[s:e]

				if idxEnd+1 >= len(part) {
					break
				}
				next := strings.Index(part[idxEnd+1:], "[")
				if next == -1 {
					return nil, false, ErrInvalidPath
				}
				idxStart = idxEnd + 1 + next
			}
			continue
		}

		if isAllDigits(part) {
			idx, _ := strconv.Atoi(part)
			s, e := getArrayElementRange(window, idx)
			if s < 0 {
				return nil, false, nil
			}
			baseOffset += s
			window = window[s:e]
			continue
		}

		// simple key
	s, e := getObjectValueRange(window, part)
	if s < 0 {
			return nil, false, nil
		}
	baseOffset += s
	window = window[s:e]
		_ = i
	}

	// Now window is the parent container's value bytes; parentStart..parentEnd in data
	parentStart := baseOffset
	parentEnd := parentStart + len(window)
	if parentStart < 0 || parentEnd > len(data) || parentStart >= parentEnd {
		return nil, false, nil
	}

	// Determine last part and whether parent is object or array
	last := parts[len(parts)-1]
	// Peek first non-space of window to determine type
	ws := 0
	for ws < len(window) && window[ws] <= ' ' {
		ws++
	}
	if ws >= len(window) {
		return nil, false, nil
	}

	// Encode new value
	encVal, err := fastEncodeJSONValue(value)
	if err != nil {
		return nil, false, err
	}

	if window[ws] == '{' {
		// Insert new key if missing
		// First, check if key already exists; if exists, this isn't insert
		keySeg := fastGetObjectValue(window, last)
		if keySeg != nil {
			return nil, false, nil
		}
		// Find insertion point: before closing '}'
		endObj := findBlockEnd(window, ws, '{', '}')
		if endObj == -1 {
			return nil, false, ErrInvalidJSON
		}
		// Build key bytes
		keyJSON, _ := json.Marshal(last)
		// Determine if object currently empty
		inner := bytes.TrimSpace(window[ws+1 : endObj-1])
		needComma := len(inner) > 0
		insert := make([]byte, 0, len(window)+(len(keyJSON)+1+len(encVal)+1+1))
		insert = append(insert, window[:endObj-1]...)
		if needComma {
			insert = append(insert, ',')
		}
		insert = append(insert, keyJSON...)
		insert = append(insert, ':')
		insert = append(insert, encVal...)
		insert = append(insert, window[endObj-1:]...)

		// Splice back into data
		out := make([]byte, 0, len(data)-len(window)+len(insert))
		out = append(out, data[:parentStart]...)
		out = append(out, insert...)
		out = append(out, data[parentEnd:]...)
		return out, true, nil
	}

	if window[ws] == '[' {
		// Array append/extend when last is numeric index >= len(arr)
		if !isAllDigits(last) {
			return nil, false, nil
		}
		// Find array end and current length by scanning elements
		endArr := findBlockEnd(window, ws, '[', ']')
		if endArr == -1 {
			return nil, false, ErrInvalidJSON
		}
		// Compute current length by scanning commas at top-level
		// Quick count: count values separated by commas at depth 0 within [ws+1, endArr-1]
		inner := bytes.TrimSpace(window[ws+1 : endArr-1])
		curLen := 0
		if len(inner) > 0 {
			// count values by simple scan using findValueEnd
			pos := 0
			for pos < len(inner) {
				for pos < len(inner) && inner[pos] <= ' ' {
					pos++
				}
				if pos >= len(inner) {
					break
				}
				curLen++
				ve := findValueEnd(inner, pos)
				if ve == -1 {
					break
				}
				pos = ve
				for pos < len(inner) && inner[pos] != ',' {
					if inner[pos] <= ' ' {
						pos++
						continue
					}
					break
				}
				if pos < len(inner) && inner[pos] == ',' {
					pos++
				}
			}
		}

		targetIdx, _ := strconv.Atoi(last)
		if targetIdx < curLen {
			// not append/extend
			return nil, false, nil
		}

		// Build new array content by inserting values/nulls before closing ']'
		insert := make([]byte, 0, len(window)+32)
		insert = append(insert, window[:endArr-1]...)
		if curLen > 0 {
			insert = append(insert, ',')
		}
		// Add nulls for gaps
		for i := curLen; i < targetIdx; i++ {
			if i > curLen {
				insert = append(insert, ',')
			}
			insert = append(insert, 'n', 'u', 'l', 'l')
		}
		// Comma between last null (if any) and value when targetIdx > curLen
		if targetIdx > curLen {
			insert = append(insert, ',')
		}
		// Finally add value
		insert = append(insert, encVal...)
		insert = append(insert, window[endArr-1:]...)

		out := make([]byte, 0, len(data)-len(window)+len(insert))
		out = append(out, data[:parentStart]...)
		out = append(out, insert...)
		out = append(out, data[parentEnd:]...)
		return out, true, nil
	}

	return nil, false, nil
}

// setFastDeepCreateObjects creates missing nested object keys for dot-only object paths on compact JSON.
// e.g., set "a.b.c" when a exists as object but b/c are missing. It inserts {"b":{"c":value}} in one splice.
func setFastDeepCreateObjects(data []byte, path string, value interface{}) ([]byte, bool, error) {
	if len(data) == 0 || isLikelyPretty(data) {
		return nil, false, nil
	}
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return nil, false, nil
	}
	// Find deepest existing parent object along the path
	window := data
	baseOffset := 0
	lastExisting := -1
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if part == "" || isAllDigits(part) || strings.Contains(part, "[") {
			return nil, false, nil // only object keys supported
		}
		s, e := getObjectValueRange(window, part)
		if s < 0 { // missing here; parent is current window object
			lastExisting = i - 1
			break
		}
		// Move into the existing child; must be object to continue
		// If not object, stop (no fast path)
		// Check first non-space
		k := s
		for k < e && window[k] <= ' ' {
			k++
		}
		if k >= e || window[k] != '{' {
			// child is not object; cannot deep-create purely objects
			return nil, false, nil
		}
		baseOffset += s
		window = window[s:e]
		lastExisting = i
	}
	// If nothing exists along the path, allow creating from root object
	if lastExisting < 0 {
		// Ensure root is an object
		rs := 0
		for rs < len(window) && window[rs] <= ' ' {
			rs++
		}
		if rs >= len(window) || window[rs] != '{' {
			return nil, false, nil
		}
		baseOffset = rs
		window = window[rs:findBlockEnd(window, rs, '{', '}')]
	}
	if lastExisting >= len(parts)-1 {
		return nil, false, nil // nothing to create
	}
	// We have an object window for parent at baseOffset
	parentStart := baseOffset
	parentEnd := parentStart + len(window)
	// Insert nested object chain before closing '}' of parent
	// Build nested: {"k1":{...{"kn":<value>}...}}
	encVal, err := fastEncodeJSONValue(value)
	if err != nil {
		return nil, false, err
	}
	// Build chain from parts[lastExisting+1:]
	keys := parts[lastExisting+1:]
	var buf bytes.Buffer
	for i, k := range keys {
		kb, _ := json.Marshal(k)
		buf.Write(kb)
		buf.WriteByte(':')
		if i < len(keys)-1 {
			buf.WriteByte('{')
		}
	}
	buf.Write(encVal)
	for i := 0; i < len(keys)-1; i++ {
		buf.WriteByte('}')
	}
	// Splice into parent object
	// Find end of object
	ws := 0
	for ws < len(window) && window[ws] <= ' ' {
		ws++
	}
	if ws >= len(window) || window[ws] != '{' {
		return nil, false, nil
	}
	endObj := findBlockEnd(window, ws, '{', '}')
	if endObj == -1 {
		return nil, false, ErrInvalidJSON
	}
	inner := bytes.TrimSpace(window[ws+1 : endObj-1])
	needComma := len(inner) > 0
	insert := make([]byte, 0, len(window)+buf.Len()+3)
	insert = append(insert, window[:endObj-1]...)
	if needComma {
		insert = append(insert, ',')
	}
	insert = append(insert, buf.Bytes()...)
	insert = append(insert, window[endObj-1:]...)
	out := make([]byte, 0, len(data)-len(window)+len(insert))
	out = append(out, data[:parentStart]...)
	out = append(out, insert...)
	out = append(out, data[parentEnd:]...)
	return out, true, nil
}

// fastDelete removes a value at path from compact JSON by splicing bytes.
// For arrays it replaces the element with null (keeps commas consistent).
func fastDelete(data []byte, path string) ([]byte, bool, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, false, nil
	}
	window := data
	baseOffset := 0
	// Navigate to parent
	for _, part := range parts[:len(parts)-1] {
		if strings.Contains(part, "[") {
			base := part[:strings.Index(part, "[")]
			if base != "" {
				s, e := getObjectValueRange(window, base)
				if s < 0 {
					return nil, false, nil
				}
				baseOffset += s
				window = window[s:e]
			}
			idxStart := strings.Index(part, "[")
			for idxStart != -1 {
				idxEnd := strings.Index(part[idxStart+1:], "]")
				if idxEnd == -1 {
					return nil, false, ErrInvalidPath
				}
				idxEnd += idxStart + 1
				idxStr := part[idxStart+1 : idxEnd]
				idx, err := strconv.Atoi(idxStr)
				if err != nil {
					return nil, false, ErrInvalidPath
				}
				s, e := getArrayElementRange(window, idx)
				if s < 0 {
					return nil, false, nil
				}
				baseOffset += s
				window = window[s:e]
				if idxEnd+1 >= len(part) {
					break
				}
				next := strings.Index(part[idxEnd+1:], "[")
				if next == -1 {
					return nil, false, ErrInvalidPath
				}
				idxStart = idxEnd + 1 + next
			}
			continue
		}
		if isAllDigits(part) {
			idx, _ := strconv.Atoi(part)
			s, e := getArrayElementRange(window, idx)
			if s < 0 {
				return nil, false, nil
			}
			baseOffset += s
			window = window[s:e]
			continue
		}
	s, e := getObjectValueRange(window, part)
	if s < 0 {
			return nil, false, nil
		}
	baseOffset += s
	window = window[s:e]
	}

	// Identify parent container type
	parentStart := baseOffset
	parentEnd := parentStart + len(window)
	if parentStart < 0 || parentEnd > len(data) {
		return nil, false, nil
	}
	ws := 0
	for ws < len(window) && window[ws] <= ' ' {
		ws++
	}
	if ws >= len(window) {
		return nil, false, nil
	}
	last := parts[len(parts)-1]

	if window[ws] == '{' {
		// Need to locate the key-value pair and remove it including an optional comma
		// Strategy: scan object entries at depth 1 and find the key.
		keyJSON, _ := json.Marshal(last)
		endOf := findBlockEnd(window, ws, '{', '}')
		if endOf == -1 {
			return nil, false, ErrInvalidJSON
		}
		obj := window[ws:endOf]
		// Scan entries
		pos := 1               // skip '{'
		for pos < len(obj)-1 { // until before '}'
			// key start
			for pos < len(obj) && obj[pos] <= ' ' {
				pos++
			}
			if pos >= len(obj) || obj[pos] != '"' {
				break
			}
			kStart := pos
			pos++
			for pos < len(obj) && obj[pos] != '"' {
				if obj[pos] == '\\' {
					pos++
				}
				pos++
			}
			if pos >= len(obj) {
				break
			}
			kEnd := pos + 1
			// colon
			pos = kEnd
			for pos < len(obj) && obj[pos] != ':' {
				pos++
			}
			if pos >= len(obj) {
				break
			}
			pos++
			for pos < len(obj) && obj[pos] <= ' ' {
				pos++
			}
			vStart := pos
			vEnd := findValueEnd(obj, vStart)
			if vEnd == -1 {
				break
			}
			// Compare key
			if bytes.Equal(obj[kStart:kEnd], keyJSON) {
				// record full span including preceding comma or following comma
				absStart := parentStart + ws + kStart
				absEnd := parentStart + ws + vEnd
				// Expand to include commas/spaces safely
				// Prefer removing trailing comma; if last element, remove preceding comma
				// Look forward for next comma
				tail := window[ws+vEnd : endOf]
				trimEnd := absEnd
				trimStart := absStart
				// Skip whitespace
				tpos := 0
				for tpos < len(tail) && tail[tpos] <= ' ' {
					tpos++
				}
				if tpos < len(tail) && tail[tpos] == ',' {
					// remove trailing comma
					trimEnd += tpos + 1
				} else {
					// remove any preceding comma and optional space
					// find previous comma before kStart
					p := ws + kStart - 1
					for p >= 0 && window[p] <= ' ' {
						p--
					}
					if p >= 0 && window[p] == ',' {
						trimStart = parentStart + p
					}
				}
				out := make([]byte, 0, len(data)-(trimEnd-trimStart))
				out = append(out, data[:trimStart]...)
				out = append(out, data[trimEnd:]...)
				return out, true, nil
			}
			// move past value and comma
			pos = vEnd
			for pos < len(obj) && obj[pos] != ',' && obj[pos] != '}' {
				pos++
			}
			if pos < len(obj) && obj[pos] == ',' {
				pos++
			}
		}
		return nil, false, nil
	}

	if window[ws] == '[' {
		if !isAllDigits(last) {
			return nil, false, nil
		}
		idx, _ := strconv.Atoi(last)
		s, e := getArrayElementRange(window, idx)
		if s < 0 {
			return nil, false, nil
		}
		absStart := parentStart + s
		absEnd := parentStart + e
		out := make([]byte, 0, len(data)-(absEnd-absStart)+4)
		out = append(out, data[:absStart]...)
		out = append(out, 'n', 'u', 'l', 'l')
		out = append(out, data[absEnd:]...)
		return out, true, nil
	}
	return nil, false, nil
}

// fastGetObjectValue returns the raw value bytes for a key within an object slice
func fastGetObjectValue(obj []byte, key string) []byte {
	// Reuse reader from njson_get
	return getObjectValue(obj, key)
}

// fastGetArrayElement returns the raw bytes for idx within an array slice
func fastGetArrayElement(arr []byte, idx int) []byte {
	return getArrayElement(arr, idx)
}

// fastEncodeJSONValue encodes basic Go values to JSON without full marshal when possible
func fastEncodeJSONValue(v interface{}) ([]byte, error) {
	switch val := v.(type) {
	case nil:
		return []byte("null"), nil
	case string:
	return encodeJSONString(val), nil
	case bool:
		if val {
			return []byte("true"), nil
		}
		return []byte("false"), nil
	case int:
		return []byte(strconv.FormatInt(int64(val), 10)), nil
	case int64:
		return []byte(strconv.FormatInt(val, 10)), nil
	case uint64:
		return []byte(strconv.FormatUint(val, 10)), nil
	case float64:
		// Default formatting similar to json.Marshal
		return []byte(strconv.FormatFloat(val, 'f', -1, 64)), nil
	case []byte:
		// Assume raw JSON if parsable; else treat as string
		var tmp interface{}
		if json.Unmarshal(val, &tmp) == nil {
			return val, nil
		}
		return json.Marshal(string(val))
	default:
		// Fallback
		return json.Marshal(v)
	}
}

// encodeJSONString encodes s as a JSON string with minimal allocations
func encodeJSONString(s string) []byte {
	// Fast path: check if escaping is needed
	needsEscape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' || c == '\\' || c < 0x20 {
			needsEscape = true
			break
		}
	}
	if !needsEscape {
		out := make([]byte, 0, len(s)+2)
		out = append(out, '"')
		out = append(out, s...)
		out = append(out, '"')
		return out
	}
	// Escape
	// Worst-case every char becomes \u00XX (6 bytes) + quotes
	out := make([]byte, 0, len(s)*6+2)
	out = append(out, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"', '\\':
			out = append(out, '\\', c)
		case '\b':
			out = append(out, '\\', 'b')
		case '\f':
			out = append(out, '\\', 'f')
		case '\n':
			out = append(out, '\\', 'n')
		case '\r':
			out = append(out, '\\', 'r')
		case '\t':
			out = append(out, '\\', 't')
		default:
			if c < 0x20 {
				// \u00XX
				out = append(out, '\\', 'u', '0', '0')
				const hexdigits = "0123456789abcdef"
				out = append(out, hexdigits[c>>4], hexdigits[c&0xF])
			} else {
				out = append(out, c)
			}
		}
	}
	out = append(out, '"')
	return out
}

// setSimplePath sets a value at a simple path (dot notation or basic array access)
func setSimplePath(json []byte, path string, value interface{}, options SetOptions) ([]byte, error) {
	// Parse the JSON into a generic structure
	var data interface{}
	if err := JSON.Unmarshal(json, &data); err != nil {
		return json, ErrInvalidJSON
	}

	// Split the path into components
	pathParts := strings.Split(path, ".")

	// Navigate to the target location
	current := &data
	var parent interface{}
	var lastKey string
	var lastIndex int
	var isArrayElement bool

	for i, part := range pathParts {
		if part == "" {
			continue
		}

		isLast := i == len(pathParts)-1

		// Handle array access [n]
		if strings.Contains(part, "[") {
			base := part[:strings.Index(part, "[")]

			// Navigate to the base object first
			if base != "" {
				m, ok := (*current).(map[string]interface{})
				if !ok {
					// If not a map, create one
					if isLast && parent != nil {
						// If this is the last segment, set an empty map
						newMap := make(map[string]interface{})
						setInParent(parent, lastKey, lastIndex, isArrayElement, newMap)
						m = newMap
						*current = m
					} else {
						return json, ErrTypeMismatch
					}
				}

				// Update parent tracking
				parent = current
				lastKey = base
				isArrayElement = false

				// Get or create the value at this key
				next, exists := m[base]
				if !exists {
					if isLast {
						// If last component, we'll set it below
						next = make(map[string]interface{})
						m[base] = next
					} else {
						// Create based on next path part
						nextPart := pathParts[i+1]
						if strings.Contains(nextPart, "[") {
							next = make([]interface{}, 0)
						} else {
							next = make(map[string]interface{})
						}
						m[base] = next
					}
				}
				current = &next
			}

			// Process array indexes
			start := strings.Index(part, "[")
			for start != -1 && start < len(part) {
				end := strings.Index(part[start:], "]")
				if end == -1 {
					return json, ErrInvalidPath
				}
				end += start

				// Get array index
				idx, err := strconv.Atoi(part[start+1 : end])
				if err != nil {
					return json, ErrInvalidPath
				}

				// Check if we're at the array or need to create it
				arr, ok := (*current).([]interface{})
				if !ok {
					// If not an array, create one
					if parent != nil {
						newArr := make([]interface{}, 0)
						setInParent(parent, lastKey, lastIndex, isArrayElement, newArr)
						arr = newArr
						*current = arr
					} else {
						return json, ErrTypeMismatch
					}
				}

				// Update parent tracking
				parent = current
				lastIndex = idx
				isArrayElement = true

				// Ensure array has enough elements
				if idx >= len(arr) {
					if isLast && end+1 >= len(part) {
						// If this is the final index and we're setting a value,
						// expand the array to accommodate the new index
						newArr := make([]interface{}, idx+1)
						copy(newArr, arr)
						for i := len(arr); i < idx; i++ {
							newArr[i] = nil
						}
						setInParent(parent, lastKey, lastIndex, isArrayElement, newArr)
						arr = newArr
						*current = arr
					} else {
						return json, ErrArrayIndex
					}
				}

				// Get the value at this index
				next := arr[idx]
				current = &next

				// Check if we need to create a new object/array for the next part
				if !isLast && end+1 >= len(part) && i+1 < len(pathParts) {
					nextPart := pathParts[i+1]
					if next == nil {
						if strings.Contains(nextPart, "[") {
							next = make([]interface{}, 0)
						} else {
							next = make(map[string]interface{})
						}
						arr[idx] = next
						*current = next
					}
				}

				// Move to next bracket if any
				if end+1 < len(part) {
					start = strings.Index(part[end+1:], "[")
					if start != -1 {
						start += end + 1
					}
				} else {
					start = -1
				}
			}
		} else {
			// Support numeric segment as array index
			if isAllDigits(part) {
				idx, _ := strconv.Atoi(part)
				// The holder of the current value (array) is described by the existing parent/lastKey
				holderParent := parent
				holderKey := lastKey

				arr, ok := (*current).([]interface{})
				if !ok {
					return json, ErrTypeMismatch
				}

				// If we need to expand the array for the set, replace it in the holder container
				if idx >= len(arr) {
					if isLast {
						newArr := make([]interface{}, idx+1)
						copy(newArr, arr)
						for i := len(arr); i < idx; i++ {
							newArr[i] = nil
						}
						// write back expanded array into holder (object key usually)
						setInParent(holderParent, holderKey, 0, false, newArr)
						arr = newArr
						*current = arr
					} else {
						return json, ErrArrayIndex
					}
				}

				// Now set context to the array for the element traversal
				parent = arr
				lastIndex = idx
				isArrayElement = true

				next := arr[idx]
				current = &next

				// Create container for next if needed
				if !isLast && next == nil && i+1 < len(pathParts) {
					nextPart := pathParts[i+1]
					var newVal interface{}
					if strings.Contains(nextPart, "[") || isAllDigits(nextPart) {
						newVal = make([]interface{}, 0)
					} else {
						newVal = make(map[string]interface{})
					}
					arr[idx] = newVal
					*current = newVal
				}
			} else {
				// Simple key access
				m, ok := (*current).(map[string]interface{})
				if !ok {
					// If not a map, create one (only if last)
					if isLast && parent != nil {
						newMap := make(map[string]interface{})
						setInParent(parent, lastKey, lastIndex, isArrayElement, newMap)
						m = newMap
						*current = m
					} else {
						return json, ErrTypeMismatch
					}
				}

				parent = current
				lastKey = part
				isArrayElement = false

				next, exists := m[part]
				if !exists {
					if isLast {
						// leave empty; will set later
					} else {
						nextPart := pathParts[i+1]
						if strings.Contains(nextPart, "[") || isAllDigits(nextPart) {
							next = make([]interface{}, 0)
						} else {
							next = make(map[string]interface{})
						}
						m[part] = next
					}
				}
				current = &next
			}
		}
	}

	// Handle deletion
	if value == nil {
		// Use helper to delete or null-out
		if !deleteFromParent(parent, lastKey, lastIndex, isArrayElement) {
			return json, ErrNoChange
		}
	} else {
		// Set the value at the final location
		// Convert the value to a JSON-compatible type
		jsonValue, err := convertToJSONValue(value)
		if err != nil {
			return json, err
		}

		// Optional merge behavior
		if options.MergeObjects && isMap(jsonValue) && parent != nil {
			if existing, ok := getFromParent(parent, lastKey, lastIndex, isArrayElement); ok && isMap(existing) {
				jsonValue = mergeObjects(existing, jsonValue)
			}
		}
		if options.MergeArrays && isSlice(jsonValue) && parent != nil {
			if existing, ok := getFromParent(parent, lastKey, lastIndex, isArrayElement); ok && isSlice(existing) {
				jsonValue = mergeArrays(existing, jsonValue)
			}
		}

		// Set in parent
		if parent != nil {
			setInParent(parent, lastKey, lastIndex, isArrayElement, jsonValue)
		} else {
			data = jsonValue
		}
	}

	// Marshal back to JSON (pretty-printed to match examples/tests)
	result, err := JSON.MarshalIndent(data, "", "  ")
	if err != nil {
		return json, err
	}

	return result, nil
}

// setInParent sets a value in a parent object or array
func setInParent(parent interface{}, key string, index int, isArray bool, value interface{}) {
	switch p := parent.(type) {
	case map[string]interface{}:
		// Direct object write
		p[key] = value
		return
	case []interface{}:
		// Direct array write
		if index >= 0 {
			if index < len(p) {
				p[index] = value
			} else {
				// Expand array to fit index
				newArr := make([]interface{}, index+1)
				copy(newArr, p)
				for i := len(p); i < index; i++ {
					newArr[i] = nil
				}
				newArr[index] = value
				// best effort: cannot reassign original slice reference held elsewhere here
				// so try to update via pointer if we actually received a pointer; otherwise caller should handle
			}
		}
		return
	case *interface{}:
		// Parent is a pointer to an interface holding either a map or slice (common in this package)
		if isArray {
			// Ensure it is a slice
			if (*p) == nil {
				if index < 0 {
					return
				}
				arr := make([]interface{}, index+1)
				arr[index] = value
				*p = arr
				return
			}
			if arr, ok := (*p).([]interface{}); ok {
				if index >= 0 {
					if index < len(arr) {
						arr[index] = value
						*p = arr
						return
					}
					// expand
					newArr := make([]interface{}, index+1)
					copy(newArr, arr)
					for i := len(arr); i < index; i++ {
						newArr[i] = nil
					}
					newArr[index] = value
					*p = newArr
				}
				return
			}
			// not a slice; don't attempt unsafe mutation
			return
		}

		// Map/object path
		if (*p) == nil {
			m := make(map[string]interface{})
			m[key] = value
			*p = m
			return
		}
		if m, ok := (*p).(map[string]interface{}); ok {
			if m == nil {
				m = make(map[string]interface{})
				*p = m
			}
			m[key] = value
		}
		return
	case *map[string]interface{}:
		if *p == nil {
			*p = make(map[string]interface{})
		}
		(*p)[key] = value
		return
	case *[]interface{}:
		if index < 0 {
			return
		}
		if *p == nil {
			arr := make([]interface{}, index+1)
			arr[index] = value
			*p = arr
			return
		}
		arr := *p
		if index < len(arr) {
			arr[index] = value
			return
		}
		newArr := make([]interface{}, index+1)
		copy(newArr, arr)
		for i := len(arr); i < index; i++ {
			newArr[i] = nil
		}
		newArr[index] = value
		*p = newArr
		return
	default:
		// Unknown parent type; no-op to avoid panic
		return
	}
}

// getFromParent returns the child value at key/index from the given parent container
func getFromParent(parent interface{}, key string, index int, isArray bool) (interface{}, bool) {
	if isArray {
		switch p := parent.(type) {
		case []interface{}:
			if index >= 0 && index < len(p) {
				return p[index], true
			}
		case *interface{}:
			if arr, ok := (*p).([]interface{}); ok {
				if index >= 0 && index < len(arr) {
					return arr[index], true
				}
			}
		case *[]interface{}:
			if p != nil && index >= 0 && index < len(*p) {
				return (*p)[index], true
			}
		}
		return nil, false
	}
	switch p := parent.(type) {
	case map[string]interface{}:
		v, ok := p[key]
		return v, ok
	case *interface{}:
		if m, ok := (*p).(map[string]interface{}); ok {
			v, ok2 := m[key]
			return v, ok2
		}
	case *map[string]interface{}:
		if *p == nil {
			return nil, false
		}
		v, ok := (*p)[key]
		return v, ok
	}
	return nil, false
}

// deleteFromParent deletes object key or nulls-out array index; returns true if a change occurred
func deleteFromParent(parent interface{}, key string, index int, isArray bool) bool {
	if isArray {
		switch p := parent.(type) {
		case []interface{}:
			if index >= 0 && index < len(p) {
				p[index] = nil
				return true
			}
		case *interface{}:
			if arr, ok := (*p).([]interface{}); ok {
				if index >= 0 && index < len(arr) {
					arr[index] = nil
					*p = arr
					return true
				}
			}
		case *[]interface{}:
			if p != nil && index >= 0 && index < len(*p) {
				(*p)[index] = nil
				return true
			}
		}
		return false
	}
	switch p := parent.(type) {
	case map[string]interface{}:
		if _, ok := p[key]; ok {
			delete(p, key)
			return true
		}
	case *interface{}:
		if m, ok := (*p).(map[string]interface{}); ok {
			if _, ok2 := m[key]; ok2 {
				delete(m, key)
				*p = m
				return true
			}
		}
	case *map[string]interface{}:
		if *p != nil {
			if _, ok := (*p)[key]; ok {
				delete((*p), key)
				return true
			}
		}
	}
	return false
}

// parseSetPath parses a path string into segments for compiled paths
func parseSetPath(path string) ([]setPathSegment, error) {
	if path == "" {
		return nil, ErrInvalidPath
	}

	var segments []setPathSegment
	parts := strings.Split(path, ".")

	for i, part := range parts {
		if part == "" {
			continue
		}

		// Check if this is the last segment
		isLast := i == len(parts)-1

		// Handle array access [n]
		if strings.Contains(part, "[") {
			base := part[:strings.Index(part, "[")]

			// Add the base key if it exists
			if base != "" {
				segments = append(segments, setPathSegment{
					key:   base,
					index: -1,
					last:  false,
				})
			}

			// Process array indexes
			start := strings.Index(part, "[")
			for start != -1 && start < len(part) {
				end := strings.Index(part[start:], "]")
				if end == -1 {
					return nil, ErrInvalidPath
				}
				end += start

				// Get array index
				idx, err := strconv.Atoi(part[start+1 : end])
				if err != nil {
					return nil, ErrInvalidPath
				}

				// Add the index segment
				isLastIndex := isLast && end+1 >= len(part)
				segments = append(segments, setPathSegment{
					key:   "",
					index: idx,
					last:  isLastIndex,
				})

				// Move to next bracket if any
				if end+1 < len(part) {
					start = strings.Index(part[end+1:], "[")
					if start != -1 {
						start += end + 1
					}
				} else {
					start = -1
				}
			}
		} else {
			// Simple key or numeric index (dot-separated)
			if isAllDigits(part) {
				idx, _ := strconv.Atoi(part)
				segments = append(segments, setPathSegment{key: "", index: idx, last: isLast})
			} else {
				segments = append(segments, setPathSegment{key: part, index: -1, last: isLast})
			}
		}
	}

	return segments, nil
}

// setValueWithPath sets a value using a compiled path
func setValueWithPath(json []byte, path *SetPath, value interface{}, options *SetOptions) ([]byte, bool, error) {
	// Implementation would navigate through the path segments and update the JSON
	// For brevity, this is a simplified version

	// Parse the JSON into a generic structure
	var data interface{}
	if err := JSON.Unmarshal(json, &data); err != nil {
		return nil, false, ErrInvalidJSON
	}

	// Navigate to the target location
	current := &data
	var parent interface{}
	var lastKey string
	var lastIndex int
	var isArrayElement bool

	for i, segment := range path.segments {
		isLast := i == len(path.segments)-1 || segment.last

		if segment.index >= 0 {
			// Array access
			arr, ok := (*current).([]interface{})
			if !ok {
				// Not an array, can't proceed
				if isLast && options.Optimistic {
					return nil, false, ErrNoChange
				}
				return nil, false, ErrTypeMismatch
			}

			// Update parent tracking
			parent = current
			lastIndex = segment.index
			isArrayElement = true

			// Check array bounds
			if segment.index >= len(arr) {
				if isLast && value != nil {
					// Expand array for setting
					newArr := make([]interface{}, segment.index+1)
					copy(newArr, arr)
					for i := len(arr); i < segment.index; i++ {
						newArr[i] = nil
					}
					// Update parent/container
					if p, ok := parent.(*interface{}); ok {
						*p = newArr
						arr = newArr
					} else {
						// Fallbacks
						if parent == &data {
							data = newArr
							arr = newArr
						} else if parentArr, ok2 := parent.([]interface{}); ok2 {
							if lastIndex >= 0 && lastIndex < len(parentArr) {
								parentArr[lastIndex] = newArr
								arr = newArr
							}
						} else if parentMap, ok3 := parent.(map[string]interface{}); ok3 {
							parentMap[lastKey] = newArr
							arr = newArr
						}
					}
				} else {
					return nil, false, ErrArrayIndex
				}
			}

			// Get the array element
			next := arr[segment.index]
			current = &next

			// Create nested structure if needed
			if next == nil && !isLast {
				var newVal interface{}
				if i+1 < len(path.segments) && path.segments[i+1].index >= 0 {
					newVal = make([]interface{}, 0)
				} else {
					newVal = make(map[string]interface{})
				}
				arr[segment.index] = newVal
				*current = newVal
			}
		} else {
			// Object key access
			m, ok := (*current).(map[string]interface{})
			if !ok {
				// Not an object, can't proceed
				if isLast && options.Optimistic {
					return nil, false, ErrNoChange
				}
				return nil, false, ErrTypeMismatch
			}

			// Update parent tracking
			parent = current
			lastKey = segment.key
			isArrayElement = false

			// Get or create the value at this key
			next, exists := m[segment.key]
			if !exists {
				if isLast {
					// If last component, we'll set it below
					break
				} else {
					// Create based on next path segment
					var newVal interface{}
					if i+1 < len(path.segments) && path.segments[i+1].index >= 0 {
						newVal = make([]interface{}, 0)
					} else {
						newVal = make(map[string]interface{})
					}
					m[segment.key] = newVal
					next = newVal
				}
			}
			current = &next
		}
	}

	// Handle deletion
	if value == nil {
		if isArrayElement {
			// For arrays, we need special handling
			if !deleteFromParent(parent, lastKey, lastIndex, true) {
				return nil, false, ErrNoChange
			}
		} else {
			// For objects, just delete the key
			if !deleteFromParent(parent, lastKey, lastIndex, false) {
				return nil, false, ErrNoChange
			}
		}
	} else {
		// Set the value at the final location
		// Convert the value to a JSON-compatible type
		jsonValue, err := convertToJSONValue(value)
		if err != nil {
			return nil, false, err
		}

		// Check if we need to merge
		if options.MergeObjects && isMap(jsonValue) && parent != nil {
			if existing, ok := getFromParent(parent, lastKey, lastIndex, isArrayElement); ok && isMap(existing) {
				merged := mergeObjects(existing, jsonValue)
				setInParent(parent, lastKey, lastIndex, isArrayElement, merged)
				goto marshal
			}
		} else if options.MergeArrays && isSlice(jsonValue) && parent != nil {
			if existing, ok := getFromParent(parent, lastKey, lastIndex, isArrayElement); ok && isSlice(existing) {
				merged := mergeArrays(existing, jsonValue)
				setInParent(parent, lastKey, lastIndex, isArrayElement, merged)
				goto marshal
			}
		}

		// Set in parent
		if parent != nil {
			setInParent(parent, lastKey, lastIndex, isArrayElement, jsonValue)
		} else {
			// Setting the root, which shouldn't happen with valid paths
			data = jsonValue
		}
	}

marshal:
	// Marshal back to JSON (pretty-printed to match examples/tests)
	result, err := JSON.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, false, err
	}

	// Check if anything changed
	if bytes.Equal(json, result) {
		return result, false, nil
	}
	return result, true, nil
}

// isAllDigits returns true if s contains only digit characters
func isAllDigits(s string) bool {
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

// tryOptimisticReplace attempts an in-place replacement for simple cases
func tryOptimisticReplace(json []byte, path *SetPath, value interface{}) ([]byte, bool, error) {
	// This is a specialized function for performance optimization
	// It would directly replace values in the JSON byte slice without parsing
	// the entire document when certain conditions are met

	// For brevity, this is a simplified placeholder
	return json, false, ErrOperationFailed
}

// convertToJSONValue converts a Go value to a JSON-compatible value
func convertToJSONValue(value interface{}) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	// Handle simple types directly
	switch v := value.(type) {
	case string, float64, int, int64, uint64, bool:
		return v, nil
	case []byte:
		// Try to parse as JSON first
		var jsonVal interface{}
		if err := json.Unmarshal(v, &jsonVal); err == nil {
			return jsonVal, nil
		}
		// Fall back to treating as string
		return string(v), nil
	case time.Time:
		return v.Format(time.RFC3339), nil
	}

	// For complex types, marshal and unmarshal to ensure JSON compatibility
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var jsonVal interface{}
	if err := json.Unmarshal(data, &jsonVal); err != nil {
		return nil, err
	}

	return jsonVal, nil
}

// isMap checks if a value is a map[string]interface{}
func isMap(v interface{}) bool {
	_, ok := v.(map[string]interface{})
	return ok
}

// isSlice checks if a value is a []interface{}
func isSlice(v interface{}) bool {
	_, ok := v.([]interface{})
	return ok
}

// mergeObjects combines two objects, with values from the second overriding the first
func mergeObjects(obj1, obj2 interface{}) interface{} {
	map1, ok1 := obj1.(map[string]interface{})
	map2, ok2 := obj2.(map[string]interface{})

	if !ok1 || !ok2 {
		return obj2
	}

	result := make(map[string]interface{})

	// Copy all values from first map
	for k, v := range map1 {
		result[k] = v
	}

	// Merge with second map, recursively for nested objects
	for k, v2 := range map2 {
		if v1, ok := result[k]; ok && isMap(v1) && isMap(v2) {
			result[k] = mergeObjects(v1, v2)
		} else {
			result[k] = v2
		}
	}

	return result
}

// mergeArrays combines two arrays
func mergeArrays(arr1, arr2 interface{}) interface{} {
	slice1, ok1 := arr1.([]interface{})
	slice2, ok2 := arr2.([]interface{})

	if !ok1 || !ok2 {
		return arr2
	}

	result := make([]interface{}, len(slice1)+len(slice2))
	copy(result, slice1)
	copy(result[len(slice1):], slice2)

	return result
}

// JSON provides a configurable JSON implementation
var JSON = struct {
	Marshal       func(v interface{}) ([]byte, error)
	MarshalIndent func(v interface{}, prefix, indent string) ([]byte, error)
	Unmarshal     func(data []byte, v interface{}) error
}{
	Marshal:       json.Marshal,
	MarshalIndent: json.MarshalIndent,
	Unmarshal:     json.Unmarshal,
}

// marshalJSONAccordingToStyle marshals v to JSON preserving original style:
// if src appears compact (no newlines), return compact; else pretty-print.
func marshalJSONAccordingToStyle(src []byte, v interface{}) ([]byte, error) {
	if isLikelyPretty(src) {
		return JSON.MarshalIndent(v, "", "  ")
	}
	return JSON.Marshal(v)
}
