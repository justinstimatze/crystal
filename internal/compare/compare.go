// Package compare holds the per-tool fidelity comparators — the
// load-bearing logic of the eval harness.
//
// There is no single string-equality comparator: toolUseResult is shaped
// differently per tool, so each tool gets its own Comparator. A tool with
// no registered comparator is *unverifiable* and can never be promoted
// (hard rule #2: no verifier, no crystallization).
//
// Every comparator's FIRST check is the outcome-class gate: produced and
// historical must agree on IsError. Flipping a historical error into a
// success shape is the highest-value regression for a drift detector and
// must never pass.
package compare

import (
	"regexp"
	"strings"

	"github.com/justinstimatze/crystal/internal/record"
)

// Verdict is the result of comparing one produced Output against the
// historical (frontier) Output for the same input.
type Verdict struct {
	Match      bool
	Similarity float64 // 0..1
	Reason     string  // localized divergence explanation when !Match
}

// Comparator compares a produced tool result against the historical one.
type Comparator interface {
	Tool() string
	Compare(produced, historical record.Output) Verdict
}

var registry = map[string]Comparator{}

func register(c Comparator) { registry[c.Tool()] = c }

// Lookup returns the comparator for a tool. A miss means the tool is
// unverifiable — callers must treat this as "never promote", not a pass.
func Lookup(tool string) (Comparator, bool) {
	c, ok := registry[tool]
	return c, ok
}

// Tools returns the registered tool names (for reporting/quotas).
func Tools() []string {
	out := make([]string, 0, len(registry))
	for t := range registry {
		out = append(out, t)
	}
	return out
}

// outcomeGate is every comparator's first check. It returns a reject
// Verdict and true when the outcome classes differ.
func outcomeGate(produced, historical record.Output) (Verdict, bool) {
	if produced.IsError != historical.IsError {
		return Verdict{Match: false, Similarity: 0, Reason: "outcome class differs (error vs success)"}, true
	}
	// Both errors: compare the error envelopes, never success fields.
	if historical.IsError {
		if strings.TrimSpace(produced.Scalar) == strings.TrimSpace(historical.Scalar) {
			return Verdict{Match: true, Similarity: 1}, true
		}
		return Verdict{Match: false, Reason: "error envelope differs"}, true
	}
	return Verdict{}, false
}

func match() Verdict          { return Verdict{Match: true, Similarity: 1} }
func reject(r string) Verdict { return Verdict{Match: false, Similarity: 0, Reason: r} }

// --- benign-volatility normalization (explicit allowlist, never global) ---

var volatile = []*regexp.Regexp{
	regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:?\d{2})?\b`), // ISO timestamps
	regexp.MustCompile(`\b\d{2}:\d{2}:\d{2}\b`),                                                   // clock times
	regexp.MustCompile(`/tmp/[A-Za-z0-9._\-/]+`),                                                  // tmp paths
	regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`),                                                      // hex addresses
	regexp.MustCompile(`\bpid[= ]\d+\b`),                                                          // pids
	regexp.MustCompile(`out-\d+\.log`),                                                            // timestamped log names
}

// normalizeVolatile collapses allowlisted volatile spans to fixed tokens.
// It returns the normalized string and the number of bytes rewritten, so
// callers (and the over-normalization counter-test) can detect a
// normalizer that is masking too much.
func normalizeVolatile(s string) (string, int) {
	changed := 0
	for _, re := range volatile {
		s = re.ReplaceAllStringFunc(s, func(m string) string {
			changed += len(m)
			return "<v>"
		})
	}
	return s, changed
}

// NormalizedBytesChanged exposes the volatility-normalizer's rewrite count
// for a string (used by the over-normalization counter-test).
func NormalizedBytesChanged(s string) int {
	_, n := normalizeVolatile(s)
	return n
}
