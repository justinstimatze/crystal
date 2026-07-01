// Package library is crystal's serve-from-a-library layer, modeled on
// groupchat's deployed meme system: a library of crystallized artifacts, each
// with applicability metadata, served per-context behind a CONFIDENCE + COOLDOWN
// gate, abstaining when uncertain ("a wrong artifact is worse than no artifact").
//
// This is the serve tier of the recipe ladder (docs/RECIPE_LADDER.md) and the
// rule-library dispatch the README roadmaps. The serve DECISION is deterministic
// (keyword-signature match, no model) — that is what makes it the cheap tier:
// reproducible, gate-able, zero round-trip. The artifact a matched entry carries
// is the recipe a weaker executor (or pure code) then runs.
package library

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
)

// Entry is one crystallized artifact plus the groupchat-style metadata that
// decides when to serve it.
type Entry struct {
	Name       string   `json:"name"`        // unique id
	Rung       string   `json:"rung"`        // plan|recipe|pseudocode|code — ladder position
	DeployWhen string   `json:"deploy_when"` // human: when this applies
	TooMuchIf  string   `json:"too_much_if"` // human: when NOT to serve even on a match
	Mechanism  string   `json:"mechanism"`   // human: why it works (audit)
	Key        string   `json:"key"`         // distinguishing fingerprint vs similar entries
	Match      []string `json:"match"`       // deterministic trigger tokens
	Avoid      []string `json:"avoid"`       // deterministic too-much guard tokens
	Artifact   string   `json:"artifact"`    // the recipe/rule body the cheap tier serves
	MinConf    float64  `json:"min_conf"`    // confidence floor to serve (the gate)
}

// Library is a set of entries plus per-entry serve state (cooldown + demotion).
type Library struct {
	Entries    []Entry
	Window     int            // cooldown window: serving again within W steps needs a higher bar
	Penalty    float64        // extra confidence required while on cooldown
	lastServed map[string]int // entry name → step it was last served
	demoted    map[string]bool
}

// New builds a library with the default cooldown discipline (tight for W steps).
func New(entries []Entry) *Library {
	return &Library{
		Entries:    entries,
		Window:     3,
		Penalty:    0.3,
		lastServed: map[string]int{},
		demoted:    map[string]bool{},
	}
}

// Load reads a library spec (JSON {"entries":[...]}) from disk.
func Load(path string) (*Library, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Entries []Entry `json:"entries"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	return New(doc.Entries), nil
}

// Demote marks an entry as drifted — it is never served again until re-promoted.
func (l *Library) Demote(name string) { l.demoted[name] = true }

// Promote clears a demotion (the inverse, for the closed loop).
func (l *Library) Promote(name string) { delete(l.demoted, name) }

// ServeState is the cross-invocation serve state a LIVE hook persists so
// cooldown and demotion survive the fresh-process-per-event boundary: each
// Claude Code hook call is a separate process, so the cooldown window and the
// demoted set only exist because they round-trip through a state file on disk —
// that disk round-trip IS the "live" part (the same shape the classifier hook's
// drift window uses). Step advances once per event and is carried here too.
type ServeState struct {
	Step       int            `json:"step"`
	LastServed map[string]int `json:"last_served"`
	Demoted    []string       `json:"demoted"`
}

// Snapshot captures the serve state (at the given step) for persistence.
func (l *Library) Snapshot(step int) ServeState {
	ls := make(map[string]int, len(l.lastServed))
	for k, v := range l.lastServed {
		ls[k] = v
	}
	dem := make([]string, 0, len(l.demoted))
	for k := range l.demoted {
		dem = append(dem, k)
	}
	sort.Strings(dem)
	return ServeState{Step: step, LastServed: ls, Demoted: dem}
}

// Restore reinstates a persisted serve state (cooldown map + demotions) onto a
// freshly-loaded library — how a new hook process picks up where the last left
// off. Demotions are additive (never silently cleared by a restore).
func (l *Library) Restore(s ServeState) {
	if s.LastServed != nil {
		l.lastServed = s.LastServed
	}
	for _, n := range s.Demoted {
		l.demoted[n] = true
	}
}

// Decision is the outcome of one serve attempt.
type Decision struct {
	Entry      *Entry  // the served entry, nil on abstain
	Confidence float64 // best match confidence found
	Threshold  float64 // the (cooldown-adjusted) bar it had to clear
	Outcome    string  // served | abstain-low-conf | abstain-cooldown | abstain-too-much | demoted-skip | no-match
}

// Served reports whether an artifact was actually served.
func (d Decision) Served() bool { return d.Outcome == "served" }

// Serve matches ctx against the library at the given step and either serves the
// best entry (high enough confidence, cooldown clear, no too-much guard) or
// abstains. Abstaining beats serving the wrong artifact.
func (l *Library) Serve(ctx string, step int) Decision {
	toks := tokenize(ctx)

	var best *Entry
	var bestConf float64
	for i := range l.Entries {
		e := &l.Entries[i]
		if l.demoted[e.Name] {
			continue
		}
		c := confidence(e, toks)
		if c > bestConf {
			best, bestConf = e, c
		}
	}
	if best == nil || bestConf == 0 {
		return Decision{Confidence: bestConf, Outcome: "no-match"}
	}
	// Too-much guard: a context that trips an Avoid token is a no-serve even on a match.
	if tripsAvoid(best, toks) {
		return Decision{Entry: best, Confidence: bestConf, Outcome: "abstain-too-much"}
	}
	// Cooldown: serving the same entry again within the window needs a higher bar.
	thr := best.MinConf
	onCooldown := false
	if last, ok := l.lastServed[best.Name]; ok && step-last <= l.Window {
		thr += l.Penalty
		onCooldown = true
	}
	if bestConf < thr {
		out := "abstain-low-conf"
		if onCooldown {
			out = "abstain-cooldown"
		}
		return Decision{Entry: best, Confidence: bestConf, Threshold: thr, Outcome: out}
	}
	l.lastServed[best.Name] = step
	return Decision{Entry: best, Confidence: bestConf, Threshold: thr, Outcome: "served"}
}

// confidence is the deterministic match score: fraction of an entry's trigger
// tokens present in the context.
func confidence(e *Entry, toks map[string]bool) float64 {
	if len(e.Match) == 0 {
		return 0
	}
	hit := 0
	for _, m := range e.Match {
		if toks[strings.ToLower(m)] {
			hit++
		}
	}
	return float64(hit) / float64(len(e.Match))
}

func tripsAvoid(e *Entry, toks map[string]bool) bool {
	for _, a := range e.Avoid {
		if toks[strings.ToLower(a)] {
			return true
		}
	}
	return false
}

func tokenize(ctx string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.Fields(strings.ToLower(ctx)) {
		out[strings.Trim(f, ".,;:!?\"'`()[]{}")] = true
		out[f] = true // keep the flag form too (e.g. "-a")
	}
	return out
}

// ByRung groups entries by ladder rung (for reporting the library's shape).
func (l *Library) ByRung() map[string]int {
	m := map[string]int{}
	for _, e := range l.Entries {
		m[e.Rung]++
	}
	return m
}

// Rungs returns the rung labels present, sorted.
func (l *Library) Rungs() []string {
	seen := l.ByRung()
	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}
