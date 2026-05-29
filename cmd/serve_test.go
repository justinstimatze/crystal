package cmd

import "testing"

func TestPercentile(t *testing.T) {
	xs := []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	// index formula is int(p*(len-1)): p99 of 10 items lands at index 8 = 90.
	cases := map[float64]int64{
		0.0:  10,
		0.5:  xs[int(0.5*float64(len(xs)-1))], // index 4 = 50
		0.99: 90,
	}
	for p, want := range cases {
		if got := percentile(xs, p); got != want {
			t.Errorf("percentile(%.2f) = %d, want %d", p, got, want)
		}
	}
	if percentile(nil, 0.5) != 0 {
		t.Error("percentile of empty should be 0")
	}
}

func TestFmtNS(t *testing.T) {
	cases := map[int64]string{
		500:       "500ns",
		1500:      "1.50µs",
		2_500_000: "2.50ms",
	}
	for ns, want := range cases {
		if got := fmtNS(ns); got != want {
			t.Errorf("fmtNS(%d) = %q, want %q", ns, got, want)
		}
	}
}

func TestParseCategory(t *testing.T) {
	if parseCategory("  build/test  ") != "build/test" {
		t.Error("exact category not parsed")
	}
	if parseCategory("I think this is git related") != "git" {
		t.Error("substring category not matched")
	}
	if parseCategory("frobnicate") != "frobnicate" {
		t.Error("unknown category should pass through lowercased for the schema gate to catch")
	}
}
