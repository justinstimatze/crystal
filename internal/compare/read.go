package compare

import (
	"github.com/justinstimatze/crystal/internal/record"
)

type readComparator struct{}

func (readComparator) Tool() string { return "Read" }

// Compare matches Read on the returned file content.
func (readComparator) Compare(produced, historical record.Output) Verdict {
	if v, done := outcomeGate(produced, historical); done {
		return v
	}
	if produced.File != historical.File {
		return reject("file content differs")
	}
	return match()
}

func init() { register(readComparator{}) }
