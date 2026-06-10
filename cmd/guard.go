package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// GuardCmd is crystal's first CONSTRAINT-type crystallization — the reflexive
// rung (ROADMAP rung 6, SWEEP_FINDINGS.md). The sweep found one standing rule
// independently re-encoded in FOUR projects' memories (a private project, calque,
// lucida, plancheck): "never `git add -A` / `git add .` — stage explicit paths." A rule
// re-written four times is recall failing to chunk; the fix is to mint the
// artifact once and retire the four memory encodings. weir already proves the
// shape (a `which`→`command-v` correction promoted to a blocking PreToolUse
// hook); this is a new rule of weir's shape, produced by crystal.
//
// It is NOT just a deny. A classifier-type crystallization (the `hook` tier)
// self-monitors via coverage-collapse; a CONSTRAINT produces no answers to
// verify, so its drift signal is different: OVERRIDE FREQUENCY. If the rule is
// bypassed often (CRYSTAL_GUARD_SKIP=1), that is the live evidence the rule went
// wrong — too aggressive, or the context changed — and it flags itself for
// revision. That override gate is the constraint analog of the hook's demote;
// it makes this a self-monitoring sub-hybrid-loop, not a dead rule. The answer
// to "how does crystal track what it chunked": each artifact tracks itself; the
// installed hooks ARE the registry.
//
// Contract (Claude Code PreToolUse hook):
//
//	stdin:  {"hook_event_name":"PreToolUse","tool_name":"Bash",
//	         "tool_input":{"command":"git add -A"}}
//	stdout: {"hookSpecificOutput":{"hookEventName":"PreToolUse",
//	         "permissionDecision":"deny","permissionDecisionReason":"..."}}
//
// Fail-open: anything that isn't an unambiguous `git add -A|.|--all` is allowed
// (a guard that false-denies its host is worse than one that misses a variant —
// and over-denial would show up as the user disabling it, the loudest drift).
type GuardCmd struct {
	State string `help:"Path to the self-monitoring state file (fire/override counts — the constraint's drift signal)." default:".crystal-guard-state.json"`
	// Override gate (the constraint analog of the hook's M-in-W demote): once the
	// rule has triggered MinN times, if the bypass RATE is ≥ Rate, the rule flags
	// itself NeedsRevision — "you keep overriding me, I'm probably wrong."
	MinN int     `help:"Override gate stays inactive until the rule has triggered this many times." default:"5"`
	Rate float64 `help:"Override gate: flag NeedsRevision when bypasses/triggers ≥ this." default:"0.5"`
}

// guardOutput is the PreToolUse deny/allow contract.
type guardOutput struct {
	HookSpecificOutput guardSpecific `json:"hookSpecificOutput"`
}
type guardSpecific struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
}

// guardState is the constraint's own substrate: how often it fired vs was
// overridden. This is the "sub-hybrid-loop" QA — the artifact monitoring itself.
type guardState struct {
	Triggered     int      `json:"triggered"`      // matches (denies + bypasses)
	Denied        int      `json:"denied"`         // blocked the command
	Bypassed      int      `json:"bypassed"`       // overridden via CRYSTAL_GUARD_SKIP (the drift signal)
	NeedsRevision bool     `json:"needs_revision"` // override rate crossed the gate → rule may be wrong
	Examples      []string `json:"examples"`       // capped sample of matched commands
}

// overrideTripped is the constraint's drift gate: once the rule has triggered
// enough times, a high bypass rate means the rule is probably wrong.
func (st *guardState) overrideTripped(minN int, rate float64) bool {
	return st.Triggered >= minN && rate > 0 && float64(st.Bypassed)/float64(st.Triggered) >= rate
}

func (st *guardState) record(cmd string) {
	for _, e := range st.Examples {
		if e == cmd {
			return
		}
	}
	st.Examples = append(st.Examples, cmd)
	if len(st.Examples) > 16 {
		st.Examples = st.Examples[len(st.Examples)-16:]
	}
}

// matchGitAddAll reports whether the command stages everything via `git add -A`,
// `git add --all`, or `git add .` — in any `&&`/`;` segment. It tokenizes per
// segment (reusing splitSegments) so it does NOT fire on incidental dots/flags
// elsewhere (e.g. `git commit -m "add ."` or `git add path/to/file`). Returns
// the offending form for the deny reason.
func matchGitAddAll(command string) (bool, string) {
	for _, seg := range splitSegments(command) {
		f := strings.Fields(strings.TrimSpace(seg))
		if len(f) < 3 || f[0] != "git" || f[1] != "add" {
			continue
		}
		for _, a := range f[2:] {
			switch a {
			case "-A", "--all", ".":
				return true, a
			}
		}
	}
	return false, ""
}

func (c *GuardCmd) Run() error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return usageError{fmt.Errorf("reading hook event: %w", err)}
	}
	var ev hookEvent
	_ = json.Unmarshal(raw, &ev) // fail-open: a malformed event allows

	allow := func() error {
		return emitGuard("allow", "")
	}
	if ev.ToolName != "Bash" || ev.ToolInput.Command == "" {
		return allow()
	}
	matched, form := matchGitAddAll(ev.ToolInput.Command)
	if !matched {
		return allow()
	}

	st := loadGuardState(c.State)
	st.Triggered++
	st.record(ev.ToolInput.Command)
	bypass := os.Getenv("CRYSTAL_GUARD_SKIP") == "1"

	var decision, reason string
	if bypass {
		st.Bypassed++
		decision, reason = "allow", ""
	} else {
		st.Denied++
		decision = "deny"
		reason = fmt.Sprintf("crystal guard: `git add %s` stages everything indiscriminately — "+
			"stage explicit paths instead (`git add path/to/file ...`). This is a standing rule "+
			"crystal found re-encoded in 4 projects. Override for one call with CRYSTAL_GUARD_SKIP=1.", form)
	}

	// Override gate — the constraint's drift signal (analog of the hook's demote).
	if st.overrideTripped(c.MinN, c.Rate) {
		st.NeedsRevision = true
	}
	saveGuardState(c.State, st)
	return emitGuard(decision, reason)
}

func emitGuard(decision, reason string) error {
	out := guardOutput{HookSpecificOutput: guardSpecific{
		HookEventName:            "PreToolUse",
		PermissionDecision:       decision,
		PermissionDecisionReason: reason,
	}}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
	return nil
}

func loadGuardState(path string) *guardState {
	st := &guardState{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, st)
	}
	return st
}

func saveGuardState(path string, st *guardState) {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}
