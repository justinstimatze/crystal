package compare

import (
	"sort"

	"github.com/justinstimatze/crystal/internal/record"
)

type grepComparator struct{}

func (grepComparator) Tool() string { return "Grep" }

// Compare matches Grep on the set of matched filenames (order-independent)
// and the file count.
func (grepComparator) Compare(produced, historical record.Output) Verdict {
	if v, done := outcomeGate(produced, historical); done {
		return v
	}
	if produced.NumFiles != historical.NumFiles {
		return reject("numFiles differs")
	}
	if !sameSet(produced.Filenames, historical.Filenames) {
		return reject("matched filename set differs")
	}
	// Match content too when present (Grep results carry the matched text);
	// a content drift is a real regression even if the filename set holds.
	if produced.File != historical.File {
		return reject("match content differs")
	}
	return match()
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}

func init() { register(grepComparator{}) }
