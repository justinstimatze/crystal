package publicai

import "testing"

// TestHashKeyNamespacedAndStable pins two invariants the shared disk cache relies
// on: keys are prefixed "publicai-" (so they never collide with the llm/local
// tiers writing into the SAME cacheDir) and identical inputs hash identically.
func TestHashKeyNamespacedAndStable(t *testing.T) {
	a := hashKey("m", "sys", "prompt", 16)
	b := hashKey("m", "sys", "prompt", 16)
	if a != b {
		t.Fatalf("hashKey not stable: %q != %q", a, b)
	}
	if a[:len("publicai-")] != "publicai-" {
		t.Errorf("hashKey not namespaced: %q", a)
	}
	if hashKey("m2", "sys", "prompt", 16) == a {
		t.Error("hashKey collides across distinct models")
	}
}

// TestSnippetCollapsesAndCaps keeps error messages one-line and bounded, so a
// 504 HTML page or a long problem+json body can't flood the terminal.
func TestSnippetCollapsesAndCaps(t *testing.T) {
	if got := snippet([]byte("")); got != "(empty body)" {
		t.Errorf("snippet(empty) = %q", got)
	}
	if got := snippet([]byte("  line one\n\tline two  ")); got != "line one line two" {
		t.Errorf("snippet did not collapse whitespace: %q", got)
	}
	long := make([]byte, 500)
	for i := range long {
		long[i] = 'x'
	}
	if got := snippet(long); len(got) > 205 { // 200 + ellipsis rune
		t.Errorf("snippet not capped: len=%d", len(got))
	}
}
