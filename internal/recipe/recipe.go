// Package recipe is crystal's crystallized-artifact SCHEMA: a recipe is a
// first-class, typed, diffable object — not a prose blob a strong model emits and
// a weak one parses ad hoc. Giving the artifact a schema is what makes it
// portable across executor tiers, lintable like code, and renderable as data.
//
// The design follows Manuel Odendahl's "Tool use and notation as shaping LLM
// generalization" (the.scapegoat.dev, 25 Feb 2026). His thesis is crystal's:
// "Using notation, tools or code don't make the model smarter. They make the task
// simpler." A recipe is a unit of that GENERALIZATION SHAPING — a re-representation
// of a chore so a weaker executor's RESIDUAL TASK is simpler. The schema encodes
// the two axes he tracks separately:
//
//   - Rung      = actuator expressivity: how powerful is the notation the executor
//                 targets (prose plan → stepwise recipe → pseudocode → code).
//   - Executor  = the model-burden floor: the WEAKEST tier that can still run the
//                 body. "The sweet spot ... is maximizing actuator expressivity
//                 while minimizing model burden."
//
// and the two things that make an interface easy to target:
//
//   - Inputs/Output = a NARROW WAIST: typed in/out, few degrees of freedom. "The
//                 model's search problem scales with the degrees of freedom at the
//                 interface, so minimize them."
//   - Verifier  = the deterministic machinery that "carries correctness" off the
//                 model, and Residual names what is deliberately LEFT to the model.
package recipe

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Rung is an artifact's position on the actuator-expressivity ladder. Each rung
// down re-represents the work so a weaker executor suffices.
type Rung string

const (
	Memory     Rung = "memory"     // the answer; no execution (degenerate — exact-repeat only)
	Plan       Rung = "plan"       // prose intent; highest model burden
	RecipeRung Rung = "recipe"     // stepwise + bounded judgment
	Pseudocode Rung = "pseudocode" // near-mechanical
	Code       Rung = "code"       // deterministic notation; no model at run time
)

// rungOrder ranks rungs by descending model burden (Plan hardest, Code none).
var rungOrder = map[Rung]int{Memory: 0, Plan: 4, RecipeRung: 3, Pseudocode: 2, Code: 1}

// Executor is the weakest tier that can run a recipe's body (the burden floor).
type Executor string

const (
	None      Executor = "none"       // deterministic — no model at run time
	LocalOpen Executor = "local-open" // an open-weight model on local/house compute
	Haiku     Executor = "haiku"      // a cheap frontier model
	Opus      Executor = "opus"       // the strong frontier model
)

// executorRank ranks executors weakest→strongest (None cheapest).
var executorRank = map[Executor]int{None: 0, LocalOpen: 1, Haiku: 2, Opus: 3}

// fieldTypes is the SMALL primitive vocabulary a narrow-waist signature may use.
// `any` and `json` are permitted but counted against the degrees of freedom —
// the {query: any} smell Odendahl warns is harder to target than {category,date}.
var fieldTypes = map[string]bool{
	"string": true, "int": true, "float": true, "bool": true,
	"list": true, "map": true, "table": true, "struct": true,
	"json": true, "any": true,
}

// looseTypes are the under-constrained types that widen the interface.
var looseTypes = map[string]bool{"any": true, "json": true, "map": true}

// verifierKinds is the vocabulary of correctness-carrying machinery. A recipe at
// a deterministic rung must name a deterministic verifier; the plan rung's
// verifier is plancheck (the seam to the gated-plan transfer harness).
var verifierKinds = map[string]bool{
	"none": true, "golden": true, "gofmt": true, "build": true,
	"go-test": true, "defn-test": true, "kong-contract": true,
	"plancheck": true, "judge": true,
}

// deterministicVerifier reports whether a verifier carries correctness without a
// model (so a Code/Pseudocode rung gated by it can truly drop to executor None).
var deterministicVerifier = map[string]bool{
	"golden": true, "gofmt": true, "build": true, "go-test": true,
	"defn-test": true, "kong-contract": true,
}

// Field is one typed slot of the narrow-waist signature.
type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Desc string `json:"desc,omitempty"`
}

// Recipe is a crystallized artifact as a typed, diffable object.
type Recipe struct {
	Name     string   `json:"name"`
	Rung     Rung     `json:"rung"`
	Intent   string   `json:"intent"`             // one line: the residual task this shapes
	Inputs   []Field  `json:"inputs"`             // narrow waist: typed in
	Output   Field    `json:"output"`             // narrow waist: typed out
	Body     string   `json:"body"`               // the notation at this rung
	Executor Executor `json:"executor"`           // weakest tier that can run Body
	Verifier string   `json:"verifier"`           // the machinery that carries correctness
	Residual string   `json:"residual,omitempty"` // what is deliberately LEFT to the model
}

// DegreesOfFreedom is the narrow-waist metric: the number of typed inputs, plus a
// penalty for each loosely-typed (any/json/map) field. Lower = narrower waist =
// easier for a weak executor to target. A signature of {query: any} scores worse
// than {category: string, date_range: list} even though it has fewer fields.
func (r Recipe) DegreesOfFreedom() int {
	dof := len(r.Inputs)
	for _, f := range r.Inputs {
		if looseTypes[strings.ToLower(f.Type)] {
			dof += 2 // an under-constrained field widens the search space
		}
	}
	if looseTypes[strings.ToLower(r.Output.Type)] {
		dof += 2
	}
	return dof
}

// Validate lints the recipe like code — the producer-verifier discipline applied
// to the artifact schema itself. It returns every problem (not just the first) so
// a lint pass surfaces the whole list.
func (r Recipe) Validate() []string {
	var issues []string
	if strings.TrimSpace(r.Name) == "" {
		issues = append(issues, "name is empty")
	}
	if strings.TrimSpace(r.Intent) == "" {
		issues = append(issues, "intent is empty (a recipe must name the residual task it shapes)")
	}
	if strings.TrimSpace(r.Body) == "" {
		issues = append(issues, "body is empty (no notation to run)")
	}
	if _, ok := rungOrder[r.Rung]; !ok {
		issues = append(issues, fmt.Sprintf("unknown rung %q", r.Rung))
	}
	if _, ok := executorRank[r.Executor]; !ok {
		issues = append(issues, fmt.Sprintf("unknown executor %q", r.Executor))
	}
	if !verifierKinds[r.Verifier] {
		issues = append(issues, fmt.Sprintf("unknown verifier %q", r.Verifier))
	}
	for _, f := range r.Inputs {
		if !fieldTypes[strings.ToLower(f.Type)] {
			issues = append(issues, fmt.Sprintf("input %q has unknown type %q", f.Name, f.Type))
		}
	}
	if r.Output.Type != "" && !fieldTypes[strings.ToLower(r.Output.Type)] {
		issues = append(issues, fmt.Sprintf("output has unknown type %q", r.Output.Type))
	}

	// Axis coupling (the load-bearing checks — they encode Odendahl's point that
	// the rung determines the burden floor):
	//  - a Code/Memory rung must run with NO model (executor none).
	//  - a deterministic-rung artifact gated by a non-deterministic verifier is a
	//    contradiction: it can't actually drop to executor none.
	switch r.Rung {
	case Code, Memory:
		if r.Executor != None {
			issues = append(issues, fmt.Sprintf("rung %q must run deterministically (executor none), got %q", r.Rung, r.Executor))
		}
		if r.Rung == Code && !deterministicVerifier[r.Verifier] {
			issues = append(issues, fmt.Sprintf("code rung needs a deterministic verifier to carry correctness, got %q", r.Verifier))
		}
	case Plan, RecipeRung, Pseudocode:
		if r.Executor == None {
			issues = append(issues, fmt.Sprintf("rung %q is run by a model; executor cannot be none", r.Rung))
		}
	}
	// The plan rung's verifier is plancheck (a plan is gated by checking the plan,
	// not by running it) — flag a plan with no verifier as ungated.
	if r.Rung == Plan && r.Verifier == "none" {
		issues = append(issues, "plan rung is ungated (verifier none); a plan should be gated by plancheck")
	}
	return issues
}

// Valid reports whether the recipe passes Validate with no issues.
func (r Recipe) Valid() bool { return len(r.Validate()) == 0 }

// Load reads one recipe from a JSON file.
func Load(path string) (Recipe, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Recipe{}, err
	}
	var r Recipe
	if err := json.Unmarshal(b, &r); err != nil {
		return Recipe{}, fmt.Errorf("%s: %w", path, err)
	}
	return r, nil
}

// Ladder sorts recipes for the SAME intent by descending model burden (Plan
// first, Code last) so a set authored at several rungs renders as a ladder.
func Ladder(rs []Recipe) {
	sort.SliceStable(rs, func(i, j int) bool {
		return rungOrder[rs[i].Rung] > rungOrder[rs[j].Rung]
	})
}
