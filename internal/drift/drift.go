// Package drift tests the project's riskiest assumption: that drift
// detection catches real distribution shift fast enough to prevent
// sustained silent-wrong output (the brief's undercity nightmare).
//
// The experiment is temporal replay. A crystallized artifact is promoted
// on a pattern's EARLY occurrences, then its LATER occurrences are streamed
// in timestamp order. The detector serves the artifact's output, compares
// it to what the frontier actually produced, and demotes after K
// consecutive divergences (the brief's rule). It reports not just whether
// it demoted, but how many wrong outputs LEAKED before demotion — the real
// safety measure.
//
// It deliberately surfaces the consecutive-K rule's weakness: intermittent
// (flapping) drift never accumulates K-in-a-row, so it can leak unbounded
// silent-wrong output without ever demoting.
package drift

import (
	"github.com/justinstimatze/crystal/internal/artifact"
	"github.com/justinstimatze/crystal/internal/compare"
	"github.com/justinstimatze/crystal/internal/record"
	"github.com/justinstimatze/crystal/internal/redact"
)

// Report is the outcome of a temporal replay.
type Report struct {
	Pattern          string  `json:"pattern"`
	Tool             string  `json:"tool"`
	RuleM            int     `json:"rule_m"` // demote on M divergences...
	RuleW            int     `json:"rule_w"` // ...within a sliding window of W (W==M == consecutive-K)
	TrainN           int     `json:"train_n"`
	TrainDeterminism float64 `json:"train_determinism"`
	Promoted         bool    `json:"promoted"` // train determinism met the gate
	StreamN          int     `json:"stream_n"`
	ServedCorrect    int     `json:"served_correct"`  // matched the frontier
	Leaked           int     `json:"leaked"`          // served WRONG before demotion (the danger)
	MaxConsecutive   int     `json:"max_consecutive"` // longest divergence run seen
	DemotedAtIndex   int     `json:"demoted_at_index"`
	Decision         string  `json:"decision"` // "served-clean" | "demoted" | "leaked-without-demote" | "unverifiable"
}

// Replay promotes on train, then streams test in order, returning the
// safety report. test MUST already be timestamp-ordered by the caller.
//
// Demotion rule: M divergences within a sliding window of the last W
// outputs. This generalizes the brief's consecutive-K rule — W==M==K is
// exactly "K in a row" — but with W>M it bounds leakage under intermittent
// (flapping) drift, which the consecutive-only rule lets leak unbounded.
func Replay(name string, tool string, train, test []record.Record, m, w int, promoteThr float64, loud bool) Report {
	art, trainDet := artifact.NewModal(name, train)
	rep := Report{
		Pattern: name, Tool: tool, RuleM: m, RuleW: w,
		TrainN: len(train), TrainDeterminism: trainDet,
		Promoted: trainDet >= promoteThr,
		StreamN:  len(test), DemotedAtIndex: -1,
	}
	cmp, ok := compare.Lookup(tool)
	if !ok {
		rep.Decision = "unverifiable"
		return rep
	}
	// A pattern that wouldn't promote shouldn't be deployed at all — report
	// it, but still replay so we can see what demotion WOULD have done.
	window := make([]bool, 0, w) // true = diverged
	divInWindow := 0
	consecutive := 0
	for i, r := range test {
		produced, err := art.Produce(r)
		if err != nil {
			continue
		}
		diverged := !cmp.Compare(produced, r.Result).Match
		if diverged {
			rep.Leaked++
			consecutive++
			if consecutive > rep.MaxConsecutive {
				rep.MaxConsecutive = consecutive
			}
		} else {
			rep.ServedCorrect++
			consecutive = 0
		}
		window = append(window, diverged)
		if diverged {
			divInWindow++
		}
		if len(window) > w {
			if window[0] {
				divInWindow--
			}
			window = window[1:]
		}
		if divInWindow >= m {
			rep.DemotedAtIndex = i
			if loud {
				redact.Warnf("DEMOTE %q at stream index %d: %d/%d divergences in window (%d wrong outputs leaked before demotion)",
					name, i, divInWindow, w, rep.Leaked)
			}
			break
		}
	}
	switch {
	case rep.DemotedAtIndex >= 0:
		rep.Decision = "demoted"
	case rep.Leaked > 0:
		// Diverged but the rule never fired, yet wrong outputs leaked. With
		// a consecutive rule (W==M) this is the dangerous intermittent-drift
		// evasion case; a wide-enough window bounds it.
		rep.Decision = "leaked-without-demote"
		if loud {
			redact.Warnf("UNDETECTED DRIFT %q: %d wrong outputs leaked, max consecutive run %d, rule %d-in-%d never fired",
				name, rep.Leaked, rep.MaxConsecutive, m, w)
		}
	default:
		rep.Decision = "served-clean"
	}
	return rep
}
