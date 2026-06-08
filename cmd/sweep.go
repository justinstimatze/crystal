package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// SweepCmd is the autonomous DISCOVERY front-end of the crystal loop — the one
// link the manual `SWEEP_FINDINGS.md` did by hand. The vision's first verb is
// "crystal DETECTS the recurring pattern"; this is that detector.
//
// The promotion signal (from SWEEP_FINDINGS, sharper than "recurs N times"): a
// rule RE-ENCODED across N projects' standing instructions — the same rule
// independently re-written in N memories/CLAUDE.md files because recall failed to
// generalize. That is countable and deterministic (no model, no transcript scan).
//
// The tractable automation: the PROMOTABLE rules all anchor on a shell command
// (`git add -A`, `gh repo create`, `git config init.defaultBranch`); the
// non-promotable tail (taste, citation philosophy) has no command anchor and
// self-excludes. So sweep clusters rule-lines by their COMMAND SIGNATURE — which
// also maps 1:1 onto the dispatch library's bash-matcher vocabulary.
//
// Honest scope: sweep PROPOSES candidates (ranked by project-count) + a drafted
// library stub; it does NOT auto-enable a rule, because dispatch matchers are
// TESTED CODE, not arbitrary regex. Authoring+gating the matcher is the next step
// (the producer-verifier discipline: don't auto-trust a generated matcher).
type SweepCmd struct {
	DocsRoot     string `help:"Root holding per-project dirs with CLAUDE.md files." default:"~/Documents"`
	MemoryRoot   string `help:"Root holding <encoded-project>/memory/*.md files." default:"~/.claude/projects"`
	MinProjects  int    `help:"Constraints: report rules re-encoded across ≥ this many projects. Procedures: across ≥ this many sessions." default:"2"`
	Top          int    `help:"Show at most this many candidates (0 = all)." default:"0"`
	Procedures   bool   `help:"Switch to PROCEDURE discovery: mine session transcripts for recurring multi-command sequences (the cupel release-dance pattern) instead of doc constraints."`
	MinProc      int    `help:"Procedures: a sequence must recur at least this many times to be a candidate." default:"3"`
	Novel        bool   `help:"Procedures: only show sequences with an UNCOMMON step (filter out generic git add/commit/push churn) — surfaces distinctive ceremonies worth a custom command."`
	Author       bool   `help:"Procedures: author a draft shell script for the TOP candidate (expensive tier), gated by a no-run structural check (no hallucinated commands). Emits a proposal; never installs or runs it."`
	CacheDir     string `help:"Disk cache dir for the authoring model call." default:".crystal-cache"`
	Model        string `help:"Authoring model (the expensive tier)." default:"claude-opus-4-8"`
	EmitStull    bool   `help:"Constraints: emit the top constraint as a provably-sound stull machine (PreToolUse deny), run stull's static soundness check, and compile a settings.json hook. The formal-proof upgrade to --author's structural gate."`
	EmitDispatch bool   `help:"Constraints: author + GATE a data-driven regex matcher for the top NEW constraint and emit a crystal dispatch-library rule (stateless block-every-time serve). The crystal-native complement to --emit-stull."`
}

// commandPrefix maps the user-Documents path prefix so a memory's encoded dir name
// (…-Documents-<project>) resolves to the SAME project id as that project's
// CLAUDE.md (so a rule in both is counted once, not twice).
var documentsEncodedPrefix = "-Documents-"

// ruleLine is one re-encoded rule occurrence: a command signature, the project it
// was found in, and the source line (evidence).
type ruleOccurrence struct {
	signature string
	project   string
	example   string
	source    string
}

func (c *SweepCmd) Run() error {
	if c.Procedures {
		return c.runProcedures()
	}
	docsRoot := expandHome(c.DocsRoot)
	memRoot := expandHome(c.MemoryRoot)

	var occ []ruleOccurrence
	// Source 1: each project's CLAUDE.md (project = parent dir name).
	claudeFiles, _ := filepath.Glob(filepath.Join(docsRoot, "*", "CLAUDE.md"))
	for _, f := range claudeFiles {
		project := filepath.Base(filepath.Dir(f))
		occ = append(occ, mineFile(f, project)...)
	}
	// Source 2: each project's memory/*.md (project = decoded from the encoded dir).
	memFiles, _ := filepath.Glob(filepath.Join(memRoot, "*", "memory", "*.md"))
	for _, f := range memFiles {
		encoded := filepath.Base(filepath.Dir(filepath.Dir(f))) // …/<encoded>/memory/x.md
		occ = append(occ, mineFile(f, projectFromEncoded(encoded))...)
	}

	if len(occ) == 0 {
		return usageError{fmt.Errorf("no command-anchored rule lines found under %s or %s", docsRoot, memRoot)}
	}

	// Cluster by signature → distinct projects + an example line.
	type cluster struct {
		signature string
		projects  map[string]bool
		example   string
	}
	clusters := map[string]*cluster{}
	for _, o := range occ {
		cl := clusters[o.signature]
		if cl == nil {
			cl = &cluster{signature: o.signature, projects: map[string]bool{}}
			clusters[o.signature] = cl
		}
		cl.projects[o.project] = true
		if cl.example == "" {
			cl.example = o.example
		}
	}

	// Rank by distinct-project count (the re-encoded-despite-a-rule signal).
	var ranked []*cluster
	for _, cl := range clusters {
		if len(cl.projects) >= c.MinProjects {
			ranked = append(ranked, cl)
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if len(ranked[i].projects) != len(ranked[j].projects) {
			return len(ranked[i].projects) > len(ranked[j].projects)
		}
		return ranked[i].signature < ranked[j].signature
	})

	// --emit-stull: crystallize the top constraint as a provably-sound stull machine.
	if c.EmitStull {
		if len(ranked) == 0 {
			return usageError{fmt.Errorf("no constraint reached --min-projects=%d to emit", c.MinProjects)}
		}
		return c.emitConstraintStull(ranked[0].signature)
	}

	// --emit-dispatch: author + gate a regex rule for the top NEW constraint (one
	// not already served by a registry matcher — that's the case needing a matcher).
	if c.EmitDispatch {
		covered := librarySignatures()
		for _, cl := range ranked {
			if !covered[cl.signature] {
				return c.emitDispatchRule(cl.signature, cl.example)
			}
		}
		return usageError{fmt.Errorf("every constraint ≥--min-projects is already covered by a registry matcher; nothing new to author")}
	}

	covered := librarySignatures()
	fmt.Printf("crystal sweep: %d command-anchored rule occurrences → %d signatures re-encoded across ≥%d projects\n",
		len(occ), len(ranked), c.MinProjects)
	fmt.Printf("(sources: %d CLAUDE.md + %d memory files; the re-encoded-across-N-projects signal is deterministic, no model)\n\n",
		len(claudeFiles), len(memFiles))

	shown := 0
	for _, cl := range ranked {
		if c.Top > 0 && shown >= c.Top {
			fmt.Printf("  … (%d more; raise --top)\n", len(ranked)-shown)
			break
		}
		projs := sortedKeys(cl.projects)
		status := "NEW — propose to the dispatch library (matcher must be authored + tested)"
		if covered[cl.signature] {
			status = "already covered by an existing dispatch matcher"
		}
		fmt.Printf("  [%d projects] %q\n", len(cl.projects), cl.signature)
		fmt.Printf("      projects: %s\n", strings.Join(projs, ", "))
		fmt.Printf("      example : %s\n", truncate(strings.TrimSpace(cl.example), 100))
		fmt.Printf("      status  : %s\n\n", status)
		shown++
	}
	if shown == 0 {
		fmt.Printf("  (no signature reached --min-projects=%d; lower the threshold to see the long tail)\n", c.MinProjects)
	}
	return nil
}

// commandSpan matches a backtick-delimited span (the way rules cite commands).
var commandSpan = regexp.MustCompile("`([^`]+)`")

// commandLeads is the allowlist of command-leading tokens that make a backtick
// span a crystallizable command rule (vs an inline code reference to a var/path).
var commandLeads = map[string]bool{
	"git": true, "gh": true, "docker": true, "kubectl": true, "podman": true,
	"helm": true, "npm": true, "npx": true, "go": true, "cargo": true, "make": true,
	"curl": true, "wget": true, "rm": true, "ssh": true, "scp": true, "rsync": true,
	"pip": true, "python": true, "python3": true, "node": true, "yarn": true,
}

// mineFile extracts command-signature occurrences from one file's rule lines.
func mineFile(path, project string) []ruleOccurrence {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []ruleOccurrence
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !isRuleLine(line) {
			continue // a command MENTION (how-to/example), not a constraint RULE
		}
		for _, sig := range extractCommandSignatures(line) {
			out = append(out, ruleOccurrence{signature: sig, project: project, example: line, source: path})
		}
	}
	return out
}

// constraintMarkers are the imperative/prohibition words that make a line a RULE
// ("never git add -A", "default to private", "main not master") rather than a
// command mention ("run `python discover.py`"). This is the filter that separates
// the crystallizable signal from how-to noise — the judgment the manual sweep made.
var constraintMarkers = []string{
	"never", "always", "don't", "do not", "must ", "must not", "avoid", "prefer",
	"default to", "instead of", " not ", "only use", "only ever", "refuse", "ban ",
	"don’t", "no public", "by default", "should always", "should never",
}

// isRuleLine reports whether a line expresses a standing constraint (not a mention).
func isRuleLine(line string) bool {
	l := strings.ToLower(line)
	for _, m := range constraintMarkers {
		if strings.Contains(l, m) {
			return true
		}
	}
	return false
}

// extractCommandSignatures pulls every command signature cited in a line.
func extractCommandSignatures(line string) []string {
	var sigs []string
	seen := map[string]bool{}
	for _, m := range commandSpan.FindAllStringSubmatch(line, -1) {
		if sig := commandSignature(m[1]); sig != "" && !seen[sig] {
			seen[sig] = true
			sigs = append(sigs, sig)
		}
	}
	return sigs
}

// commandSignature canonicalizes a cited command to a stable cluster key:
// command + subcommand + significant flags, with path-like args dropped and the
// git-add "stage everything" synonyms (-A / --all / bare .) normalized to <all>.
// Returns "" if the span is not a recognized command (so vars/paths self-exclude).
func commandSignature(span string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(span)))
	if len(fields) == 0 || !commandLeads[fields[0]] {
		return ""
	}
	cmd := fields[0]
	var subs []string // the subcommand chain (e.g. "repo create", "config")
	stageAll := false
	for _, tok := range fields[1:] {
		// In a `git add` context, -A / --all / bare . are the rule-identity "stage
		// everything" synonyms — fold them to one marker and end the chain.
		if cmd == "git" && contains(subs, "add") && (tok == "-a" || tok == "--all" || tok == ".") {
			stageAll = true
			break
		}
		// First flag or path argument ends the subcommand chain: flags are usually
		// the prescribed remedy (--private) or noise, not the rule's identity; paths
		// are per-call args. Dropping them is what lets the same rule cluster across
		// projects that wrote it with different flags/args.
		if strings.HasPrefix(tok, "-") || looksLikePath(tok) {
			break
		}
		subs = append(subs, tok)
	}
	sig := cmd
	if len(subs) > 0 {
		sig += " " + strings.Join(subs, " ")
	}
	if stageAll {
		sig += " <all>"
	}
	return sig
}

func contains(s []string, x string) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}

func looksLikePath(tok string) bool {
	return strings.ContainsAny(tok, "/.") || strings.Contains(tok, "${")
}

// librarySignatures returns the command signatures the dispatch library already
// covers, so sweep marks them rather than re-proposing them.
func librarySignatures() map[string]bool {
	cov := map[string]bool{}
	for _, r := range defaultLibrary().Rules {
		switch r.Matcher {
		case "git_add_all":
			cov["git add <all>"] = true
		}
	}
	return cov
}

func projectFromEncoded(encoded string) string {
	if i := strings.LastIndex(encoded, documentsEncodedPrefix); i >= 0 {
		return encoded[i+len(documentsEncodedPrefix):]
	}
	return encoded
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func dedupeStrings(s []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range s {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
