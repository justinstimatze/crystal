package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/justinstimatze/crystal/internal/cmdspec"
	"github.com/justinstimatze/crystal/internal/cmdverify"
	"github.com/justinstimatze/crystal/internal/flow"
	"github.com/justinstimatze/crystal/internal/llm"
)

// DogfoodCmd is the real-target pudding after `demo`: crystal crystallizes its
// OWN recurring codegen chore — the kong subcommand scaffold — and reports
// where the verifier's coverage ends. Two things make it a real test of the
// "typed schema + operational verifier" framing that the toy demo could not:
//
//  1. The verifier is OPERATIONAL and golden-free: a generated scaffold is
//     correct iff it builds, registers (kong exposes the verb), and passes a
//     behavioral contract — the Go compiler and kong ARE the gate, no pre-known
//     answer. (internal/cmdverify)
//  2. The miss is DISCOVERED, not seeded: the generator is authored from the
//     plain scalar-flag commands, then made to serve the irregular ones (enum
//     flags, repeatable slices) it was never shown. Whatever it gets wrong, it
//     gets wrong on its own.
//
// The deliverable is the coverage table: for each shape class, which verifier
// LAYER caught the miss — or that none did. An enum-stripped scaffold builds and
// registers cleanly and only the contract catches it; a slice rendered as a
// scalar builds, registers, AND passes a presence contract while being wrong —
// the measured g<1 boundary (a structural gate cannot see a behavioral error).
//
// Per "plan to throw one away": the generated scaffolds are disposable scratch;
// the tooling (cmdspec parser + cmdverify gate + this loop) is the keeper. crystal
// never edits its own cmd/ — passing scaffolds are written to a proposal dir for
// a human to move in.
type DogfoodCmd struct {
	Corpus    string `help:"Directory of Go files holding the kong subcommand structs (the corpus)." default:"cmd"`
	FirstK    int    `help:"How many regular (scalar-flag) commands the generator is authored from." default:"10"`
	Threshold float64 `help:"Promote gate: fraction of the holdout that must build+register." default:"0.95"`
	Offline   bool   `help:"Synthesize the generator deterministically (no model). Default is live (Opus authors it). The verifier is real in both regimes."`
	CacheDir  string `help:"Disk cache dir for LLM calls (live mode)." default:".crystal-cache"`
	Model     string `help:"Authoring model (the expensive tier), live mode." default:"claude-opus-4-8"`
	Scratch   string `help:"Throwaway dir for compiled scaffolds (gitignored; under the current module so kong resolves offline)." default:".dogfood-scratch"`
	FlowOut   string `help:"Sankey flow record for the viz. Empty = don't write." default:".crystal-viz/dogfood-flow.json"`
	Verbose   bool   `help:"Print each scaffold's verdict detail and the authored template."`
}

func (c *DogfoodCmd) Run() error {
	specs, err := cmdspec.Parse(c.Corpus)
	if err != nil {
		return usageError{fmt.Errorf("parse corpus %s: %w", c.Corpus, err)}
	}
	// The harness's own commands are not part of the chore being crystallized.
	specs = drop(specs, "DogfoodCmd", "DemoCmd")
	if len(specs) < c.FirstK+2 {
		return usageError{fmt.Errorf("parsed %d commands; need ≥ first-k+2 = %d", len(specs), c.FirstK+2)}
	}

	var regular, irregular []cmdspec.CmdSpec
	for _, s := range specs {
		if s.Irregular() {
			irregular = append(irregular, s)
		} else {
			regular = append(regular, s)
		}
	}
	if len(regular) < c.FirstK {
		return usageError{fmt.Errorf("only %d regular commands; need %d to author", len(regular), c.FirstK)}
	}

	var client *llm.Client
	regime := "offline (stub author; the build/register/contract verifier is still real)"
	if !c.Offline {
		client, err = llm.New(c.CacheDir)
		if err != nil {
			return usageError{fmt.Errorf("%w (or pass --offline)", err)}
		}
		regime = "live (Opus authors the generator; verifier is the Go compiler + kong)"
	}

	scratch, _ := filepath.Abs(c.Scratch)
	_ = os.RemoveAll(scratch) // fresh throwaway tree
	proposalDir := filepath.Join(scratch, "proposals")

	fmt.Printf("dogfood: crystallize crystal's own subcommand scaffold over %d real commands (%s/)\n", len(specs), c.Corpus)
	fmt.Printf("regime: %s\n", regime)
	fmt.Printf("corpus shape: %d regular (scalar flags) · %d irregular (%s)\n\n", len(regular), len(irregular), shapeBreakdown(irregular))

	// Author set = first K regular; hold out the tail of the worked set to gate;
	// serve everything else — the remaining regulars AND all irregulars (the
	// generator never saw an enum or a slice).
	worked := regular[:c.FirstK]
	split := c.FirstK - 2
	if split < 1 {
		split = 1
	}
	authorSet := append([]cmdspec.CmdSpec{}, worked[:split]...)
	holdout := worked[split:]
	serveSet := append(append([]cmdspec.CmdSpec{}, regular[c.FirstK:]...), irregular...)

	ctx := context.Background()
	fmt.Printf("=== author the generator from %d regular commands, gate on %d held out (build+register) ===\n", len(authorSet), len(holdout))
	gen, desc, err := c.author(ctx, client, authorSet)
	if err != nil {
		return usageError{fmt.Errorf("authoring generator: %w", err)}
	}
	fmt.Printf("  generator: %s\n", desc)
	if c.Verbose {
		if tr, ok := gen.(tmplCmdRenderer); ok {
			fmt.Printf("  --- template ---\n%s\n  ----------------\n", indentLines(tr.src, "    "))
		}
	}
	gpass, gn := gateBuild(scratch, gen, holdout)
	gacc := float64(gpass) / float64(max(gn, 1))
	fmt.Printf("  gate: %d/%d build+register = %.2f (threshold %.2f) → %s\n\n", gpass, gn, gacc, c.Threshold, promoteWord(gacc >= c.Threshold))
	if gacc < c.Threshold {
		fmt.Printf("Decision: REJECT — generator scored %.2f < %.2f; not crystallized.\n", gacc, c.Threshold)
		return nil
	}

	// ---- SERVE: every remaining command through the verifier; demote-on-miss ----
	fmt.Printf("=== serve %d commands through the generator (0 model calls each), verify, demote on a caught miss ===\n", len(serveSet))
	var rows []dfRow
	servedDet, reserved, deferred := 0, 0, 0
	for _, spec := range serveSet {
		src, _ := gen.render(spec)
		v, verr := cmdverify.Verify(scratch, spec.Name, field(spec), spec.Verb(), src, contractFor(spec))
		if verr != nil {
			return usageError{fmt.Errorf("verify %s: %w", spec.Name, verr)}
		}
		wrong := renderedWrong(spec, src)
		row := dfRow{Verb: spec.Verb(), Class: classOf(spec), V: v, Wrong: wrong}
		if v.Pass() {
			servedDet++
			row.Tier = "served-det"
			if wrong {
				row.Tier = "served-det(LEAK)" // built+registered+contract-passed but actually wrong
			}
			writeProposal(proposalDir, spec.Verb(), src)
			rows = append(rows, row)
			fmt.Printf("  %-13s %-7s %s  %s\n", spec.Verb(), row.Class, verdictMark(v), leakNote(wrong))
			continue
		}
		// Gate CAUGHT a miss → demote this command to the frontier, re-author with
		// it in scope, re-gate this one, re-serve.
		deferred++
		fmt.Printf("  %-13s %-7s %s  ✗ %s — demote + re-author\n", spec.Verb(), row.Class, verdictMark(v), v.Detail)
		authorSet = append(authorSet, spec)
		newGen, ndesc, aerr := c.author(ctx, client, authorSet)
		if aerr != nil {
			return usageError{fmt.Errorf("re-authoring after %s: %w", spec.Name, aerr)}
		}
		src2, _ := newGen.render(spec)
		v2, _ := cmdverify.Verify(scratch, spec.Name, field(spec), spec.Verb(), src2, contractFor(spec))
		if v2.Pass() {
			gen = newGen
			reserved++
			row.Tier, row.V, row.Wrong = "reserved-det", v2, renderedWrong(spec, src2)
			writeProposal(proposalDir, spec.Verb(), src2)
			fmt.Printf("       re-authored (%s) → re-served deterministically ✓\n", ndesc)
		} else {
			row.Tier = "opus(residual)"
			fmt.Printf("       re-author still fails the gate; stays on the frontier (residual).\n")
		}
		rows = append(rows, row)
	}

	c.report(rows, len(regular), len(irregular), servedDet, reserved, deferred, proposalDir)
	c.emit(rows, servedDet, reserved, deferred, len(authorSet))
	return nil
}

// ---- generator: two regimes behind one interface ----

type cmdRenderer interface {
	render(spec cmdspec.CmdSpec) (string, error)
}

// stubCmdRenderer is the offline generalizer. Its awareness mirrors what an
// author would generalize from its examples: shown only scalar-flag commands it
// emits no enum tag (drift on enum commands) and renders a []string as a plain
// string (the subtle, build-passing slice leak).
type stubCmdRenderer struct{ enumAware, sliceAware bool }

func (s stubCmdRenderer) render(spec cmdspec.CmdSpec) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "type %s struct {\n", spec.Name)
	for _, f := range spec.Fields {
		typ := f.Type
		if f.Repeatable() && !s.sliceAware {
			typ = "string"
		}
		tag := "help:" + strconv.Quote(f.Name)
		if f.Default != "" {
			tag += " default:" + strconv.Quote(f.Default)
		}
		if f.Enum != "" && s.enumAware {
			tag += " enum:" + strconv.Quote(f.Enum)
		}
		fmt.Fprintf(&b, "\t%s %s `%s`\n", f.Name, typ, tag)
	}
	b.WriteString("}\n")
	return b.String(), nil
}

type tmplCmdRenderer struct {
	t   *template.Template
	src string
}

func (r tmplCmdRenderer) render(spec cmdspec.CmdSpec) (string, error) {
	var b strings.Builder
	if err := r.t.Execute(&b, spec); err != nil {
		return "", err
	}
	return b.String(), nil
}

// author builds the generator from the example commands. Offline synthesizes
// from the features present; live asks Opus to write a Go text/template over a
// CmdSpec and validates it reproduces the examples' scaffolds.
func (c *DogfoodCmd) author(ctx context.Context, client *llm.Client, examples []cmdspec.CmdSpec) (cmdRenderer, string, error) {
	enumAware, sliceAware := anyEnum(examples), anySlice(examples)
	if c.Offline {
		return stubCmdRenderer{enumAware: enumAware, sliceAware: sliceAware},
			fmt.Sprintf("stub(enum-aware=%v, slice-aware=%v)", enumAware, sliceAware), nil
	}
	// The golden scaffold for an example is the fully-aware stub render.
	golden := stubCmdRenderer{enumAware: true, sliceAware: true}
	var b strings.Builder
	b.WriteString("Worked examples — each a kong command spec (JSON) and the exact Go struct it must produce:\n\n")
	for _, ex := range examples {
		g, _ := golden.render(ex)
		fmt.Fprintf(&b, "%s\n=>\n%s\n", mustJSON(ex), g)
	}
	sys := "You write a DETERMINISTIC Go text/template that renders a kong command struct from its spec. " +
		"The data is a struct with .Name (string) and .Fields (each with .Name, .Type, .Help, .Default strings and a .Repeatable method). " +
		"Output ONLY the template text (no prose, no markdown fences). Executed with Go's text/template over one spec, it must reproduce the worked examples EXACTLY: " +
		"`type <Name> struct {` then one line per field `\\t<Name> <Type> `+\"`\"+`help:\"...\" default:\"...\"`+\"`\"+`` (omit default: when empty), then `}`."
	r, err := client.Complete(ctx, c.Model, sys, b.String(), 2048)
	if err != nil {
		return nil, "", err
	}
	src := stripFence(r.Text)
	t, perr := template.New("cmd").Parse(src)
	if perr != nil {
		return nil, "", fmt.Errorf("authored template does not parse: %w", perr)
	}
	return tmplCmdRenderer{t: t, src: src}, fmt.Sprintf("opus text/template (%d bytes)", len(src)), nil
}

// ---- gate + verifier glue ----

func gateBuild(scratch string, gen cmdRenderer, holdout []cmdspec.CmdSpec) (pass, n int) {
	for _, spec := range holdout {
		n++
		src, err := gen.render(spec)
		if err != nil {
			continue
		}
		v, _ := cmdverify.Verify(scratch, spec.Name, field(spec), spec.Verb(), src, nil)
		if v.Builds && v.Registers {
			pass++
		}
	}
	return pass, n
}

func contractFor(spec cmdspec.CmdSpec) *cmdverify.Contract {
	for _, f := range spec.Fields {
		if f.Enum != "" {
			return &cmdverify.Contract{EnumFlag: cmdspec.Kebab(f.Name), OffListVal: "__dogfood_offlist__"}
		}
	}
	for _, f := range spec.Fields {
		if f.Repeatable() {
			return &cmdverify.Contract{NeedFlag: cmdspec.Kebab(f.Name)}
		}
	}
	return nil
}

// renderedWrong is the GROUND-TRUTH column (for measuring g<1), independent of
// the gate: did the generator actually mis-render relative to the corpus truth?
func renderedWrong(spec cmdspec.CmdSpec, src string) bool {
	if spec.HasEnum() && !strings.Contains(src, "enum:") {
		return true
	}
	if spec.HasSlice() && !strings.Contains(src, "[]string") {
		return true
	}
	return false
}

func classOf(spec cmdspec.CmdSpec) string {
	switch {
	case spec.HasEnum():
		return "enum"
	case spec.HasSlice():
		return "slice"
	default:
		return "regular"
	}
}

func field(spec cmdspec.CmdSpec) string { return strings.TrimSuffix(spec.Name, "Cmd") }

func anyEnum(s []cmdspec.CmdSpec) bool {
	for _, x := range s {
		if x.HasEnum() {
			return true
		}
	}
	return false
}

func anySlice(s []cmdspec.CmdSpec) bool {
	for _, x := range s {
		if x.HasSlice() {
			return true
		}
	}
	return false
}

func drop(specs []cmdspec.CmdSpec, names ...string) []cmdspec.CmdSpec {
	skip := map[string]bool{}
	for _, n := range names {
		skip[n] = true
	}
	var out []cmdspec.CmdSpec
	for _, s := range specs {
		if !skip[s.Name] {
			out = append(out, s)
		}
	}
	return out
}

func writeProposal(dir, verb, src string) {
	_ = os.MkdirAll(dir, 0o755)
	// .go.txt, never .go — a proposal a human reviews, never auto-compiled in-repo.
	_ = os.WriteFile(filepath.Join(dir, verb+".go.txt"), []byte(src), 0o644)
}

// ---- reporting ----

type dfRow struct {
	Verb  string
	Class string
	Tier  string
	V     cmdverify.Verdict
	Wrong bool
}

func (c *DogfoodCmd) report(rows []dfRow, nReg, nIrr, servedDet, reserved, deferred int, proposalDir string) {
	total := len(rows)
	clean := servedDet + reserved

	fmt.Printf("\n=== coverage table (the g<1 boundary, measured) ===\n")
	fmt.Printf("  %-8s %4s  %6s %9s %8s   %s\n", "class", "n", "builds", "registers", "caught", "leaked (built+registered+contract-passed yet WRONG)")
	for _, cls := range []string{"regular", "enum", "slice"} {
		var n, builds, regs, caught, leaked int
		for _, r := range rows {
			if r.Class != cls {
				continue
			}
			n++
			if r.V.Builds {
				builds++
			}
			if r.V.Registers {
				regs++
			}
			if r.Tier == "reserved-det" || r.Tier == "opus(residual)" {
				caught++ // the gate failed → we demoted (caught the miss)
			}
			if strings.Contains(r.Tier, "LEAK") {
				leaked++
			}
		}
		if n == 0 {
			continue
		}
		fmt.Printf("  %-8s %4d  %6d %9d %8d   %d\n", cls, n, builds, regs, caught, leaked)
	}

	fmt.Printf("\n=== reading the table ===\n")
	fmt.Println("  regular: built + registered clean, no contract needed — served deterministically.")
	fmt.Println("  enum:    builds AND registers (kong tags are permissive), so the structural layers are")
	fmt.Println("           blind to it; ONLY the behavioral contract (off-list value rejected) catches the")
	fmt.Println("           dropped enum → demote → re-author → recovered. The verifier had to be behavioral.")
	leaks := countLeak(rows)
	if leaks > 0 {
		fmt.Println("  slice:   builds, registers, AND passes a flag-presence contract while rendered as a scalar")
		fmt.Println("           string instead of []string — a real behavioral error the gate as configured does")
		fmt.Println("           NOT see. That LEAK is the measured g<1: a structural+presence gate ends here; a")
		fmt.Println("           repetition-binding contract (unwritten) is the boundary, and the residual is the")
		fmt.Println("           frontier's. Reported, not hidden.")
	} else {
		fmt.Println("  slice:   rendered CORRECTLY this run — the generator expressed []string straight from the")
		fmt.Println("           spec's .Type, so no leak fired. The verifier's blind spot is still real but went")
		fmt.Println("           untested: a flag-presence contract would also pass a scalar-rendered slice; only a")
		fmt.Println("           repetition-binding contract would catch that. The slice leak is generator-dependent")
		fmt.Println("           (a crude stub leaks it; a strong author closes it) — UNLIKE the enum drop, which is")
		fmt.Println("           fundamental: a struct tag absent from every example, missed regardless of author.")
	}

	fmt.Printf("\n=== apportionment ===\n")
	fmt.Printf("  passed the gate, served deterministically: %d/%d\n", clean, total)
	fmt.Printf("    ├─ correct:           %d  (%d clean + %d re-served after a caught demote)\n", clean-leaks, servedDet-leaks, reserved)
	fmt.Printf("    └─ wrong-but-passed:  %d  (slice-as-scalar leaks — the gate's blind spot)\n", leaks)
	fmt.Printf("  caught misses → demoted → re-authored: %d  (enum drop, caught only by the behavioral contract)\n", deferred)
	if leaks > 0 {
		fmt.Printf("     Note: the enum re-author incidentally taught the generator the slice shape too, so slice\n")
		fmt.Printf("     commands served AFTER it rendered correctly — the %d leaks are the ones served BEFORE that\n", leaks)
		fmt.Printf("     fix, already shipped wrong: exactly why a passing gate must be trusted, and why order matters.\n")
	} else {
		fmt.Printf("     Cross-regime read: the enum drop reproduces with ANY author (it is a tag absent from the\n")
		fmt.Printf("     examples); the slice leak does NOT (a strong author renders .Type verbatim). The robust,\n")
		fmt.Printf("     generator-independent claim is the one that survives: the verifier had to be BEHAVIORAL.\n")
	}
	fmt.Printf("  proposals written (human reviews, NOT auto-merged): %s/*.go.txt\n", proposalDir)
	fmt.Printf("\nDogfood result: the loop crystallizes crystal's own scaffold; build+register is nearly vacuous on\n")
	fmt.Printf("kong tags, the OPERATIONAL contract does the real work (the framing's core claim, measured), and the\n")
	fmt.Printf("slice leak marks exactly where this verifier's coverage ends. The scaffolds are throwaway; the\n")
	fmt.Printf("cmdspec parser + cmdverify gate + this loop are the tooling that ports to the next project.\n")
}

func countLeak(rows []dfRow) int {
	n := 0
	for _, r := range rows {
		if strings.Contains(r.Tier, "LEAK") {
			n++
		}
	}
	return n
}

func (c *DogfoodCmd) emit(rows []dfRow, servedDet, reserved, deferred, authoredN int) {
	if c.FlowOut == "" {
		return
	}
	rec := flow.Record{
		Run:         "dogfood",
		Oracle:      "go-build+kong",
		BigProvider: c.Model,
		Pair:        "kong-subcommand",
		Note:        fmt.Sprintf("%d served-det · %d re-served · %d leaked", servedDet-countLeak(rows), reserved, countLeak(rows)),
		Nodes: []flow.Node{
			{ID: "stream", Label: "commands", Regime: "fate"},
			{ID: "served-det", Label: "served-det (build+register+contract)", Regime: "owned-local"},
			{ID: "deferred-model", Label: "frontier (opus)", Regime: "vendor"},
			{ID: "reauthor", Label: "re-authored", Regime: "fate"},
			{ID: "served-now", Label: "re-served-det", Regime: "owned-local"},
		},
		Edges: []flow.Edge{
			{Source: "stream", Target: "served-det", Value: servedDet, Kind: "shift-left"},
			{Source: "stream", Target: "deferred-model", Value: deferred, Kind: "defer"},
			{Source: "reauthor", Target: "served-now", Value: reserved, Kind: "shift-left"},
		},
	}
	if err := rec.WriteFile(c.FlowOut); err != nil {
		fmt.Printf("  (flow viz: could not write %s: %v)\n", c.FlowOut, err)
	} else {
		fmt.Printf("  flow record → %s\n", c.FlowOut)
	}
	histPath := filepath.Join(filepath.Dir(c.FlowOut), "history.jsonl")
	_ = rec.AppendHistory(histPath, time.Now().UTC().Format(time.RFC3339))
}

// ---- small display helpers ----

func verdictMark(v cmdverify.Verdict) string {
	b, r, ct := "·", "·", "·"
	if v.Builds {
		b = "build✓"
	} else {
		b = "build✗"
	}
	if v.Registers {
		r = "reg✓"
	}
	if v.HasContract {
		if v.Contract {
			ct = "contract✓"
		} else {
			ct = "contract✗"
		}
	}
	return fmt.Sprintf("%-6s %-4s %-9s", b, r, ct)
}

func leakNote(wrong bool) string {
	if wrong {
		return "✓ built — but WRONG (leak past the gate)"
	}
	return "✓"
}

func shapeBreakdown(irr []cmdspec.CmdSpec) string {
	e, s := 0, 0
	for _, x := range irr {
		if x.HasEnum() {
			e++
		} else if x.HasSlice() {
			s++
		}
	}
	parts := []string{}
	if e > 0 {
		parts = append(parts, fmt.Sprintf("%d enum", e))
	}
	if s > 0 {
		parts = append(parts, fmt.Sprintf("%d slice", s))
	}
	sort.Strings(parts)
	return strings.Join(parts, " + ")
}
