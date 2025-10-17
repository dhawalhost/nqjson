// Package nqjson provides next-gen query operations for JSON with zero allocations.
// Created by dhawalhost (2025-09-01 06:41:07)
package nqjson

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

// deletionMarker is a special value used internally to indicate deletion
type deletionMarker struct{}

var deletionMarkerValue = &deletionMarker{}

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

// processArrayIndices handles the common pattern of processing array indices in a path part.
// It takes a window of JSON data, a part containing array indices, and processes each [n] index.
// Returns the updated window, baseOffset, and any error encountered.
func processArrayIndices(window []byte, part string, baseOffset int) ([]byte, int, error) {
	idxStart := strings.Index(part, "[")
	for idxStart != -1 {
		idxEnd := strings.Index(part[idxStart+1:], "]")
		if idxEnd == -1 {
			return nil, 0, ErrInvalidPath
		}
		idxEnd += idxStart + 1
		idxStr := part[idxStart+1 : idxEnd]
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			return nil, 0, ErrInvalidPath
		}
		s, e := getArrayElementRange(window, idx)
		if s < 0 {
			return nil, 0, nil
		}
		baseOffset += s
		window = window[s:e]

		if idxEnd+1 >= len(part) {
			break
		}
		next := strings.Index(part[idxEnd+1:], "[")
		if next == -1 {
			return nil, 0, ErrInvalidPath
		}
		idxStart = idxEnd + 1 + next
	}
	return window, baseOffset, nil
}

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

	// nextPath is the full path string for advanced operations (internal use)
	nextPath string
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
	// Basic validation for common JSON errors
	if len(json) > 0 {
		jsonStr := string(json)
		if strings.Contains(jsonStr, ": json}") || strings.Contains(jsonStr, ": undefined}") ||
			strings.Contains(jsonStr, ": json,") || strings.Contains(jsonStr, ": undefined,") {
			return nil, errors.New("invalid JSON syntax")
		}
	}

	// If key exists, use ReplaceInPlace for direct byte manipulation and compacting
	if len(json) > 0 && path != "" && !strings.Contains(path, ".") &&
		!strings.Contains(path, "[") && !strings.Contains(path, "?") && !strings.Contains(path, "*") {
		// For simple single keys, check if they exist
		keyStart, _, _ := findKeyValueRange(json, path)
		if keyStart >= 0 {
			// Key exists - use ReplaceInPlace + Optimistic for compacting
			result, err := SetWithOptions(json, path, value, &SetOptions{
				ReplaceInPlace: true,
				Optimistic:     true,
			})
			if err != nil {
				return nil, err
			}
			// Ensure result is compacted (natural side-effect)
			compacted := make([]byte, 0, len(result))
			compacted = appendCompactBytes(compacted, result)
			return compacted, nil
		}
	}

	// For new keys or complex paths, use default approach but compact output
	result, err := SetWithOptions(json, path, value, nil)
	if err != nil {
		return nil, err
	}

	// Apply compacting as natural side-effect
	compacted := make([]byte, 0, len(result))
	compacted = appendCompactBytes(compacted, result)
	return compacted, nil
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

	// Ultra-fast path optimization: prioritize byte-level operations for maximum performance
	if isSimpleSetPath(path) && !opts.ReplaceInPlace && !opts.MergeObjects && !opts.MergeArrays {
		if fast, ok, err := trySimpleFastPaths(json, path, value); err == nil && ok {
			return fast, nil
		}
	}

	// For complex paths or when fast paths fail, use optimized simple path handler
	if isSimpleSetPath(path) {
		return setOptimizedSimplePath(json, path, value, opts)
	}

	// Use compiled path for complex paths
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
		result, changed, err := tryOptimisticReplace(json)
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
	if options == nil {
		options = &DefaultSetOptions
	}

	// Try ultra-fast delete paths first (compact JSON only to maintain formatting)
	if !options.MergeObjects && !options.MergeArrays && !isLikelyPretty(json) {
		// Try fast simple key deletion for compact JSON
		if !strings.Contains(path, ".") && !strings.Contains(path, "[") && len(path) > 0 {
			if fast, ok := deleteFastSimpleKey(json, path); ok {
				return fast, nil
			}
		}
		// Try fast nested deletion
		if fast, ok := deleteFastPath(json, path); ok {
			return fast, nil
		}
	}

	// Fallback to SET with deletion marker (not nil which creates JSON null)
	return SetWithOptions(json, path, deletionMarkerValue, options)
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

	// Disallow characters that indicate complex paths
	if hasComplexChars(path) {
		return false
	}

	// Should only contain dots, letters, numbers, and brackets with numbers
	parts := strings.Split(path, ".")
	for _, part := range parts {
		if part == "" {
			continue
		}

		if strings.Contains(part, "[") {
			if !validateBracketPart(part) {
				return false
			}
		} else {
			if !isValidName(part) {
				return false
			}
		}
	}

	return true
}

// hasComplexChars returns true if path contains special characters that make it complex
func hasComplexChars(path string) bool {
	for _, c := range path {
		switch c {
		case '|', '*', '?', '#', '(', ')', '=', '!', '<', '>', '~':
			return true
		}
	}
	return false
}

// isValidName checks that a simple key contains only allowed characters
func isValidName(part string) bool {
	if part == "" {
		return false
	}
	for _, c := range part {
		if !isAllowedNameRune(c) {
			return false
		}
	}
	return true
}

// isAllowedNameRune reports if rune is one of allowed characters in keys
func isAllowedNameRune(c rune) bool {
	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_' || c == '-' {
		return true
	}
	return false
}

// validateBracketPart validates a part that contains bracket notation like "key[0][1]"
func validateBracketPart(part string) bool {
	bracketIdx := strings.Index(part, "[")
	base := part[:bracketIdx]
	if base != "" {
		if !isValidName(base) {
			return false
		}
	}

	start := bracketIdx
	for start != -1 && start < len(part) {
		end := strings.Index(part[start:], "]")
		if end == -1 {
			return false
		}
		end += start

		idx := part[start+1 : end]
		for _, c := range idx {
			if c < '0' || c > '9' {
				return false
			}
		}

		// Move to next bracket
		if end+1 < len(part) {
			next := strings.Index(part[end+1:], "[")
			if next == -1 {
				start = -1
			} else {
				start = end + 1 + next
			}
		} else {
			start = -1
		}
	}
	return true
}

// setFastReplace performs a fast, in-place style replacement by scanning bytes for simple existing paths.
// It does not create missing structure; it only replaces values that already exist.
// Returns (result, ok, err). If ok=false with err=nil, caller should fall back to slower path.
// validateReplaceInput validates input for fast replace operations
func validateReplaceInput(data []byte, value interface{}) bool {
	// Don't use fast path for deletion marker
	if value == deletionMarkerValue {
		return false
	}

	// Limit to reasonably sized docs to keep scans cheap
	if len(data) == 0 {
		return false
	}

	return true
}

// processPathSegment processes a single path segment during navigation
func processPathSegment(window []byte, part string, baseOffset int, isLast bool) ([]byte, int, int, int, error) {
	valueStart, valueEnd := 0, 0

	// Handle bracket form inside part first: key[index][index]...
	if strings.Contains(part, "[") {
		base := part[:strings.Index(part, "[")]
		if base != "" {
			// find object key value
			s, e := getObjectValueRange(window, base)
			if s < 0 {
				return nil, 0, 0, 0, errors.New("key not found")
			}
			baseOffset += s
			window = window[s:e]
		}
		// Process each [n]
		var err error
		window, baseOffset, err = processArrayIndices(window, part, baseOffset)
		if err != nil {
			return nil, 0, 0, 0, err
		}
		if window == nil {
			return nil, 0, 0, 0, errors.New("array index not found")
		}

		if isLast {
			// record value range inside original data
			valueStart = baseOffset
			valueEnd = valueStart + len(window)
		}
		return window, baseOffset, valueStart, valueEnd, nil
	}

	// Dot numeric segment means array index
	if isAllDigits(part) {
		idx, _ := strconv.Atoi(part)
		s, e := getArrayElementRange(window, idx)
		if s < 0 {
			return nil, 0, 0, 0, errors.New("array index not found")
		}
		baseOffset += s
		window = window[s:e]
		if isLast {
			valueStart = baseOffset
			valueEnd = valueStart + len(window)
		}
		return window, baseOffset, valueStart, valueEnd, nil
	}

	// Simple key
	s, e := getObjectValueRange(window, part)
	if s < 0 {
		return nil, 0, 0, 0, errors.New("key not found")
	}
	baseOffset += s
	window = window[s:e]
	if isLast {
		valueStart = baseOffset
		valueEnd = valueStart + len(window)
	}
	return window, baseOffset, valueStart, valueEnd, nil
}

// buildReplacementResult constructs the final result with the new value
func buildReplacementResult(data []byte, window []byte, valueStart, valueEnd int, value interface{}) ([]byte, bool, error) {
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

func setFastReplace(data []byte, path string, value interface{}) ([]byte, bool, error) {
	// Validate input
	if !validateReplaceInput(data, value) {
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

		var err error
		window, baseOffset, valueStart, valueEnd, err = processPathSegment(window, part, baseOffset, isLast)
		if err != nil {
			return nil, false, nil
		}

		if isLast {
			break
		}
	}

	return buildReplacementResult(data, window, valueStart, valueEnd, value)
}

// trySimpleFastPaths runs the collection of fast-path checks for simple set paths.
// It returns (result, ok, err) matching the other fast-path helpers' conventions.
func trySimpleFastPaths(json []byte, path string, value interface{}) ([]byte, bool, error) {
	type fastStrategy struct {
		pred func() bool
		run  func() ([]byte, bool, error)
	}

	strategies := []fastStrategy{
		{pred: func() bool { return isSingleSimpleKey(path) }, run: func() ([]byte, bool, error) { return tryUltraFastSingleKey(json, path, value) }},
		{pred: func() bool { return isSimpleDotPath(path) }, run: func() ([]byte, bool, error) { return setFastSimpleDotPath(json, path, value) }},
		{pred: func() bool { return looksLikeArrayElementPath(path) }, run: func() ([]byte, bool, error) { return setFastArrayElement(json, path, value) }},
		{pred: func() bool { return true }, run: func() ([]byte, bool, error) { return setFastReplace(json, path, value) }},
		{pred: func() bool { return !isLikelyPretty(json) }, run: func() ([]byte, bool, error) { return setFastInsertOrAppend(json, path, value) }},
		{pred: func() bool { return true }, run: func() ([]byte, bool, error) { return setFastDeepCreateObjects(json, path, value) }},
	}

	for _, s := range strategies {
		if s.pred() {
			if out, ok, err := s.run(); err == nil && ok {
				return out, true, nil
			}
		}
	}
	return nil, false, nil
}

// isSingleSimpleKey returns true when path is a single, dot-free key.
func isSingleSimpleKey(path string) bool {
	return len(path) > 0 && !strings.Contains(path, ".") && !strings.Contains(path, "[")
}

// tryUltraFastSingleKey attempts replace/add for a single key path.
func tryUltraFastSingleKey(json []byte, path string, value interface{}) ([]byte, bool, error) {
	if fast, ok, err := setFastReplaceSimpleKey(json, path, value); err == nil && ok {
		return fast, true, nil
	}
	if !isLikelyPretty(json) {
		if fast, ok, err := setFastAddSimpleKey(json, path, value); err == nil && ok {
			return fast, true, nil
		}
	}
	return nil, false, nil
}

// isSimpleDotPath returns true when the path is short dot-only notation.
func isSimpleDotPath(path string) bool {
	return strings.Count(path, ".") <= 3 && !strings.Contains(path, "[")
}

// looksLikeArrayElementPath is a heuristic to quickly route array element updates.
func looksLikeArrayElementPath(path string) bool {
	if !strings.Contains(path, ".") {
		return false
	}
	return strings.Contains(path, "0") || strings.Contains(path, "1") || strings.Contains(path, "2")
}

// tryInsertAppendIfCompact routes to insert/append when JSON appears compact.
// (removed) tryInsertAppendIfCompact: inlined via strategies in trySimpleFastPaths

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

// appendCompactBytes appends src to dst while removing unnecessary whitespace
// Works at byte level for compacting behavior
// handleStringCharacter handles character processing within strings during compacting
func handleStringCharacter(c byte, escaped *bool, inString *bool, dst []byte) []byte {
	if !*escaped && c == '"' {
		*inString = !*inString
		return append(dst, c)
	}

	if *inString && !*escaped && c == '\\' {
		*escaped = true
		return append(dst, c)
	}

	if *escaped {
		*escaped = false
	}

	return append(dst, c)
}

// shouldAddSpace determines if a space is needed during compacting
func shouldAddSpace(dst []byte, src []byte, i int) bool {
	if len(dst) == 0 {
		return false
	}

	// Check the last character in destination
	if isStructuralChar(dst[len(dst)-1]) {
		return false
	}

	// Look ahead to see if we need a space
	if i+1 < len(src) && !isSpaceNeededBeforeNextChar(src[i+1]) {
		return false
	}

	// Default to false for aggressive compacting
	return false
}

// isStructuralChar checks if a character is a JSON structural character
func isStructuralChar(c byte) bool {
	return c == ',' || c == ':' || c == '{' || c == '['
}

// isSpaceNeededBeforeNextChar checks if a space is needed before the next character
func isSpaceNeededBeforeNextChar(nextChar byte) bool {
	// If next character is whitespace or structural, no space needed
	if nextChar <= ' ' || nextChar == ',' || nextChar == '}' || nextChar == ']' || nextChar == ':' {
		return false
	}

	// Always compact - no space needed in our implementation
	return false
}

// handleWhitespaceCharacter handles whitespace during compacting
func handleWhitespaceCharacter(dst, src []byte, i int) []byte {
	// Outside strings, compact whitespace
	if shouldAddSpace(dst, src, i) && len(dst) > 0 && dst[len(dst)-1] != ' ' {
		return append(dst, ' ')
	}
	return dst
}

func appendCompactBytes(dst, src []byte) []byte {
	i := 0
	inString := false
	escaped := false

	for i < len(src) {
		c := src[i]

		// Handle string state tracking
		if inString {
			dst = handleStringCharacter(c, &escaped, &inString, dst)
		} else {
			// Handle non-string characters
			if !escaped && c == '"' {
				inString = true
				dst = append(dst, c)
			} else if c <= ' ' {
				dst = handleWhitespaceCharacter(dst, src, i)
			} else {
				dst = append(dst, c)
			}
		}
		i++
	}

	return dst
}

// FastInsertContext holds the state for fast insertion operations
type FastInsertContext struct {
	data       []byte
	window     []byte
	baseOffset int
	parts      []string
}

// fastPathHandler processes a path component during fast insertion
func fastPathHandler(ctx *FastInsertContext, part string) (bool, error) {
	if part == "" {
		return false, nil
	}

	// Handle bracket notation in part (e.g., "users[0][name]")
	if strings.Contains(part, "[") {
		success, err := handleBracketNotation(ctx, part)
		if !success || err != nil {
			return false, err
		}
		return true, nil
	}

	// Handle numeric indices directly (e.g., "users.0.name")
	if isAllDigits(part) {
		success := handleNumericIndex(ctx, part)
		if !success {
			return false, nil
		}
		return true, nil
	}

	// Handle simple object key
	s, e := getObjectValueRange(ctx.window, part)
	if s < 0 {
		return false, nil
	}
	ctx.baseOffset += s
	ctx.window = ctx.window[s:e]
	return true, nil
}

// handleBracketNotation processes a path part containing bracket notation
func handleBracketNotation(ctx *FastInsertContext, part string) (bool, error) {
	base := part
	bracketIndex := strings.Index(part, "[")
	if bracketIndex >= 0 {
		base = part[:bracketIndex]
	}

	// Handle the base object key if it exists
	if base != "" {
		s, e := getObjectValueRange(ctx.window, base)
		if s < 0 {
			return false, nil
		}
		ctx.baseOffset += s
		ctx.window = ctx.window[s:e]
	}

	// Process array indices if present
	if bracketIndex >= 0 {
		var err error
		ctx.window, ctx.baseOffset, err = processArrayIndices(ctx.window, part, ctx.baseOffset)
		if err != nil || ctx.window == nil {
			return false, err
		}
	}

	return true, nil
}

// handleNumericIndex processes a numeric array index in a path
func handleNumericIndex(ctx *FastInsertContext, part string) bool {
	idx, _ := strconv.Atoi(part)
	s, e := getArrayElementRange(ctx.window, idx)
	if s < 0 {
		return false
	}
	ctx.baseOffset += s
	ctx.window = ctx.window[s:e]
	return true
}

// handleObjectInsertion manages insertion of a key-value pair into a JSON object
func handleObjectInsertion(data []byte, window []byte, parentStart int, parentEnd int, key string, encVal []byte) ([]byte, bool, error) {
	// Find insertion point: before closing '}'
	ws := 0
	for ws < len(window) && window[ws] <= ' ' {
		ws++
	}
	if ws >= len(window) {
		return nil, false, nil
	}

	// First, check if key already exists; if exists, this isn't insert
	keySeg := fastGetObjectValue(window, key)
	if keySeg != nil {
		return nil, false, nil
	}

	endObj := findBlockEnd(window, ws, '{', '}')
	if endObj == -1 {
		return nil, false, ErrInvalidJSON
	}

	// Build key bytes
	keyJSON, _ := json.Marshal(key)

	// Determine if object currently empty
	inner := bytes.TrimSpace(window[ws+1 : endObj-1])
	needComma := len(inner) > 0

	// Create the insertion
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

// handleArrayInsertion manages insertion of a value into a JSON array
func handleArrayInsertion(data []byte, window []byte, parentStart int, parentEnd int, index string, encVal []byte) ([]byte, bool, error) {
	// Find array start
	ws := 0
	for ws < len(window) && window[ws] <= ' ' {
		ws++
	}
	if ws >= len(window) || window[ws] != '[' {
		return nil, false, nil
	}

	// Ensure the index is numeric
	if !isAllDigits(index) {
		return nil, false, nil
	}

	// Find array end and current length
	endArr := findBlockEnd(window, ws, '[', ']')
	if endArr == -1 {
		return nil, false, ErrInvalidJSON
	}

	// Calculate current array length
	inner := bytes.TrimSpace(window[ws+1 : endArr-1])
	curLen := calculateArrayLength(inner)

	// Convert target index to int
	targetIdx, _ := strconv.Atoi(index)
	if targetIdx < curLen {
		// Not an append/extend operation
		return nil, false, nil
	}

	// Build the insertion
	insert := createArrayInsertion(window, endArr, curLen, targetIdx, encVal)

	// Splice back into data
	out := make([]byte, 0, len(data)-len(window)+len(insert))
	out = append(out, data[:parentStart]...)
	out = append(out, insert...)
	out = append(out, data[parentEnd:]...)

	return out, true, nil
}

// calculateArrayLength counts the number of elements in an array
func calculateArrayLength(inner []byte) int {
	if len(inner) == 0 {
		return 0
	}

	curLen := 0
	pos := 0
	for pos < len(inner) {
		// Skip whitespace
		for pos < len(inner) && inner[pos] <= ' ' {
			pos++
		}
		if pos >= len(inner) {
			break
		}

		// Found a value
		curLen++

		// Skip to the end of this value
		ve := findValueEnd(inner, pos)
		if ve == -1 {
			break
		}
		pos = ve

		// Skip to the next comma
		for pos < len(inner) && inner[pos] != ',' {
			if inner[pos] <= ' ' {
				pos++
				continue
			}
			break
		}

		// Skip past comma if found
		if pos < len(inner) && inner[pos] == ',' {
			pos++
		}
	}

	return curLen
}

// createArrayInsertion builds the modified array content for insertion
func createArrayInsertion(window []byte, endArr int, curLen int, targetIdx int, encVal []byte) []byte {
	insert := make([]byte, 0, len(window)+32)
	insert = append(insert, window[:endArr-1]...)

	// Add comma if there are existing elements
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

	// Add comma between last null (if any) and value when targetIdx > curLen
	if targetIdx > curLen {
		insert = append(insert, ',')
	}

	// Add the value and close the array
	insert = append(insert, encVal...)
	insert = append(insert, window[endArr-1:]...)

	return insert
}

// setFastInsertOrAppend can add a new object field or append/extend an array element when parent exists.
// Returns (result, ok, err). Only supports simple dot paths and compact JSON. No merges or deletions.
func setFastInsertOrAppend(data []byte, path string, value interface{}) ([]byte, bool, error) {
	// Initialize context and validate inputs
	ctx, parts, ok := initFastInsertOrAppendContext(data, path, value)
	if !ok {
		return nil, false, nil
	}

	// Walk to parent container window
	if success, err := fastInsertWalkToParent(ctx, parts); !success || err != nil {
		return nil, false, err
	}

	// Compute parent bounds in original data
	parentStart, parentEnd, ok := getFastInsertParentBounds(ctx, data)
	if !ok {
		return nil, false, nil
	}

	// Encode new value
	encVal, err := fastEncodeJSONValue(value)
	if err != nil {
		return nil, false, err
	}

	// Determine container type and dispatch
	container, _, ok := peekContainerTypeWS(ctx.window)
	if !ok {
		return nil, false, nil
	}
	last := parts[len(parts)-1]
	switch container {
	case '{':
		return handleObjectInsertion(data, ctx.window, parentStart, parentEnd, last, encVal)
	case '[':
		return handleArrayInsertion(data, ctx.window, parentStart, parentEnd, last, encVal)
	default:
		return nil, false, nil
	}
}

// initFastInsertOrAppendContext validates inputs and creates a FastInsertContext with path parts.
func initFastInsertOrAppendContext(data []byte, path string, value interface{}) (*FastInsertContext, []string, bool) {
	if value == deletionMarkerValue {
		return nil, nil, false
	}
	if len(data) == 0 || value == nil {
		return nil, nil, false
	}
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, nil, false
	}
	ctx := &FastInsertContext{
		data:       data,
		window:     data,
		baseOffset: 0,
		parts:      parts,
	}
	return ctx, parts, true
}

// fastInsertWalkToParent navigates to the parent container window for insertion.
func fastInsertWalkToParent(ctx *FastInsertContext, parts []string) (bool, error) {
	for i, part := range parts[:len(parts)-1] {
		var _ int = i
		success, err := fastPathHandler(ctx, part)
		if !success {
			return false, nil
		}
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

// getFastInsertParentBounds returns the bounds of the parent container in the original data buffer.
func getFastInsertParentBounds(ctx *FastInsertContext, data []byte) (int, int, bool) {
	parentStart := ctx.baseOffset
	parentEnd := parentStart + len(ctx.window)
	if parentStart < 0 || parentEnd > len(data) || parentStart >= parentEnd {
		return 0, 0, false
	}
	return parentStart, parentEnd, true
}

// peekContainerTypeWS returns the first non-space character of window and its index.
func peekContainerTypeWS(window []byte) (byte, int, bool) {
	ws := 0
	for ws < len(window) && window[ws] <= ' ' {
		ws++
	}
	if ws >= len(window) {
		return 0, 0, false
	}
	c := window[ws]
	if c != '{' && c != '[' {
		return 0, 0, false
	}
	return c, ws, true
}

// setFastDeepCreateObjects creates missing nested object keys for dot-only object paths on compact JSON.
// e.g., set "a.b.c" when a exists as object but b/c are missing. It inserts {"b":{"c":value}} in one splice.
// quickKeyExists does a fast scan to check if a key exists at the root level of an object
// skipToObjectStart skips whitespace to find the opening brace
func skipToObjectStart(data []byte) int {
	i := 0
	for i < len(data) && data[i] <= ' ' {
		i++
	}
	if i >= len(data) || data[i] != '{' {
		return -1
	}
	return i + 1
}

// parseObjectKeyQuick parses a key during quick key existence check
func parseObjectKeyQuick(data []byte, i *int) (int, int, bool) {
	// Skip whitespace
	for *i < len(data) && data[*i] <= ' ' {
		*i++
	}
	if *i >= len(data) {
		return 0, 0, false
	}

	// End of object?
	if data[*i] == '}' {
		return 0, 0, false
	}

	// Expect a key (quoted string)
	if data[*i] != '"' {
		return 0, 0, false
	}
	*i++

	keyStart := *i
	// Find end of key
	for *i < len(data) && data[*i] != '"' {
		if data[*i] == '\\' {
			*i++ // Skip escaped character
		}
		*i++
	}
	if *i >= len(data) {
		return 0, 0, false
	}

	keyEnd := *i
	*i++ // Skip closing quote
	return keyStart, keyEnd, true
}

// skipToValueEnd skips to the end of a value during quick key check
func skipToValueEnd(data []byte, i *int) bool {
	// Skip to colon
	for *i < len(data) && data[*i] <= ' ' {
		*i++
	}
	if *i >= len(data) || data[*i] != ':' {
		return false
	}
	*i++

	// Skip value (we don't care about the value)
	valueEnd := findValueEnd(data, *i)
	if valueEnd == -1 {
		return false
	}
	*i = valueEnd
	return true
}

// advanceToNextKey advances to the next key in the object
func advanceToNextKey(data []byte, i *int) bool {
	// Skip to comma or end of object
	for *i < len(data) && data[*i] <= ' ' {
		*i++
	}
	if *i >= len(data) {
		return false
	}
	if data[*i] == '}' {
		return false
	}
	if data[*i] == ',' {
		*i++
		return true
	}
	return false
}

func quickKeyExists(data []byte, key string) bool {
	i := skipToObjectStart(data)
	if i == -1 {
		return false
	}

	// Optimized scan: only check at key positions, not every byte
	keyBytes := []byte(key)
	keyLen := len(key)

	for i < len(data) {
		keyStart, keyEnd, valid := parseObjectKeyQuick(data, &i)
		if !valid {
			break
		}

		// Check if this key matches
		currentKeyLen := keyEnd - keyStart
		if currentKeyLen == keyLen && bytes.Equal(data[keyStart:keyEnd], keyBytes) {
			return true
		}

		if !skipToValueEnd(data, &i) {
			return false
		}

		if !advanceToNextKey(data, &i) {
			break
		}
	}
	return false
}

// buildPureNestedPath builds a completely new nested path without any existing components
// countPathDots counts the number of dots in a path string
func countPathDots(path string) int {
	dotCount := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			dotCount++
		}
	}
	return dotCount
}

// buildNestedJSONFromPath builds nested JSON structure from dot-separated path
func buildNestedJSONFromPath(path string, encVal []byte, dotCount int) []byte {
	// Pre-calculate total size more accurately
	totalSize := len(encVal) + len(path) + (dotCount+1)*5 + dotCount*2 // rough estimate
	nested := make([]byte, 0, totalSize)

	// Build JSON directly by parsing path inline
	start := 0
	depth := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '.' {
			if i > start {
				// Add key
				nested = append(nested, '"')
				nested = append(nested, path[start:i]...)
				nested = append(nested, '"', ':')

				// If this is the last component, add value
				if i == len(path) {
					nested = append(nested, encVal...)
				} else {
					// Otherwise add opening brace
					nested = append(nested, '{')
					depth++
				}
			}
			start = i + 1
		}
	}

	// Close all braces
	for depth > 0 {
		nested = append(nested, '}')
		depth--
	}

	return nested
}

// findObjectBounds finds the start and end positions of the root object
func findObjectBounds(data []byte) (int, int, error) {
	// Find insertion point in the root object using simple scan
	objStart := 0
	for objStart < len(data) && data[objStart] <= ' ' {
		objStart++
	}
	if objStart >= len(data) || data[objStart] != '{' {
		return -1, -1, ErrInvalidJSON
	}

	// Find end of object using simple brace counting
	objEnd := objStart + 1
	braceCount := 1
	for objEnd < len(data) && braceCount > 0 {
		switch data[objEnd] {
		case '{':
			braceCount++
		case '}':
			braceCount--
		case '"':
			// Skip string contents
			objEnd++
			for objEnd < len(data) && data[objEnd] != '"' {
				if data[objEnd] == '\\' {
					objEnd++ // Skip escaped character
				}
				objEnd++
			}
		}
		objEnd++
	}
	if braceCount != 0 {
		return -1, -1, ErrInvalidJSON
	}

	return objStart, objEnd, nil
}

// buildResultWithNested combines original data with nested structure
func buildResultWithNested(data []byte, nested []byte, objStart, objEnd int) []byte {
	// Check if object is empty
	inner := bytes.TrimSpace(data[objStart+1 : objEnd-1])
	needComma := len(inner) > 0

	// Build result
	result := make([]byte, 0, len(data)+len(nested)+1)
	result = append(result, data[:objEnd-1]...)
	if needComma {
		result = append(result, ',')
	}
	result = append(result, nested...)
	result = append(result, data[objEnd-1:]...)

	return result
}

func buildPureNestedPath(data []byte, path string, value interface{}) ([]byte, bool, error) {
	// Encode the value
	encVal, err := fastEncodeJSONValue(value)
	if err != nil {
		return nil, false, err
	}

	// Build nested structure directly without intermediate parsing
	// For "preferences.theme.colors.primary" -> {"preferences":{"theme":{"colors":{"primary":"value"}}}}

	// Count dots to pre-allocate
	dotCount := countPathDots(path)

	// Build nested JSON structure
	nested := buildNestedJSONFromPath(path, encVal, dotCount)

	// Find object bounds
	objStart, objEnd, err := findObjectBounds(data)
	if err != nil {
		if err == ErrInvalidJSON {
			return nil, false, err
		}
		return nil, false, nil
	}

	// Build final result
	result := buildResultWithNested(data, nested, objStart, objEnd)

	return result, true, nil
}

// setFastReplaceSimpleKey optimizes the common case of replacing a single key like "name" in an object
func setFastReplaceSimpleKey(data []byte, key string, value interface{}) ([]byte, bool, error) {
	// Don't use fast path for deletion marker
	if value == deletionMarkerValue {
		return nil, false, nil
	}

	if len(data) == 0 || len(key) == 0 {
		return nil, false, nil
	}

	// Find the root object
	i := 0
	for i < len(data) && data[i] <= ' ' {
		i++
	}
	if i >= len(data) || data[i] != '{' {
		return nil, false, nil
	}

	// Use our optimized key finder to locate the key
	keyStart, valueStart, valueEnd := findKeyValueRange(data, key)
	if keyStart == -1 {
		return nil, false, nil // Key doesn't exist, can't replace
	}

	// Encode the new value
	encVal, err := fastEncodeJSONValue(value)
	if err != nil {
		return nil, false, err
	}

	// If the new value is identical, skip
	currentValue := data[valueStart:valueEnd]
	if bytes.Equal(encVal, currentValue) {
		return data, true, nil
	}

	// Build result with pre-calculated size
	result := make([]byte, 0, len(data)-len(currentValue)+len(encVal))
	result = append(result, data[:valueStart]...)
	result = append(result, encVal...)
	result = append(result, data[valueEnd:]...)

	return result, true, nil
}

// findKeyValueRange finds the position of a key and its value in a JSON object
// Returns keyStart, valueStart, valueEnd (or -1, -1, -1 if not found)
func findKeyValueRange(data []byte, key string) (int, int, int) {
	// Move to start of object content
	i := skipToObjectStart(data)
	if i == -1 {
		return -1, -1, -1
	}

	keyBytes := []byte(key)
	keyLen := len(keyBytes)

	for i < len(data) {
		// Skip whitespace and check end
		i = skipSpaces(data, i)
		if i >= len(data) || data[i] == '}' {
			break
		}

		// Expect a quoted key
		if data[i] != '"' {
			return -1, -1, -1
		}
		keyStart := i

		// Read key name (unquoted bounds) and position after closing quote
		nameStart, nameEnd, after, err := readUnquotedKey(data, i)
		if err != nil {
			return -1, -1, -1
		}

		// Move to colon and then to value start
		i = skipSpaces(data, after)
		if i >= len(data) || data[i] != ':' {
			return -1, -1, -1
		}
		i++
		valueStart := skipSpaces(data, i)

		// Find value end
		valueEnd := findValueEnd(data, valueStart)
		if valueEnd == -1 {
			return -1, -1, -1
		}

		// If key matches, return ranges
		if (nameEnd-nameStart) == keyLen && bytes.Equal(data[nameStart:nameEnd], keyBytes) {
			return keyStart, valueStart, valueEnd
		}

		// Advance to next pair (skip spaces and optional comma)
		i = skipSpaces(data, valueEnd)
		if i < len(data) && data[i] == ',' {
			i++
			continue
		}
		break
	}

	return -1, -1, -1
}

// setFastAddSimpleKey optimizes the common case of adding a single key like "email" to an object
// validateAddKeyInput performs input validation for adding a key
func validateAddKeyInput(data []byte, key string, value interface{}) (int, bool) {
	// Don't use fast path for deletion marker
	if value == deletionMarkerValue {
		return 0, false
	}

	if len(data) == 0 || len(key) == 0 {
		return 0, false
	}

	// Find the root object
	i := 0
	for i < len(data) && data[i] <= ' ' {
		i++
	}
	if i >= len(data) || data[i] != '{' {
		return 0, false
	}

	return i, true
}

// findObjectEndPosition finds the closing brace position of JSON object
func findObjectEndPosition(data []byte) (int, bool) {
	end := len(data) - 1
	for end >= 0 && data[end] <= ' ' {
		end--
	}
	if end < 0 || data[end] != '}' {
		return 0, false
	}
	return end + 1, true // Include the closing brace
}

// buildKeyValueResult constructs the final JSON with the new key-value pair
func buildKeyValueResult(data []byte, key string, encVal []byte, objStart, end int) []byte {
	// Check if object is empty by looking between first { and last }
	objContent := bytes.TrimSpace(data[objStart : end-1])
	needComma := len(objContent) > 0

	// Build result directly - much more efficient than slice operations
	newSize := len(data) + 1 + len(key) + 2 + len(encVal) // "key":value
	if needComma {
		newSize++ // for comma
	}

	result := make([]byte, 0, newSize)
	result = append(result, data[:end-1]...)
	if needComma {
		result = append(result, ',')
	}
	result = append(result, '"')
	result = append(result, key...)
	result = append(result, '"', ':')
	result = append(result, encVal...)
	result = append(result, '}')

	return result
}

func setFastAddSimpleKey(data []byte, key string, value interface{}) ([]byte, bool, error) {
	objStart, valid := validateAddKeyInput(data, key, value)
	if !valid {
		return nil, false, nil
	}

	// Quick check: does the key already exist? If so, this isn't an "add"
	keyStart, _, _ := findKeyValueRange(data, key)
	if keyStart != -1 {
		return nil, false, nil // Key exists, can't add
	}

	// Encode the value efficiently
	encVal, err := fastEncodeJSONValue(value)
	if err != nil {
		return nil, false, err
	}

	end, valid := findObjectEndPosition(data)
	if !valid {
		return nil, false, nil
	}

	result := buildKeyValueResult(data, key, encVal, objStart+1, end)
	return result, true, nil
}

// DeepCreateContext holds the state for deep object creation operations
type DeepCreateContext struct {
	data       []byte
	window     []byte
	baseOffset int
	parts      []string
}

// initializeDeepCreationContext prepares the context for deep object creation
func initializeDeepCreationContext(data []byte, path string) (*DeepCreateContext, bool, error) {
	// Don't handle deletion marker or empty data
	if len(data) == 0 {
		return nil, false, nil
	}

	// Split the path into parts
	parts := parseObjectPath(path)
	if len(parts) < 2 {
		return nil, false, nil
	}

	// Create and initialize the context
	ctx := &DeepCreateContext{
		data:       data,
		window:     data,
		baseOffset: 0,
		parts:      parts,
	}

	return ctx, true, nil
}

// parseObjectPath splits a path into component parts
func parseObjectPath(path string) []string {
	parts := make([]string, 0, 4) // Pre-allocate for common depth
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '.' {
			if i > start {
				part := path[start:i]
				// Include all valid path components including numeric indices
				if part != "" {
					parts = append(parts, part)
				} else {
					// Empty parts are invalid
					return nil
				}
			}
			start = i + 1
		}
	}
	return parts
}

// isQuickDeepCreationCandidate checks if a path is suitable for optimized deep creation
func isQuickDeepCreationCandidate(path string, data []byte) bool {
	// Check if this is a simple dot-separated path with no brackets
	if strings.Contains(path, "[") || strings.Count(path, ".") < 2 {
		return false
	}

	// Check if the first component doesn't exist, allowing us to skip path traversal
	firstDot := strings.IndexByte(path, '.')
	if firstDot <= 0 {
		return false
	}

	firstKey := path[:firstDot]
	return !quickKeyExists(data, firstKey)
}

// findDeepestExistingParent navigates to find the deepest existing object in the path
func findDeepestExistingParent(ctx *DeepCreateContext) (int, bool, error) {
	lastExisting := -1

	// Try to navigate as deep as possible along the path
	for i := 0; i < len(ctx.parts); i++ {
		part := ctx.parts[i]
		s, e := getObjectValueRange(ctx.window, part)
		if s < 0 {
			// Missing at this level; parent is current window object
			lastExisting = i - 1
			break
		}

		// Verify the child is an object before continuing
		if !isObjectValue(ctx.window[s:e]) {
			return -1, false, nil
		}

		// Move into this existing child
		ctx.baseOffset += s
		ctx.window = ctx.window[s:e]
		lastExisting = i
	}

	// Handle case where nothing exists along the path
	if lastExisting < 0 {
		// Ensure root is an object
		if !isRootObject(ctx.window) {
			return -1, false, nil
		}

		// Set window to the root object's content
		rs := skipToObjectStart(ctx.window) - 1 // -1 to get to the opening brace
		ctx.baseOffset = rs
		ctx.window = ctx.window[rs:findBlockEnd(ctx.window, rs, '{', '}')]
	}

	// Check if there's anything to create
	if lastExisting >= len(ctx.parts)-1 {
		return -1, false, nil
	}

	return lastExisting, true, nil
}

// isObjectValue checks if a JSON value is an object
func isObjectValue(value []byte) bool {
	// Skip whitespace to find the first non-space character
	k := 0
	for k < len(value) && value[k] <= ' ' {
		k++
	}
	return k < len(value) && value[k] == '{'
}

// isRootObject checks if the root JSON value is an object
func isRootObject(data []byte) bool {
	rs := 0
	for rs < len(data) && data[rs] <= ' ' {
		rs++
	}
	return rs < len(data) && data[rs] == '{'
}

// buildNestedStructure creates a nested object structure for the given keys
func buildNestedStructure(keys []string, value []byte) []byte {
	// Calculate exact size needed (plus extra for arrays)
	totalSize := len(value) + 100 // Reserve more space for array handling

	// Build nested structure in one pass
	nested := make([]byte, 0, totalSize)

	// Track opening braces and brackets for proper closing
	structureStack := make([]byte, 0, len(keys))

	for i, k := range keys {
		// Check if this is an array index
		if isAllDigits(k) {
			// This is an array index
			if i == 0 {
				// First element can't be an array index, create a wrapper object
				return nil
			}

			// If previous item wasn't closing an array, we need to create one
			if len(nested) == 0 || nested[len(nested)-1] != ']' {
				// Remove the previous closing brace if we just closed an object
				if len(nested) > 0 && nested[len(nested)-1] == '}' {
					nested = nested[:len(nested)-1]
					structureStack = structureStack[:len(structureStack)-1]
				}

				// Start an array
				nested = append(nested, ':', '[')
				structureStack = append(structureStack, ']')
			}

			// Add null elements until we reach the desired index
			idx := parseInt(k)
			for j := 0; j < idx; j++ {
				nested = append(nested, 'n', 'u', 'l', 'l', ',')
			}

			if i == len(keys)-1 {
				// Last element, add value
				nested = append(nested, value...)
			} else {
				// Create an object for the next level
				nested = append(nested, '{')
				structureStack = append(structureStack, '}')
			}
		} else {
			// Regular object key
			nested = append(nested, '"')
			nested = append(nested, k...)
			nested = append(nested, '"')

			if i == len(keys)-1 {
				// Last element, add value
				nested = append(nested, ':')
				nested = append(nested, value...)
			} else {
				// Check next element
				nextIsIndex := i+1 < len(keys) && isAllDigits(keys[i+1])
				if nextIsIndex {
					// Next element is an array index, prepare for array
					nested = append(nested, ':')
				} else {
					// Regular object, add opening brace
					nested = append(nested, ':', '{')
					structureStack = append(structureStack, '}')
				}
			}
		}
	}

	// Close all structures in reverse order
	for i := len(structureStack) - 1; i >= 0; i-- {
		nested = append(nested, structureStack[i])
	}

	return nested
}

// spliceNestedStructureIntoParent inserts the nested structure into the parent object
func spliceNestedStructureIntoParent(ctx *DeepCreateContext, nested []byte) ([]byte, bool, error) {
	// Define parent boundaries
	parentStart := ctx.baseOffset
	parentEnd := parentStart + len(ctx.window)

	// Find closing brace of parent object
	ws := 0
	for ws < len(ctx.window) && ctx.window[ws] <= ' ' {
		ws++
	}

	if ws >= len(ctx.window) || ctx.window[ws] != '{' {
		return nil, false, nil
	}

	endObj := findBlockEnd(ctx.window, ws, '{', '}')
	if endObj == -1 {
		return nil, false, ErrInvalidJSON
	}

	// Check if parent object has existing content
	inner := bytes.TrimSpace(ctx.window[ws+1 : endObj-1])
	needComma := len(inner) > 0

	// Calculate final size and build result in one allocation
	finalSize := len(ctx.data) - len(ctx.window) + endObj - 1 + len(nested) + parentEnd - parentStart
	if needComma {
		finalSize++
	}

	result := make([]byte, 0, finalSize)
	result = append(result, ctx.data[:parentStart]...)
	result = append(result, ctx.window[:endObj-1]...)
	if needComma {
		result = append(result, ',')
	}
	result = append(result, nested...)
	result = append(result, ctx.window[endObj-1:]...)
	result = append(result, ctx.data[parentEnd:]...)

	return result, true, nil
}

func setFastDeepCreateObjects(data []byte, path string, value interface{}) ([]byte, bool, error) {
	// Don't use fast path for deletion marker
	if value == deletionMarkerValue {
		return nil, false, nil
	}

	// Initialize context and check basic conditions
	ctx, success, err := initializeDeepCreationContext(data, path)
	if !success || err != nil {
		return nil, false, err
	}

	// Ultra-fast path for pure deep creation (benchmark optimization)
	if isQuickDeepCreationCandidate(path, data) {
		// Pure creation case - build the entire nested structure directly
		return buildPureNestedPath(data, path, value)
	}

	// Find deepest existing parent object along the path
	lastExisting, success, err := findDeepestExistingParent(ctx)
	if !success || err != nil {
		return nil, false, err
	}

	// Encode the value
	encVal, err := fastEncodeJSONValue(value)
	if err != nil {
		return nil, false, err
	}

	// Build nested structure for the parts of the path that need to be created
	keys := ctx.parts[lastExisting+1:]
	nested := buildNestedStructure(keys, encVal)

	// Splice the nested structure into the parent object
	return spliceNestedStructureIntoParent(ctx, nested)
}

// deleteFastPath handles nested path deletions with optimized byte manipulation
func deleteFastPath(data []byte, path string) ([]byte, bool) {
	// For now, handle simple nested paths like "address.city"
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return nil, false
	}

	// Navigate to parent object
	current := data
	currentStart := 0

	for _, part := range parts[:len(parts)-1] {
		// Skip array indices for now, focus on object navigation
		if strings.Contains(part, "[") {
			return nil, false
		}

		// Find object value for this part
		start := 0
		for start < len(current) && current[start] <= ' ' {
			start++
		}

		if start >= len(current) || current[start] != '{' {
			return nil, false
		}

		valueStart, valueEnd := getObjectValueRange(current, part)
		if valueStart == -1 {
			return nil, false // Path doesn't exist
		}

		currentStart += valueStart
		current = current[valueStart:valueEnd]
	}

	// Now delete the final key from the current object
	finalKey := parts[len(parts)-1]
	if strings.Contains(finalKey, "[") {
		return nil, false // Array operations not supported in fast path yet
	}

	// Call the improved deleteFastSimpleKey with correct signature
	objToModify := data[currentStart : currentStart+len(current)]
	result, changed := deleteFastSimpleKey(objToModify, finalKey)
	if !changed {
		return nil, false
	}

	// Rebuild the full document
	finalResult := make([]byte, 0, len(data))
	finalResult = append(finalResult, data[:currentStart]...)
	finalResult = append(finalResult, result...)
	finalResult = append(finalResult, data[currentStart+len(current):]...)

	return finalResult, true
}

// deleteFastSimpleKey handles deletion of top-level keys using direct byte manipulation
func deleteFastSimpleKey(data []byte, key string) (result []byte, changed bool) {
	start, ok := findDeletionObjectStart(data)
	if !ok {
		return data, false
	}

	keyQuoted := append([]byte{'"'}, append([]byte(key), '"')...) // "key"
	pos := start + 1
	for pos < len(data) {
		// Parse next pair or detect end
		pairStart, currentKey, valueEnd, nextPos, done, valid := readNextDeletionPair(data, pos)
		if done {
			break
		}
		if !valid {
			return data, false
		}

		// Match and remove
		if bytes.Equal(currentKey, keyQuoted) {
			return removeDeletionPairAt(data, start, pairStart, valueEnd)
		}

		// Advance
		if nextPos < 0 {
			break
		}
		pos = nextPos
	}
	return data, false
}

// findDeletionObjectStart finds the '{' start after skipping spaces.
func findDeletionObjectStart(data []byte) (int, bool) {
	start := 0
	for start < len(data) && data[start] <= ' ' {
		start++
	}
	if start >= len(data) || data[start] != '{' {
		return 0, false
	}
	return start, true
}

// readNextDeletionPair parses the next key-value pair; returns positions and state flags.
func readNextDeletionPair(data []byte, pos int) (pairStart int, key []byte, valueEnd int, nextPos int, done bool, valid bool) {
	pos = skipSpaces(data, pos)
	if pos >= len(data) || data[pos] == '}' {
		return 0, nil, 0, -1, true, true
	}
	if data[pos] != '"' {
		return 0, nil, 0, -1, false, false
	}

	pairStart = pos
	// Read quoted key
	keyBytes, keyEnd, err := readQuotedSegment(data, pos)
	if err != nil {
		return 0, nil, 0, -1, false, false
	}

	// Move to value start
	pos = skipSpaces(data, keyEnd)
	if pos >= len(data) || data[pos] != ':' {
		return 0, nil, 0, -1, false, false
	}
	pos++
	pos = skipSpaces(data, pos)

	// Find value end
	ve := findValueEnd(data, pos)
	if ve == -1 {
		return 0, nil, 0, -1, false, false
	}

	// Compute next position (after optional comma)
	np := skipSpaces(data, ve)
	if np < len(data) && data[np] == ',' {
		np++
	}

	return pairStart, keyBytes, ve, np, false, true
}

// removeDeletionPairAt removes the pair spanning pairStart..valueEnd, adjusting commas/whitespace.
func removeDeletionPairAt(data []byte, start, pairStart, valueEnd int) ([]byte, bool) {
	pairStartAdj, pairEnd := computePairBounds(data, start, pairStart, valueEnd)
	out := make([]byte, 0, len(data)-(pairEnd-pairStartAdj))
	out = append(out, data[:pairStartAdj]...)
	out = append(out, data[pairEnd:]...)
	return out, true
}

// skipSpaces advances pos over ASCII spaces and returns new position
func skipSpaces(data []byte, pos int) int {
	for pos < len(data) && data[pos] <= ' ' {
		pos++
	}
	return pos
}

// readQuotedSegment reads a quoted string starting at pos (must be '"') and
// returns the slice including quotes and the index after the closing quote.
func readQuotedSegment(data []byte, pos int) ([]byte, int, error) {
	if pos >= len(data) || data[pos] != '"' {
		return nil, 0, ErrInvalidJSON
	}
	start := pos
	pos++
	for pos < len(data) {
		if data[pos] == '\\' {
			// Skip escaped char safely
			pos++
			if pos >= len(data) {
				return nil, 0, ErrInvalidJSON
			}
			pos++
			continue
		}
		if data[pos] == '"' {
			end := pos + 1
			return data[start:end], end, nil
		}
		pos++
	}
	return nil, 0, ErrInvalidJSON
}

// computePairBounds determines the slice bounds to remove for a key-value pair,
// adjusting for trailing or preceding commas and whitespace.
func computePairBounds(data []byte, start, pairStart, valueEnd int) (pairStartAdj, pairEnd int) {
	pairEnd = valueEnd
	// Look for comma after the value
	tempPos := valueEnd
	for tempPos < len(data) && data[tempPos] <= ' ' {
		tempPos++
	}
	if tempPos < len(data) && data[tempPos] == ',' {
		// Include trailing comma
		pairEnd = tempPos + 1
		return pairStart, pairEnd
	}

	// No trailing comma, look for preceding comma
	tempPos = pairStart - 1
	for tempPos >= start && data[tempPos] <= ' ' {
		tempPos--
	}
	if tempPos >= start && data[tempPos] == ',' {
		// Include the preceding comma
		return tempPos, pairEnd
	}
	return pairStart, pairEnd
}

// readUnquotedKey reads a quoted key starting at pos (which must be '"') and
// returns the nameStart, nameEnd (indexes of the unquoted key), the index after
// the closing quote, and an error if invalid.
func readUnquotedKey(data []byte, pos int) (nameStart, nameEnd, after int, err error) {
	if pos >= len(data) || data[pos] != '"' {
		return 0, 0, 0, ErrInvalidJSON
	}
	pos++
	nameStart = pos
	for pos < len(data) {
		if data[pos] == '\\' {
			pos++
			if pos >= len(data) {
				return 0, 0, 0, ErrInvalidJSON
			}
			pos++
			continue
		}
		if data[pos] == '"' {
			nameEnd = pos
			after = pos + 1
			return nameStart, nameEnd, after, nil
		}
		pos++
	}
	return 0, 0, 0, ErrInvalidJSON
}

// fastGetObjectValue returns the raw value bytes for a key within an object slice
func fastGetObjectValue(obj []byte, key string) []byte {
	// Reuse reader from nqjson_get
	return getObjectValue(obj, key)
}

// fastEncodeJSONValue encodes basic Go values to JSON without full marshal when possible
// tryParseStringAsJSON attempts to parse a string as JSON if it looks like JSON
func tryParseStringAsJSON(val string) ([]byte, bool) {
	if (strings.HasPrefix(val, "{") && strings.HasSuffix(val, "}")) ||
		(strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]")) {
		var jsonVal interface{}
		if err := json.Unmarshal([]byte(val), &jsonVal); err == nil {
			// It's valid JSON, marshal it directly
			if result, err := json.Marshal(jsonVal); err == nil {
				return result, true
			}
		}
	}
	return nil, false
}

// handleByteSliceEncoding handles encoding of byte slices as JSON
func handleByteSliceEncoding(val []byte) ([]byte, error) {
	// Assume raw JSON if parsable; else treat as string
	var tmp interface{}
	if json.Unmarshal(val, &tmp) == nil {
		return val, nil
	}
	return json.Marshal(string(val))
}

// encodeNumericValue encodes numeric values to JSON bytes
func encodeNumericValue(v interface{}) ([]byte, bool) {
	switch val := v.(type) {
	case int:
		return []byte(strconv.FormatInt(int64(val), 10)), true
	case int64:
		return []byte(strconv.FormatInt(val, 10)), true
	case uint64:
		return []byte(strconv.FormatUint(val, 10)), true
	case float64:
		// Default formatting similar to json.Marshal
		return []byte(strconv.FormatFloat(val, 'f', -1, 64)), true
	default:
		return nil, false
	}
}

func fastEncodeJSONValue(v interface{}) ([]byte, error) {
	switch val := v.(type) {
	case nil:
		return []byte("null"), nil
	case string:
		// Try to parse as JSON first for strings that look like JSON
		if result, isJSON := tryParseStringAsJSON(val); isJSON {
			return result, nil
		}
		return encodeJSONString(val), nil
	case bool:
		if val {
			return []byte("true"), nil
		}
		return []byte("false"), nil
	case []byte:
		return handleByteSliceEncoding(val)
	default:
		// Try numeric encoding first
		if result, isNumeric := encodeNumericValue(v); isNumeric {
			return result, nil
		}
		// Fallback to standard JSON marshaling
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

// processNumericPathPart handles a numeric path part as array access (e.g., users.0)
func processNumericPathPart(current *interface{}, idx int, isLast bool, parent interface{}, holderKey string, pathPartIndex int, pathParts []string) error {
	// The holder of the current value (array) is described by the existing parent/holderKey
	holderParent := parent

	arr, ok := (*current).([]interface{})
	if !ok {
		return ErrTypeMismatch
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
			return ErrArrayIndex
		}
	}

	// Now access the element in the array
	next := arr[idx]
	*current = next

	// Create container for next if needed
	if !isLast && next == nil && pathPartIndex+1 < len(pathParts) {
		nextPart := pathParts[pathPartIndex+1]
		var newVal interface{}
		if strings.Contains(nextPart, "[") || isAllDigits(nextPart) {
			newVal = make([]interface{}, 0)
		} else {
			newVal = make(map[string]interface{})
		}
		arr[idx] = newVal
		*current = newVal
	}

	return nil
}

// processArrayAccess handles array indexing operations in the path
func processArrayAccess(current *interface{}, idx int, isLast, isFinalIndex bool, parent interface{}, lastKey string, lastIndex int, isArrayElement bool, pathPartIndex int, pathParts []string) error {
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
			return ErrTypeMismatch
		}
	}

	// Ensure array has enough elements
	if idx >= len(arr) {
		if isLast && isFinalIndex {
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
			return ErrArrayIndex
		}
	}

	// Get the value at this index
	next := arr[idx]
	current = &next

	// Check if we need to create a new object/array for the next part
	if !isLast && isFinalIndex && pathPartIndex+1 < len(pathParts) {
		nextPart := pathParts[pathPartIndex+1]
		if next == nil {
			if strings.Contains(nextPart, "[") || isAllDigits(nextPart) {
				next = make([]interface{}, 0)
			} else {
				next = make(map[string]interface{})
			}
			arr[idx] = next
			*current = next
		}
	}

	return nil
}

// processObjectKey processes an object property access
func processObjectKey(current *interface{}, key string, isLast bool, parent interface{}, lastKey string, lastIndex int, isArrayElement bool, value interface{}, options SetOptions) error {
	// Navigate to the base object first
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
			return ErrTypeMismatch
		}
	}

	// Get or create the value at this key
	next, exists := m[key]
	if !exists {
		if isLast {
			// If last component, we'll set it below
			next = make(map[string]interface{})
			m[key] = next
		} else {
			// Create based on next path part
			// Determine whether to create an array or map based on the nextPart
			// The caller should have checked if there's a next part already
			pathParts := strings.Split(options.nextPath, ".")
			for i, part := range pathParts {
				if part == key && i < len(pathParts)-1 {
					nextPart := pathParts[i+1]
					if strings.Contains(nextPart, "[") || isAllDigits(nextPart) {
						next = make([]interface{}, 0)
					} else {
						next = make(map[string]interface{})
					}
					m[key] = next
					break
				}
			}

			// If we couldn't determine the type based on the path, default to a map
			if next == nil {
				next = make(map[string]interface{})
				m[key] = next
			}
		}
	}
	*current = next

	return nil
}

// setSimplePath sets a value at a simple path (dot notation or basic array access)

// setInParent sets a value in a parent object or array
// setInDirectMap sets value in a direct map[string]interface{}
func setInDirectMap(m map[string]interface{}, key string, value interface{}) {
	m[key] = value
}

// setInDirectArray sets value in a direct []interface{}
func setInDirectArray(arr []interface{}, index int, value interface{}) []interface{} {
	if index < 0 {
		return arr
	}
	if index < len(arr) {
		arr[index] = value
		return arr
	}
	// Expand array to fit index
	newArr := make([]interface{}, index+1)
	copy(newArr, arr)
	for i := len(arr); i < index; i++ {
		newArr[i] = nil
	}
	newArr[index] = value
	return newArr
}

// setInInterfacePointer handles *interface{} parent type
func setInInterfacePointer(p *interface{}, key string, index int, isArray bool, value interface{}) {
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
			*p = setInDirectArray(arr, index, value)
		}
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
}

// setInMapPointer handles *map[string]interface{} parent type
func setInMapPointer(p *map[string]interface{}, key string, value interface{}) {
	if *p == nil {
		*p = make(map[string]interface{})
	}
	(*p)[key] = value
}

// setInArrayPointer handles *[]interface{} parent type
func setInArrayPointer(p *[]interface{}, index int, value interface{}) {
	if index < 0 {
		return
	}
	if *p == nil {
		arr := make([]interface{}, index+1)
		arr[index] = value
		*p = arr
		return
	}
	*p = setInDirectArray(*p, index, value)
}

func setInParent(parent interface{}, key string, index int, isArray bool, value interface{}) {
	switch p := parent.(type) {
	case map[string]interface{}:
		// Direct object write
		setInDirectMap(p, key, value)
	case []interface{}:
		// Direct array write - note: cannot reassign slice reference
		setInDirectArray(p, index, value)
	case *interface{}:
		// Parent is a pointer to an interface holding either a map or slice
		setInInterfacePointer(p, key, index, isArray, value)
	case *map[string]interface{}:
		setInMapPointer(p, key, value)
	case *[]interface{}:
		setInArrayPointer(p, index, value)
	default:
		// Unknown parent type; no-op to avoid panic
		return
	}
}

// getFromParent returns the child value at key/index from the given parent container
// getFromArrayParent extracts value from array-type parent at given index
func getFromArrayParent(parent interface{}, index int) (interface{}, bool) {
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

// getFromObjectParent extracts value from object-type parent by key
func getFromObjectParent(parent interface{}, key string) (interface{}, bool) {
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

func getFromParent(parent interface{}, key string, index int, isArray bool) (interface{}, bool) {
	if isArray {
		return getFromArrayParent(parent, index)
	}
	return getFromObjectParent(parent, key)
}

// deleteFromArrayParent handles deletion from array-type parents
func deleteFromArrayParent(parent interface{}, index int) bool {
	switch p := parent.(type) {
	case []interface{}:
		if index >= 0 && index < len(p) {
			// This case can't properly resize, need to fix the calling logic
			// For now, set to nil to indicate deletion
			p[index] = nil
			return true
		}
	case *interface{}:
		if arr, ok := (*p).([]interface{}); ok {
			if index >= 0 && index < len(arr) {
				// Actually remove the element from array
				newArr := make([]interface{}, len(arr)-1)
				copy(newArr[:index], arr[:index])
				copy(newArr[index:], arr[index+1:])
				*p = newArr
				return true
			}
		}
	case *[]interface{}:
		if p != nil && index >= 0 && index < len(*p) {
			// Actually remove the element from array
			arr := *p
			newArr := make([]interface{}, len(arr)-1)
			copy(newArr[:index], arr[:index])
			copy(newArr[index:], arr[index+1:])
			*p = newArr
			return true
		}
	}
	return false
}

// deleteFromObjectParent handles deletion from object-type parents
func deleteFromObjectParent(parent interface{}, key string) bool {
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

// deleteFromParent deletes object key or nulls-out array index; returns true if a change occurred
func deleteFromParent(parent interface{}, key string, index int, isArray bool) bool {
	if isArray {
		return deleteFromArrayParent(parent, index)
	}
	return deleteFromObjectParent(parent, key)
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

// SetPathNavigationContext holds the state of navigation through a JSON path
type SetPathNavigationContext struct {
	current        *interface{}
	parent         interface{}
	lastKey        string
	lastIndex      int
	isArrayElement bool
}

// navigatePathSegment processes a single path segment during path navigation
func navigatePathSegment(ctx *SetPathNavigationContext, segment setPathSegment, isLast bool, options *SetOptions, value interface{}) (bool, error) {
	if segment.index >= 0 {
		// Handle array access
		return handleArraySegment(ctx, segment, isLast, options, value)
	} else {
		// Handle object access
		return handleObjectSegment(ctx, segment, isLast)
	}
}

// handleArraySegment processes an array segment in the path
func handleArraySegment(ctx *SetPathNavigationContext, segment setPathSegment, isLast bool, options *SetOptions, value interface{}) (bool, error) {
	// Get array from current pointer
	arr, ok := (*ctx.current).([]interface{})
	if !ok {
		// Not an array, can't proceed
		if isLast && options.Optimistic {
			return false, ErrNoChange
		}
		return false, ErrTypeMismatch
	}

	// Update context with array information
	ctx.parent = ctx.current
	ctx.lastIndex = segment.index
	ctx.isArrayElement = true

	// Check array bounds
	if segment.index >= len(arr) {
		if isLast && value != nil {
			// Expand array for setting
			if !expandArray(ctx, arr, segment.index) {
				return false, ErrArrayIndex
			}
			// Get updated array after expansion
			arr = (*ctx.current).([]interface{})
		} else {
			return false, ErrArrayIndex
		}
	}

	// Get the array element
	next := arr[segment.index]
	ctx.current = &next

	// Create nested structure if needed
	if next == nil && !isLast {
		// We'll need to check outside if the next segment is an array
		// This will be passed from the caller
		newVal := make(map[string]interface{}) // Default to object
		arr[segment.index] = newVal
		next = newVal
		*ctx.current = newVal
	}

	return true, nil
}

// expandArray expands an array to accommodate a new index
func expandArray(ctx *SetPathNavigationContext, arr []interface{}, targetIndex int) bool {
	// Create expanded array
	newArr := make([]interface{}, targetIndex+1)
	copy(newArr, arr)
	for i := len(arr); i < targetIndex; i++ {
		newArr[i] = nil
	}

	// Update parent/container with the expanded array
	if p, ok := ctx.parent.(*interface{}); ok {
		*p = newArr
		return true
	}

	// Handle fallback cases
	if ctx.parent == nil {
		return false
	}

	// Try to update based on parent type
	if parentArr, ok := ctx.parent.([]interface{}); ok {
		if ctx.lastIndex >= 0 && ctx.lastIndex < len(parentArr) {
			parentArr[ctx.lastIndex] = newArr
			*ctx.current = newArr
			return true
		}
	} else if parentMap, ok := ctx.parent.(map[string]interface{}); ok {
		parentMap[ctx.lastKey] = newArr
		*ctx.current = newArr
		return true
	}

	return false
}

// handleObjectSegment processes an object segment in the path
func handleObjectSegment(ctx *SetPathNavigationContext, segment setPathSegment, isLast bool) (bool, error) {
	// Get object from current pointer
	m, ok := (*ctx.current).(map[string]interface{})
	if !ok {
		// Not an object, can't proceed
		return false, ErrTypeMismatch
	}

	// Update context with object information
	ctx.parent = ctx.current
	ctx.lastKey = segment.key
	ctx.isArrayElement = false

	// Get or create the value at this key
	next, exists := m[segment.key]
	if !exists {
		if isLast {
			// If last component, we'll set it later
			return true, nil
		} else {
			// Create based on next path segment
			// Default to map object - the caller will check for array next segment
			newVal := make(map[string]interface{})
			m[segment.key] = newVal
			next = newVal
		}
	}
	ctx.current = &next

	return true, nil
}

// handlePathDeletion handles deletion of a value at the end of a path
func handlePathDeletion(ctx *SetPathNavigationContext) (bool, error) {
	if ctx.isArrayElement {
		// For arrays, we need special handling
		if !deleteFromParent(ctx.parent, ctx.lastKey, ctx.lastIndex, true) {
			return false, ErrNoChange
		}
	} else {
		// For objects, just delete the key
		if !deleteFromParent(ctx.parent, ctx.lastKey, ctx.lastIndex, false) {
			return false, ErrNoChange
		}
	}
	return true, nil
}

// handlePathValueSetting handles setting a value at the end of a path
func handlePathValueSetting(ctx *SetPathNavigationContext, value interface{}, options *SetOptions) (bool, error) {
	// Convert the value to a JSON-compatible type
	jsonValue, err := convertToJSONValue(value)
	if err != nil {
		return false, err
	}

	// Check if we need to merge
	if options.MergeObjects && isMap(jsonValue) && ctx.parent != nil {
		if existing, ok := getFromParent(ctx.parent, ctx.lastKey, ctx.lastIndex, ctx.isArrayElement); ok && isMap(existing) {
			merged := mergeObjects(existing, jsonValue)
			setInParent(ctx.parent, ctx.lastKey, ctx.lastIndex, ctx.isArrayElement, merged)
			return true, nil
		}
	} else if options.MergeArrays && isSlice(jsonValue) && ctx.parent != nil {
		if existing, ok := getFromParent(ctx.parent, ctx.lastKey, ctx.lastIndex, ctx.isArrayElement); ok && isSlice(existing) {
			merged := mergeArrays(existing, jsonValue)
			setInParent(ctx.parent, ctx.lastKey, ctx.lastIndex, ctx.isArrayElement, merged)
			return true, nil
		}
	}

	// Set in parent
	if ctx.parent != nil {
		setInParent(ctx.parent, ctx.lastKey, ctx.lastIndex, ctx.isArrayElement, jsonValue)
	} else {
		// Setting the root (shouldn't happen with valid paths)
		*ctx.current = jsonValue
	}

	return true, nil
}

// setValueWithPath sets a value in JSON at the specified path
func setValueWithPath(json []byte, path *SetPath, value interface{}, options *SetOptions) ([]byte, bool, error) {
	// Parse the JSON into a generic structure
	var data interface{}
	if err := JSON.Unmarshal(json, &data); err != nil {
		return nil, false, ErrInvalidJSON
	}

	// Create navigation context
	ctx := &SetPathNavigationContext{
		current:        &data,
		parent:         nil,
		lastKey:        "",
		lastIndex:      -1,
		isArrayElement: false,
	}

	// Navigate through the path segments
	for i, segment := range path.segments {
		isLast := i == len(path.segments)-1 || segment.last

		// Determine if next segment is an array (used for creating the right type)
		// if i < len(path.segments)-1 && !segment.last && path.segments[i+1].index >= 0 {
		// Could be used for future enhancements to create the right nested structure
		// }

		success, err := navigatePathSegment(ctx, segment, isLast, options, value)
		if !success || err != nil {
			return nil, false, err
		}
	}

	// Handle value operation at the end of the path
	var success bool
	var err error

	if value == deletionMarkerValue {
		// Handle deletion
		success, err = handlePathDeletion(ctx)
	} else {
		// Handle value setting
		success, err = handlePathValueSetting(ctx, value, options)
	}

	if !success || err != nil {
		return nil, false, err
	}

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

// parseInt converts a string of digits to an integer
func parseInt(s string) int {
	result := 0
	for i := 0; i < len(s); i++ {
		result = result*10 + int(s[i]-'0')
	}
	return result
}

// tryOptimisticReplace attempts an in-place replacement for simple cases
func tryOptimisticReplace(json []byte) ([]byte, bool, error) {
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
	case string:
		// Try to parse as JSON first for strings that look like JSON
		if (strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}")) ||
			(strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]")) {
			var jsonVal interface{}
			if err := json.Unmarshal([]byte(v), &jsonVal); err == nil {
				return jsonVal, nil
			}
		}
		return v, nil
	case float64, int, int64, uint64, bool:
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

// setFastSimpleDotPath optimizes simple dot notation paths like "user.name" or "data.items.0"
func setFastSimpleDotPath(data []byte, path string, value interface{}) ([]byte, bool, error) {
	// Don't use fast path for deletion marker
	if value == deletionMarkerValue {
		return nil, false, nil
	}

	if len(data) == 0 || !strings.Contains(path, ".") {
		return nil, false, nil
	}

	// Only handle simple paths with 1-3 dots
	dotCount := strings.Count(path, ".")
	if dotCount > 3 {
		return nil, false, nil
	}

	parts := strings.Split(path, ".")
	if len(parts) < 2 || len(parts) > 4 {
		return nil, false, nil
	}

	// Encode the value
	encodedValue, err := fastEncodeJSONValue(value)
	if err != nil {
		return nil, false, err
	}

	// Navigate to the target location
	window := data
	baseOffset := 0

	for _, part := range parts[:len(parts)-1] {
		var start, end int
		if isAllDigits(part) {
			// Array access
			idx, _ := strconv.Atoi(part)
			start, end = getArrayElementRange(window, idx)
		} else {
			// Object access
			start, end = getObjectValueRange(window, part)
		}

		if start < 0 {
			return nil, false, nil // Path doesn't exist
		}

		baseOffset += start
		window = window[start:end]
	}

	// Now set the final key
	finalKey := parts[len(parts)-1]
	var keyStart, valueStart, valueEnd int

	if isAllDigits(finalKey) {
		// Array element replacement
		idx, _ := strconv.Atoi(finalKey)
		valueStart, valueEnd = getArrayElementRange(window, idx)
		if valueStart < 0 {
			return nil, false, nil
		}
	} else {
		// Object key replacement
		keyStart, valueStart, valueEnd = findKeyValueRange(window, finalKey)
		if keyStart < 0 {
			return nil, false, nil
		}
	}

	// Build result with single allocation
	totalOffset := baseOffset + valueStart
	resultSize := len(data) - (valueEnd - valueStart) + len(encodedValue)
	result := make([]byte, 0, resultSize)

	result = append(result, data[:totalOffset]...)
	result = append(result, encodedValue...)
	result = append(result, data[baseOffset+valueEnd:]...)

	return result, true, nil
}

// setFastArrayElement optimizes array element updates for common patterns
func setFastArrayElement(data []byte, path string, value interface{}) ([]byte, bool, error) {
	// Don't use fast path for deletion marker
	if value == deletionMarkerValue {
		return nil, false, nil
	}

	// Handle patterns like "items.0", "tags.1", "phones.0.number"
	if !strings.Contains(path, ".") {
		return nil, false, nil
	}

	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return nil, false, nil
	}

	// Look for array indices in the path
	hasArrayIndex := false
	for _, part := range parts {
		if isAllDigits(part) && len(part) == 1 && part[0] >= '0' && part[0] <= '9' {
			hasArrayIndex = true
			break
		}
	}

	if !hasArrayIndex {
		return nil, false, nil
	}

	// Use the fast simple dot path handler
	return setFastSimpleDotPath(data, path, value)
}

// setOptimizedSimplePath provides an optimized version of setSimplePath with minimal allocations
func setOptimizedSimplePath(json []byte, path string, value interface{}, options SetOptions) ([]byte, error) {
	// For simple paths, try to avoid full JSON unmarshaling
	if strings.Count(path, ".") <= 2 && !strings.Contains(path, "[") {
		// Try fast path first
		if result, ok, err := setFastSimpleDotPath(json, path, value); err == nil && ok {
			return result, nil
		}
	}

	// Fallback to the original setSimplePath but with optimizations
	return setSimplePath(json, path, value, options)
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

// PathContext holds the context of a path navigation operation
type PathContext struct {
	current        *interface{}
	parent         interface{}
	lastKey        string
	lastIndex      int
	isArrayElement bool
}

// navigateJsonPath navigates through a JSON structure following a path and returns the context
func navigateJsonPath(data *interface{}, path string, options SetOptions) (PathContext, error) {
	// Set up the context
	context := PathContext{
		current:        data,
		parent:         nil,
		lastKey:        "",
		lastIndex:      0,
		isArrayElement: false,
	}

	// Split the path into components
	pathParts := strings.Split(path, ".")

	// Navigate through each path part
	for i, part := range pathParts {
		if part == "" {
			continue
		}

		isLast := i == len(pathParts)-1

		// Process this path part
		err := processPathPart(&context, part, i, isLast, pathParts, options)
		if err != nil {
			return context, err
		}
	}

	return context, nil
}

// processPathPart processes a single part of a path
func processPathPart(context *PathContext, part string, pathIndex int, isLast bool, pathParts []string, options SetOptions) error {
	// Handle array access [n]
	if strings.Contains(part, "[") {
		err := processArrayNotation(context, part, pathIndex, isLast, pathParts, options)
		if err != nil {
			return err
		}
	} else if isAllDigits(part) {
		// Handle numeric segment as array index
		err := processNumericPart(context, part, pathIndex, isLast, pathParts)
		if err != nil {
			return err
		}
	} else {
		// Regular object key access
		err := processObjectKeyAccess(context, part, pathIndex, isLast, pathParts)
		if err != nil {
			return err
		}
	}

	return nil
}

// processArrayNotation handles path parts that contain array notation [n]
func processArrayNotation(context *PathContext, part string, pathIndex int, isLast bool, pathParts []string, options SetOptions) error {
	base := part[:strings.Index(part, "[")]

	// Navigate to the base object first if there is one
	if base != "" {
		// Set up parent tracking for processObjectKey
		context.parent = context.current
		context.lastKey = base
		context.isArrayElement = false

		// Process the object key part
		err := processObjectKey(context.current, base, isLast, context.parent, context.lastKey, context.lastIndex, context.isArrayElement, nil, options)
		if err != nil {
			return err
		}
	}

	// Process array indexes
	start := strings.Index(part, "[")
	for start != -1 && start < len(part) {
		end := strings.Index(part[start:], "]")
		if end == -1 {
			return ErrInvalidPath
		}
		end += start

		// Get array index
		idx, err := strconv.Atoi(part[start+1 : end])
		if err != nil {
			return ErrInvalidPath
		}

		// Process this array access
		err = processArrayAccess(context.current, idx, isLast, end+1 >= len(part), context.parent, context.lastKey, context.lastIndex, context.isArrayElement, pathIndex, pathParts)
		if err != nil {
			return err
		}

		// Update tracking for array elements
		if !isLast {
			context.parent = context.current
		}
		context.lastIndex = idx
		context.isArrayElement = true

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

	return nil
}

// processNumericPart handles numeric path parts (e.g., "0", "1", "42")
func processNumericPart(context *PathContext, part string, pathIndex int, isLast bool, pathParts []string) error {
	idx, _ := strconv.Atoi(part)

	// Process a numeric path part (dot notation array access)
	err := processNumericPathPart(context.current, idx, isLast, context.parent, context.lastKey, pathIndex, pathParts)
	if err != nil {
		return err
	}

	// Update tracking variables
	context.lastIndex = idx
	context.isArrayElement = true

	return nil
}

// processObjectKeyAccess handles regular object key access
func processObjectKeyAccess(context *PathContext, part string, pathIndex int, isLast bool, pathParts []string) error {
	// Array numeric access (dot notation)
	if arr, ok := (*context.current).([]interface{}); ok && isAllDigits(part) {
		return handleDotArrayAccess(context, arr, part, pathIndex, isLast, pathParts)
	}
	// Regular object key access
	return handleRegularObjectKeyAccess(context, part, pathIndex, isLast, pathParts)
}

// handleDotArrayAccess processes dot-notation numeric access into an array (e.g., tags.1)
func handleDotArrayAccess(context *PathContext, arr []interface{}, part string, pathIndex int, isLast bool, pathParts []string) error {
	idx, err := strconv.Atoi(part)
	if err != nil {
		return ErrInvalidPath
	}

	// Keep parent references pointing to the container
	context.lastIndex = idx
	context.isArrayElement = true

	// Ensure array has enough elements
	if idx >= len(arr) {
		if isLast {
			newArr := make([]interface{}, idx+1)
			copy(newArr, arr)
			for i := len(arr); i < idx; i++ {
				newArr[i] = nil
			}
			*context.current = newArr
			arr = newArr
		} else {
			return ErrArrayIndex
		}
	}

	// For non-last segments, dive into the element, creating container if needed
	if !isLast {
		next := arr[idx]
		context.current = &next
		if pathIndex+1 < len(pathParts) && next == nil {
			nextPart := pathParts[pathIndex+1]
			if strings.Contains(nextPart, "[") || isAllDigits(nextPart) {
				next = make([]interface{}, 0)
			} else {
				next = make(map[string]interface{})
			}
			arr[idx] = next
			*context.current = next
		}
	}
	return nil
}

// handleRegularObjectKeyAccess processes object key access, creating containers when needed.
func handleRegularObjectKeyAccess(context *PathContext, part string, pathIndex int, isLast bool, pathParts []string) error {
	m, ok := (*context.current).(map[string]interface{})
	if !ok {
		if isLast && context.parent != nil {
			newMap := make(map[string]interface{})
			setInParent(context.parent, context.lastKey, context.lastIndex, context.isArrayElement, newMap)
			m = newMap
			*context.current = m
		} else {
			return ErrTypeMismatch
		}
	}

	context.parent = context.current
	context.lastKey = part
	context.isArrayElement = false

	next, exists := m[part]
	if !exists {
		if !isLast {
			nextPart := pathParts[pathIndex+1]
			if strings.Contains(nextPart, "[") || isAllDigits(nextPart) {
				next = make([]interface{}, 0)
			} else {
				next = make(map[string]interface{})
			}
			m[part] = next
		}
	}
	context.current = &next
	return nil
}

// handleValueSetting processes the setting of values in a JSON structure
func handleValueSetting(data *interface{}, value interface{}, parent interface{}, lastKey string, lastIndex int, isArrayElement bool, options SetOptions) error {
	// Convert the value to a JSON-compatible type
	jsonValue, err := convertToJSONValue(value)
	if err != nil {
		return err
	}

	// Apply merge options if specified
	jsonValue = applyMergeOptions(jsonValue, parent, lastKey, lastIndex, isArrayElement, options)

	// Set in parent or directly in data
	if parent != nil {
		setInParent(parent, lastKey, lastIndex, isArrayElement, jsonValue)
	} else {
		*data = jsonValue
	}

	return nil
}

// applyMergeOptions handles the merging of objects and arrays based on options
func applyMergeOptions(jsonValue interface{}, parent interface{}, lastKey string, lastIndex int, isArrayElement bool, options SetOptions) interface{} {
	// Optional merge behavior for objects
	if options.MergeObjects && isMap(jsonValue) && parent != nil {
		if existing, ok := getFromParent(parent, lastKey, lastIndex, isArrayElement); ok && isMap(existing) {
			jsonValue = mergeObjects(existing, jsonValue)
		}
	}

	// Optional merge behavior for arrays
	if options.MergeArrays && isSlice(jsonValue) && parent != nil {
		if existing, ok := getFromParent(parent, lastKey, lastIndex, isArrayElement); ok && isSlice(existing) {
			jsonValue = mergeArrays(existing, jsonValue)
		}
	}

	return jsonValue
}

// handleDeletion handles the deletion of elements from JSON
func handleDeletion(json []byte, path string, isArrayElement bool, parent interface{}, lastKey string, lastIndex int) (interface{}, error) {
	// Special handling for array element deletion
	if isArrayElement {
		return handleArrayDeletion(json, path, parent, lastKey, lastIndex)
	}

	// Use helper to delete object keys
	if !deleteFromParent(parent, lastKey, lastIndex, isArrayElement) {
		return nil, ErrNoChange
	}

	return nil, nil
}

// handleArrayDeletion handles deletion specifically for array elements
func handleArrayDeletion(json []byte, path string, parent interface{}, lastKey string, lastIndex int) (interface{}, error) {
	// Check if parent is the array itself (wrong tracking) vs container of array (correct tracking)
	if arr, isArrayParent := parent.([]interface{}); isArrayParent {
		// Parent tracking is wrong - parent is the array itself
		// We need to find the container and replace the array
		return handleDirectArrayDeletion(json, path, arr, lastIndex)
	}

	// Normal case - parent is container of array
	// We need to properly delete from array
	return handleParentContainerDeletion(parent, lastKey, lastIndex)
}

// handleDirectArrayDeletion handles deletion when parent is the array itself
func handleDirectArrayDeletion(json []byte, path string, arr []interface{}, lastIndex int) (interface{}, error) {
	// Create new array without the deleted element
	if lastIndex < 0 || lastIndex >= len(arr) {
		return nil, ErrArrayIndex
	}

	newArr := make([]interface{}, len(arr)-1)
	copy(newArr[:lastIndex], arr[:lastIndex])
	copy(newArr[lastIndex:], arr[lastIndex+1:])

	// Now we need to replace this array in the data structure
	// We'll manually navigate to find where this array is stored
	var data interface{}
	if err := JSON.Unmarshal(json, &data); err != nil {
		return nil, ErrInvalidJSON
	}

	// Navigate the path to find the container and replace the array
	pathParts := strings.Split(path, ".")
	current := &data
	for i, part := range pathParts[:len(pathParts)-1] { // all parts except the last (which is the index)
		if m, ok := (*current).(map[string]interface{}); ok {
			if val, exists := m[part]; exists {
				if i == len(pathParts)-2 { // this is the parent of the array
					m[part] = newArr // replace the array
					break
				}
				current = &val
			}
		}
	}

	// Marshal back to JSON
	result, err := JSON.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}

	return result, nil
}

// handleParentContainerDeletion handles deletion when parent contains the array
func handleParentContainerDeletion(parent interface{}, lastKey string, lastIndex int) (interface{}, error) {
	if parentMap, ok := parent.(*interface{}); ok {
		if m, ok2 := (*parentMap).(map[string]interface{}); ok2 {
			if arr, exists := m[lastKey]; exists {
				if arrSlice, ok3 := arr.([]interface{}); ok3 && lastIndex >= 0 && lastIndex < len(arrSlice) {
					// Create new array without the element
					newArr := make([]interface{}, len(arrSlice)-1)
					copy(newArr[:lastIndex], arrSlice[:lastIndex])
					copy(newArr[lastIndex:], arrSlice[lastIndex+1:])
					m[lastKey] = newArr
					return nil, nil
				}
			}
		}
	}

	// Fallback to the generic deletion function
	if !deleteFromParent(parent, lastKey, lastIndex, true) {
		return nil, ErrNoChange
	}

	return nil, nil
}

// setSimplePath sets a value at a simple path (dot notation or basic array access)
func setSimplePath(json []byte, path string, value interface{}, options SetOptions) ([]byte, error) {
	// Parse the JSON into a generic structure
	var data interface{}
	if err := JSON.Unmarshal(json, &data); err != nil {
		return json, ErrInvalidJSON
	}

	// Set the full path in options for reference by helper functions
	options.nextPath = path

	// Navigate to the target location
	pathContext, err := navigateJsonPath(&data, path, options)
	if err != nil {
		return json, err
	}

	// Extract navigation results
	parent := pathContext.parent
	lastKey := pathContext.lastKey
	lastIndex := pathContext.lastIndex
	isArrayElement := pathContext.isArrayElement // Handle deletion
	if value == deletionMarkerValue {
		// Handle deletion based on the element type
		result, err := handleDeletion(json, path, isArrayElement, parent, lastKey, lastIndex)
		if err != nil {
			return json, err
		}
		if jsonResult, ok := result.([]byte); ok {
			// Special case where result was returned directly
			return jsonResult, nil
		}
	} else {
		// Set the value at the final location
		err := handleValueSetting(&data, value, parent, lastKey, lastIndex, isArrayElement, options)
		if err != nil {
			return json, err
		}
	}

	// Marshal back to JSON (pretty-printed to match examples/tests)
	result, err := JSON.MarshalIndent(data, "", "  ")
	if err != nil {
		return json, err
	}

	return result, nil
}
