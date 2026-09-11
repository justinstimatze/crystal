package step

import "math/rand"

// classRunStats computes the same run-length and authored-gap histograms as
// RunProfile, but over a flat, already-contiguous class sequence (one
// session's steps, in order, with no turn/session boundaries to detect —
// the caller has already sliced to one contiguous run). It's the shared
// core ShuffleBaseline needs: shuffling only makes sense within a session's
// own sequence, so the null model is built on this rather than RunProfile's
// Step-based adjacency detection.
func classRunStats(classes []Class) (runs map[Class]map[int]int, gaps map[int]int) {
	runs = map[Class]map[int]int{}
	gaps = map[int]int{}
	bump := func(c Class, n int) {
		m := runs[c]
		if m == nil {
			m = map[int]int{}
			runs[c] = m
		}
		m[n]++
	}

	var prev Class
	runLen := 0
	gapLen := 0
	haveOpenGap := false
	for i, c := range classes {
		newRun := i == 0 || c != prev
		if newRun {
			if i > 0 {
				bump(prev, runLen)
				// The run that just ended was Authored -- start counting
				// the gap toward the NEXT Authored run. Opening this only
				// at a run boundary (not on every Authored step) is what
				// keeps two adjacent same-run Authored steps from
				// registering as a spurious zero-length gap.
				if prev == Authored {
					gapLen, haveOpenGap = 0, true
				}
			}
			if c == Authored && haveOpenGap {
				gaps[gapLen]++
				haveOpenGap = false
			}
			prev, runLen = c, 1
		} else {
			runLen++
		}
		if c != Authored && haveOpenGap {
			gapLen++
		}
	}
	if len(classes) > 0 {
		bump(prev, runLen)
	}
	return runs, gaps
}

// isolatedFrac and meanGap read the same two summary numbers RunProfile
// exposes, but from the raw histograms classRunStats returns.
func isolatedFrac(runs map[Class]map[int]int, c Class) float64 {
	m := runs[c]
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

func meanGap(gaps map[int]int) float64 {
	var n, sum int
	for length, count := range gaps {
		n += count
		sum += length * count
	}
	if n == 0 {
		return 0
	}
	return float64(sum) / float64(n)
}

// ShuffleBaseline answers the question RunProfile's numbers can't answer on
// their own: is observed clustering a real ORDERING effect, or just an
// artifact of averaging one global authored-rate over sessions/shapes with
// very different LOCAL rates (Bash→Bash runs authored 96%+ of the time;
// TaskUpdate→TaskUpdate under 17%)? A single shuffle of one session's class
// sequence preserves that session's own class multiset — its local rate is
// untouched — and destroys only the temporal order. Averaging many shuffles
// gives the isolated-fraction and mean-gap you'd expect from rate
// heterogeneity ALONE, with zero real structure. Sessions are shuffled and
// scored independently and pooled, matching how RunProfile pools sessions.
//
// seed makes the trials reproducible — this is a randomized null, not a
// nondeterministic one; the same seed and inputs always reproduce the same
// baseline.
func ShuffleBaseline(sessions [][]Class, trials int, seed int64) (isolatedAuthoredFrac, meanGapSteps float64) {
	if trials <= 0 {
		return 0, 0
	}
	rng := rand.New(rand.NewSource(seed))
	var totalRuns, totalIsolated, totalGapN, totalGapSum int

	buf := make([]Class, 0, 4096)
	for t := 0; t < trials; t++ {
		for _, seq := range sessions {
			buf = append(buf[:0], seq...)
			rng.Shuffle(len(buf), func(i, j int) { buf[i], buf[j] = buf[j], buf[i] })

			runs, gaps := classRunStats(buf)
			for n, count := range runs[Authored] {
				totalRuns += count
				if n == 1 {
					totalIsolated += count
				}
			}
			for length, count := range gaps {
				totalGapN += count
				totalGapSum += length * count
			}
		}
	}
	if totalRuns > 0 {
		isolatedAuthoredFrac = float64(totalIsolated) / float64(totalRuns)
	}
	if totalGapN > 0 {
		meanGapSteps = float64(totalGapSum) / float64(totalGapN)
	}
	return isolatedAuthoredFrac, meanGapSteps
}
