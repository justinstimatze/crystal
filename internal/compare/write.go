package compare

import (
	"encoding/json"
	"sort"

	"github.com/justinstimatze/crystal/internal/record"
)

type writeComparator struct{}

func (writeComparator) Tool() string { return "Write" }

// Compare matches Write on the written content and structuredPatch.
func (writeComparator) Compare(produced, historical record.Output) Verdict {
	if v, done := outcomeGate(produced, historical); done {
		return v
	}
	if produced.NewString != historical.NewString {
		return reject("content differs")
	}
	if !patchEqual(produced.StructuredPatch, historical.StructuredPatch) {
		return reject("structuredPatch differs")
	}
	return match()
}

// patchEqual compares two structuredPatch values as order-independent sets
// of hunks. A patch is a JSON array; each hunk is canonicalized to its
// JSON encoding, sorted, then compared. Falls back to raw-byte equality
// when either side is not a JSON array.
func patchEqual(a, b json.RawMessage) bool {
	as, aok := hunkSet(a)
	bs, bok := hunkSet(b)
	if !aok || !bok {
		return string(a) == string(b)
	}
	if len(as) != len(bs) {
		return false
	}
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

func hunkSet(raw json.RawMessage) ([]string, bool) {
	if len(raw) == 0 {
		return nil, true
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) != nil {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, h := range arr {
		// Re-marshal through a generic value to canonicalize key order.
		var v any
		if json.Unmarshal(h, &v) != nil {
			out = append(out, string(h))
			continue
		}
		c, _ := json.Marshal(v)
		out = append(out, string(c))
	}
	sort.Strings(out)
	return out, true
}

func init() { register(writeComparator{}) }
