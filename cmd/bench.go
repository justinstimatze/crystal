package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/justinstimatze/crystal/internal/artifact"
	"github.com/justinstimatze/crystal/internal/compare"
	"github.com/justinstimatze/crystal/internal/corpus"
	"github.com/justinstimatze/crystal/internal/eval"
	"github.com/justinstimatze/crystal/internal/record"
)

// BenchCmd emits the single citable quantitative report the project was
// missing: the verifier gate's integrity over the committed corpus (the
// load-bearing claim — "no verifier, no crystallization"), plus the menu /
// displacement frame, each row honestly labelled by what reproducing its
// numbers requires.
//
// The OFFLINE block (gate integrity, determinism) needs no API key and no
// GPU — it is what a third party reproduces from `crystal bench` or in CI.
// The cascade and open-model rows need credentials and are measured by
// `crystal payoff` / `crystal serve` / `crystal publicai-probe`; bench
// reports their reproducibility class and whether the credential is present,
// never launders their findings-doc numbers as if measured here.
type BenchCmd struct {
	Corpus string `help:"Corpus dir to benchmark against." default:"testdata/corpus"`
	JSON   bool   `help:"Emit the report as JSON."`
}

// BenchReport is the structured, citable output.
type BenchReport struct {
	Corpus  CorpusStats `json:"corpus"`
	Gate    GateReport  `json:"gate"`
	Tiers   []TierRow   `json:"tiers"`
	Verdict string      `json:"verdict"` // PASS | FAIL
}

// CorpusStats describes what was benchmarked.
type CorpusStats struct {
	Records int        `json:"records"`
	Tools   []ToolStat `json:"tools"`
}

// ToolStat is one tool's cohort shape — including the input-group structure
// that decides whether a crystallizable unit exists at all.
type ToolStat struct {
	Tool           string  `json:"tool"`
	N              int     `json:"n"`
	UniqueInputs   int     `json:"unique_inputs"`
	RepeatedInputs int     `json:"repeated_inputs"`
	MaxGroup       int     `json:"max_group"`
	Fidelity       float64 `json:"identity_fidelity"` // exact-repro of the det tier
}

// GateReport is the verifier gate's measured sensitivity and specificity.
type GateReport struct {
	IdentityPromotes  bool             `json:"identity_promotes"`  // sanity precondition
	Determinism       float64          `json:"determinism"`        // min identity fidelity (exact-repro)
	Corruptors        []CorruptorScore `json:"corruptors"`         // per-corruptor catch
	CorruptorsTouched int              `json:"corruptors_touched"` // records a corruptor mutated
	CorruptorsEscaped int              `json:"corruptors_escaped"` // mutations the gate missed
	Sensitivity       float64          `json:"sensitivity"`        // (touched-escaped)/touched
	SpecificityPass   bool             `json:"specificity_pass"`   // benign volatility never false-alarms
	FalseAlarms       int              `json:"false_alarms"`
}

// CorruptorScore is one deliberate regression's catch rate.
type CorruptorScore struct {
	Name    string `json:"name"`
	Target  string `json:"target"` // tool ("" = any)
	Touched int    `json:"touched"`
	Escaped int    `json:"escaped"`
}

// TierRow is one rung of the executor × placement × openness menu, with the
// reproducibility class of its numbers.
type TierRow struct {
	Tier       string `json:"tier"`
	Placement  string `json:"placement"`
	Openness   string `json:"openness"`
	Measured   string `json:"measured"`    // what's measured here, or what to run
	ReproClass string `json:"repro_class"` // OFFLINE | anthropic-key | publicai-key | local-gpu
	Available  string `json:"available"`   // live readiness for this environment
}

func (c *BenchCmd) Run() error {
	recs, err := corpus.Load(c.Corpus)
	if err != nil {
		return usageError{fmt.Errorf("loading corpus %q: %w", c.Corpus, err)}
	}
	if len(recs) == 0 {
		return usageError{fmt.Errorf("corpus %q is empty — run `crystal synth-corpus` first", c.Corpus)}
	}

	rep := BenchReport{
		Corpus: corpusStats(recs),
		Gate:   gateReport(recs),
		Tiers:  tierRows(),
	}
	rep.Verdict = "FAIL"
	if rep.Gate.IdentityPromotes && rep.Gate.CorruptorsEscaped == 0 && rep.Gate.SpecificityPass {
		rep.Verdict = "PASS"
	}

	if c.JSON {
		b, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	fmt.Print(renderBench(rep))
	return nil
}

// corpusStats summarises the cohort shape per tool.
func corpusStats(recs []record.Record) CorpusStats {
	groups := eval.GroupByTool(recs)
	identity := map[string]float64{}
	for _, r := range eval.RunAll(artifact.Identity{}, recs) {
		identity[r.Tool] = r.Fidelity
	}
	tools := make([]string, 0, len(groups))
	for t := range groups {
		tools = append(tools, t)
	}
	sort.Strings(tools)
	cs := CorpusStats{Records: len(recs)}
	for _, t := range tools {
		cohort := groups[t]
		hist := eval.InputGroupHistogram(cohort)
		maxGroup, repeated := 0, 0
		for _, n := range hist {
			if n > maxGroup {
				maxGroup = n
			}
			if n > 1 {
				repeated++
			}
		}
		cs.Tools = append(cs.Tools, ToolStat{
			Tool: t, N: len(cohort), UniqueInputs: len(hist),
			RepeatedInputs: repeated, MaxGroup: maxGroup, Fidelity: identity[t],
		})
	}
	return cs
}

// gateReport measures sensitivity (every deliberate corruption caught) and
// specificity (benign volatility never false-alarms) — mirroring the Phase-1
// go/no-go test, surfaced as a citable report rather than a test log.
func gateReport(recs []record.Record) GateReport {
	groups := eval.GroupByTool(recs)

	g := GateReport{IdentityPromotes: true, Determinism: 1.0}
	for _, r := range eval.RunAll(artifact.Identity{}, recs) {
		if r.Decision != "promote" {
			g.IdentityPromotes = false
		}
		if r.Fidelity < g.Determinism {
			g.Determinism = r.Fidelity
		}
	}

	// Sensitivity: per corruptor, over the records it actually mutates, the
	// comparator must reject every one. Scope by target tool ("" = any).
	for _, corruptor := range artifact.Corruptors() {
		score := CorruptorScore{Name: corruptor.Name(), Target: corruptor.Target()}
		for tool, cohort := range groups {
			if corruptor.Target() != "" && corruptor.Target() != tool {
				continue
			}
			cmp, ok := compare.Lookup(tool)
			if !ok {
				continue
			}
			for _, r := range cohort {
				if !corruptor.Mutated(r) {
					continue
				}
				score.Touched++
				produced, _ := corruptor.Produce(r)
				if v := cmp.Compare(produced, r.Result); v.Match {
					score.Escaped++
				}
			}
		}
		g.CorruptorsTouched += score.Touched
		g.CorruptorsEscaped += score.Escaped
		g.Corruptors = append(g.Corruptors, score)
	}
	sort.Slice(g.Corruptors, func(i, j int) bool { return g.Corruptors[i].Name < g.Corruptors[j].Name })
	if g.CorruptorsTouched > 0 {
		g.Sensitivity = float64(g.CorruptorsTouched-g.CorruptorsEscaped) / float64(g.CorruptorsTouched)
	}

	// Specificity: benign volatility must promote on every tool.
	g.SpecificityPass = true
	for _, r := range eval.RunAll(artifact.BenignVolatility{}, recs) {
		if r.Decision != "promote" {
			g.SpecificityPass = false
			g.FalseAlarms += len(r.Divergences)
		}
	}
	return g
}

func tierRows() []TierRow {
	anthropic := keyState("ANTHROPIC_API_KEY")
	publicai := keyState("PUBLICAI_API_KEY")
	return []TierRow{
		{
			Tier: "code (deterministic)", Placement: "local", Openness: "—",
			Measured:   "gate integrity + determinism (this report)",
			ReproClass: "OFFLINE", Available: "✓ no credential",
		},
		{
			Tier: "Haiku", Placement: "someone's cloud", Openness: "proprietary",
			Measured:   "`crystal payoff` / `crystal serve` — cheap-tier accuracy + latency, gated",
			ReproClass: "anthropic-key", Available: anthropic,
		},
		{
			Tier: "Sonnet", Placement: "someone's cloud", Openness: "proprietary",
			Measured:   "`crystal payoff` — middle rung of the Opus→Sonnet→Haiku cascade",
			ReproClass: "anthropic-key", Available: anthropic,
		},
		{
			Tier: "Opus (frontier reference)", Placement: "someone's cloud", Openness: "proprietary",
			Measured:   "`crystal payoff` — always-Opus baseline the cascade is scored against",
			ReproClass: "anthropic-key", Available: anthropic,
		},
		{
			Tier: "open model, hosted (OLMo · Apertus)", Placement: "public gateway", Openness: "open-source",
			Measured:   "`crystal publicai-probe` — cloud-cheap open-model rung, no GPU",
			ReproClass: "publicai-key", Available: publicai,
		},
		{
			Tier: "local model (qwen 8B+35B agreement)", Placement: "your machine", Openness: "open-weight",
			Measured:   "`crystal local-probe` — last measured N=250 (author GPU); deferred while away from it",
			ReproClass: "local-gpu", Available: "deferred (no local GPU here)",
		},
	}
}

// keyState reports whether a credential is reachable (env or ./.env),
// without loading it into the process or printing any value.
func keyState(name string) string {
	if os.Getenv(name) != "" {
		return "✓ key present"
	}
	if dotEnvHasKey(".env", name) {
		return "✓ in .env"
	}
	return "✗ needs " + name
}

func dotEnvHasKey(path, name string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(line[len("export "):])
		}
		if k, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == name {
			return strings.TrimSpace(v) != ""
		}
	}
	return false
}

// renderBench formats the report as a human-readable markdown-ish block.
func renderBench(rep BenchReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "crystal bench — verifier gate + menu, over %d records\n\n", rep.Corpus.Records)

	// OFFLINE gate integrity — the citable, key-free core.
	fmt.Fprintf(&b, "GATE INTEGRITY (offline, reproducible by anyone)\n")
	fmt.Fprintf(&b, "  identity promotes      %s\n", passmark(rep.Gate.IdentityPromotes))
	fmt.Fprintf(&b, "  determinism            %.3f  (det tier reproduces the recorded output exactly)\n", rep.Gate.Determinism)
	fmt.Fprintf(&b, "  sensitivity            %.3f  (%d corruptions, %d escaped) %s\n",
		rep.Gate.Sensitivity, rep.Gate.CorruptorsTouched, rep.Gate.CorruptorsEscaped, passmark(rep.Gate.CorruptorsEscaped == 0))
	fmt.Fprintf(&b, "  specificity            %s  (benign volatility never false-alarms, %d alarms)\n",
		passmark(rep.Gate.SpecificityPass), rep.Gate.FalseAlarms)
	fmt.Fprintf(&b, "\n  per corruptor (deliberate single-field regressions, all must be caught):\n")
	for _, c := range rep.Gate.Corruptors {
		tgt := c.Target
		if tgt == "" {
			tgt = "any"
		}
		fmt.Fprintf(&b, "    %-18s target=%-6s touched=%-3d escaped=%d %s\n",
			c.Name, tgt, c.Touched, c.Escaped, passmark(c.Escaped == 0))
	}

	// Corpus shape.
	fmt.Fprintf(&b, "\nCORPUS SHAPE (per tool)\n")
	for _, t := range rep.Corpus.Tools {
		fmt.Fprintf(&b, "  %-6s N=%-3d uniqueInputs=%-3d repeated=%-3d maxGroup=%-3d identity=%.3f\n",
			t.Tool, t.N, t.UniqueInputs, t.RepeatedInputs, t.MaxGroup, t.Fidelity)
	}

	// Menu / displacement frame.
	fmt.Fprintf(&b, "\nMENU — displacement down the executor × placement × openness axes\n")
	fmt.Fprintf(&b, "  %-36s %-16s %-12s %-14s %s\n", "tier", "placement", "openness", "repro-class", "available here")
	fmt.Fprintf(&b, "  %s\n", strings.Repeat("─", 104))
	for _, r := range rep.Tiers {
		fmt.Fprintf(&b, "  %-36s %-16s %-12s %-14s %s\n", r.Tier, r.Placement, r.Openness, r.ReproClass, r.Available)
	}
	fmt.Fprintf(&b, "\n  measured by:\n")
	for _, r := range rep.Tiers {
		fmt.Fprintf(&b, "    %-36s %s\n", r.Tier, r.Measured)
	}

	fmt.Fprintf(&b, "\nVERDICT: %s", rep.Verdict)
	if rep.Verdict == "PASS" {
		fmt.Fprintf(&b, " — the gate catches every deliberate regression with no false alarms; "+
			"crystallization is safe to promote behind it.\n")
	} else {
		fmt.Fprintf(&b, " — a corruption escaped or benign volatility false-alarmed; STOP and rethink (the brief's go/no-go).\n")
	}
	return b.String()
}

func passmark(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}
