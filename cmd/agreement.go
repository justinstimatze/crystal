package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
	"github.com/justinstimatze/crystal/internal/local"
	"github.com/justinstimatze/crystal/internal/publicai"
)

// agreement.go is the reusable all-local label oracle: two independent local
// models classify the same command, and AGREEMENT is the trust signal (validated
// at N=250 — coverage 0.74, accuracy-on-agree 0.87, > both solo, with errors
// concentrated in the disagreeing set; see A5_PROBE_FINDINGS.md). It is the piece
// `hook-loop` wires in to LABEL a drifted class with no cloud and no human —
// tri-training (Zhou & Li 2005) at N=2 feeding crystal's deterministic gate (the
// gate, not the agreement, is the novelty: agreement proposes, the gate verifies).

// classifySys is the shared system prompt every tier uses, so a local model and a
// cloud model are asked the SAME question — a precondition for agreement to mean
// anything (an apples-to-apples comparison, not a prompt-format artifact).
func classifySys(cats []string) string {
	return "Classify this shell command into EXACTLY ONE category, reply with only the category word: " +
		strings.Join(cats, ", ") + "."
}

// classifier labels one command over a category set, returning the parsed
// category and the real call latency. It abstracts a model's EXECUTOR and
// PLACEMENT (local ollama, cloud-open PublicAI, cloud-closed Anthropic) behind a
// single signature, so the agreement oracle can pair ANY two — e.g. a local 8B
// with a cloud-OPEN ~30-70B that has no VRAM-spill stall (the menu, not a ladder).
type classifier struct {
	name string // model id, for reporting
	fn   func(ctx context.Context, cats []string, cmd string) (string, int64, error)
}

// localClassifier wraps a local ollama model as a classifier.
func localClassifier(lc *local.Client, model string) classifier {
	return classifier{name: model, fn: func(ctx context.Context, cats []string, cmd string) (string, int64, error) {
		return localClassifyCats(ctx, lc, model, cats, cmd)
	}}
}

// publicaiClassifier wraps a Public AI Gateway (cloud-open) model as a classifier
// — the spill-free big-model option for the agreement pair.
func publicaiClassifier(pc *publicai.Client, model string) classifier {
	return classifier{name: model, fn: func(ctx context.Context, cats []string, cmd string) (string, int64, error) {
		if pc == nil {
			return "", 0, fmt.Errorf("publicaiClassifier: nil client (oracle big-provider=publicai needs setup it didn't get)")
		}
		r, err := pc.Classify(ctx, model, classifySys(cats), cmd, 16)
		if err != nil {
			return "", 0, err
		}
		return parseCategoryFrom(r.Text, cats), r.LatencyMS, nil
	}}
}

// agreementOf runs two classifiers over the category set and returns the agreed
// label when they MATCH (the trust signal), or ("", false) on disagreement (the
// honest abstention). A non-nil error is surfaced so the caller fails loud rather
// than treating an unreachable model as universal abstention. This is the
// placement-agnostic core; agreementLabel is the local+local convenience wrapper.
func agreementOf(ctx context.Context, a, b classifier, cats []string, cmd string) (label string, agreed bool, err error) {
	c1, _, err := a.fn(ctx, cats, cmd)
	if err != nil {
		return "", false, err
	}
	c2, _, err := b.fn(ctx, cats, cmd)
	if err != nil {
		return "", false, err
	}
	if c1 != "" && c1 == c2 {
		return c1, true, nil
	}
	return "", false, nil
}

// parseCategoryFrom matches a model's reply against a GIVEN category set (not just
// the compiled triageCategories), so a re-author over an expanded set can recognize
// a new class name like "container". Falls back to the raw lowercased text.
func parseCategoryFrom(text string, cats []string) string {
	got := strings.ToLower(strings.TrimSpace(text))
	for _, cat := range cats {
		if strings.Contains(got, cat) {
			return cat
		}
	}
	return got
}

// localClassifyCats classifies one command over an arbitrary category set and
// returns the parsed category + real latency (cached after first call). The
// thinking-free, temp-0 path in internal/local makes it deterministic.
func localClassifyCats(ctx context.Context, lc *local.Client, model string, cats []string, cmd string) (string, int64, error) {
	if lc == nil {
		return "", 0, fmt.Errorf("localClassifyCats: nil local client (an oracle mode needs local setup it didn't get)")
	}
	r, err := lc.Classify(ctx, model, classifySys(cats), cmd, 16)
	if err != nil {
		return "", 0, err
	}
	return parseCategoryFrom(r.Text, cats), r.LatencyMS, nil
}

// agreementLabel runs both local models over the category set and returns the
// agreed label when they MATCH (the trust signal), or ("", false) when they
// disagree — the honest abstention. A non-nil error (e.g. the box unreachable or
// a call timing out) is surfaced so the caller fails loud rather than silently
// treating an unreachable oracle as universal abstention.
func agreementLabel(ctx context.Context, lc *local.Client, m1, m2 string, cats []string, cmd string) (label string, agreed bool, err error) {
	return agreementOf(ctx, localClassifier(lc, m1), localClassifier(lc, m2), cats, cmd)
}

// cloudClassifyCats is the CONFIRM tier of the cascade: when the two local models
// DISAGREE (abstain), escalate just that command to a cloud model over the same
// expanded category set. This is the targeted-spend pattern (FrugalGPT/AutoMix):
// pay cloud only on the small uncertain slice the local agreement could not cover,
// not the whole class. parseCategoryFrom keeps the new class name recognizable.
func cloudClassifyCats(ctx context.Context, client *llm.Client, model string, cats []string, cmd string) (string, error) {
	r, err := client.Classify(ctx, model, classifySys(cats), cmd, 16)
	if err != nil {
		return "", err
	}
	return parseCategoryFrom(r.Text, cats), nil
}
