package step

import "testing"

func TestClassRunStatsMatchesHandCount(t *testing.T) {
	// A B A A B B B A -- authored runs at lengths 1, 2, 1; gaps (mechanical
	// stretches between two authored occurrences) of length 1 then 3.
	seq := []Class{Authored, Bare, Authored, Authored, Bare, Bare, Bare, Authored}
	runs, gaps := classRunStats(seq)

	wantRuns := map[int]int{1: 2, 2: 1} // two length-1 Authored runs, one length-2
	for n, want := range wantRuns {
		if runs[Authored][n] != want {
			t.Errorf("Authored run length %d: got %d, want %d (full histogram %v)", n, runs[Authored][n], want, runs[Authored])
		}
	}
	if gaps[1] != 1 || gaps[3] != 1 {
		t.Errorf("gaps = %v, want {1:1, 3:1}", gaps)
	}
	if got := isolatedFrac(runs, Authored); got != 2.0/3.0 {
		t.Errorf("isolatedFrac = %v, want 2/3", got)
	}
	if got := meanGap(gaps); got != 2 {
		t.Errorf("meanGap = %v, want 2 ((1+3)/2)", got)
	}
}

func TestShuffleBaselinePreservesMultiset(t *testing.T) {
	// A single all-Authored session has no possible isolated run and no
	// possible gap, no matter how it's shuffled -- a permutation of an
	// all-one-class sequence is invariant under shuffling.
	sessions := [][]Class{
		{Authored, Authored, Authored, Authored, Authored},
	}
	isolated, gap := ShuffleBaseline(sessions, 10, 1)
	if isolated != 0 {
		t.Errorf("isolatedFrac = %v, want 0 (single run of length 5, never length 1)", isolated)
	}
	if gap != 0 {
		t.Errorf("meanGap = %v, want 0 (no non-authored steps to form a gap)", gap)
	}
}

func TestShuffleBaselineIsReproducible(t *testing.T) {
	sessions := [][]Class{
		{Authored, Bare, Bare, Authored, Repeat, Authored, Authored, Bare, Authored},
		{Authored, Derived, Derived, Authored, Bare, Authored},
	}
	i1, g1 := ShuffleBaseline(sessions, 25, 42)
	i2, g2 := ShuffleBaseline(sessions, 25, 42)
	if i1 != i2 || g1 != g2 {
		t.Errorf("same seed produced different results: (%v,%v) vs (%v,%v)", i1, g1, i2, g2)
	}
}

func TestShuffleBaselineZeroTrialsIsZero(t *testing.T) {
	isolated, gap := ShuffleBaseline([][]Class{{Authored, Bare}}, 0, 1)
	if isolated != 0 || gap != 0 {
		t.Errorf("zero trials should return (0, 0), got (%v, %v)", isolated, gap)
	}
}
