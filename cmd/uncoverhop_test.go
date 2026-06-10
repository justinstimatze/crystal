package cmd

import (
	"strings"
	"testing"
)

// TestUncoverHopConstruction validates the experiment's deterministic
// invariants BEFORE any tokens are spent — a mislabeled item would invalidate
// the run the way paraphrased gold invalidated `experiment`. It asserts:
//   - every gold field is a substring of its source (faithful is grounded → no
//     det-grounded false positive),
//   - tier-2's distractor IS in the source (survives det-grounded → uncovered),
//   - tier-1's unrelated value is NOT in the source (caught by det-grounded),
//   - a clean tier-1 value exists for every item.
func TestUncoverHopConstruction(t *testing.T) {
	items := exItems()
	for i, it := range items {
		gold := extract3{it.Name, it.Role, it.Org}

		if !detGrounded(it.Text, gold) {
			t.Errorf("item %d: a gold field is not a substring of the source — det-grounded would false-positive on faithful: %q", i, it.Text)
		}

		// tier-2 distractor must appear in the source AND differ from gold.
		if !strings.Contains(strings.ToLower(it.Text), strings.ToLower(it.DValue)) {
			t.Errorf("item %d: tier-2 distractor %q not present in source — it would NOT be uncovered", i, it.DValue)
		}
		if strings.EqualFold(it.DValue, gold.get(it.DField)) {
			t.Errorf("item %d: tier-2 distractor equals the gold value — not a corruption", i)
		}
		tier2 := gold
		tier2.set(it.DField, it.DValue)
		if !detGrounded(it.Text, tier2) {
			t.Errorf("item %d: tier-2 is caught by det-grounded — it must be the UNCOVERED residual", i)
		}

		// tier-1 unrelated value must exist and be caught by det-grounded.
		t1 := tier1Value(items, i)
		if t1 == "" {
			t.Errorf("item %d: no clean unrelated tier-1 value found", i)
			continue
		}
		tier1 := gold
		tier1.set(it.DField, t1)
		if detGrounded(it.Text, tier1) {
			t.Errorf("item %d: tier-1 value %q is in the source — det-grounded should catch it but won't", i, t1)
		}
	}
}
