package step

import "github.com/justinstimatze/crystal/internal/record"

// RunProfile answers the question the step-level Profile can't: does an
// Authored move show up as an isolated one-step decision wrapped in
// mechanical steps, or does judgment come in multi-step stretches? A step is
// too small a unit to hand a cheap executor ("do the next Read") — the
// candidate unit above it is an EPISODE: a maximal run of mechanical steps
// bounded by two Authored decisions. RunProfile measures both sides of that
// hypothesis directly from the same classified steps, with no new model
// calls and no new corpus pass beyond what Profile already needs.
type RunProfile struct {
	// Runs is the contiguous run-length histogram per Class: Runs[c][n] is
	// the count of maximal runs of exactly n consecutive steps of class c.
	Runs map[Class]map[int]int
	// Gaps is the run-length histogram of MECHANICAL steps (any class other
	// than Authored) strictly between two consecutive Authored steps — the
	// candidate episode interior. Only gaps bounded by an Authored step on
	// both sides are counted; a mechanical run before the first or after the
	// last Authored step in a session isn't a bounded episode interior.
	Gaps map[int]int

	hasPrev   bool
	prevClass Class
	runLen    int
	prevNext  record.Record // Next of the last step folded in, for adjacency

	gapLen      int
	haveOpenGap bool // true once an Authored step has opened a gap to close
}

// NewRunProfile returns an empty RunProfile ready for incremental Add calls.
func NewRunProfile() *RunProfile {
	return &RunProfile{Runs: map[Class]map[int]int{}, Gaps: map[int]int{}}
}

// Add folds one step into the run/gap histograms. Steps must arrive in
// transcript order (as from Stream or a Streamer callback); Add detects
// adjacency itself from the Prev/Next record identity, so a caller can feed
// it directly without pre-checking turn/session boundaries.
func (p *RunProfile) Add(s Step) {
	adjacent := p.hasPrev &&
		p.prevNext.SessionID == s.Prev.SessionID &&
		p.prevNext.Turn == s.Prev.Turn &&
		p.prevNext.Seq == s.Prev.Seq

	newRun := !adjacent || s.Class != p.prevClass
	if newRun {
		if adjacent {
			// The run that just ended is real -- close it, and if it was
			// Authored, start counting the gap toward the next Authored
			// run. Only doing this at the run boundary (not on every
			// Authored step) is what keeps two adjacent same-run Authored
			// steps from registering as a spurious zero-length gap.
			p.closeRun()
			if p.prevClass == Authored {
				p.gapLen, p.haveOpenGap = 0, true
			}
		} else {
			// A session/turn boundary: nothing open before it bounds
			// anything real on this side, so drop it rather than count a
			// gap or run across the seam.
			p.closeRun()
			p.gapLen, p.haveOpenGap = 0, false
		}
		if s.Class == Authored && p.haveOpenGap {
			p.Gaps[p.gapLen]++
			p.haveOpenGap = false
		}
		p.prevClass, p.runLen = s.Class, 1
	} else {
		p.runLen++
	}

	if s.Class != Authored && p.haveOpenGap {
		p.gapLen++
	}

	p.prevNext, p.hasPrev = s.Next, true
}

// Flush closes out the run in progress. Call once after the last step of the
// whole stream — Add only closes a run when it sees the NEXT step, so the
// final run is never counted without this.
func (p *RunProfile) Flush() {
	p.closeRun()
}

func (p *RunProfile) closeRun() {
	if p.hasPrev && p.runLen > 0 {
		m := p.Runs[p.prevClass]
		if m == nil {
			m = map[int]int{}
			p.Runs[p.prevClass] = m
		}
		m[p.runLen]++
	}
	p.runLen = 0
}

// IsolatedFrac returns the fraction of class c's runs that are length 1 —
// a single-step occurrence, not part of a longer same-class stretch. For
// Authored, high IsolatedFrac means judgment shows up as one decision at a
// time (supports the episode hypothesis: a small authored kernel wrapped in
// mechanical steps); low IsolatedFrac means judgment itself comes in
// multi-step runs a single-decision episode boundary would cut through.
func (p RunProfile) IsolatedFrac(c Class) float64 {
	m := p.Runs[c]
	if len(m) == 0 {
		return 0
	}
	var total, isolated int
	for n, count := range m {
		total += count
		if n == 1 {
			isolated += count
		}
	}
	if total == 0 {
		return 0
	}
	return float64(isolated) / float64(total)
}

// MeanGap returns the mean length of mechanical runs strictly between two
// Authored steps — the average size of a candidate episode interior. Zero
// gaps observed returns 0, which the caller must distinguish from "no
// interior exists" (check GapCount).
func (p RunProfile) MeanGap() float64 {
	n, sum := p.GapCount()
	if n == 0 {
		return 0
	}
	return float64(sum) / float64(n)
}

// GapCount returns the number of bounded gaps observed and their total
// length in steps.
func (p RunProfile) GapCount() (n, sum int) {
	for length, count := range p.Gaps {
		n += count
		sum += length * count
	}
	return n, sum
}
