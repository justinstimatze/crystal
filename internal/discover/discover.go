// Package discover is the watch-don't-ask front half of the codegen loop: given
// a parsed corpus, it finds the DOMINANT recurring unit-shape by structure and
// surfaces it as the crystallization candidate — without being told which
// structs are the chore. It is the codegen sibling of the Bash `sweep`/`measure`
// discovery: crystal crystallizes only what it finds RECURRING.
//
// The signal is recurrence (how many units share the shape) and coverage (what
// fraction of the scanned structs the dominant shape accounts for). The shape is
// detected structurally (cmdspec.Commandish — help-tagged flag fields), so a
// help-tagged command not named "*Cmd" is found and a bare-field helper that is
// named "*Cmd" is rejected.
package discover

import (
	"sort"

	"github.com/justinstimatze/crystal/internal/cmdspec"
)

// Cluster is a set of corpus units sharing a structural shape.
type Cluster struct {
	Signature string            // human label for the shape
	Members   []cmdspec.CmdSpec // the units in this shape
}

// Recurrence is how many times the shape recurs in the corpus.
func (c Cluster) Recurrence() int { return len(c.Members) }

// Breakdown counts the within-shape variety the downstream loop cares about:
// how many members are the regular scalar-flag shape vs the irregular ones the
// generator will first drift on.
func (c Cluster) Breakdown() (regular, enum, slice int) {
	for _, m := range c.Members {
		switch {
		case m.HasEnum():
			enum++
		case m.HasSlice():
			slice++
		default:
			regular++
		}
	}
	return regular, enum, slice
}

// Report is the discovery result over one corpus.
type Report struct {
	Scanned   int       // total structs parsed
	Clusters  []Cluster // shape clusters, recurrence desc
	Dominant  Cluster   // the largest command-shaped cluster (the candidate)
	Coverage  float64   // Dominant.Recurrence / Scanned
	Candidate bool      // dominant recurrence cleared the threshold
}

// Scan classifies every struct, clusters the command-shaped ones into the
// dominant candidate, and reports the rest as a non-command residue. minRecur is
// the recurrence floor below which there is no crystallizable chore (loud).
func Scan(all []cmdspec.CmdSpec, minRecur int) Report {
	var command, other []cmdspec.CmdSpec
	for _, s := range all {
		if s.Commandish() {
			command = append(command, s)
		} else {
			other = append(other, s)
		}
	}
	dom := Cluster{Signature: "command (help-tagged flag struct)", Members: command}
	clusters := []Cluster{dom}
	if len(other) > 0 {
		clusters = append(clusters, Cluster{Signature: "non-command struct (helpers, configs)", Members: other})
	}
	sort.SliceStable(clusters, func(i, j int) bool {
		return clusters[i].Recurrence() > clusters[j].Recurrence()
	})
	rep := Report{Scanned: len(all), Clusters: clusters, Dominant: dom}
	if len(all) > 0 {
		rep.Coverage = float64(dom.Recurrence()) / float64(len(all))
	}
	rep.Candidate = dom.Recurrence() >= minRecur
	return rep
}
