package cmd

import "testing"

// TestGateHasTeeth is the load-bearing test: the no-run gate must REJECT bad
// drafts, not rubber-stamp them. A gate that always passes is worthless — this is
// the producer-verifier guard for generated procedure scripts.
func TestGateHasTeeth(t *testing.T) {
	proc := []string{"go run", "python3"}

	// 1. A faithful script using only the observed commands → PASS.
	good := "#!/bin/sh\nset -eu\ngo run ./cmd/x doctor 2>&1 | head\npython3 /tmp/probe.py 2>&1 | head\n"
	if g := gateAuthoredScript(good, proc); !g.passed {
		t.Errorf("faithful script rejected: %v", g.report)
	}

	// 2. Hallucinated dangerous command (rm) the ceremony never used → REJECT.
	rogue := "#!/bin/sh\ngo run ./cmd/x\nrm -rf /tmp/scratch\npython3 /tmp/probe.py\n"
	if g := gateAuthoredScript(rogue, proc); g.passed {
		t.Error("script with a hallucinated `rm` should be REJECTED (hallucination guard)")
	}

	// 3. Wrong order / a ceremony step missing → REJECT (fidelity).
	reordered := "#!/bin/sh\npython3 /tmp/probe.py\ngo run ./cmd/x\n" // python3 before go run
	if g := gateAuthoredScript(reordered, proc); g.passed {
		t.Error("out-of-order script should be REJECTED (fidelity)")
	}

	// 4. Syntactically broken → REJECT (bash -n).
	broken := "#!/bin/sh\nif then\ngo run ./x\npython3 y.py\n"
	if g := gateAuthoredScript(broken, proc); g.passed {
		t.Error("syntactically broken script should be REJECTED (bash -n)")
	}
}

func TestScriptSignaturesAndSubsequence(t *testing.T) {
	script := "#!/bin/sh\n# a comment\nset -eu\nVAR=x go run ./cmd/x && echo done\npython3 y.py\n"
	got := scriptSignatures(script)
	// set/echo are not crystallizable leads; VAR= prefix stripped; expect go run, python3.
	if len(got) != 2 || got[0] != "go run" || got[1] != "python3" {
		t.Errorf("scriptSignatures = %v, want [go run python3]", got)
	}
	if !isOrderedSubsequence([]string{"go run", "python3"}, got) {
		t.Error("ceremony should be an in-order subsequence")
	}
	if isOrderedSubsequence([]string{"python3", "go run"}, got) {
		t.Error("reversed ceremony must NOT be a subsequence")
	}
}

func TestStripFences(t *testing.T) {
	withFence := "```bash\necho hi\n```"
	if got := stripFences(withFence); got != "echo hi" {
		t.Errorf("stripFences kept the fence: %q", got)
	}
	plain := "echo hi"
	if got := stripFences(plain); got != plain {
		t.Errorf("stripFences mangled un-fenced text: %q", got)
	}
}
