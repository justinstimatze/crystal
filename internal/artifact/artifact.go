// Package artifact defines the thing the eval harness scores: a candidate
// crystallized hook. Given a Record's input, it produces a tool Output.
// The eval replays it against historical Records and compares the produced
// Output to the frontier's actual one.
//
// In Phase 1 the only artifacts are synthetic (see synthetic.go): an
// identity reproducer that must promote, deliberate corruptors that must
// be rejected, and a benign-volatility artifact that must NOT be falsely
// rejected. These prove the eval's sensitivity and specificity before any
// real artifact exists.
package artifact

import "github.com/justinstimatze/crystal/internal/record"

// Artifact produces a tool Output for a given input Record.
type Artifact interface {
	Name() string
	Produce(in record.Record) (record.Output, error)
}
