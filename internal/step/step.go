// Package step lifts the substrate from the TOOL CALL to the STEP — the
// move the model made between one result and the next call.
//
// This is the unit the rest of crystal was missing. A Record answers "what
// did this tool return", which is already deterministic given its input;
// the interesting variable is what the MODEL did next. A step is
//
//	(visible state after call k) → (the model's call k+1)
//
// and because a move is a tool call — a typed object with a name and args —
// comparing moves needs no LLM judge. That is what makes the frontier-work
// decomposition measurable with the same strong-verifier discipline the
// rest of the project already uses.
//
// Classification here is deliberately conservative: it measures how much of
// the next move's argument text was ALREADY PRESENT in the visible state.
// Presence means only that the information was available — a selection rule
// is still missing. So a "derived" step is not proof a script could produce
// it; it is the band where a cheap executor plus a harness plausibly could,
// and the band worth paying a real verifier to settle. Steps whose arguments
// contain text found nowhere in prior state are the judgment residual.
package step

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/justinstimatze/crystal/internal/record"
)

// Class buckets a step by how much of its move was recoverable from state
// the model could already see.
type Class string

const (
	// Repeat: this exact (tool, args) move already occurred in the session.
	// Pure memoization — the cheapest possible substitution.
	Repeat Class = "repeat"
	// Derived: every substantive argument string was present verbatim in the
	// visible state. Information-complete; only a selection rule is missing.
	Derived Class = "derived"
	// Partial: some argument text was present, some was not.
	Partial Class = "partial"
	// Authored: no substantive argument text appeared in prior state — the
	// model produced it. The judgment residual.
	Authored Class = "authored"
	// Bare: the move carries no substantive argument text to attribute
	// (e.g. an argument-free call), so derivation is not measurable.
	Bare Class = "bare"
)

// minTok is the shortest argument string worth attributing. Below this,
// containment is coincidence (every corpus contains "go", "-l", "n").
const minTok = 6

// Step is one model move: the call it made given the state it could see.
type Step struct {
	Prev record.Record // the call whose result is the input state
	Next record.Record // the move the model made

	Class Class
	// DerivedFrac is the fraction of substantive argument BYTES in Next
	// that appear verbatim in the visible state. A continuous readout of
	// how mechanical the move was; Class is a bucketing of it.
	DerivedFrac float64
	// Missing holds the argument strings that were NOT found in prior
	// state — the concrete text the model had to supply itself.
	Missing []string
}

// Build reconstructs steps from records in transcript order. A step is only
// formed between consecutive records in the same session AND the same turn:
// across a turn boundary the user supplied the state, not the prior result,
// so the model's move is not attributable to the previous call.
func Build(recs []record.Record) []Step {
	var steps []Step
	Stream(recs, func(s Step) { steps = append(steps, s) })
	return steps
}

// Streamer classifies steps from records fed one at a time, holding only a
// single-record lookback plus the set of moves already seen. This is what
// lets a multi-GB transcript be attributed in bounded memory: a step spans
// exactly two consecutive records, so nothing older need be retained.
type Streamer struct {
	fn   func(Step)
	prev record.Record
	has  bool
	seen map[string]bool
}

// NewStreamer returns a Streamer delivering each classified step to fn.
func NewStreamer(fn func(Step)) *Streamer {
	return &Streamer{fn: fn, seen: map[string]bool{}}
}

// Push feeds the next record in transcript order.
func (s *Streamer) Push(r record.Record) {
	if s.has && s.prev.SessionID == r.SessionID && s.prev.Turn == r.Turn {
		s.fn(classify(s.prev, r, s.seen))
	}
	if s.has && s.prev.SessionID != r.SessionID {
		s.seen = map[string]bool{}
	}
	s.seen[moveKey(r)] = true
	s.prev, s.has = r, true
}

// Stream is Build without materializing the result: it invokes fn once per
// step. A whole-corpus run produces hundreds of thousands of steps, each
// holding two Records, so the caller usually wants to fold each into an
// aggregate and drop it rather than keep them all.
func Stream(recs []record.Record, fn func(Step)) {
	ordered := append([]record.Record(nil), recs...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].SessionID != ordered[j].SessionID {
			return ordered[i].SessionID < ordered[j].SessionID
		}
		return ordered[i].Seq < ordered[j].Seq
	})

	seen := map[string]bool{}
	var curSession string
	for i, cur := range ordered {
		if cur.SessionID != curSession {
			curSession = cur.SessionID
			seen = map[string]bool{}
		}
		// seen now holds every move strictly before cur in this session.
		if i > 0 {
			prev := ordered[i-1]
			if prev.SessionID == cur.SessionID && prev.Turn == cur.Turn {
				fn(classify(prev, cur, seen))
			}
		}
		seen[moveKey(cur)] = true
	}
}

// Profile is the decomposition readout for a set of steps: what fraction of
// the model's moves were mechanical enough to shift off the frontier tier.
type Profile struct {
	N        int
	Counts   map[Class]int
	MeanFrac float64

	fracSum float64 // running sum behind MeanFrac
}

// Summarize aggregates steps into a Profile.
func Summarize(steps []Step) Profile {
	var p Profile
	p.Counts = map[Class]int{}
	for _, s := range steps {
		p.Add(s)
	}
	return p
}

// Add folds one step into the profile. Accumulating incrementally lets a
// caller stream a large corpus (one transcript at a time) without holding
// every Step in memory — the corpus is tens of GB of tool output, and a
// step only ever spans records within a single session anyway.
func (p *Profile) Add(s Step) {
	if p.Counts == nil {
		p.Counts = map[Class]int{}
	}
	p.Counts[s.Class]++
	p.fracSum += s.DerivedFrac
	p.N++
	p.MeanFrac = p.fracSum / float64(p.N)
}

// Shiftable is the fraction of steps whose move was fully recoverable from
// state the model could already see (repeat or derived) — the ceiling on
// what a harness plus a cheap executor could take over, before any verifier
// has confirmed a specific substitution.
func (p Profile) Shiftable() float64 {
	if p.N == 0 {
		return 0
	}
	return float64(p.Counts[Repeat]+p.Counts[Derived]) / float64(p.N)
}

// classify measures how much of next's move was recoverable from the state
// visible after prev.
func classify(prev, next record.Record, seen map[string]bool) Step {
	s := Step{Prev: prev, Next: next}

	if seen[moveKey(next)] {
		s.Class, s.DerivedFrac = Repeat, 1
		return s
	}

	args := argStrings(next.Args)
	if len(args) == 0 {
		s.Class = Bare
		return s
	}

	state := visibleState(prev)
	var total, found int
	for _, a := range args {
		total += len(a)
		if state.contains(a) {
			found += len(a)
			continue
		}
		s.Missing = append(s.Missing, a)
	}
	if total == 0 {
		s.Class = Bare
		return s
	}

	s.DerivedFrac = float64(found) / float64(total)
	switch {
	case found == total:
		s.Class = Derived
	case found == 0:
		s.Class = Authored
	default:
		s.Class = Partial
	}
	return s
}

// state is everything the model could read before choosing its next move,
// held as the ORIGINAL field slices rather than one concatenated copy.
// Joining them allocated a fresh multi-KB string per step, which on a real
// corpus is hundreds of thousands of copies of tool output — the whole
// working set, churned. Scanning the fields in place is equivalent (a match
// can only occur within a field; an argument that straddled a join boundary
// would be a false positive anyway) and allocates nothing.
type state struct {
	fields []string
	raw    []byte // verbatim result; scanned without being copied to a string
}

// contains reports whether any single visible field holds s verbatim.
func (st state) contains(s string) bool {
	for _, f := range st.fields {
		if len(f) >= len(s) && strings.Contains(f, s) {
			return true
		}
	}
	return len(st.raw) >= len(s) && bytes.Contains(st.raw, []byte(s))
}

// visibleState collects the previous call's result, its own arguments, and
// the surrounding prose (which carries the user's request).
func visibleState(prev record.Record) state {
	st := state{fields: make([]string, 0, 12)}
	// NOTE: prev.Context and prev.Followup are deliberately EXCLUDED. They
	// hold the model's own prose; counting it as available input would let
	// the model's stated intention ("now I'll read handler.go") make its own
	// next move look mechanical, inflating the shiftable ceiling by
	// construction. Only the user's prompt counts as given prose.
	st.fields = append(st.fields,
		prev.Result.Scalar, prev.Result.Stdout, prev.Result.Stderr,
		prev.Result.File, prev.Result.NewString,
		prev.UserPrompt,
	)
	st.fields = append(st.fields, prev.Result.Filenames...)
	st.fields = append(st.fields, argStrings(prev.Args)...)
	// Raw is the verbatim result and the only source for tools without a
	// typed extraction, so it is scanned last (it subsumes the typed
	// fields, which are cheaper to hit first).
	st.raw = prev.Result.Raw
	return st
}

// argStrings collects every substantive string leaf in an argument map,
// deduped and sorted for determinism.
func argStrings(args map[string]any) []string {
	set := map[string]bool{}
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			if s := strings.TrimSpace(t); len(s) >= minTok {
				set[s] = true
			}
		case []any:
			for _, e := range t {
				walk(e)
			}
		case map[string]any:
			for _, e := range t {
				walk(e)
			}
		}
	}
	for _, v := range args {
		walk(v)
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// moveKey identifies a move (tool + arguments) for repeat detection.
func moveKey(r record.Record) string {
	b, err := json.Marshal(r.Args)
	if err != nil {
		return r.Tool
	}
	return r.Tool + "\x00" + string(b)
}
