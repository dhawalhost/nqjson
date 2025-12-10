package nqjson

import "strings"

// EscapePathSegment escapes characters that have special meaning in nqjson paths so they
// are treated as literal property names. Useful when keys contain dots, wildcards, or
// query operators. If you intentionally pass a leading ':' (to force numeric-looking keys
// to be treated as object properties), that prefix is preserved.
func EscapePathSegment(seg string) string {
	if seg == "" {
		return ""
	}

	// Preserve an intentional leading ':' prefix (used to force object-key semantics).
	prefix := ""
	if len(seg) > 0 && seg[0] == ':' {
		prefix = ":"
		seg = seg[1:]
	}

	needsEscape := false
	for i := 0; i < len(seg); i++ {
		if shouldEscapePathChar(seg[i]) {
			needsEscape = true
			break
		}
	}
	if !needsEscape {
		return prefix + seg
	}

	var b strings.Builder
	b.Grow(len(seg) * 2)
	for i := 0; i < len(seg); i++ {
		c := seg[i]
		if shouldEscapePathChar(c) {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	return prefix + b.String()
}

// BuildEscapedPath joins literal segments using dot notation after escaping each one.
// Example: BuildEscapedPath("config", "foo.bar@baz", "*key") -> "config.foo\\.bar\\@baz.\\*key".
func BuildEscapedPath(segments ...string) string {
	if len(segments) == 0 {
		return ""
	}

	escaped := make([]string, len(segments))
	for i, s := range segments {
		escaped[i] = EscapePathSegment(s)
	}
	return strings.Join(escaped, ".")
}

func shouldEscapePathChar(c byte) bool {
	switch c {
	case '\\', '.', ':', '|', '@', '*', '?', '#', ',', '(', ')', '=', '!', '<', '>', '~':
		return true
	}
	return false
}
