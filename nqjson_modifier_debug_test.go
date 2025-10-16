package nqjson

import (
	"testing"
)

func TestDebugModifier(t *testing.T) {
	data := []byte(`{"nums":[1,2,3]}`)
	path := "nums|@reverse"

	// Manually check what isSimplePath returns
	if isSimplePath(path) {
		t.Logf("isSimplePath returned true - WRONG!")
	} else {
		t.Logf("isSimplePath returned false - correct, will use getComplexPath")
	}

	// Tokenize the path
	tokens := tokenizePath(path)
	t.Logf("Tokens count: %d", len(tokens))
	for i, tok := range tokens {
		t.Logf("  Token %d: kind=%d str=%s", i, tok.kind, tok.str)
	}

	// Execute the path
	result := Get(data, path)
	t.Logf("Result: exists=%v type=%v", result.Exists(), result.Type)
	if result.Exists() {
		t.Logf("Result value: %s", result.String())
	}
}
