// Package measure sweeps signature granularities over the substrate to
// answer the existential Phase 3 question: does a crystallizable pattern
// exist at all? A pattern is crystallizable when an input-signature group
// is both frequent (N >= minSamples) and deterministic — its outputs
// collapse to a single comparator-equivalence class (>= promote fidelity).
//
// Frequency and determinism pull against each other: looser signatures
// cluster more records but mix more distinct outputs. This sweep makes
// that trade-off visible per granularity instead of guessing.
//
// It streams: each record is reduced to its signature keys and a hashed
// output fingerprint, then discarded. Memory is bounded by the number of
// distinct groups, not the (large) total content of the substrate.
package measure

import (
	"hash/fnv"
	"sort"

	"github.com/justinstimatze/crystal/internal/compare"
	"github.com/justinstimatze/crystal/internal/eval"
	"github.com/justinstimatze/crystal/internal/record"
)

// GroupStat describes one signature group.
type GroupStat struct {
	Sig             string  `json:"sig"`
	Tool            string  `json:"tool"`
	N               int     `json:"n"`
	DistinctOutputs int     `json:"distinct_outputs"`
	Determinism     float64 `json:"determinism"` // largest equivalence class / N
}

// Crystallizable reports whether this group meets the promote gate.
func (g GroupStat) Crystallizable(minSamples int, promote float64) bool {
	return g.N >= minSamples && g.Determinism >= promote
}

// GranularityReport summarizes one signature definition over the corpus.
type GranularityReport struct {
	Name           string      `json:"name"`
	TotalGroups    int         `json:"total_groups"`
	GroupsAtSize   int         `json:"groups_at_min_samples"` // N >= minSamples
	Crystallizable []GroupStat `json:"crystallizable"`        // frequent AND deterministic
	TopBySize      []GroupStat `json:"top_by_size"`           // largest groups, for context
}

type groupAgg struct {
	sample  string         // first-seen signature, for display
	tool    string         // first-seen tool
	n       int            // group size
	classes map[uint64]int // output-fingerprint hash -> count
}

// Accumulator ingests records one at a time, retaining only small
// per-group aggregates. Safe to feed the entire substrate without holding
// record content in memory.
type Accumulator struct {
	// per granularity index -> (group-key hash -> aggregate)
	groups []map[uint64]*groupAgg
}

// New returns an Accumulator wired to eval.Signatures.
func New() *Accumulator {
	g := make([]map[uint64]*groupAgg, len(eval.Signatures))
	for i := range g {
		g[i] = map[uint64]*groupAgg{}
	}
	return &Accumulator{groups: g}
}

// Add folds one record into every granularity's group index.
func (a *Accumulator) Add(r record.Record) {
	fp := hash64(compare.Fingerprint(r.Tool, r.Result))
	for i, sig := range eval.Signatures {
		key := sig.Fn(r)
		h := hash64(key)
		ag := a.groups[i][h]
		if ag == nil {
			// Store a truncated sample for display only; keeping full keys
			// (exact-granularity Bash commands are large, ~137k of them)
			// is what drove RSS to GBs. The hash is the identity.
			ag = &groupAgg{sample: sampleTrunc(key), tool: r.Tool, classes: map[uint64]int{}}
			a.groups[i][h] = ag
		}
		ag.n++
		ag.classes[fp]++
	}
}

// Report renders the sweep.
func (a *Accumulator) Report(minSamples int, promote float64) []GranularityReport {
	var out []GranularityReport
	for i, sig := range eval.Signatures {
		out = append(out, a.reportOne(sig.Name, a.groups[i], minSamples, promote))
	}
	return out
}

func (a *Accumulator) reportOne(name string, groups map[uint64]*groupAgg, minSamples int, promote float64) GranularityReport {
	rep := GranularityReport{Name: name, TotalGroups: len(groups)}
	stats := make([]GroupStat, 0, len(groups))
	for _, ag := range groups {
		st := statOf(ag)
		stats = append(stats, st)
		if st.N >= minSamples {
			rep.GroupsAtSize++
			if st.Crystallizable(minSamples, promote) {
				rep.Crystallizable = append(rep.Crystallizable, st)
			}
		}
	}
	sort.Slice(rep.Crystallizable, func(i, j int) bool { return rep.Crystallizable[i].N > rep.Crystallizable[j].N })
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].N != stats[j].N {
			return stats[i].N > stats[j].N
		}
		return stats[i].Sig < stats[j].Sig
	})
	for i := 0; i < len(stats) && i < 8; i++ {
		rep.TopBySize = append(rep.TopBySize, stats[i])
	}
	return rep
}

func statOf(ag *groupAgg) GroupStat {
	max := 0
	for _, c := range ag.classes {
		if c > max {
			max = c
		}
	}
	det := 0.0
	if ag.n > 0 {
		det = float64(max) / float64(ag.n)
	}
	return GroupStat{Sig: ag.sample, Tool: ag.tool, N: ag.n, DistinctOutputs: len(ag.classes), Determinism: det}
}

func hash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

// sampleTrunc bounds a stored display sample. Group identity is the hash;
// the sample is only for human-readable output, so a long key need not be
// retained in full across hundreds of thousands of groups.
func sampleTrunc(s string) string {
	const max = 160
	if len(s) > max {
		return s[:max]
	}
	return s
}
