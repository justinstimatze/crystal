package cmd

import "testing"

// TestParseCategoryFromRecognizesExpandedSet is the load-bearing check for the
// local agreement oracle's ability to NAME a new class: a re-author over an
// expanded category set must recognize "container" (not a compiled triageCategory),
// else the oracle could never label the drift class and 1b would silently fail.
func TestParseCategoryFromRecognizesExpandedSet(t *testing.T) {
	cats := append(append([]string{}, triageCategories...), "container")
	tests := []struct {
		reply string
		want  string
	}{
		{"container", "container"},
		{"Container", "container"},                 // case-insensitive
		{"  container\n", "container"},             // trimmed
		{"the category is container", "container"}, // substring
		{"network", "network"},                     // an existing class still resolves
		{"banana", "banana"},                       // unknown → raw text (not silently mapped)
	}
	for _, tc := range tests {
		if got := parseCategoryFrom(tc.reply, cats); got != tc.want {
			t.Errorf("parseCategoryFrom(%q) = %q, want %q", tc.reply, got, tc.want)
		}
	}
}

// TestParseCategoryFromEmptySetFallsBackToRaw guards the degenerate path so the
// oracle never panics or mislabels when handed no categories.
func TestParseCategoryFromEmptySetFallsBackToRaw(t *testing.T) {
	if got := parseCategoryFrom("  Container ", nil); got != "container" {
		t.Errorf("empty cats: got %q, want lowercased-trimmed raw %q", got, "container")
	}
}
