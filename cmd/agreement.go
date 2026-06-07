package cmd

import (
	"context"
	"strings"

	"github.com/justinstimatze/crystal/internal/local"
)

// agreement.go is the reusable all-local label oracle: two independent local
// models classify the same command, and AGREEMENT is the trust signal (validated
// at N=250 — coverage 0.74, accuracy-on-agree 0.87, > both solo, with errors
// concentrated in the disagreeing set; see A5_PROBE_FINDINGS.md). It is the piece
// `hook-loop` wires in to LABEL a drifted class with no cloud and no human —
// tri-training (Zhou & Li 2005) at N=2 feeding crystal's deterministic gate (the
// gate, not the agreement, is the novelty: agreement proposes, the gate verifies).

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
	sys := "Classify this shell command into EXACTLY ONE category, reply with only the category word: " +
		strings.Join(cats, ", ") + "."
	r, err := lc.Classify(ctx, model, sys, cmd, 16)
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
	c1, _, err := localClassifyCats(ctx, lc, m1, cats, cmd)
	if err != nil {
		return "", false, err
	}
	c2, _, err := localClassifyCats(ctx, lc, m2, cats, cmd)
	if err != nil {
		return "", false, err
	}
	if c1 != "" && c1 == c2 {
		return c1, true, nil
	}
	return "", false, nil
}
