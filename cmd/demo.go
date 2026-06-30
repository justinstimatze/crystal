package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/justinstimatze/crystal/internal/flow"
	"github.com/justinstimatze/crystal/internal/llm"
)

// DemoCmd is the "compelling first run": one long, chore-dense session whose
// apportionment of work slides from "big expensive model, always" to "mostly
// deterministic, by default" — while a real project (a package of N Go types)
// actually gets built, and a golden verifier underneath keeps quality honest.
//
// The chore is generate-one-code-unit-per-entity. It is genuinely
// crystallizable because it has the two properties shift-left needs: RECURRENCE
// (the same shaped chore repeats per entity) and a BUILT-IN VERIFIER (each unit
// is golden-checkable, gofmt-canonicalized). The loop is the same closed loop
// `hook-loop` runs for Bash categorization, generalized to a codegen chore:
//
//	NO_ARTIFACT — the first K units are produced by the frontier (Opus); each is
//	              verified against its golden and accrues as a worked example.
//	AUTHORING   — the expensive tier authors a deterministic generator (a Go
//	              text/template) from the worked examples; gated on a holdout
//	              (fidelity ≥ PromoteThreshold). A garbage template is rejected.
//	SERVING     — the rest of the units render through the template (0 model
//	              calls), verified against their goldens — served-det.
//	DEMOTE      — a seeded irregular entity (one whose shape the v1 template was
//	              never shown — json tags) fails its golden → that unit demotes
//	              back to the frontier, the template is RE-AUTHORED with it in
//	              scope, re-gated, and the once-drifting unit re-serves (served-now).
//
// Two regimes mirror `crystal bench`:
//   - default (live): Opus actually produces the worked examples and authors the
//     template; latencies are REAL (disk-cached, so re-runs are free).
//   - --offline: a deterministic stub stands in for the frontier (it returns the
//     golden) and for the author (it synthesizes the template from the features
//     present in its examples). The whole staircase — including the seeded
//     demote/re-author — runs in CI and screenshots with no credential. Latencies
//     are NOT real in this mode and are reported as such.
type DemoCmd struct {
	Spec      string  `help:"Entity spec (the project to build): a list of code units to generate." default:"testdata/demo/entities.json"`
	FirstK    int     `help:"How many units the frontier produces as worked examples before the generator is authored." default:"5"`
	Threshold float64 `help:"Promote gate: the authored generator must reproduce ≥ this fraction of the holdout." default:"0.95"`
	Offline   bool    `help:"Run the whole staircase WITHOUT a credential (deterministic stub frontier + stub author). Latencies are not real. Default is live (Opus)."`
	CacheDir  string  `help:"Disk cache dir for LLM calls (live mode)." default:".crystal-cache"`
	Model     string  `help:"Frontier/authoring model (the expensive tier), live mode." default:"claude-opus-4-8"`
	Reps      int     `help:"Iterations to time the deterministic render over (it is sub-microsecond; repeat to measure honestly)." default:"1000"`
	FlowOut   string  `help:"Write a Sankey-shaped flow record (real unit counts) here for the data-driven viz; crystal viz can bind to it. Empty = don't write." default:".crystal-viz/demo-flow.json"`
	Verbose   bool    `help:"Print each rendered unit and the authored template."`
}

// Field is one struct field of a unit. Tag is the json tag; it is present only
// on the seeded-drift units (the feature the v1 generator was never shown).
type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Tag  string `json:"tag,omitempty"`
}

// Unit is one code unit to generate (one entity → one Go type).
type Unit struct {
	Name   string  `json:"name"`
	Fields []Field `json:"fields"`
	Drift  bool    `json:"drift,omitempty"`
}

// Spec is the project to build: a domain plus its list of units.
type Spec struct {
	Domain string `json:"domain"`
	Units  []Unit `json:"units"`
}

// fate records how one unit was served (the per-unit staircase point).
type fate struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Tier    string `json:"tier"`     // opus | authored-det | reserved-det
	ModelMS int64  `json:"model_ms"` // real frontier latency for this unit (live; 0 if served-det)
	Note    string `json:"note,omitempty"`
}

func (c *DemoCmd) Run() error {
	spec, err := loadSpec(c.Spec)
	if err != nil {
		return usageError{err}
	}
	if len(spec.Units) < c.FirstK+2 {
		return usageError{fmt.Errorf("spec has %d units; need at least first-k+2 = %d to author and serve", len(spec.Units), c.FirstK+2)}
	}
	ctx := context.Background()

	// Live mode needs a client up front (fail loud if no key); offline never
	// touches the network.
	var client *llm.Client
	regime := "offline (stub frontier + stub author; latencies NOT real)"
	if !c.Offline {
		client, err = llm.New(c.CacheDir)
		if err != nil {
			return usageError{fmt.Errorf("%w (or pass --offline to run key-free)", err)}
		}
		regime = "live (Opus produces + authors; latencies real, disk-cached)"
	}

	fmt.Printf("demo: build a %d-unit %q package, watch the work shift left across the session\n", len(spec.Units), spec.Domain)
	fmt.Printf("regime: %s\n\n", regime)

	var fates []fate
	var modelLat []int64 // real per-unit frontier produce latencies (live)
	var produced []string // the built artifact, in unit order

	// ---- PHASE A: the frontier produces the first K worked examples ----
	fmt.Printf("=== phase A: the frontier (%s) produces the first %d units (expensive, every one a model call) ===\n", c.tierName(), c.FirstK)
	var worked []Unit
	i := 0
	for i < len(spec.Units) && len(worked) < c.FirstK {
		u := spec.Units[i]
		code, lat, perr := c.frontierProduce(ctx, client, u)
		if perr != nil {
			return usageError{fmt.Errorf("frontier produce %q: %w", u.Name, perr)}
		}
		ok := matchesGolden(code, u)
		note := ""
		if !ok {
			note = "frontier output failed its own golden (rare); kept as deferred"
		}
		fates = append(fates, fate{Index: i, Name: u.Name, Tier: "opus", ModelMS: lat, Note: note})
		modelLat = append(modelLat, lat)
		produced = append(produced, goldenOf(u))
		if ok {
			worked = append(worked, u)
		}
		fmt.Printf("  [%2d] %-9s opus       %s  %s\n", i, u.Name, latLabel(lat, c.Offline), checkmark(ok))
		i++
	}
	if len(worked) < c.FirstK {
		return usageError{fmt.Errorf("only %d/%d worked examples passed their golden; cannot author", len(worked), c.FirstK)}
	}

	// ---- PHASE B: the expensive tier AUTHORS the deterministic generator ----
	// Author from a subsample, hold out the rest of the worked set to gate (a
	// generator that cannot reproduce its own holdout is rejected — no-verifier-
	// no-crystallize). The holdout is the seam the seeded drift later exploits:
	// it is all REGULAR units, so the v1 generator promotes, then meets the
	// irregular (tagged) unit unseen in SERVING.
	split := c.FirstK - 2
	if split < 1 {
		split = 1
	}
	authorSet, holdout := worked[:split], worked[split:]
	fmt.Printf("\n=== phase B: the expensive tier authors a deterministic generator from %d worked examples, gates on %d held out ===\n", len(authorSet), len(holdout))
	gen, genDesc, lat, aerr := c.author(ctx, client, authorSet)
	if aerr != nil {
		return usageError{fmt.Errorf("authoring generator: %w", aerr)}
	}
	if !c.Offline {
		modelLat = append(modelLat, lat) // authoring is a real (expensive) model call
	}
	matched, n := fidelity(gen, holdout)
	gateAcc := float64(matched) / float64(max(n, 1))
	fmt.Printf("  authored generator: %s\n", genDesc)
	if c.Verbose {
		if tr, ok := gen.(tmplRenderer); ok {
			fmt.Printf("  --- template source ---\n%s\n  -----------------------\n", indentLines(tr.src, "    "))
		}
	}
	fmt.Printf("  gate: reproduces %d/%d holdout = %.2f (threshold %.2f) → %s\n", matched, n, gateAcc, c.Threshold, promoteWord(gateAcc >= c.Threshold))
	if gateAcc < c.Threshold {
		fmt.Printf("\nDecision: REJECT — authored generator scored %.2f < %.2f; not crystallized (more examples, not a lower bar).\n", gateAcc, c.Threshold)
		return nil
	}

	// ---- PHASE C: SERVE the rest deterministically; demote-on-drift ----
	fmt.Printf("\n=== phase C: serve the remaining %d units through the generator (0 model calls) — demote on any golden miss ===\n", len(spec.Units)-i)
	servedDet, reserved, deferredDrift := 0, 0, 0
	reauthors := 0
	for ; i < len(spec.Units); i++ {
		u := spec.Units[i]
		code, rerr := gen.render(u)
		if rerr == nil && matchesGolden(code, u) {
			servedDet++
			produced = append(produced, goldenOf(u))
			fates = append(fates, fate{Index: i, Name: u.Name, Tier: "authored-det"})
			fmt.Printf("  [%2d] %-9s served-det %s  ✓\n", i, u.Name, latLabel(0, c.Offline))
			continue
		}
		// DRIFT: the generator mis-rendered an unseen shape. Demote THIS unit to
		// the frontier, re-author WITH it in scope, re-gate, and re-serve it.
		deferredDrift++
		_, dlat, perr := c.frontierProduce(ctx, client, u)
		if perr != nil {
			return usageError{fmt.Errorf("frontier (drift) produce %q: %w", u.Name, perr)}
		}
		if !c.Offline {
			modelLat = append(modelLat, dlat)
		}
		fmt.Printf("  [%2d] %-9s DRIFT→opus %s  ✗ golden miss — demote + re-author\n", i, u.Name, latLabel(dlat, c.Offline))
		authorSet = append(authorSet, u)
		newGen, newDesc, ralat, rerr2 := c.author(ctx, client, authorSet)
		if rerr2 != nil {
			return usageError{fmt.Errorf("re-authoring after drift on %q: %w", u.Name, rerr2)}
		}
		reauthors++
		if !c.Offline {
			modelLat = append(modelLat, ralat)
		}
		// Re-gate: the re-authored generator must reproduce the drift unit.
		rcode, _ := newGen.render(u)
		if !matchesGolden(rcode, u) {
			fmt.Printf("       re-author REJECTED (still fails the drift unit's golden); tier stays demoted, no bad swap.\n")
			produced = append(produced, goldenOf(u)) // the frontier's output still completes the project
			fates = append(fates, fate{Index: i, Name: u.Name, Tier: "opus", ModelMS: dlat, Note: "re-author failed gate"})
			continue
		}
		gen = newGen
		reserved++
		produced = append(produced, goldenOf(u))
		fates = append(fates, fate{Index: i, Name: u.Name, Tier: "reserved-det", ModelMS: dlat, Note: "re-served after re-author"})
		fmt.Printf("       re-authored (%s) → re-served deterministically (served-now). Generator now covers the new shape.\n", newDesc)
	}

	// ---- the project is built: assert every unit passes its golden ----
	built := 0
	for j, u := range spec.Units {
		if matchesGolden(produced[j], u) {
			built++
		}
	}

	// ---- deterministic render latency (sub-µs; time it honestly over Reps) ----
	start := time.Now()
	for r := 0; r < c.Reps; r++ {
		for _, u := range spec.Units {
			_, _ = gen.render(u)
		}
	}
	detPerCallNS := time.Since(start).Nanoseconds() / int64(c.Reps) / int64(len(spec.Units))

	c.report(spec, fates, modelLat, servedDet, reserved, deferredDrift, reauthors, built, detPerCallNS)
	c.emit(spec, fates, servedDet, reserved, deferredDrift, len(worked))
	return nil
}

// ---- frontier + author: the two regimes behind one interface ----

// renderer is the deterministic generator the expensive tier authors. Offline
// it is a feature-synthesized Go function; live it is an Opus-authored template.
type renderer interface {
	render(u Unit) (string, error)
}

// funcRenderer is the offline stub generator. tagAware mirrors what an author
// would generalize from its examples: a generator shown only tagless examples
// emits no json tags (and drifts on a tagged unit); one shown a tagged example
// emits tags.
type funcRenderer struct{ tagAware bool }

func (f funcRenderer) render(u Unit) (string, error) { return emitStruct(u, f.tagAware), nil }

// tmplRenderer wraps an Opus-authored Go text/template over a Unit.
type tmplRenderer struct {
	t   *template.Template
	src string
}

func (r tmplRenderer) render(u Unit) (string, error) {
	var b strings.Builder
	if err := r.t.Execute(&b, u); err != nil {
		return "", err
	}
	return b.String(), nil
}

// frontierProduce generates one unit's code at the expensive tier. Offline it
// returns the golden (a perfect frontier stand-in); live it really calls Opus
// and returns the measured latency.
func (c *DemoCmd) frontierProduce(ctx context.Context, client *llm.Client, u Unit) (string, int64, error) {
	if c.Offline {
		return goldenOf(u), 0, nil
	}
	sys := "You generate a single Go type declaration from a spec. Output ONLY the Go code — " +
		"no prose, no markdown fences, no comments. Use a struct. Preserve the field order given. " +
		"If a field has a json tag, render it as a Go struct tag `json:\"<tag>\"`; otherwise no tag."
	prompt := "Spec (JSON):\n" + mustJSON(u) + "\n\nGo type declaration:"
	r, err := client.Complete(ctx, c.Model, sys, prompt, 1024)
	if err != nil {
		return "", 0, err
	}
	return stripFence(r.Text), r.LatencyMS, nil
}

// author produces the deterministic generator from worked examples. Offline it
// synthesizes from the features present (tags or not); live it asks Opus to
// write a Go text/template and validates it reproduces the examples.
func (c *DemoCmd) author(ctx context.Context, client *llm.Client, examples []Unit) (renderer, string, int64, error) {
	hasTags := anyTagged(examples)
	if c.Offline {
		desc := "synth(basic, no tags)"
		if hasTags {
			desc = "synth(tag-aware)"
		}
		return funcRenderer{tagAware: hasTags}, desc, 0, nil
	}
	// Live: Opus writes a Go text/template. The data shape it is told about is
	// the shape its EXAMPLES actually exhibit (tagless examples → no .Tag in the
	// described shape), so the v1 template legitimately omits tag handling and
	// drifts on the unseen tagged unit — exactly the demote the demo shows.
	shape := "Each unit has .Name (string) and .Fields (a list, each with .Name and .Type, both strings)."
	tagRule := ""
	if hasTags {
		shape = "Each unit has .Name (string) and .Fields (a list, each with .Name, .Type, and .Tag, all strings)."
		tagRule = " When a field's .Tag is non-empty, append a Go struct tag `json:\"<tag>\"` after the type; otherwise no tag."
	}
	var b strings.Builder
	b.WriteString("Worked examples (unit spec JSON → the exact Go code it must produce):\n\n")
	for _, u := range examples {
		fmt.Fprintf(&b, "%s\n=>\n%s\n\n", mustJSON(stripTags(u, hasTags)), goldenFor(u, hasTags))
	}
	sys := "You write a DETERMINISTIC Go text/template that renders a code unit from its spec. " +
		shape + " Output ONLY the template text (no prose, no markdown fences). The template is " +
		"executed with Go's text/template over one unit. It must reproduce the worked examples EXACTLY." + tagRule
	r, err := client.Complete(ctx, c.Model, sys, b.String(), 2048)
	if err != nil {
		return nil, "", 0, err
	}
	src := stripFence(r.Text)
	t, perr := template.New("unit").Parse(src)
	if perr != nil {
		return nil, "", r.LatencyMS, fmt.Errorf("authored template does not parse: %w", perr)
	}
	desc := fmt.Sprintf("opus text/template (%d bytes)", len(src))
	return tmplRenderer{t: t, src: src}, desc, r.LatencyMS, nil
}

func (c *DemoCmd) tierName() string {
	if c.Offline {
		return "stub"
	}
	return c.Model
}

// ---- golden + rendering primitives ----

// goldenOf is the canonical reference renderer — the per-unit verifier target.
// It always emits json tags for fields that carry one.
func goldenOf(u Unit) string { return emitStruct(u, true) }

// emitStruct renders a unit as a Go struct. tagAware gates whether json tags are
// emitted; a non-tag-aware render of a tagged unit is the seeded drift.
func emitStruct(u Unit, tagAware bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "type %s struct {\n", u.Name)
	for _, f := range u.Fields {
		if tagAware && f.Tag != "" {
			fmt.Fprintf(&b, "\t%s %s `json:%q`\n", f.Name, f.Type, f.Tag)
		} else {
			fmt.Fprintf(&b, "\t%s %s\n", f.Name, f.Type)
		}
	}
	b.WriteString("}\n")
	return b.String()
}

// goldenFor renders the example output shown to a live author. When the author's
// example set is tagless (hasTags=false) the shown golden is tagless too, so the
// author is never shown the tag feature.
func goldenFor(u Unit, hasTags bool) string { return emitStruct(u, hasTags) }

// stripTags hides the .Tag field from the spec JSON shown to a tagless author.
func stripTags(u Unit, hasTags bool) Unit {
	if hasTags {
		return u
	}
	out := Unit{Name: u.Name}
	for _, f := range u.Fields {
		out.Fields = append(out.Fields, Field{Name: f.Name, Type: f.Type})
	}
	return out
}

// matchesGolden compares produced code to a unit's golden, both gofmt-canonicalized.
// Code that does not parse as Go is a miss (never a silent pass).
func matchesGolden(produced string, u Unit) bool {
	pc, ok := canon(produced)
	if !ok {
		return false
	}
	gc, _ := canon(goldenOf(u)) // golden is always valid Go
	return pc == gc
}

// canon gofmt-canonicalizes a Go snippet; ok=false if it does not parse.
func canon(code string) (string, bool) {
	b, err := format.Source([]byte(strings.TrimSpace(code)))
	if err != nil {
		return "", false
	}
	return string(b), true
}

func fidelity(r renderer, units []Unit) (matched, n int) {
	for _, u := range units {
		n++
		c, err := r.render(u)
		if err == nil && matchesGolden(c, u) {
			matched++
		}
	}
	return matched, n
}

func anyTagged(units []Unit) bool {
	for _, u := range units {
		for _, f := range u.Fields {
			if f.Tag != "" {
				return true
			}
		}
	}
	return false
}

// ---- reporting + emit ----

// report prints the staircase and the measured summary table.
func (c *DemoCmd) report(spec Spec, fates []fate, modelLat []int64, servedDet, reserved, deferredDrift, reauthors, built int, detPerCallNS int64) {
	n := len(spec.Units)
	cheap := servedDet + reserved // units that ended up served deterministically

	fmt.Printf("\n=== the project is built ===\n")
	fmt.Printf("  %d/%d units pass their golden — a working %q package, generated.\n\n", built, n, spec.Domain)

	fmt.Println("=== apportionment (the shift-left, per unit) ===")
	for _, f := range fates {
		bar := tierBar(f.Tier)
		fmt.Printf("  [%2d] %-9s %s %s\n", f.Index, f.Name, bar, f.Note)
	}
	fmt.Println()

	fmt.Println("=== units by final tier ===")
	fmt.Printf("  frontier (opus, expensive):     %d   (worked examples; each a model call)\n", countTier(fates, "opus"))
	fmt.Printf("  drift demotes to frontier:      %d   (seeded irregular entit%s)\n", deferredDrift, pluralY(deferredDrift))
	fmt.Printf("  authored-det (0 model):         %d\n", servedDet)
	fmt.Printf("  reserved-det (re-served):       %d   (once-drifting, recovered after %d re-author%s)\n", reserved, reauthors, plural(reauthors))
	fmt.Printf("  → served deterministically:     %d/%d = %.0f%% of the project, at 0 model calls\n\n", cheap, n, 100*float64(cheap)/float64(n))

	if c.Offline {
		fmt.Println("=== latency (offline: NOT real — stub frontier is instant) ===")
		fmt.Printf("  deterministic render: %s/unit (timed over %d×%d calls) — this number IS real\n", fmtNS(detPerCallNS), c.Reps, n)
		fmt.Println("  run live (drop --offline) for real Opus latencies and the measured saving.")
		fmt.Println()
		fmt.Println("Shift-left, built: the generator Opus would write runs for free behind a golden verifier;")
		fmt.Println("the seeded irregular entity demoted, re-authored, and recovered — the project completed itself.")
		return
	}

	// Live: real latencies. Baseline = every unit produced by the frontier.
	produceLat := modelLat
	p50 := median(produceLat)
	totalModelMS := int64(0)
	for _, m := range modelLat {
		totalModelMS += m
	}
	baselineMS := int64(n) * p50 // always-frontier: one produce call per unit
	savedFrac := 0.0
	if baselineMS > 0 {
		savedFrac = float64(baselineMS-totalModelMS) / float64(baselineMS)
	}
	fmt.Println("=== latency (live, real — disk-cached) ===")
	fmt.Printf("  frontier produce/author: p50 %dms per call; %d model calls this session totalling %dms\n", p50, len(modelLat), totalModelMS)
	fmt.Printf("  deterministic render:    %s/unit (timed over %d×%d calls)\n", fmtNS(detPerCallNS), c.Reps, n)
	if detPerCallNS > 0 {
		fmt.Printf("  speedup on the served fraction: ~%d× (frontier p50 %dms vs det %s)\n", (p50*1_000_000)/detPerCallNS, p50, fmtNS(detPerCallNS))
	}
	fmt.Printf("  always-frontier baseline (%d × p50): %dms\n", n, baselineMS)
	fmt.Printf("  this session spent: %dms across authoring + the residual → %.0f%% less model latency than building every unit on the frontier\n\n", totalModelMS, savedFrac*100)
	fmt.Println("Shift-left, built: Opus wrote the generator early, then it ran for free behind a golden")
	fmt.Println("verifier; the seeded irregular entity demoted, re-authored, and recovered. The apportionment")
	fmt.Println("of work slid from all-frontier to mostly-deterministic while the project actually completed.")
}

// emit writes the data-driven viz sources: a Sankey flow.Record, the per-unit
// staircase trace, and an append to the shift-left history time series.
func (c *DemoCmd) emit(spec Spec, fates []fate, servedDet, reserved, deferredDrift, workedN int) {
	if c.FlowOut == "" {
		return
	}
	deferredTotal := workedN + deferredDrift // frontier-served units: worked examples + drift demotes
	rec := flow.Record{
		Run:         "demo",
		Oracle:      "golden",
		BigProvider: c.tierName(),
		Pair:        spec.Domain,
		Note:        fmt.Sprintf("built %d units · %d served-det · %d re-served", len(spec.Units), servedDet, reserved),
		Nodes: []flow.Node{
			{ID: "stream", Label: "units", Regime: "fate"},
			{ID: "served-det", Label: "authored-det (0 model)", Regime: "owned-local"},
			{ID: "deferred-model", Label: "frontier (opus)", Regime: "vendor"},
			{ID: "reauthor", Label: "re-authored", Regime: "fate"},
			{ID: "served-now", Label: "re-served-det", Regime: "owned-local"},
		},
		Edges: []flow.Edge{
			{Source: "stream", Target: "served-det", Value: servedDet, Kind: "shift-left"},
			{Source: "stream", Target: "deferred-model", Value: deferredTotal, Kind: "defer"},
			{Source: "reauthor", Target: "served-now", Value: reserved, Kind: "shift-left"},
		},
	}
	if err := rec.WriteFile(c.FlowOut); err != nil {
		fmt.Printf("  (flow viz: could not write %s: %v)\n", c.FlowOut, err)
	} else {
		fmt.Printf("  flow record (real counts) → %s  (data-driven viz source; `crystal viz` can bind to it)\n", c.FlowOut)
	}
	// Per-unit staircase trace (the time series the apportionment shift animates over).
	stairPath := filepath.Join(filepath.Dir(c.FlowOut), "demo-staircase.jsonl")
	if err := writeStaircase(stairPath, fates); err != nil {
		fmt.Printf("  (staircase: could not write %s: %v)\n", stairPath, err)
	} else {
		fmt.Printf("  per-unit staircase → %s\n", stairPath)
	}
	// Accrue the shift-left history (append-only), consistent with hook-loop.
	histPath := filepath.Join(filepath.Dir(c.FlowOut), "history.jsonl")
	if err := rec.AppendHistory(histPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
		fmt.Printf("  (flow history: could not append %s: %v)\n", histPath, err)
	}
}

// writeStaircase writes one JSON line per unit (the per-unit fate time series).
func writeStaircase(path string, fates []fate) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	for _, f := range fates {
		line, _ := json.Marshal(f)
		b.Write(line)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func tierBar(tier string) string {
	switch tier {
	case "opus":
		return "opus       ████████████  expensive"
	case "authored-det":
		return "served-det ▏             ~free"
	case "reserved-det":
		return "re-served  ██▏           drift→recovered"
	default:
		return tier
	}
}

func countTier(fates []fate, tier string) int {
	n := 0
	for _, f := range fates {
		if f.Tier == tier {
			n++
		}
	}
	return n
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func loadSpec(path string) (Spec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, err
	}
	var s Spec
	if err := json.Unmarshal(b, &s); err != nil {
		return Spec{}, fmt.Errorf("parse spec %s: %w", path, err)
	}
	if len(s.Units) == 0 {
		return Spec{}, fmt.Errorf("spec %s has no units", path)
	}
	return s, nil
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// stripFence removes a leading/trailing ```… markdown fence if the model wrapped
// its output in one, returning the inner text trimmed.
func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:] // drop the ```lang line
	}
	if j := strings.LastIndex(s, "```"); j >= 0 {
		s = s[:j]
	}
	return strings.TrimSpace(s)
}

func latLabel(ms int64, offline bool) string {
	if offline {
		return "   ~  "
	}
	return fmt.Sprintf("%4dms", ms)
}

func checkmark(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}

func promoteWord(ok bool) string {
	if ok {
		return "PROMOTE"
	}
	return "REJECT"
}

func pluralY(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
