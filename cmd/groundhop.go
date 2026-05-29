package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinstimatze/crystal/internal/artifact"
	"github.com/justinstimatze/crystal/internal/compare"
	"github.com/justinstimatze/crystal/internal/corpus"
	"github.com/justinstimatze/crystal/internal/llm"
	"github.com/justinstimatze/crystal/internal/record"
)

// GroundHopCmd is the minimal grounding hop: it measures per-hop signal
// loss (λ) and guardrail coverage (g) on the REAL corpus by contrasting two
// up-channels carrying a supervisor's faithfulness signal:
//
//	det   — crystal's deterministic typed comparator (no LLM)
//	prose — a cheaper model's prose diff summary, judged by the supervisor
//
// against a reference (full-signal) supervisor verdict, on a population with
// GROUND-TRUTH-BY-CONSTRUCTION labels (we inject known corruptions, so truth
// is not a hand-written gold or a model's vibe).
//
// This deliberately fixes the three failure modes that invalidated the prior
// `experiment` run (see docs/EXPERIMENT_FINDINGS.md):
//  1. Labels are ground truth by construction (injected, known corruptions),
//     not exact-match against hand-paraphrased gold.
//  2. The up-channel carries VERIFIABLE CONTENT (a concrete diff / a typed
//     divergence record), never a self-graded verdict to rubber-stamp.
//  3. The full-signal and prose judges share the same model, question, and
//     bar — they differ ONLY in how much signal reaches them.
//
// Scope (stated, not hidden): for byte-exact tools the deterministic
// comparator covers ~all injected corruptions, so λ_det≈0 is EXPECTED and
// partly tautological. The result here is the CONTRAST (prose adds loss for
// no benefit on covered classes). The interesting λ — semantic / uncovered
// drift a comparator cannot judge — is OUT of scope for this minimal hop and
// is the next experiment. The full judge is itself a model; its own accuracy
// vs the hard labels is reported rather than treated as an oracle.
type GroundHopCmd struct {
	Corpus   string `help:"Corpus dir to draw real records from." default:"testdata/corpus"`
	Tool     string `help:"Tool whose records to use (must have a registered comparator)." default:"Bash"`
	CacheDir string `help:"Disk cache dir for LLM calls." default:".crystal-cache"`
	Verbose  bool   `help:"Dump per-item labels, channel verdicts, and prose summaries to diagnose scoring artifacts."`
}

type hopRow struct {
	idx           int
	corrupted     bool // ground-truth label (true = drift was injected)
	corruptor     string
	detFaithful   bool
	proseFaithful bool
	proseParsed   bool // false = judge returned empty/ambiguous text (invalid verdict)
	fullFaithful  bool
	fullParsed    bool
	prose         string
}

func (c *GroundHopCmd) Run() error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	recs, err := corpus.Load(c.Corpus)
	if err != nil {
		return usageError{fmt.Errorf("loading corpus %q: %w", c.Corpus, err)}
	}
	cmp, ok := compare.Lookup(c.Tool)
	if !ok {
		return usageError{fmt.Errorf("tool %q has no registered comparator — it is unverifiable", c.Tool)}
	}

	// Population: real records for the chosen tool.
	var pool []record.Record
	for _, r := range recs {
		if r.Tool == c.Tool {
			pool = append(pool, r)
		}
	}
	if len(pool) < 30 {
		return usageError{fmt.Errorf("only %d %s records in corpus; need >=30 for a meaningful hop", len(pool), c.Tool)}
	}

	// Corruptors targeting this tool — the source of ground-truth drift labels.
	var corruptors []artifact.Mutator
	for _, m := range artifact.Corruptors() {
		if m.Target() == c.Tool {
			corruptors = append(corruptors, m)
		}
	}
	if len(corruptors) == 0 {
		return usageError{fmt.Errorf("no corruptors target tool %q — cannot build a labeled drift population", c.Tool)}
	}

	ctx := context.Background()
	var rows []hopRow

	for i, r := range pool {
		var produced record.Output
		var label bool
		var corruptorName string

		// Deterministic split: odd indices are corrupt-slots. A corrupt-slot
		// is only LABELED corrupted when the chosen corruptor actually fired
		// (Mutated==true) — otherwise produced==historical, so it is faithful.
		// This keeps labels truthful (no "corrupted" label on an untouched record).
		if i%2 == 1 {
			m := corruptors[(i/2)%len(corruptors)]
			if m.Mutated(r) {
				produced, _ = m.Produce(r)
				label = true
				corruptorName = m.Name()
			}
		}
		if !label {
			produced, _ = (artifact.Identity{}).Produce(r) // faithful reproduction
		}

		// Channel 1 (det): deterministic typed comparator, no LLM.
		detV := cmp.Compare(produced, r.Result)

		// Channel 3 (full): supervisor judges from the FULL signal.
		full, fullParsed := judgeFull(ctx, client, r.Result, produced)

		// Channel 2 (prose): cheaper model writes a concrete diff summary
		// (verifiable content, NOT a verdict); supervisor judges from it alone.
		prose := proseDiff(ctx, client, r.Result, produced)
		proseFaithful, proseParsed := judgeProse(ctx, client, prose)

		rows = append(rows, hopRow{
			idx:           i,
			corrupted:     label,
			corruptor:     corruptorName,
			detFaithful:   detV.Match,
			proseFaithful: proseFaithful,
			proseParsed:   proseParsed,
			fullFaithful:  full,
			fullParsed:    fullParsed,
			prose:         prose,
		})
	}

	report(rows, c.Verbose)
	return nil
}

// confusion holds counts for one channel against the ground-truth labels.
// "drift" is the positive class; a channel "predicts drift" when !faithful.
type confusion struct{ tp, fp, tn, fn int }

func (m *confusion) add(predictDrift, isDrift bool) {
	switch {
	case predictDrift && isDrift:
		m.tp++
	case predictDrift && !isDrift:
		m.fp++
	case !predictDrift && isDrift:
		m.fn++
	default:
		m.tn++
	}
}
func (m confusion) n() int { return m.tp + m.fp + m.tn + m.fn }
func (m confusion) recall() float64 {
	if m.tp+m.fn == 0 {
		return 0
	}
	return float64(m.tp) / float64(m.tp+m.fn)
}
func (m confusion) precision() float64 {
	if m.tp+m.fp == 0 {
		return 0
	}
	return float64(m.tp) / float64(m.tp+m.fp)
}
func (m confusion) accuracy() float64 {
	if m.n() == 0 {
		return 0
	}
	return float64(m.tp+m.tn) / float64(m.n())
}

func report(rows []hopRow, verbose bool) {
	var det, prose, full confusion
	corrupted, parseFail := 0, 0
	// λ is only defined where BOTH the full (reference) and the compared
	// channel produced a parseable verdict. We count over that valid subset
	// and surface any parse failures loudly — an empty/ambiguous verdict is a
	// broken instrument, never a silent classification (the bug that sank run 1).
	disagreeDet, validDet := 0, 0
	disagreeProse, validProse := 0, 0
	for _, r := range rows {
		if r.corrupted {
			corrupted++
		}
		if !r.fullParsed || !r.proseParsed {
			parseFail++
		}
		// Confusion vs hard labels: only score parseable verdicts (det is
		// always parseable — pure Go).
		det.add(!r.detFaithful, r.corrupted)
		if r.proseParsed {
			prose.add(!r.proseFaithful, r.corrupted)
		}
		if r.fullParsed {
			full.add(!r.fullFaithful, r.corrupted)
		}
		if r.fullParsed {
			validDet++
			if r.detFaithful != r.fullFaithful {
				disagreeDet++
			}
			if r.proseParsed {
				validProse++
				if r.proseFaithful != r.fullFaithful {
					disagreeProse++
				}
			}
		}
	}
	n := len(rows)
	lambdaDet := safeRatio(disagreeDet, validDet)
	lambdaProse := safeRatio(disagreeProse, validProse)

	if parseFail > 0 {
		fmt.Printf("⚠ INSTRUMENT WARNING: %d/%d rows had an unparseable LLM verdict (empty/ambiguous).\n", parseFail, n)
		fmt.Println("  λ is computed only over rows with a parseable reference verdict; treat as suspect if this count is high.")
		fmt.Println()
	}

	if verbose {
		fmt.Println("=== per-item (label | det / prose / full) ===")
		for _, r := range rows {
			lab := "faithful"
			if r.corrupted {
				lab = "DRIFT(" + r.corruptor + ")"
			}
			fmt.Printf("  %2d %-22s det=%-5v prose=%-5s full=%-5s | prose=%q\n",
				r.idx, lab, r.detFaithful, verdictStr(r.proseFaithful, r.proseParsed), verdictStr(r.fullFaithful, r.fullParsed), truncate(r.prose, 70))
		}
		fmt.Println()
	}

	fmt.Printf("population: N=%d (%d injected-drift, %d faithful)\n\n", n, corrupted, n-corrupted)

	fmt.Println("=== guardrail coverage g (deterministic channel, no API) ===")
	fmt.Printf("  g = det recall on injected drift = %d/%d = %.2f\n",
		det.tp, corrupted, det.recall())
	fmt.Printf("  det vs labels: acc=%.2f precision=%.2f recall=%.2f\n\n",
		det.accuracy(), det.precision(), det.recall())

	fmt.Println("=== per-hop signal loss λ (channel verdict vs full-signal verdict, parseable rows only) ===")
	fmt.Printf("  λ_det   (full vs deterministic typed channel) = %d/%d = %.2f\n", disagreeDet, validDet, lambdaDet)
	fmt.Printf("  λ_prose (full vs prose-summary channel)       = %d/%d = %.2f\n", disagreeProse, validProse, lambdaProse)
	fmt.Printf("  contrast: prose channel loses %+.2f vs the deterministic channel\n\n", lambdaProse-lambdaDet)

	fmt.Println("=== each channel's own accuracy vs the hard labels ===")
	fmt.Printf("  %-6s acc=%.2f precision=%.2f recall=%.2f\n", "det", det.accuracy(), det.precision(), det.recall())
	fmt.Printf("  %-6s acc=%.2f precision=%.2f recall=%.2f\n", "prose", prose.accuracy(), prose.precision(), prose.recall())
	fmt.Printf("  %-6s acc=%.2f precision=%.2f recall=%.2f  (reference judge — a model, not an oracle)\n",
		"full", full.accuracy(), full.precision(), full.recall())

	fmt.Println("\nNOTE: for a byte-exact tool the deterministic comparator covers ~all injected")
	fmt.Println("corruptions, so λ_det≈0 is EXPECTED and partly tautological. The finding is the")
	fmt.Println("CONTRAST and whether the prose channel loses signal the typed channel keeps.")
	fmt.Println("Verify these numbers against --verbose per-item output before recording any as a finding.")
}

// --- channel implementations ---

const judgeBar = `Reply with EXACTLY ONE WORD and nothing else: FAITHFUL if the worker output faithfully reproduces the reference (ignoring volatile spans like timestamps, tmp paths, pids, hex addresses), or DRIFT if it differs substantively (missing/changed/extra lines, changed numbers, changed exit or interrupt state, error-vs-success flip).`

// judgeFull and judgeProse return (faithful, parsed). parsed=false means the
// model produced empty/ambiguous text — an invalid verdict the caller must
// NOT silently treat as a class. They use Classify (thinking disabled) so the
// one-word answer is never starved by thinking tokens.
func judgeFull(ctx context.Context, c *llm.Client, historical, produced record.Output) (bool, bool) {
	sys := "You compare a worker's tool output against a reference. " + judgeBar
	p := fmt.Sprintf("REFERENCE:\n%s\n\nWORKER:\n%s", renderOutput(historical), renderOutput(produced))
	r, err := c.Classify(ctx, llm.ModelOpus, sys, p, 16)
	if err != nil {
		return false, false
	}
	return parseVerdict(r.Text)
}

func proseDiff(ctx context.Context, c *llm.Client, historical, produced record.Output) string {
	sys := `You compare a worker's tool output against a reference. In 25 words or fewer, describe CONCRETELY how the worker output differs from the reference (missing/changed/extra lines, changed numbers, changed exit/interrupt state), or say "identical" if they match. Report only observable differences. Do NOT give a verdict on correctness.`
	p := fmt.Sprintf("REFERENCE:\n%s\n\nWORKER:\n%s", renderOutput(historical), renderOutput(produced))
	r, err := c.Complete(ctx, llm.ModelHaiku, sys, p, 60)
	if err != nil {
		return "(summary unavailable)"
	}
	return strings.TrimSpace(r.Text)
}

func judgeProse(ctx context.Context, c *llm.Client, prose string) (bool, bool) {
	sys := "A reviewer described how a worker's tool output differs from a reference. Based ONLY on this difference report, " + judgeBar
	r, err := c.Classify(ctx, llm.ModelOpus, sys, "Difference report: "+prose, 16)
	if err != nil {
		return false, false
	}
	return parseVerdict(r.Text)
}

// parseVerdict returns (faithful, parsed). It looks for the keywords anywhere
// (robust to stray punctuation/whitespace) but treats BOTH-present or
// NEITHER-present as a parse failure rather than guessing — an unparseable
// verdict is a broken instrument, surfaced loudly, never a silent default.
func parseVerdict(text string) (bool, bool) {
	up := strings.ToUpper(text)
	hasF := strings.Contains(up, "FAITHFUL")
	hasD := strings.Contains(up, "DRIFT")
	switch {
	case hasF && !hasD:
		return true, true
	case hasD && !hasF:
		return false, true
	default:
		return false, false // empty or ambiguous
	}
}

func safeRatio(num, den int) float64 {
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}

func verdictStr(faithful, parsed bool) string {
	if !parsed {
		return "??"
	}
	if faithful {
		return "true"
	}
	return "false"
}

// renderOutput produces a bounded text view of an Output for an LLM channel:
// the error envelope when it is an error, else stdout plus stderr/interrupt
// signal. Bounded so token cost stays controlled.
func renderOutput(o record.Output) string {
	if o.IsError {
		return "ERROR: " + truncate(o.Scalar, 1500)
	}
	var b strings.Builder
	b.WriteString(truncate(o.Stdout, 1500))
	if o.Stderr != "" {
		b.WriteString("\n[stderr] " + truncate(o.Stderr, 400))
	}
	if o.Interrupted {
		b.WriteString("\n[interrupted]")
	}
	s := b.String()
	if s == "" {
		return "(empty output)"
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
