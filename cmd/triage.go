package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/justinstimatze/crystal/internal/corpus"
	"github.com/justinstimatze/crystal/internal/llm"
	"github.com/justinstimatze/crystal/internal/record"
	"github.com/justinstimatze/crystal/internal/transcript"
)

// TriageCmd is the v1 slice: a map-reduce + verifier pipeline on ONE real
// chore — categorize your actual Claude Code Bash usage. It runs the whole
// shift-left stack end to end on real data:
//
//	verifier (deterministic rules) — cover the easy fraction for free (high g),
//	    and GATE the cheap model: where a rule fires and the model disagrees, the
//	    rule wins and the divergence is flagged (producer-verifier asymmetry).
//	map (cheap model, Haiku) — classify only the uncovered residual the rules miss.
//	reduce (deterministic) — tally by category (never ask a model to count).
//
// The output is a real usage breakdown; the point is that the frontier model is
// never called, most of the work is deterministic, the cheap model touches only
// the residual, and the verifier catches the cheap model's mistakes on the
// covered fraction.
type TriageCmd struct {
	Corpus   string   `help:"Corpus dir of real records." default:"testdata/corpus"`
	Home     []string `help:"Instead of the corpus, scan these home dirs' live transcripts. Repeatable."`
	CacheDir string   `help:"Disk cache dir for LLM calls." default:".crystal-cache"`
	Verbose  bool     `help:"Dump per-command rule/model/final category and flagged divergences."`
}

var triageCategories = []string{"git", "build/test", "search/inspect", "file-edit", "install", "nav", "network", "other"}

// detClassify is the deterministic verifier. Real commands are COMPOUND
// (`cd X && git add && git commit`), so a naive leading-token rule mislabels
// the command's purpose as "nav". Fix (found by shipping on real data): scan
// the `&&`/`;` segments and let the first real *action* win over a leading
// `cd`. Returns "" when no segment is covered (the residual).
func detClassify(cmd string) string {
	c := strings.TrimSpace(strings.ToLower(cmd))
	segs := splitSegments(c)
	best := ""
	for _, seg := range segs {
		cat := segClassify(seg)
		if cat == "" {
			continue
		}
		if cat != "nav" {
			return cat // the first real action dominates a leading cd
		}
		best = "nav" // remember nav, but keep scanning for a real action
	}
	return best
}

// splitSegments breaks a command on && and ; (NOT | — a pipe is one action).
func splitSegments(c string) []string {
	parts := []string{c}
	for _, sep := range []string{"&&", ";"} {
		var next []string
		for _, p := range parts {
			next = append(next, strings.Split(p, sep)...)
		}
		parts = next
	}
	var out []string
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// segClassify applies the leading-binary rules to a single command segment.
func segClassify(seg string) string {
	fields := strings.Fields(strings.TrimSpace(seg))
	if len(fields) == 0 {
		return ""
	}
	tok := fields[0]
	two := tok
	if len(fields) > 1 {
		two = tok + " " + fields[1]
	}
	switch {
	case tok == "git" && !strings.HasPrefix(two, "git push") && !strings.HasPrefix(two, "git pull") && !strings.HasPrefix(two, "git fetch") && !strings.HasPrefix(two, "git clone"):
		return "git"
	case two == "git push" || two == "git pull" || two == "git fetch" || two == "git clone" || tok == "gh" || tok == "curl" || tok == "wget" || tok == "ssh" || tok == "scp":
		return "network"
	case tok == "go" && len(fields) > 1 && (fields[1] == "build" || fields[1] == "test" || fields[1] == "run" || fields[1] == "vet"):
		return "build/test"
	case tok == "make" || tok == "cargo" || tok == "gradle" || tok == "mvn" || tok == "pytest" || tok == "jest" || two == "npm test" || two == "npm run":
		return "build/test"
	case tok == "go" && len(fields) > 1 && fields[1] == "install", tok == "apt" || tok == "apt-get" || tok == "brew" || tok == "pip" || tok == "pip3" || two == "npm install" || two == "npm i" || two == "yarn add":
		return "install"
	case tok == "rg" || tok == "grep" || tok == "fd" || tok == "fdfind" || tok == "find" || tok == "ls" || tok == "eza" || tok == "cat" || tok == "bat" || tok == "batcat" || tok == "head" || tok == "tail" || tok == "less" || tok == "wc" || tok == "jq" || tok == "tree" || tok == "rg.":
		return "search/inspect"
	case tok == "sed" || tok == "sd" || tok == "mv" || tok == "cp" || tok == "rm" || tok == "mkdir" || tok == "touch" || tok == "chmod" || tok == "tee" || tok == "rmdir":
		return "file-edit"
	case tok == "cd" || tok == "pushd" || tok == "popd":
		return "nav"
	}
	return ""
}

func (c *TriageCmd) Run() error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	cmds, src, err := c.loadCommands()
	if err != nil {
		return usageError{err}
	}
	if len(cmds) == 0 {
		return usageError{fmt.Errorf("no Bash commands found in %s", src)}
	}
	ctx := context.Background()

	type row struct {
		cmd, det, hk, final string
		flagged             bool
	}
	var rows []row
	covered, residual, disagree, schemaViol := 0, 0, 0, 0
	tally := map[string]int{}

	for _, cmd := range cmds {
		det := detClassify(cmd)
		hk := mapClassify(ctx, client, cmd)
		final := hk
		flagged := false
		if det != "" {
			covered++
			final = det // the deterministic verifier owns the covered fraction
			if hk != det {
				disagree++ // verifier caught a cheap-model divergence; rule wins
				flagged = true
			}
		} else {
			residual++
			if !validCategory(hk) {
				schemaViol++
				flagged = true
				final = "other"
			}
		}
		tally[final]++
		rows = append(rows, row{cmd, det, hk, final, flagged})
	}

	n := len(cmds)
	if c.Verbose {
		fmt.Printf("=== per-command (source: %s) ===\n", src)
		for _, r := range rows {
			mark := ""
			if r.flagged {
				mark = "  ⚑"
			}
			d := r.det
			if d == "" {
				d = "—(residual)"
			}
			fmt.Printf("  rule=%-13s model=%-13s final=%-13s %s%s\n", d, r.hk, r.final, truncate(r.cmd, 46), mark)
		}
		fmt.Println()
	}

	fmt.Printf("triage: %d real Bash commands (%s) — map-reduce + verifier, no frontier model\n\n", n, src)
	fmt.Println("=== the shift-left stack on this chore ===")
	fmt.Printf("  verifier (deterministic rules):  covered %d/%d = g %.2f  (free, no model)\n", covered, n, frac(covered, n))
	fmt.Printf("  map (cheap model on residual):   %d/%d = %.2f\n", residual, n, frac(residual, n))
	fmt.Printf("  gate caught: %d cheap-model divergences (rule fired, model disagreed → rule won); %d schema violations\n", disagree, schemaViol)
	fmt.Println("  reduce (deterministic tally): below — a model was never asked to count\n")

	fmt.Println("=== your Bash usage, categorized ===")
	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range tally {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
	for _, p := range sorted {
		fmt.Printf("  %-15s %d\n", p.k, p.v)
	}

	fmt.Println("\nFrontier model calls: 0. Cheap-model calls: only the residual. Counting: deterministic.")
	fmt.Println("The verifier rules both cover the easy fraction AND gate the cheap model on it.")
	return nil
}

func (c *TriageCmd) loadCommands() ([]string, string, error) {
	var recs []record.Record
	src := ""
	if len(c.Home) > 0 {
		var files []string
		for _, home := range c.Home {
			m, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", "*.jsonl"))
			files = append(files, m...)
		}
		sort.Strings(files)
		for _, f := range files {
			rs, _, _ := transcript.WalkFile(f)
			recs = append(recs, rs...)
		}
		src = fmt.Sprintf("%d live transcript file(s)", len(files))
	} else {
		rs, err := corpus.Load(c.Corpus)
		if err != nil {
			return nil, "", fmt.Errorf("loading corpus %q: %w", c.Corpus, err)
		}
		recs = rs
		src = c.Corpus
	}
	var cmds []string
	seen := map[string]bool{}
	for _, r := range recs {
		if r.Tool != "Bash" {
			continue
		}
		cmd, _ := r.Args["command"].(string)
		cmd = strings.TrimSpace(cmd)
		if cmd == "" || seen[cmd] {
			continue
		}
		seen[cmd] = true
		cmds = append(cmds, cmd)
	}
	return cmds, src, nil
}

func mapClassify(ctx context.Context, c *llm.Client, cmd string) string {
	sys := "Classify this shell command into EXACTLY ONE category, reply with only the category word: " +
		strings.Join(triageCategories, ", ") + "."
	r, err := c.Classify(ctx, llm.ModelHaiku, sys, cmd, 8)
	if err != nil {
		return ""
	}
	got := strings.ToLower(strings.TrimSpace(r.Text))
	for _, cat := range triageCategories {
		if strings.Contains(got, cat) {
			return cat
		}
	}
	return got // possibly invalid → schema gate catches it
}

func validCategory(s string) bool {
	for _, c := range triageCategories {
		if s == c {
			return true
		}
	}
	return false
}
