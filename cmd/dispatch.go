package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
)

// DispatchCmd is the scaling architecture the sweep's forward-constraint
// demanded (SWEEP_FINDINGS.md, the dispatcher seam). `guard` proved the
// constraint-loop shape with ONE hardcoded rule; but one PreToolUse hook PER
// rule means N process-forks per Bash call (~5.9ms × N — seconds at N=1000).
// `dispatch` is the fix: a SINGLE hook process that loads a rule LIBRARY and
// evaluates every rule in-process. It subsumes `guard` (the git-add rule is the
// default library's first entry).
//
// The honest data/code split (the design call): a rule is DATA — id, reason,
// enabled, and its own self-monitoring state — so the library scales to
// thousands and is per-user/shareable. But each rule references a MATCHER by
// name from a small, unit-tested registry (matchGitAddAll, …). A NAKED
// regex-per-rule would reintroduce the false-deny risk guard's typed matcher
// avoids (matching `git commit -m "add ."`) — so a data-driven "regex" matcher
// is allowed ONLY when emitted behind a producer-verifier GATE (`sweep
// --emit-dispatch`): the authored pattern must match the bad forms AND reject a
// benign-command set (the `git commit -m "add ."` false-deny is a gate negative).
// New rule with an existing matcher = pure data; a novel match = a gated regex,
// or a tested Go predicate for the registry (rare). The library is the registry of what's
// crystallized (the dir/file IS the answer to "what have I chunked"); the engine
// (this dispatcher + the matcher vocabulary + schema) is what ships publicly,
// each user growing their own library.
//
// Contract: same Claude Code PreToolUse deny/allow shape as `guard`. Per-rule
// state lives in one --state file keyed by rule id; each rule's override gate
// (the constraint drift signal) is evaluated independently.
type DispatchCmd struct {
	Rules string  `help:"Path to a rule-library JSON file. If absent, serves the built-in default library (the git-add-all constraint)." default:""`
	State string  `help:"Path to the per-rule self-monitoring state file (id → fire/override counts)." default:".crystal-dispatch-state.json"`
	MinN  int     `help:"Per-rule override gate: inactive until a rule has triggered this many times." default:"5"`
	Rate  float64 `help:"Per-rule override gate: flag NeedsRevision when a rule's bypasses/triggers ≥ this." default:"0.5"`
}

// libraryRule is one crystallized constraint, as DATA.
type libraryRule struct {
	ID      string `json:"id"`
	Matcher string `json:"matcher"`           // a key into the matcher registry, or "regex"
	Pattern string `json:"pattern,omitempty"` // the regex (used iff Matcher == "regex") — a DATA matcher needing no new code
	Reason  string `json:"reason"`            // deny message (the fix to point the model at)
	Enabled bool   `json:"enabled"`
}

type ruleLibrary struct {
	Rules []libraryRule `json:"rules"`
}

// bashMatcher reports whether a command trips the rule, and the offending form.
type bashMatcher func(command string) (bool, string)

// matcherRegistry is the tested-code vocabulary the data library draws on.
// Adding a key here is the only code change a new KIND of match needs; new rules
// reusing an existing matcher are pure data.
var matcherRegistry = map[string]bashMatcher{
	"git_add_all": matchGitAddAll,
}

// resolveMatcher returns the predicate for a rule. Named matchers come from the
// tested registry; the special "regex" matcher is DATA-driven — it compiles the
// rule's Pattern, so a newly-crystallized constraint is a pure data row needing
// no new Go code. An uncompilable pattern resolves to not-ok → fail-open.
func resolveMatcher(r libraryRule) (bashMatcher, bool) {
	if r.Matcher == "regex" {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, false
		}
		return func(command string) (bool, string) {
			if loc := re.FindString(command); loc != "" {
				return true, loc
			}
			return false, ""
		}, true
	}
	m, ok := matcherRegistry[r.Matcher]
	return m, ok
}

// defaultLibrary is served when no --rules file is given, so the dispatcher works
// out of the box and subsumes `guard`. As more rules promote, they land here (or
// in a user library file), NOT as new `crystal <rule>` subcommands.
func defaultLibrary() ruleLibrary {
	return ruleLibrary{Rules: []libraryRule{
		{
			ID:      "git-add-all",
			Matcher: "git_add_all",
			Reason: "crystal: `git add %s` stages everything indiscriminately — stage explicit " +
				"paths instead (`git add path/to/file ...`). Standing rule re-encoded in 4 projects. " +
				"Override one call with CRYSTAL_GUARD_SKIP=1.",
			Enabled: true,
		},
	}}
}

func loadLibrary(path string) (ruleLibrary, error) {
	if path == "" {
		return defaultLibrary(), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ruleLibrary{}, err
	}
	var lib ruleLibrary
	if err := json.Unmarshal(b, &lib); err != nil {
		return ruleLibrary{}, err
	}
	return lib, nil
}

// dispatchState is the per-rule self-monitoring substrate (id → guardState).
type dispatchState map[string]*guardState

func loadDispatchState(path string) dispatchState {
	st := dispatchState{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &st)
	}
	return st
}

func saveDispatchState(path string, st dispatchState) {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}

// decideDispatch is the pure core: evaluate the library against a command and
// the bypass flag, mutate per-rule state, and return the decision + reason. The
// first enabled, matched, non-bypassed rule denies (first-deny-wins); a matched
// rule under bypass is allowed but counted (the override drift signal). Unknown
// matchers are skipped fail-open. minN/rate drive each rule's override gate.
func decideDispatch(lib ruleLibrary, st dispatchState, command string, bypass bool, minN int, rate float64) (decision, reason string) {
	decision = "allow"
	for _, r := range lib.Rules {
		if !r.Enabled {
			continue
		}
		m, ok := resolveMatcher(r)
		if !ok {
			continue // unknown/uncompilable matcher → fail-open (a missing predicate never denies)
		}
		matched, form := m(command)
		if !matched {
			continue
		}
		rs := st[r.ID]
		if rs == nil {
			rs = &guardState{}
			st[r.ID] = rs
		}
		rs.Triggered++
		rs.record(command)
		if bypass {
			rs.Bypassed++
		} else {
			rs.Denied++
			if decision != "deny" { // first-deny-wins for the emitted reason
				decision = "deny"
				reason = fmt.Sprintf(r.Reason, form)
			}
		}
		if rs.overrideTripped(minN, rate) {
			rs.NeedsRevision = true
		}
	}
	return decision, reason
}

func (c *DispatchCmd) Run() error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return usageError{fmt.Errorf("reading hook event: %w", err)}
	}
	var ev hookEvent
	_ = json.Unmarshal(raw, &ev) // fail-open

	if ev.ToolName != "Bash" || ev.ToolInput.Command == "" {
		return emitGuard("allow", "")
	}
	lib, err := loadLibrary(c.Rules)
	if err != nil {
		return emitGuard("allow", "") // a broken library never blocks the host (fail-open)
	}
	st := loadDispatchState(c.State)
	bypass := os.Getenv("CRYSTAL_GUARD_SKIP") == "1"
	decision, reason := decideDispatch(lib, st, ev.ToolInput.Command, bypass, c.MinN, c.Rate)
	saveDispatchState(c.State, st)
	return emitGuard(decision, reason)
}

// ruleIDs returns the library's rule ids, sorted — for diagnostics/tests.
func (l ruleLibrary) ruleIDs() []string {
	ids := make([]string, 0, len(l.Rules))
	for _, r := range l.Rules {
		ids = append(ids, r.ID)
	}
	sort.Strings(ids)
	return ids
}
