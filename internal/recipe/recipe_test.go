package recipe

import "testing"

func base() Recipe {
	return Recipe{
		Name:     "entity-to-struct",
		Rung:     Code,
		Intent:   "entity spec → Go struct declaration",
		Inputs:   []Field{{Name: "name", Type: "string"}, {Name: "fields", Type: "list"}},
		Output:   Field{Name: "decl", Type: "string"},
		Body:     "template render",
		Executor: None,
		Verifier: "gofmt",
	}
}

func TestValidCodeRecipe(t *testing.T) {
	if iss := base().Validate(); len(iss) != 0 {
		t.Fatalf("expected valid, got issues: %v", iss)
	}
}

func TestCodeRungMustBeDeterministic(t *testing.T) {
	r := base()
	r.Executor = Haiku // a code rung run by a model is a contradiction
	if r.Valid() {
		t.Fatalf("code rung with a model executor must be invalid")
	}
}

func TestCodeRungNeedsDeterministicVerifier(t *testing.T) {
	r := base()
	r.Verifier = "judge" // a model-judge can't carry correctness for a no-model rung
	if r.Valid() {
		t.Fatalf("code rung gated by a non-deterministic verifier must be invalid")
	}
}

func TestPlanRungNeedsModelExecutor(t *testing.T) {
	r := base()
	r.Rung = Plan
	r.Verifier = "plancheck"
	r.Executor = None // a plan is run by a model
	if r.Valid() {
		t.Fatalf("plan rung with executor none must be invalid")
	}
}

func TestPlanRungUngatedFlagged(t *testing.T) {
	r := base()
	r.Rung = Plan
	r.Executor = Opus
	r.Verifier = "none"
	if r.Valid() {
		t.Fatalf("ungated plan (verifier none) must be flagged")
	}
}

func TestDegreesOfFreedomPenalizesLooseTypes(t *testing.T) {
	narrow := base() // 2 typed inputs → dof 2
	loose := base()
	loose.Inputs = []Field{{Name: "query", Type: "any"}} // {query: any} smell
	if narrow.DegreesOfFreedom() >= loose.DegreesOfFreedom() {
		t.Fatalf("a {query: any} signature must score worse (higher dof) than {name,fields}: narrow=%d loose=%d",
			narrow.DegreesOfFreedom(), loose.DegreesOfFreedom())
	}
}

func TestLadderOrdersByBurden(t *testing.T) {
	rs := []Recipe{{Rung: Code}, {Rung: Plan}, {Rung: Pseudocode}}
	Ladder(rs)
	if rs[0].Rung != Plan || rs[2].Rung != Code {
		t.Fatalf("ladder should run Plan→…→Code, got %v", []Rung{rs[0].Rung, rs[1].Rung, rs[2].Rung})
	}
}

func TestUnknownTypeFlagged(t *testing.T) {
	r := base()
	r.Inputs = []Field{{Name: "x", Type: "widget"}}
	if r.Valid() {
		t.Fatalf("an input with an unknown type must be flagged")
	}
}
