package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/justinstimatze/crystal/internal/transcript"
)

// sweep_procedures.go is the class-B half of discovery: recurring multi-command
// PROCEDURES (the cupel "release dance" / commit ceremony), as opposed to the
// single-command CONSTRAINTS sweep.go finds. The promotion signal is DIFFERENT:
// a constraint is "re-encoded across N projects' docs"; a procedure is "the same
// ordered command SEQUENCE actually executed K times across the real transcripts."
// So this mines SESSIONS (ordered Bash history), not docs — frequent ordered
// N-grams of command signatures. Deterministic, no model. It PROPOSES candidates
// to crystallize into one command (the thing the user did by hand for cupel).

// runProcedures is the --procedures entry point.
func (c *SweepCmd) runProcedures() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return usageError{fmt.Errorf("resolving home dir: %w", err)}
	}
	sessions, _, err := loadBashSessions([]string{home}) // the current user's transcripts

	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return usageError{fmt.Errorf("no sessions with ≥2 Bash commands under %s/.claude/projects", home)}
	}

	// Count every ordered N-gram (length 2..maxLen) of consecutive command
	// signatures, across all sessions. A procedure is a frequent, multi-step,
	// multi-session N-gram.
	const maxLen = 4
	type ngram struct {
		steps    []string
		count    int
		sessions map[int]bool
		example  string
	}
	grams := map[string]*ngram{}
	for si, sess := range sessions {
		sig := sessionSignatures(sess) // ordered, runs of identical sigs collapsed
		for n := 2; n <= maxLen; n++ {
			for i := 0; i+n <= len(sig.sigs); i++ {
				steps := sig.sigs[i : i+n]
				if !hasTwoDistinct(steps) {
					continue // a→a→… is not a procedure
				}
				key := strings.Join(steps, " → ")
				g := grams[key]
				if g == nil {
					g = &ngram{steps: append([]string{}, steps...), sessions: map[int]bool{}, example: strings.Join(sig.examples[i:i+n], "  ;  ")}
					grams[key] = g
				}
				g.count++
				g.sessions[si] = true
			}
		}
	}

	// A candidate procedure: recurs ≥ MinProc times across ≥ MinProjects sessions.
	// With --novel, require an UNCOMMON step (filter out generic git CRUD churn,
	// which is a real sequence but not a distinctive ceremony worth crystallizing).
	type cand struct{ *ngram }
	var ranked []cand
	for _, g := range grams {
		if g.count < c.MinProc || len(g.sessions) < c.MinProjects {
			continue
		}
		if c.Novel && !hasUncommonStep(g.steps) {
			continue
		}
		ranked = append(ranked, cand{g})
	}
	// Rank by occurrences, then by LENGTH (a longer recurring sequence is the more
	// valuable crystallization target), then the key for determinism.
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		if len(ranked[i].steps) != len(ranked[j].steps) {
			return len(ranked[i].steps) > len(ranked[j].steps)
		}
		return strings.Join(ranked[i].steps, " → ") < strings.Join(ranked[j].steps, " → ")
	})

	fmt.Printf("crystal sweep --procedures: %d session(s); %d recurring procedure(s) (≥%d times across ≥%d sessions)\n",
		len(sessions), len(ranked), c.MinProc, c.MinProjects)
	fmt.Printf("(the cupel release-dance pattern: a multi-command sequence executed enough to be worth one command; deterministic, no model)\n\n")

	shown := 0
	for _, g := range ranked {
		if c.Top > 0 && shown >= c.Top {
			fmt.Printf("  … (%d more; raise --top)\n", len(ranked)-shown)
			break
		}
		fmt.Printf("  [%d×, %d sessions] %d-step: %s\n", g.count, len(g.sessions), len(g.steps), strings.Join(g.steps, " → "))
		fmt.Printf("      example: %s\n\n", truncate(g.example, 120))
		shown++
	}
	if shown == 0 {
		fmt.Printf("  (no sequence reached --min-proc=%d across --min-projects=%d sessions; lower the thresholds)\n", c.MinProc, c.MinProjects)
	}
	return nil
}

// loadBashSessions returns ordered Bash command lists, ONE slice per transcript
// session (file) — order and frequency PRESERVED (NOT deduped, unlike
// loadBashCommands), because a procedure is a sequence, not a set.
func loadBashSessions(homes []string) ([][]string, string, error) {
	var files []string
	for _, home := range homes {
		m, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", "*.jsonl"))
		files = append(files, m...)
	}
	sort.Strings(files)
	var sessions [][]string
	for _, f := range files {
		recs, _, err := transcript.WalkFile(f)
		if err != nil {
			continue
		}
		var seq []string
		for _, r := range recs {
			if r.Tool != "Bash" {
				continue
			}
			cmd, _ := r.Args["command"].(string)
			if cmd = strings.TrimSpace(cmd); cmd != "" {
				seq = append(seq, cmd)
			}
		}
		if len(seq) >= 2 {
			sessions = append(sessions, seq)
		}
	}
	return sessions, fmt.Sprintf("%d session(s)", len(files)), nil
}

// sigSeq is a session's command-signature sequence plus a parallel raw-example
// slice (so a found procedure can show what it actually looked like).
type sigSeq struct {
	sigs     []string
	examples []string
}

// sessionSignatures maps a session's raw Bash commands to an ordered signature
// sequence: each record is split on sequential shell operators (&&, ;, ||, \n) so
// a compound one-liner contributes multiple steps, each step is reduced to its
// command signature, empties (non-crystallizable leads) drop out, and runs of the
// same signature collapse to one (so "git add x; git add y" is one step).
func sessionSignatures(raw []string) sigSeq {
	var out sigSeq
	last := ""
	for _, rec := range raw {
		for _, sub := range splitShell(rec) {
			sig := procSignature(sub)
			if sig == "" || sig == last {
				continue
			}
			out.sigs = append(out.sigs, sig)
			out.examples = append(out.examples, strings.TrimSpace(sub))
			last = sig
		}
	}
	return out
}

// splitShell splits a (possibly compound) command on SEQUENTIAL operators only —
// &&, ||, ;, and newlines. Pipes are left intact (a pipeline is one logical step;
// procSignature takes its leading command).
func splitShell(cmd string) []string {
	repl := strings.NewReplacer("&&", "\n", "||", "\n", ";", "\n")
	parts := strings.Split(repl.Replace(cmd), "\n")
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// procSignature reduces one (sub)command to "lead [subcommand]" — the leading
// crystallizable command plus a plain subcommand word if present, ignoring all
// args/flags/refs. Coarser than commandSignature (which is tuned for doc spans):
// transcript commands carry volatile refs/args, so a coarse, robust key clusters
// the same operation across calls. Returns "" if the lead isn't a known command.
func procSignature(cmd string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	lead := strings.ToLower(fields[0])
	if !commandLeads[lead] {
		return ""
	}
	sig := lead
	if len(fields) > 1 && isSubcommandWord(fields[1]) {
		sig += " " + strings.ToLower(fields[1])
	}
	return sig
}

// isSubcommandWord reports whether a token is a plain subcommand name (alnum +
// hyphen), not a flag, path, ref, or assignment.
func isSubcommandWord(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}

// commonSteps are the everyday git/CRUD operations everyone runs — true recurring
// sequences, but not distinctive ceremonies worth a custom command. A procedure is
// "novel" (interesting) iff it contains at least one step OUTSIDE this set.
var commonSteps = map[string]bool{
	"git add": true, "git commit": true, "git push": true, "git status": true,
	"git log": true, "git diff": true, "git pull": true, "git checkout": true,
	"git stash": true, "git fetch": true, "git branch": true, "git restore": true,
}

// hasUncommonStep reports whether a procedure has at least one step that is NOT
// everyday git CRUD — the signal that it's a distinctive, crystallizable ceremony.
func hasUncommonStep(steps []string) bool {
	for _, s := range steps {
		if !commonSteps[s] {
			return true
		}
	}
	return false
}

// hasTwoDistinct reports whether a step list has at least two distinct signatures
// (a real procedure spans more than one operation).
func hasTwoDistinct(steps []string) bool {
	for _, s := range steps[1:] {
		if s != steps[0] {
			return true
		}
	}
	return false
}
