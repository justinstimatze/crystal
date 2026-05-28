package compare

import (
	"github.com/justinstimatze/crystal/internal/record"
)

type editComparator struct{}

func (editComparator) Tool() string { return "Edit" }

// Compare matches Edit on the structuredPatch hunks and the resulting
// newString. structuredPatch is compared as canonical bytes; newString
// must be exactly equal.
func (editComparator) Compare(produced, historical record.Output) Verdict {
	if v, done := outcomeGate(produced, historical); done {
		return v
	}
	if produced.NewString != historical.NewString {
		return reject("newString differs")
	}
	if !patchEqual(produced.StructuredPatch, historical.StructuredPatch) {
		return reject("structuredPatch differs")
	}
	return match()
}

func init() { register(editComparator{}) }
