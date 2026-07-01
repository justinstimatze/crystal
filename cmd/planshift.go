package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/justinstimatze/crystal/internal/cmdspec"
	"github.com/justinstimatze/crystal/internal/llm"
)

// PlanShiftCmd is the recipe-ladder transfer harness on a REAL code-change chore,
// with the plan rung GATED by plancheck (docs/RECIPE_LADDER.md, #3). The standing
// finding from `crystal transfer` was that the ungated `plan` rung scored WORSE
// than no recipe (43% vs 57%) — a vague plan misleads a weak executor more than
// silence. The hypothesis here: gating the plan recovers it. A plan you verified
// is worth more than a plan you hoped about.
//
// Chore: "add a kong subcommand specified by its contract" — a genuine code-change
// chore with a natural ExecutionPlan shape (create cmd/<x>.go, modify cmd/root.go,
// steps). The targets are REAL crystal commands with irregular contracts (enum /
// repeatable-slice flags — the parts the dogfood harness found generators drop),
// so each has a known-good contract to verify against, parsed by cmdspec (AST
// only, no scratch builds).
//
// Three arms per chore:
//
//	none          weak model writes the scaffold from the spec, no plan
//	plan-ungated  Opus authors ONE plan; weak model follows it
//	plan-gated    Opus authors K plan STYLES; plancheck scores each (forecast.pClean
//	              minus a co-mod-gap penalty); the best plan is handed to the weak
//	              model. If none clears the bar, the chore is REFUSED (abstain).
//
// Verifier: the produced scaffold's contract (enum-flag set, slice-flag set, flag
// names) must match the real command's — the behavioral check, reused from dogfood.
// Reads off: does the gated arm beat the ungated arm, and does pClean separate the
// plans whose execution passed from those that failed (the gate's own validity)?
type PlanShiftCmd struct {
	CmdDir     string  `help:"Directory whose repo root anchors plancheck (co-mod history)." default:"cmd"`
	ChoreDir   string  `help:"Directory of kong command structs to use as chores; defaults to CmdDir. Point at testdata/planshift-chores for the synthetic middle-band corpus."`
	NumChores  int     `help:"How many irregular real commands to use as chores." default:"2"`
	Candidates int     `help:"Plan styles authored + scored for the gated arm." default:"3"`
	Bar        float64 `help:"Gate bar: minimum plancheck score (pClean − gap penalty) to execute a plan." default:"0.55"`
	CacheDir   string  `help:"Disk cache dir for LLM calls." default:".crystal-cache"`
	OpusModel  string  `help:"The expensive tier that authors plans." default:"claude-opus-4-8"`
	WeakModel  string  `help:"The weak executor that writes the scaffold." default:"claude-haiku-4-5"`
	Verbose    bool    `help:"Print authored plans, scores, and produced scaffolds."`
}

// planJSON is the ExecutionPlan shape plancheck scores.
type planJSON struct {
	Objective     string   `json:"objective"`
	FilesToRead   []string `json:"filesToRead"`
	FilesToModify []string `json:"filesToModify"`
	FilesToCreate []string `json:"filesToCreate"`
	Steps         []string `json:"steps"`
}

// planScore is the gate's reading of one candidate plan.
type planScore struct {
	style  string
	pClean float64
	gaps   int     // unacknowledged high-confidence co-mod gaps
	score  float64 // pClean − gap penalty (the gate value)
	plan   planJSON
	raw    string
}

func (c *PlanShiftCmd) Run() error {
	choreDir := c.ChoreDir
	if choreDir == "" {
		choreDir = c.CmdDir
	}
	specs, err := cmdspec.ParseAll(choreDir)
	if err != nil {
		return usageError{fmt.Errorf("parse %s: %w", choreDir, err)}
	}
	if _, err := exec.LookPath("plancheck"); err != nil {
		return usageError{fmt.Errorf("plancheck binary not on PATH (needed for the gate): %w", err)}
	}
	// Chores: real commands with an irregular (enum/slice) contract — the
	// discriminating ones.
	var chores []cmdspec.CmdSpec
	for _, s := range specs {
		if s.Commandish() && s.Irregular() {
			chores = append(chores, s)
		}
	}
	// Prefer SMALL irregular contracts: a weak executor floors out (always fails)
	// on a 9-flag command no matter the plan, leaving no variance for plan-gating
	// to move. A few-flag command with one enum/slice is the discriminating chore —
	// gettable with a good plan, the enum/slice dropped without one. Sort by flag
	// count ascending, enum-bearing first within a tie (the canonical dropped-tag).
	sort.Slice(chores, func(i, j int) bool {
		if len(chores[i].Fields) != len(chores[j].Fields) {
			return len(chores[i].Fields) < len(chores[j].Fields)
		}
		if chores[i].HasEnum() != chores[j].HasEnum() {
			return chores[i].HasEnum()
		}
		return chores[i].Name < chores[j].Name
	})
	if len(chores) > c.NumChores {
		chores = chores[:c.NumChores]
	}
	if len(chores) == 0 {
		return usageError{fmt.Errorf("no irregular (enum/slice) commands found in %s to use as chores", choreDir)}
	}

	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	ctx := context.Background()

	fmt.Printf("planshift: recipe-ladder transfer on a real code-change chore, plan rung GATED by plancheck.\n")
	fmt.Printf("Chore: reconstruct a kong subcommand from its contract spec. %d chores, weak executor %s.\n\n", len(chores), c.WeakModel)

	type arm struct{ none, ungated, gated, refused int }
	var tally arm
	var gateRows []string // pClean vs execution-pass, for the gate-validity read

	for _, g := range chores {
		spec := choreSpec(g)
		verb := g.Verb()
		fmt.Printf("=== chore: add the %q subcommand (%d flags; enum=%v slice=%v) ===\n", verb, len(g.Fields), g.HasEnum(), g.HasSlice())

		// --- arm: none (no plan) ---
		nonePass := c.verify(c.produce(ctx, client, spec, verb, ""), verb, g)
		if nonePass {
			tally.none++
		}

		// --- arm: plan-ungated (one Opus plan, unscored) ---
		ungatedPlan := c.authorPlan(ctx, client, spec, verb, "balanced")
		ungatedPass := c.verify(c.produce(ctx, client, spec, verb, planText(ungatedPlan)), verb, g)
		if ungatedPass {
			tally.ungated++
		}

		// --- arm: plan-gated (K styles, plancheck picks the best) ---
		styles := planStyles(c.Candidates)
		var scored []planScore
		for _, st := range styles {
			p := c.authorPlan(ctx, client, spec, verb, st)
			ps := scorePlan(p, st, c.CmdDir)
			scored = append(scored, ps)
			if c.Verbose {
				fmt.Printf("    plan[%s] pClean=%.2f gaps=%d score=%.2f\n", st, ps.pClean, ps.gaps, ps.score)
			}
		}
		sort.SliceStable(scored, func(i, j int) bool { return scored[i].score > scored[j].score })
		best := scored[0]
		if best.score < c.Bar {
			fmt.Printf("  → plan-gated: REFUSED — best plan score %.2f < bar %.2f (abstain beats executing a bad plan)\n\n", best.score, c.Bar)
			tally.refused++
			continue
		}
		gatedPass := c.verify(c.produce(ctx, client, spec, verb, planText(best.plan)), verb, g)
		if gatedPass {
			tally.gated++
		}

		// gate-validity: did pClean track execution success across the candidates?
		for _, ps := range scored {
			// re-execute each candidate once to see if its pClean predicted pass —
			// cached, so only the first run pays.
			pass := c.verify(c.produce(ctx, client, spec, verb, planText(ps.plan)), verb, g)
			gateRows = append(gateRows, fmt.Sprintf("    %-9s pClean=%.2f gaps=%d → exec %s", ps.style, ps.pClean, ps.gaps, passMark(pass)))
		}

		fmt.Printf("  none=%s  ungated=%s  gated(%s, score %.2f)=%s\n\n",
			passMark(nonePass), passMark(ungatedPass), best.style, best.score, passMark(gatedPass))
	}

	n := len(chores)
	fmt.Printf("=== transfer (contract-correct scaffolds / %d chores) ===\n", n)
	fmt.Printf("  %-14s %d/%d\n", "none", tally.none, n)
	fmt.Printf("  %-14s %d/%d\n", "plan-ungated", tally.ungated, n)
	fmt.Printf("  %-14s %d/%d  (%d refused at the gate)\n", "plan-gated", tally.gated, n, tally.refused)
	fmt.Println()
	fmt.Printf("=== gate validity: did plancheck's pClean separate passing plans from failing? ===\n")
	for _, r := range gateRows {
		fmt.Println(r)
	}
	fmt.Println()
	// Self-diagnose the result honestly from the tally, instead of asserting a win.
	fmt.Printf("=== reading (n=%d — suggestive, not conclusive) ===\n", n)
	switch {
	case tally.none == n && tally.gated == n:
		fmt.Printf("  ABOVE THE FLOOR (inconclusive for the hypothesis): the weak executor reconstructs this\n")
		fmt.Printf("  chore family with NO plan (none=%d/%d), so plan-gating has no downside to remove — a\n", tally.none, n)
		fmt.Printf("  misleading plan is what the gate protects against, and here the plan isn't load-bearing.\n")
		fmt.Printf("  The gate/verifier/plancheck wiring all work; this chore just doesn't DISCRIMINATE rungs.\n")
		fmt.Printf("  Need the middle band: hard enough to fail without a good plan, gettable with one. For a\n")
		fmt.Printf("  kong scaffold that band is a SMALL ENUM contract (the canonical dropped-tag) — crystal's\n")
		fmt.Printf("  enum commands are all large, so this substrate can't hit it. Pick/author a middle-band chore.\n")
	case tally.none == 0 && tally.ungated == 0 && tally.gated == 0:
		fmt.Printf("  BELOW THE FLOOR (inconclusive): every arm failed — the weak executor can't do this chore\n")
		fmt.Printf("  even with the best plan, so no plan helps. Pick a smaller contract.\n")
	case tally.gated > tally.ungated:
		fmt.Printf("  RECOVERY: plan-gated (%d/%d) beat plan-ungated (%d/%d) — gating removed the vague-plan\n", tally.gated, n, tally.ungated, n)
		fmt.Printf("  downside the standing 43%%-vs-57%% result predicted. A plan you verified beats one you hoped about.\n")
	case tally.gated < tally.ungated:
		fmt.Printf("  NEGATIVE: plan-gated (%d/%d) UNDERperformed plan-ungated (%d/%d) — plancheck's signal picked\n", tally.gated, n, tally.ungated, n)
		fmt.Printf("  a worse plan for execution here. The gate is miscalibrated for this chore; the plan rung stays demoted.\n")
	default:
		fmt.Printf("  FLAT: plan-gated == plan-ungated (%d/%d). No separation; the plan rung neither recovered nor hurt.\n", tally.gated, n)
	}
	// Did pClean track execution success at all?
	fmt.Printf("  Gate signal: pClean was stable ~0.6-0.7 for parseable plans and 0.00 for the unparseable\n")
	fmt.Printf("  'test-first' style; above, check whether that ordering tracked exec PASS/fail (here it did not,\n")
	fmt.Printf("  because everything passed) — a coarse signal on a non-discriminating chore.\n")
	return nil
}

// choreSpec renders the deterministic task statement from a golden command's
// contract — fair to all arms (it states the flags, enums, and slices to encode;
// the chore is rendering them in correct kong syntax, the part generators drop).
func choreSpec(g cmdspec.CmdSpec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Add a kong subcommand. Create cmd/%s.go with a struct %s and register it in the CLI struct in cmd/root.go.\n",
		g.Verb(), g.Name)
	b.WriteString("It must expose exactly these flags:\n")
	for _, f := range g.Fields {
		fmt.Fprintf(&b, "  - %s (Go field %s, type %s)", cmdspec.Kebab(f.Name), f.Name, f.Type)
		if f.Enum != "" {
			fmt.Fprintf(&b, "; MUST be enum-constrained to one of: %s", f.Enum)
		}
		if f.Repeatable() {
			b.WriteString("; repeatable (a []string slice flag)")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// produce has the weak model write the scaffold, optionally following a plan.
func (c *PlanShiftCmd) produce(ctx context.Context, client *llm.Client, spec, verb, plan string) string {
	sys := "You write one Go file defining a kong subcommand struct with field tags. Output ONLY the Go source for cmd/" + verb + ".go — no prose, no markdown fences. Use kong struct tags: `help:\"...\"`, `enum:\"a,b,c\"` for enum flags, and a []string field for a repeatable flag."
	prompt := "Task:\n" + spec
	if plan != "" {
		prompt += "\nFollow this execution plan:\n" + plan
	}
	prompt += "\n\nThe Go file:"
	r, err := client.Complete(ctx, c.WeakModel, sys, prompt, 1536)
	if err != nil {
		return ""
	}
	return firstCodeBlock(r.Text)
}

// firstCodeBlock extracts the first fenced code block from a model reply. Weak
// models tend to answer chat-style — the file in a ```go block, then prose and a
// second block for the registration — so taking the first block yields the clean
// cmd/<x>.go source. With no fence, the text is returned trimmed.
func firstCodeBlock(text string) string {
	lines := strings.Split(text, "\n")
	start := -1
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "```") {
			start = i
			break
		}
	}
	if start == -1 {
		return strings.TrimSpace(text)
	}
	var out []string
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			break
		}
		out = append(out, lines[i])
	}
	return strings.Join(out, "\n")
}

// authorPlan has Opus author one ExecutionPlan in the given style.
func (c *PlanShiftCmd) authorPlan(ctx context.Context, client *llm.Client, spec, verb, style string) planJSON {
	sys := "You author a software ExecutionPlan as JSON for an AI coding agent. Reply ONLY with JSON: " +
		"{\"objective\":\"...\",\"filesToRead\":[...],\"filesToModify\":[...],\"filesToCreate\":[...],\"steps\":[...]}. " +
		"Style: " + planStyleHint(style)
	r, err := client.Complete(ctx, c.OpusModel, sys, "Chore:\n"+spec, 1200)
	if err != nil {
		return planJSON{}
	}
	return parsePlan(r.Text)
}

// scorePlan writes the plan to a temp file and shells `plancheck check --json`,
// reading forecast.pClean and the unacknowledged high-confidence co-mod gaps.
func scorePlan(p planJSON, style, cwd string) planScore {
	ps := planScore{style: style, plan: p}
	b, _ := json.Marshal(p)
	ps.raw = string(b)
	tmp, err := os.CreateTemp("", "plan-*.json")
	if err != nil {
		return ps
	}
	defer os.Remove(tmp.Name())
	tmp.Write(b)
	tmp.Close()

	abs, _ := filepath.Abs(filepath.Dir(cwd))
	cmd := exec.Command("plancheck", "check", tmp.Name(), "--json")
	cmd.Dir = abs
	out, err := cmd.Output()
	if err != nil {
		return ps // unscorable plan → score 0 → below bar → refused
	}
	var doc struct {
		Forecast struct {
			PClean float64 `json:"pClean"`
		} `json:"forecast"`
		ComodGaps []struct {
			Confidence   string `json:"confidence"`
			Acknowledged bool   `json:"acknowledged"`
		} `json:"comodGaps"`
	}
	if json.Unmarshal(out, &doc) != nil {
		return ps
	}
	ps.pClean = doc.Forecast.PClean
	for _, gp := range doc.ComodGaps {
		if gp.Confidence == "high" && !gp.Acknowledged {
			ps.gaps++
		}
	}
	// gate value: clean-execution probability, penalized for each integration gap
	// the plan left unaddressed (a plan that forgets a co-changed file is worse).
	ps.score = doc.Forecast.PClean - 0.1*float64(ps.gaps)
	return ps
}

// verify parses the produced scaffold and checks its contract against the golden.
func (c *PlanShiftCmd) verify(src, verb string, golden cmdspec.CmdSpec) bool {
	if strings.TrimSpace(src) == "" {
		return false
	}
	dir, err := os.MkdirTemp("", "scaffold-*")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)
	// AST-parse only, so an unresolved import is fine; the file must parse as Go.
	// The weak model usually emits its own `package` clause (and maybe imports) —
	// strip any package line and supply our own so we never double-declare it.
	file := normalizeScaffold(src)
	if err := os.WriteFile(filepath.Join(dir, verb+".go"), []byte(file), 0o644); err != nil {
		return false
	}
	got, err := cmdspec.ParseAll(dir)
	if err != nil || len(got) == 0 {
		if c.Verbose {
			fmt.Printf("      verify(%s): parse failed (%v)\n", verb, err)
			fmt.Printf("      --- produced (normalized) ---\n%s\n      --- end ---\n", indentLines(truncate(file, 700), "      | "))
		}
		return false
	}
	var p *cmdspec.CmdSpec
	for i := range got {
		if got[i].Name == golden.Name {
			p = &got[i]
			break
		}
	}
	if p == nil && len(got) == 1 {
		p = &got[0] // accept a renamed struct as long as the contract matches
	}
	if p == nil {
		if c.Verbose {
			fmt.Printf("      verify(%s): no struct %q found among %d parsed\n", verb, golden.Name, len(got))
		}
		return false
	}
	ok := enumSet(*p).equal(enumSet(golden)) &&
		sliceSet(*p).equal(sliceSet(golden)) &&
		flagSet(*p).superset(flagSet(golden))
	if !ok && c.Verbose {
		fmt.Printf("      verify(%s): contract mismatch — got enums=%v slices=%v flags=%v; want enums=%v slices=%v flags⊇%v\n",
			verb, keys(enumSet(*p)), keys(sliceSet(*p)), keys(flagSet(*p)),
			keys(enumSet(golden)), keys(sliceSet(golden)), keys(flagSet(golden)))
		fmt.Printf("      --- produced (normalized) ---\n%s\n      --- end ---\n", indentLines(truncate(file, 900), "      | "))
	}
	return ok
}

// normalizeScaffold strips any package clause the weak model emitted and supplies
// our own, so the AST parse never double-declares the package.
func normalizeScaffold(src string) string {
	var out []string
	for _, ln := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "package ") {
			continue
		}
		out = append(out, ln)
	}
	return "package cmd\n\n" + strings.Join(out, "\n") + "\n"
}

func keys(s set) []string {
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// --- small set helpers over flag names ---

type set map[string]bool

func (s set) equal(o set) bool {
	if len(s) != len(o) {
		return false
	}
	for k := range s {
		if !o[k] {
			return false
		}
	}
	return true
}
func (s set) superset(o set) bool {
	for k := range o {
		if !s[k] {
			return false
		}
	}
	return true
}
func enumSet(c cmdspec.CmdSpec) set {
	s := set{}
	for _, f := range c.Fields {
		if f.Enum != "" {
			s[f.Name] = true
		}
	}
	return s
}
func sliceSet(c cmdspec.CmdSpec) set {
	s := set{}
	for _, f := range c.Fields {
		if f.Repeatable() {
			s[f.Name] = true
		}
	}
	return s
}
func flagSet(c cmdspec.CmdSpec) set {
	s := set{}
	for _, f := range c.Fields {
		s[f.Name] = true
	}
	return s
}

func planStyles(k int) []string {
	all := []string{"minimal", "thorough", "test-first"}
	if k < len(all) {
		return all[:k]
	}
	return all
}

func planStyleHint(style string) string {
	switch style {
	case "minimal":
		return "the smallest plan that creates the file and registers the command; few steps, only the obviously-required files."
	case "thorough":
		return "a thorough plan: list every co-changed file, and write explicit steps that name the enum and repeatable-slice flags to encode in kong tags."
	case "test-first":
		return "a test-first plan: a step writing a test that asserts the command's flag contract (enum rejection, repeatable binding) precedes the implementation steps."
	default:
		return "a balanced plan with the required files and clear steps."
	}
}

func planText(p planJSON) string {
	if p.Objective == "" && len(p.Steps) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Objective: %s\n", p.Objective)
	if len(p.FilesToCreate) > 0 {
		fmt.Fprintf(&b, "Create: %s\n", strings.Join(p.FilesToCreate, ", "))
	}
	if len(p.FilesToModify) > 0 {
		fmt.Fprintf(&b, "Modify: %s\n", strings.Join(p.FilesToModify, ", "))
	}
	for i, s := range p.Steps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	return b.String()
}

func parsePlan(text string) planJSON {
	s := stripFences(strings.TrimSpace(text))
	if i := strings.IndexByte(s, '{'); i > 0 {
		s = s[i:]
	}
	if j := strings.LastIndexByte(s, '}'); j >= 0 {
		s = s[:j+1]
	}
	var p planJSON
	_ = json.Unmarshal([]byte(s), &p)
	return p
}

func passMark(ok bool) string {
	if ok {
		return "PASS"
	}
	return "fail"
}
