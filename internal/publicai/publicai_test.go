package publicai

import (
	"testing"
	"time"
)

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

// TestIsBudgetThrottle pins the one retryable 400: PublicAI's transient shared
// spend-budget throttle (issue #46) is retried like an overload; a real 400 is not.
func TestIsBudgetThrottle(t *testing.T) {
	budget := []byte(`{"error":{"message":"litellm.BadRequestError - Budget has been exceeded! Team=x Max budget: 1.0"}}`)
	if !isBudgetThrottle(400, budget) {
		t.Error("budget-exceeded 400 should be a retryable throttle")
	}
	if isBudgetThrottle(400, []byte(`{"error":{"message":"model not found"}}`)) {
		t.Error("a non-budget 400 must NOT be treated as a throttle")
	}
	if isBudgetThrottle(200, budget) {
		t.Error("only non-2xx should match")
	}
}

// TestBackoffGrowsAndCaps confirms the wait grows with attempts and is capped
// (so a multi-minute budget window is waited out, but no single sleep runs away).
func TestBackoffGrowsAndCaps(t *testing.T) {
	if backoff(1) >= backoff(3) {
		t.Error("backoff should grow with attempt number")
	}
	for n := 1; n <= 10; n++ {
		if d := backoff(n); d > 75*time.Second { // 60s cap + 25% jitter
			t.Errorf("backoff(%d)=%v exceeds cap", n, d)
		}
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
