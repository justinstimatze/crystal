package cmd

import "testing"

// TestGoldenSelfConsistency: every unit's golden is valid Go that matches itself.
func TestGoldenSelfConsistency(t *testing.T) {
	spec, err := loadSpec("../testdata/demo/entities.json")
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if len(spec.Units) != 12 {
		t.Fatalf("expected 12 units, got %d", len(spec.Units))
	}
	for _, u := range spec.Units {
		if !matchesGolden(goldenOf(u), u) {
			t.Errorf("golden of %q does not match itself (gofmt parse failure?)", u.Name)
		}
	}
}

// TestDriftMechanism is the load-bearing assertion: a generator authored from
// tagless examples (v1) reproduces tagless units but MISSES a tagged (drift)
// unit, and a re-authored tag-aware generator (v2) recovers it. If v1 silently
// passed the tagged unit, the demo's demote/re-author would never fire.
func TestDriftMechanism(t *testing.T) {
	spec, err := loadSpec("../testdata/demo/entities.json")
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	var regular, drift *Unit
	for i := range spec.Units {
		u := &spec.Units[i]
		if u.Drift && drift == nil {
			drift = u
		}
		if !u.Drift && regular == nil {
			regular = u
		}
	}
	if regular == nil || drift == nil {
		t.Fatal("spec must contain at least one regular and one drift unit")
	}

	v1 := funcRenderer{tagAware: false}
	r1, _ := v1.render(*regular)
	if !matchesGolden(r1, *regular) {
		t.Errorf("v1 should reproduce the regular unit %q", regular.Name)
	}
	d1, _ := v1.render(*drift)
	if matchesGolden(d1, *drift) {
		t.Errorf("v1 must NOT reproduce the drift unit %q — drift would never fire", drift.Name)
	}

	v2 := funcRenderer{tagAware: true}
	d2, _ := v2.render(*drift)
	if !matchesGolden(d2, *drift) {
		t.Errorf("v2 (re-authored, tag-aware) must recover the drift unit %q", drift.Name)
	}
}

// TestOfflineBuildCompletes runs the whole offline staircase and asserts it
// completes without error (the key-free CI path the demo promises).
func TestOfflineBuildCompletes(t *testing.T) {
	c := &DemoCmd{
		Spec:      "../testdata/demo/entities.json",
		FirstK:    5,
		Threshold: 0.95,
		Offline:   true,
		Reps:      10,
		FlowOut:   t.TempDir() + "/flow.json",
	}
	if err := c.Run(); err != nil {
		t.Fatalf("offline demo run failed: %v", err)
	}
}

// TestAnyTagged distinguishes the example feature the author generalizes from.
func TestAnyTagged(t *testing.T) {
	tagless := []Unit{{Name: "A", Fields: []Field{{Name: "X", Type: "int"}}}}
	tagged := []Unit{{Name: "B", Fields: []Field{{Name: "Y", Type: "string", Tag: "y"}}}}
	if anyTagged(tagless) {
		t.Error("tagless example set reported as tagged")
	}
	if !anyTagged(tagged) {
		t.Error("tagged example set reported as tagless")
	}
}
