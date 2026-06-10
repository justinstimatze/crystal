package transcript

import (
	"strings"
	"testing"
)

// A synthetic transcript modeled on the real CC shape: a persona prompt that
// merely MENTIONS a command, plus two real Bash tool_use/result pairs.
//
// Regression guard for the measure miscount: the persona phrase
// "gt deacon heartbeat" must NEVER become a Bash command signature, and each
// result must pair to its own tool_use with the correct distinct command.
const synthetic = `
{"type":"user","sessionId":"s1","cwd":"/r","gitBranch":"main","timestamp":"2026-05-28T00:00:00Z","message":{"role":"user","content":"I am Deacon. Start patrol: run gt deacon heartbeat, then check gt hook."}}
{"type":"assistant","sessionId":"s1","cwd":"/r","gitBranch":"main","timestamp":"2026-05-28T00:00:01Z","message":{"role":"assistant","content":[{"type":"text","text":"running first"},{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"echo alpha"}}]}}
{"type":"user","sessionId":"s1","cwd":"/r","gitBranch":"main","timestamp":"2026-05-28T00:00:02Z","toolUseResult":{"stdout":"alpha","stderr":"","interrupted":false},"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"alpha"}]}}
{"type":"assistant","sessionId":"s1","cwd":"/r","gitBranch":"main","timestamp":"2026-05-28T00:00:03Z","message":{"role":"assistant","content":[{"type":"text","text":"now second"},{"type":"tool_use","id":"t2","name":"Bash","input":{"command":"echo beta"}}]}}
{"type":"user","sessionId":"s1","cwd":"/r","gitBranch":"main","timestamp":"2026-05-28T00:00:04Z","toolUseResult":{"stdout":"beta","stderr":"","interrupted":false},"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":"beta"}]}}
`

func TestWalkPairsCorrectlyAndDoesNotLeakPromptText(t *testing.T) {
	recs, dropped, err := Walk(strings.NewReader(strings.TrimSpace(synthetic)))
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if dropped != 0 {
		t.Errorf("dropped=%d, want 0", dropped)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	gotCmds := map[string]string{} // tool_use_id -> command
	for _, r := range recs {
		if r.Tool != "Bash" {
			t.Errorf("record %s: tool=%q want Bash", r.ToolUseID, r.Tool)
		}
		cmd, _ := r.Args["command"].(string)
		if strings.Contains(cmd, "gt deacon heartbeat") {
			t.Errorf("LEAK: persona-prompt text became a Bash command: %q", cmd)
		}
		gotCmds[r.ToolUseID] = cmd
	}
	if gotCmds["t1"] != "echo alpha" {
		t.Errorf("t1 command = %q, want 'echo alpha'", gotCmds["t1"])
	}
	if gotCmds["t2"] != "echo beta" {
		t.Errorf("t2 command = %q, want 'echo beta'", gotCmds["t2"])
	}
	// Outputs must pair to their own tool_use, not share one result.
	for _, r := range recs {
		switch r.ToolUseID {
		case "t1":
			if r.Result.Stdout != "alpha" {
				t.Errorf("t1 stdout = %q, want alpha (result mispairing?)", r.Result.Stdout)
			}
		case "t2":
			if r.Result.Stdout != "beta" {
				t.Errorf("t2 stdout = %q, want beta (result mispairing?)", r.Result.Stdout)
			}
		}
	}
}
