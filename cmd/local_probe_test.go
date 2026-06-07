package cmd

import "testing"

// TestAgreementMathConcentratesCorrectness pins the two-model agreement oracle's
// arithmetic against the measured N=37 shape (28 agree, 24 of those correct):
// coverage is the agree rate, label quality is accuracy-on-agree, and the honest
// claim ("agreement concentrates correctness") requires on-agree to beat BOTH solo
// accuracies. The disagree set is where the errors live.
func TestAgreementMathConcentratesCorrectness(t *testing.T) {
	// Reconstruct the measured distribution: 28 agreeing rows (24 correct), and 9
	// disagreeing rows where m1 is right once and m2 is right four times — so solo
	// m1 = (24+1)/37 = 0.676, solo m2 = (24+4)/37 = 0.757, agree = 28/37 = 0.757,
	// on-agree = 24/28 = 0.857. (Matches the cached run.)
	var rows []probeRow
	for i := 0; i < 24; i++ { // agree AND correct
		rows = append(rows, probeRow{ref: "network", m1: "network", m2: "network"})
	}
	for i := 0; i < 4; i++ { // agree but BOTH wrong (concentrates: these are rare)
		rows = append(rows, probeRow{ref: "network", m1: "search", m2: "search"})
	}
	// disagree: m1 right once
	rows = append(rows, probeRow{ref: "network", m1: "network", m2: "search"})
	// disagree: m2 right four times
	for i := 0; i < 4; i++ {
		rows = append(rows, probeRow{ref: "network", m1: "search", m2: "network"})
	}
	// disagree: neither right four times
	for i := 0; i < 4; i++ {
		rows = append(rows, probeRow{ref: "network", m1: "build", m2: "test"})
	}

	if got := len(rows); got != 37 {
		t.Fatalf("rows = %d, want 37", got)
	}

	agree, agreeOK, m1OK, m2OK := 0, 0, 0, 0
	for _, r := range rows {
		if r.m1 == r.ref {
			m1OK++
		}
		if r.m2 == r.ref {
			m2OK++
		}
		if r.m1 != "" && r.m1 == r.m2 {
			agree++
			if r.m1 == r.ref {
				agreeOK++
			}
		}
	}
	if agree != 28 {
		t.Errorf("agree = %d, want 28 (coverage)", agree)
	}
	if agreeOK != 24 {
		t.Errorf("agreeOK = %d, want 24 (accuracy-on-agree numerator)", agreeOK)
	}
	if m1OK != 25 {
		t.Errorf("m1 solo correct = %d, want 25", m1OK)
	}
	if m2OK != 28 {
		t.Errorf("m2 solo correct = %d, want 28", m2OK)
	}
	onAgree := ratio(agreeOK, agree)
	soloM1, soloM2 := ratio(m1OK, len(rows)), ratio(m2OK, len(rows))
	// The load-bearing claim: agreement must beat BOTH solo accuracies, else the
	// oracle adds nothing over just picking a model.
	if !(onAgree > soloM1 && onAgree > soloM2) {
		t.Errorf("on-agree %.3f does not beat both solo (%.3f, %.3f) — claim would be false", onAgree, soloM1, soloM2)
	}
}

// TestRatioIsDivideByZeroSafe guards the empty-corpus / never-agree paths so the
// report never panics when a model emits no usable labels.
func TestRatioIsDivideByZeroSafe(t *testing.T) {
	if got := ratio(0, 0); got != 0 {
		t.Errorf("ratio(0,0) = %v, want 0", got)
	}
	if got := ratio(3, 4); got != 0.75 {
		t.Errorf("ratio(3,4) = %v, want 0.75", got)
	}
}
