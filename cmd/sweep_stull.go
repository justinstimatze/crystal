package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/justinstimatze/stull/check"
	"github.com/justinstimatze/stull/compile"
	"github.com/justinstimatze/stull/spec"
)

// sweep_stull.go is the crystal→stull seam (MVP): turn a discovered CONSTRAINT
// into a provably-sound stull machine. Instead of crystal hand-coding a guard
// hook, it EMITS a spec.Machine and runs stull's static checker — upgrading the
// no-run structural gate (sweep --author) to a formal SOUNDNESS PROOF for the
// control-flow class of crystallization. Only constraints map; procedures (shell
// ceremonies) are not statecharts and stay crystal-native.

// constraintMachine builds a PreToolUse deny-machine for one command signature:
// when a Bash command matches the signature, Block it (exit 2). It is the
// simplest oracle-free machine — a deterministic guard, no Cell — so it is
// trivially free of the E-ORACLE hazard stull exists to prevent.
func constraintMachine(name, signature, reason string) spec.Machine {
	re := signatureRegex(signature)
	matches := func(c *spec.Context) bool {
		cmd, _ := c.Event["command"].(string) // PreToolUse Bash command (runtime-supplied)
		if cmd == "" {
			if ti, ok := c.Event["tool_input"].(map[string]any); ok {
				cmd, _ = ti["command"].(string)
			}
		}
		return re.MatchString(cmd)
	}
	watch := spec.State{
		Name: "watch",
		On: []spec.Transition{
			{ // the bad command appeared → block it, then halt
				On: spec.PreToolUse, To: "blocked",
				Guard: &spec.Guard{Reads: nil, When: matches},
				Do:    []spec.Effect{spec.Block{Reason: spec.S(reason)}},
			},
		},
	}
	blocked := spec.State{Name: "blocked", Terminal: true}
	return spec.Machine{
		Name:     name,
		Fuel:     2,
		Contract: "crystal crystallized this constraint from rules you re-encoded across projects. The block is your own standing rule, enforced — not an external command.",
		Initial:  "watch",
		States:   []spec.State{watch, blocked},
		Cells:    nil, // no oracle — a pure deterministic guard
	}
}

// signatureRegex turns a crystal command signature into a matcher. The flagship
// "git add <all>" folds -A/--all/bare-. ; other signatures match the command +
// subcommand prefix.
func signatureRegex(sig string) *regexp.Regexp {
	if sig == "git add <all>" {
		return regexp.MustCompile(`\bgit\s+add\s+(-A\b|--all\b|\.\s*$|\.\s)`)
	}
	parts := strings.Fields(sig)
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = regexp.QuoteMeta(p)
	}
	return regexp.MustCompile(`\b` + strings.Join(quoted, `\s+`) + `\b`)
}

// emitConstraintStull is the --emit-stull path: take the flagship constraint and
// emit + statically PROVE a stull machine, then compile it to a settings.json
// hook fragment. Reports the checker verdict honestly (the producer-verifier
// upgrade: a formal proof, not crystal's structural heuristic).
func (c *SweepCmd) emitConstraintStull(signature string) error {
	name := "deny-" + strings.ReplaceAll(strings.NewReplacer("<", "", ">", "", " ", "-").Replace(signature), "--", "-")
	reason := fmt.Sprintf("Blocked by a crystallized constraint: %q. Stage explicit paths / use the prescribed form instead.", signature)
	m := constraintMachine(name, signature, reason)

	fmt.Printf("crystal sweep --emit-stull: constraint %q → stull machine %q\n", signature, m.Name)
	fmt.Printf("  states: %d (initial %q), fuel %d, cells %d (oracle-free deterministic guard)\n\n",
		len(m.States), m.Initial, m.Fuel, len(m.Cells))

	fmt.Printf("=== stull static check (formal soundness proof) ===\n")
	if errs := check.Check(m); len(errs) > 0 {
		for _, e := range errs {
			fmt.Printf("  ✗ %s\n", e)
		}
		fmt.Printf("\n  → NOT SOUND: stull's checker rejected the machine; not emitting hooks. (This is the gate working.)\n")
		return nil
	}
	fmt.Printf("  ✓ sound: reachable halt, no orphans, fuel-bounded, no oracle on the control path\n\n")

	frag, err := compile.SettingsJSON(m, "crystal-stull")
	if err != nil {
		return fmt.Errorf("compiling settings fragment: %w", err)
	}
	fmt.Printf("=== compiled settings.json hook fragment ===\n%s\n", frag)
	fmt.Printf("\n  → PROVEN + COMPILED (proposal only): a formally-sound PreToolUse hook for the constraint.\n")
	return nil
}
