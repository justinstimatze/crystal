package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// HookCmd is the last batch→live gap closed: the crystallized deterministic
// tier served as an ACTUAL Claude Code PreToolUse hook. Everything before this
// (`triage`, `author`, `serve`, `amortize`) ran the stack as a batch over a
// corpus. This is the same rule table (`detClassify`) answering a recurring
// chore — "what category is this Bash command" — in place of a frontier call,
// live, one fresh process per tool use, with windowed demote-on-drift.
//
// Contract (Claude Code PreToolUse hook, verified against the hooks docs):
//
//	stdin:  {"hook_event_name":"PreToolUse","tool_name":"Bash",
//	         "tool_input":{"command":"git status"}, ...}
//	stdout: {"hookSpecificOutput":{"hookEventName":"PreToolUse",
//	         "permissionDecision":"allow","additionalContext":"..."}}
//
// The hook never DENIES — it only ANSWERS. On a covered command it injects the
// deterministic category as `additionalContext` (a system reminder the model
// reads), so the frontier never has to classify it: 0 model calls on the
// covered fraction. On the residual it stays silent (defers to the model). And
// it carries windowed M-in-W drift state ACROSS invocations in a small state
// file — so a sustained burst of commands the rules can't cover (a domain shift,
// e.g. container tooling) collapses coverage in-window and DEMOTES the tier
// live: it stops serving and flags the chore for re-authoring.
//
// The drift signal here differs from `author`'s (deliberately): a live hook has
// no oracle, so it cannot detect WRONG answers (that needs a reference). What it
// CAN detect with no oracle is COVERAGE COLLAPSE — the rule table going silent
// on a sustained run of commands. Same windowed M-in-W rule; the signal is
// "stopped being able to answer," not "answered wrong."
//
// Host-capability note (the weir caveat): this particular rule table is pure Go
// string matching — it shells out to NOTHING, so it is fully portable and the
// host-tool dependency is zero. A rule table that delegated to `rg`/`fd` would
// carry that dependency and the hook would need a capability probe + fallback.
type HookCmd struct {
	State string `help:"Path to the cross-invocation drift-window state file." default:".crystal-hook-state.json"`
	Rules string `help:"Path to a tier-authored rule-table artifact (JSON) to serve from. If absent, serves from the compiled detClassify baseline. This is the file the re-author loop atomically swaps." default:""`
	// Burst gate (fast): demote on a sudden uncovered cluster.
	DriftM int `help:"Demote after M uncovered commands within the sliding window (sudden-burst gate)." default:"3"`
	DriftW int `help:"Sliding window size for the burst gate." default:"5"`
	// Cumulative gate (slow): catches sustained interleaved drift that stays
	// below the burst density forever (the 2-in-5-interleave evasion the panel
	// found). Demote when the trailing long-window uncovered RATE exceeds the
	// threshold — an integral term the local-density burst gate cannot see.
	DriftLongW    int     `help:"Cumulative gate: trailing window over which the sustained uncovered RATE is measured." default:"20"`
	DriftLongMinN int     `help:"Cumulative gate stays inactive until this many commands have been seen (avoid early noise)." default:"12"`
	DriftLongRate float64 `help:"Cumulative gate: demote if the trailing uncovered rate ≥ this. Must sit ABOVE the host's baseline residual rate (1-g) and below the drift rate." default:"0.35"`
}

// driftCfg bundles the serving classifier and both demotion gates so decideHook
// stays a pure function of (state, command, cfg).
type driftCfg struct {
	classify func(string) string
	m, w     int     // burst gate
	longW    int     // cumulative window
	longMinN int     // cumulative gate activation floor
	longRate float64 // cumulative uncovered-rate threshold
}

func (c *HookCmd) cfg(classify func(string) string) driftCfg {
	return driftCfg{
		classify: classify,
		m:        c.DriftM, w: c.DriftW,
		longW: c.DriftLongW, longMinN: c.DriftLongMinN, longRate: c.DriftLongRate,
	}
}

// hookEvent is the subset of the PreToolUse stdin payload we consume.
type hookEvent struct {
	HookEventName string `json:"hook_event_name"`
	ToolName      string `json:"tool_name"`
	ToolInput     struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

// hookOutput is the PreToolUse stdout contract.
type hookOutput struct {
	HookSpecificOutput hookSpecific `json:"hookSpecificOutput"`
}
type hookSpecific struct {
	HookEventName      string `json:"hookEventName"`
	PermissionDecision string `json:"permissionDecision"`
	AdditionalContext  string `json:"additionalContext,omitempty"`
}

// hookState is the windowed drift state persisted between hook invocations.
// Each PreToolUse call is a separate process, so the sliding window only exists
// because it round-trips through this file — that disk round-trip IS the "live"
// part (the demotion accumulates across real process boundaries, not in one
// in-memory loop).
type hookState struct {
	Window          []bool   `json:"window"`           // recent coverage (burst gate): true = a rule covered it
	LongWindow      []bool   `json:"long_window"`      // longer trailing coverage (cumulative gate)
	Demoted         bool     `json:"demoted"`          // tier has been demoted; chore is back on the model
	DemoteReason    string   `json:"demote_reason"`    // "burst" | "sustained" (which gate fired)
	NeedsReauthor   bool     `json:"needs_reauthor"`   // demotion set this; the re-author loop reads it
	RecentUncovered []string `json:"recent_uncovered"` // a sample of the drifted commands, for re-author training
	Served          int      `json:"served"`           // answered deterministically (0 model calls)
	Deferred        int      `json:"deferred"`         // residual + post-demote, handed to the model
	Total           int      `json:"total"`            // Bash commands seen
	DemotedAtTotal  int      `json:"demoted_at_total"` // the command index demotion fired at (-1 = never)
	Reauthors       int      `json:"reauthors"`        // how many times the loop re-authored + re-promoted
}

// recordUncovered keeps a capped, deduped sample of drifted commands so the
// re-author step has real examples of what the rules stopped covering.
func (st *hookState) recordUncovered(cmd string) {
	for _, c := range st.RecentUncovered {
		if c == cmd {
			return
		}
	}
	st.RecentUncovered = append(st.RecentUncovered, cmd)
	const cap = 16
	if len(st.RecentUncovered) > cap {
		st.RecentUncovered = st.RecentUncovered[len(st.RecentUncovered)-cap:]
	}
}

// repromote is the missing inverse of demotion (the panel's terminal-DoS fix):
// after the re-author loop swaps in a rule table that covers the drifted class,
// it clears the demotion and resets the drift windows so the tier serves again.
func repromote(st *hookState) {
	st.Demoted = false
	st.NeedsReauthor = false
	st.DemoteReason = ""
	st.Window = nil
	st.LongWindow = nil
	st.RecentUncovered = nil
	st.DemotedAtTotal = -1
	st.Reauthors++
}

// hookDecision is the pure, testable outcome of one hook invocation.
type hookDecision struct {
	additionalContext string // "" = stay silent (defer to the model)
	served            bool   // answered deterministically this call
	demotedNow        bool   // demotion flipped on this call
	category          string // the served category (when served)
}

// demote flips the tier off, records which gate fired and a re-author flag, and
// returns the demotion decision. The triggering command is itself uncovered (or
// counts as deferred), so it goes to the model.
func demote(st *hookState, reason, detail string) hookDecision {
	st.Demoted = true
	st.DemoteReason = reason
	st.NeedsReauthor = true
	st.DemotedAtTotal = st.Total
	st.Deferred++
	return hookDecision{
		demotedNow: true,
		category:   reason,
		additionalContext: fmt.Sprintf("[crystal] DEMOTED the deterministic Bash-command categorizer (%s gate): %s. "+
			"The chore is handed back to the model tier and flagged for re-authoring (needs_reauthor); the hook will "+
			"no longer inject deterministic categories until the re-author loop swaps in a covering rule table and "+
			"re-promotes the tier.", reason, detail),
	}
}

// decideHook is the pure core: given the current state and one Bash command,
// update the drift windows and decide whether to serve a deterministic answer,
// defer to the model, or demote the tier. It mutates st in place. The serving
// classifier is injected via cfg.classify so a re-authored rule-table artifact
// can replace the compiled baseline live.
func decideHook(st *hookState, command string, cfg driftCfg) hookDecision {
	st.Total++

	// Already demoted: the chore lives on the model tier now. Silent pass-through
	// — but keep OBSERVING the uncovered commands so the pending re-author sees
	// the full drifted class, not just the few examples seen before demotion. (A
	// re-author trained only on the pre-demotion sample under-covers the class and
	// the gate rightly rejects it; collecting post-demotion examples is what lets
	// the loop actually recover.)
	if st.Demoted {
		st.Deferred++
		if cfg.classify(command) == "" {
			st.recordUncovered(command)
		}
		return hookDecision{}
	}

	cat := cfg.classify(command)
	covered := cat != ""
	if !covered {
		st.recordUncovered(command)
	}

	// Slide the burst (short) window.
	st.Window = append(st.Window, covered)
	if len(st.Window) > cfg.w {
		st.Window = st.Window[len(st.Window)-cfg.w:]
	}
	uncoveredInWindow := 0
	for _, c := range st.Window {
		if !c {
			uncoveredInWindow++
		}
	}

	// Slide the cumulative (long) window.
	st.LongWindow = append(st.LongWindow, covered)
	if cfg.longW > 0 && len(st.LongWindow) > cfg.longW {
		st.LongWindow = st.LongWindow[len(st.LongWindow)-cfg.longW:]
	}
	uncoveredLong := 0
	for _, c := range st.LongWindow {
		if !c {
			uncoveredLong++
		}
	}
	longRate := 0.0
	if n := len(st.LongWindow); n > 0 {
		longRate = float64(uncoveredLong) / float64(n)
	}

	// Burst gate (fast): M uncovered within the short window → sudden collapse.
	if uncoveredInWindow >= cfg.m {
		return demote(st, "burst", fmt.Sprintf("%d of the last %d commands were uncovered (sudden coverage collapse)",
			uncoveredInWindow, len(st.Window)))
	}

	// Cumulative gate (slow): a sustained uncovered RATE over the long window,
	// even when no short window ever reaches M. This catches the interleaved
	// drift the burst gate is structurally blind to (panel finding): e.g. a
	// steady covered,drift,covered,drift… at 40% uncovered never hits 3-in-5 but
	// trips a 0.35-over-12 rate gate.
	if cfg.longRate > 0 && len(st.LongWindow) >= cfg.longMinN && longRate >= cfg.longRate {
		return demote(st, "sustained", fmt.Sprintf("uncovered rate %.0f%% over the last %d commands (sustained drift "+
			"below the burst threshold)", longRate*100, len(st.LongWindow)))
	}

	if covered {
		st.Served++
		return hookDecision{
			served:   true,
			category: cat,
			additionalContext: fmt.Sprintf("[crystal] Bash command category (deterministic, 0 model calls): %s. "+
				"Answered by the crystallized rule table — no frontier classification needed.", cat),
		}
	}

	// Residual: uncovered but the window has not collapsed. Normal — defer to
	// the model silently (this is the fraction the cheap tier was never meant to
	// own), no additionalContext.
	st.Deferred++
	return hookDecision{}
}

func (c *HookCmd) Run() error {
	var ev hookEvent
	if err := json.NewDecoder(os.Stdin).Decode(&ev); err != nil {
		// A hook that can't parse its input must not block the tool. Emit a
		// pass-through allow and let the command proceed (fail-open for the
		// USER's command; the chore just isn't answered this turn).
		return emitAllow("")
	}

	// Not our chore (non-Bash, or a malformed event): pass through, answer nothing.
	if ev.ToolName != "Bash" || strings.TrimSpace(ev.ToolInput.Command) == "" {
		return emitAllow("")
	}

	st, err := loadHookState(c.State)
	if err != nil {
		return usageError{fmt.Errorf("loading hook state %q: %w", c.State, err)}
	}
	classify, err := servingClassifier(c.Rules)
	if err != nil {
		// A bad artifact must not block the user's command: fall back to the
		// compiled baseline and serve fail-open.
		classify = detClassify
	}
	dec := decideHook(st, ev.ToolInput.Command, c.cfg(classify))
	if err := saveHookState(c.State, st); err != nil {
		return usageError{fmt.Errorf("saving hook state %q: %w", c.State, err)}
	}
	return emitAllow(dec.additionalContext)
}

// servingClassifier returns the classifier the hook serves from: a tier-authored
// rule-table artifact if rulesPath points at one, else the compiled detClassify
// baseline. This is the indirection that lets the re-author loop change live
// serving behavior by swapping the artifact file.
func servingClassifier(rulesPath string) (func(string) string, error) {
	if rulesPath == "" {
		return detClassify, nil
	}
	b, err := os.ReadFile(rulesPath)
	if os.IsNotExist(err) {
		return detClassify, nil // not authored yet → baseline
	}
	if err != nil {
		return detClassify, err
	}
	var t ruleTable
	if err := json.Unmarshal(b, &t); err != nil {
		return detClassify, fmt.Errorf("corrupt rule artifact %q: %w", rulesPath, err)
	}
	if len(t.Rules) == 0 {
		return detClassify, fmt.Errorf("rule artifact %q has zero rules", rulesPath)
	}
	return t.classify, nil
}

// emitAllow writes a PreToolUse "allow" decision with optional injected context.
func emitAllow(additionalContext string) error {
	out := hookOutput{HookSpecificOutput: hookSpecific{
		HookEventName:      "PreToolUse",
		PermissionDecision: "allow",
		AdditionalContext:  additionalContext,
	}}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(b)
	return err
}

func loadHookState(path string) (*hookState, error) {
	st := &hookState{DemotedAtTotal: -1}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return st, nil // first invocation
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, st); err != nil {
		return nil, fmt.Errorf("corrupt state: %w", err)
	}
	return st, nil
}

func saveHookState(path string, st *hookState) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// writeRuleArtifact writes a rule table to disk atomically (temp + rename) so the
// live hook never reads a half-written artifact mid-swap.
func writeRuleArtifact(path string, t ruleTable) error {
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// HookDemoCmd proves the hook is LIVE, not a batch: it drives the real built
// binary (`crystal hook`) across a stream of PreToolUse events — each a separate
// process invocation piping JSON over stdin and reading the hook's JSON from
// stdout — with the drift window persisting only through the on-disk state file.
// It streams real covered/residual commands (the normal regime, served
// deterministically) and then injects the same container-tooling drift class
// `author` uses, and shows the tier DEMOTE live, across process boundaries.
type HookDemoCmd struct {
	Corpus  string   `help:"Corpus dir of real records (for the normal-regime command stream)." default:"testdata/corpus"`
	Home    []string `help:"Instead of the corpus, scan these home dirs' live transcripts. Repeatable."`
	Normal  int      `help:"How many real commands to stream before the injected drift." default:"16"`
	DriftM  int      `help:"Demote after M uncovered commands within the window." default:"3"`
	DriftW  int      `help:"Sliding window size for the drift trigger." default:"5"`
	Verbose bool     `help:"Print the full hook JSON response for every command."`
}

func (c *HookDemoCmd) Run() error {
	cmds, src, err := loadBashCommands(c.Corpus, c.Home)
	if err != nil {
		return usageError{err}
	}
	if len(cmds) == 0 {
		return usageError{fmt.Errorf("no Bash commands found in %s", src)}
	}

	// Normal regime: a real slice of the corpus. Injected drift: the container
	// class the rules were never trained on (same as author's drift class).
	normal := subsampleStr(cmds, c.Normal)
	stream := append(append([]string{}, normal...), driftCommands...)
	driftStart := len(normal)

	// A throwaway state file in the OS temp dir — the cross-process drift window.
	stateFile, err := os.CreateTemp("", "crystal-hookdemo-*.json")
	if err != nil {
		return usageError{fmt.Errorf("creating temp state: %w", err)}
	}
	statePath := stateFile.Name()
	stateFile.Close()
	os.Remove(statePath) // start fresh; the hook recreates it on first write
	defer os.Remove(statePath)

	self := os.Args[0] // the real crystal binary currently executing

	fmt.Printf("hookdemo: driving the REAL `crystal hook` binary (%s) over %d live PreToolUse events\n", self, len(stream))
	fmt.Printf("each line below is a SEPARATE process invocation; the %d-in-%d drift window survives only via %s\n\n",
		c.DriftM, c.DriftW, statePath)
	fmt.Printf("=== normal regime: %d real commands (%s) ===\n", len(normal), src)

	served, deferred := 0, 0
	demotedAt := -1
	for i, cmd := range stream {
		if i == driftStart {
			fmt.Printf("\n=== injected drift: %d container commands the rules never saw ===\n", len(driftCommands))
		}
		ctxText, decision, err := invokeHook(self, statePath, "", cmd, c.DriftM, c.DriftW)
		if err != nil {
			return usageError{fmt.Errorf("invoking hook on %q: %w", cmd, err)}
		}
		label := "defer→model "
		switch {
		case strings.Contains(ctxText, "DEMOTED"):
			label = "DEMOTE!     "
			if demotedAt < 0 {
				demotedAt = i
			}
		case strings.Contains(ctxText, "category"):
			label = "serve-det   "
			served++
		default:
			deferred++
		}
		// after demotion every command is a silent defer
		if demotedAt >= 0 && i > demotedAt {
			deferred++
		}
		fmt.Printf("  [%2d] %-12s %s\n", i, label, truncate(cmd, 48))
		if c.Verbose {
			fmt.Printf("        → %s\n", strings.TrimSpace(decision))
		}
	}

	fmt.Printf("\n=== outcome ===\n")
	if demotedAt >= 0 {
		fmt.Printf("  DEMOTED live at stream index %d (the %d-in-%d window collapsed on the container burst).\n", demotedAt, c.DriftM, c.DriftW)
		fmt.Printf("  served deterministically (0 model calls): %d  ·  deferred to model: %d\n", served, deferred)
		fmt.Println("  After demotion this demo injects nothing — it shows DETECTION only. The closed loop")
		fmt.Println("  (demote → auto re-author → gate → swap artifact → re-promote → RESUME serving) is")
		fmt.Println("  `crystal hook-loop`, which wires `author` to the live hook across process boundaries.")
	} else {
		fmt.Printf("  served %d, deferred %d, NO demotion — the drift window never collapsed.\n", served, deferred)
		fmt.Println("  (If the injected burst didn't demote, the window W is too wide or M too high for the burst length.)")
	}
	fmt.Println("\nThis is the real PreToolUse contract over real process boundaries — demotion accumulates live,")
	fmt.Println("not just in a benchmark. The BINARY is fully portable (shells out to nothing); the COVERAGE is")
	fmt.Println("host-specific (g→0 on a foreign command stack). Each live invocation is a fresh process (~ms")
	fmt.Println("startup), so the deployed speedup over a model call is ~50–110×, not the in-process µs figure.")
	return nil
}

// invokeHook runs the real `crystal hook` binary once with cmd as a PreToolUse
// Bash event on stdin, returning the injected additionalContext (if any) and the
// raw stdout JSON. rulesPath (may be "") is passed through as --rules so the
// orchestrator can drive the hook against a swappable artifact.
func invokeHook(self, statePath, rulesPath, cmd string, m, w int) (additionalContext, rawJSON string, err error) {
	ev := hookEvent{HookEventName: "PreToolUse", ToolName: "Bash"}
	ev.ToolInput.Command = cmd
	payload, err := json.Marshal(ev)
	if err != nil {
		return "", "", err
	}
	args := []string{"hook", "--state", statePath, "--drift-m", fmt.Sprint(m), "--drift-w", fmt.Sprint(w)}
	if rulesPath != "" {
		args = append(args, "--rules", rulesPath)
	}
	c := exec.Command(self, args...)
	c.Stdin = bytes.NewReader(payload)
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return "", out.String(), err
	}
	var resp hookOutput
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		return "", out.String(), fmt.Errorf("decoding hook output %q: %w", out.String(), err)
	}
	return resp.HookSpecificOutput.AdditionalContext, out.String(), nil
}
