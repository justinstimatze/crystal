package library

import "fmt"

// mint.go closes the watch→author half of the serve tier: a discovered
// recurrence is authored into a candidate Entry, then GATED before it can join
// the library. The gate is the load-bearing part — the producer-verifier
// discipline at the serve layer. A library entry's fallible, model-authored
// decision is its TRIGGERING (the match/avoid tokens that decide serve-vs-abstain),
// so that is what the gate checks behaviorally: the entry must SERVE on the
// contexts it claims to cover (its evidence + labeled positives), ABSTAIN with the
// too-much guard on the contexts that should trip Avoid, and NOT serve on a
// standing set of benign, unrelated contexts (the false-serve guard).
//
// The artifact body itself (the recipe prose) is the fuzzy residual a downstream
// behavioral verifier or human review must check — but the serve-time discipline
// (abstain-over-wrong) already bounds its blast radius, so a wrong recipe that
// only ever fires on its narrow trigger is contained, not loose.

// benignContexts are unrelated, always-allowed contexts a minted entry must NOT
// serve on — the false-serve guard. The classic failure is a match list so broad
// it fires on routine requests, turning a precise artifact into noise.
var benignContexts = []string{
	"list the files in this directory",
	"what does this function do",
	"run the unit tests and show the output",
	"summarize the readme for me",
	"explain how the parser works",
	"open the config and read the timeout value",
}

// GateResult is the outcome of gating a candidate entry (mirrors the dispatch
// regex gate: a human-readable report plus a single pass/fail).
type GateResult struct {
	Report []string
	Passed bool
}

// GateEntry verifies a candidate Entry WITHOUT adding it to a live library:
//  1. coverage — it SERVES on every evidence line and labeled positive (at its
//     own MinConf bar). A match list too narrow to fire on its own evidence is
//     useless and rejected.
//  2. too-much guard — on every labeled too-much context it ABSTAINS with
//     outcome abstain-too-much (the Avoid tokens actually trip). An entry that
//     serves where it claimed it should hold back is rejected.
//  3. false-serve guard — it does NOT serve on any benign, unrelated context
//     (its own controls plus the standing benignContexts set). A match list broad
//     enough to fire on routine requests is rejected.
//
// Each check runs on a FRESH single-entry library so cooldown/demotion state
// never leaks between checks.
func GateEntry(e Entry, positives, tooMuch, control []string) GateResult {
	var g GateResult
	pass := true

	if len(e.Match) == 0 {
		g.Report = append(g.Report, "✗ no match tokens: an entry with no trigger can never serve")
		g.Passed = false
		return g
	}

	// 1. coverage — serves on every positive.
	var missed []string
	for _, ctx := range positives {
		if !New([]Entry{e}).Serve(ctx, 0).Served() {
			missed = append(missed, ctx)
		}
	}
	if len(missed) > 0 {
		g.Report = append(g.Report, "✗ coverage: entry does NOT serve on contexts it should cover: "+joinTrunc(missed))
		pass = false
	} else if len(positives) > 0 {
		g.Report = append(g.Report, fmt.Sprintf("✓ coverage: serves on all %d evidence/positive contexts (bar %.2f)", len(positives), e.MinConf))
	}

	// 2. too-much guard — abstains with the avoid guard on each too-much context.
	if len(e.Avoid) == 0 && len(tooMuch) > 0 {
		g.Report = append(g.Report, "✗ too-much guard: too-much contexts were given but the entry has no Avoid tokens")
		pass = false
	}
	var notGuarded []string
	for _, ctx := range tooMuch {
		if New([]Entry{e}).Serve(ctx, 0).Outcome != "abstain-too-much" {
			notGuarded = append(notGuarded, ctx)
		}
	}
	if len(notGuarded) > 0 {
		g.Report = append(g.Report, "✗ too-much guard: Avoid tokens fail to trip on: "+joinTrunc(notGuarded))
		pass = false
	} else if len(tooMuch) > 0 {
		g.Report = append(g.Report, fmt.Sprintf("✓ too-much guard: abstains (avoid) on all %d too-much contexts", len(tooMuch)))
	}

	// 3. false-serve guard — never serves on a benign/unrelated context.
	checked := append(append([]string{}, control...), benignContexts...)
	var falseServes []string
	for _, ctx := range checked {
		if New([]Entry{e}).Serve(ctx, 0).Served() {
			falseServes = append(falseServes, ctx)
		}
	}
	if len(falseServes) > 0 {
		g.Report = append(g.Report, "✗ false-serve guard: entry serves on BENIGN/unrelated contexts: "+joinTrunc(falseServes))
		pass = false
	} else {
		g.Report = append(g.Report, fmt.Sprintf("✓ false-serve guard: serves on none of %d benign contexts", len(checked)))
	}

	g.Passed = pass
	return g
}

func joinTrunc(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += " | "
		}
		if len(s) > 60 {
			s = s[:57] + "…"
		}
		out += s
	}
	return out
}
