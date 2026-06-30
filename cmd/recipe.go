package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/justinstimatze/crystal/internal/recipe"
)

// RecipeCmd lints crystallized-artifact recipes against the schema
// (internal/recipe) — the producer-verifier discipline applied to the ARTIFACT,
// not just the code it generates. A recipe is a typed, diffable object, so it can
// be linted like code: every recipe's rung/executor/verifier coupling is checked
// (a "code" rung that still needs a model is a contradiction the lint catches),
// and its narrow-waist degrees of freedom are reported (a {query: any} interface
// scores worse than {name, fields}).
//
// Output is STRUCTURED rows by default rendered as a table; --json emits the rows
// verbatim. This is glazed's data philosophy (Odendahl): the program knows the
// rich structure of what it manipulates, so it should not flatten it to printf —
// the table is a render of the rows, and the rows compose into the next stage.
type RecipeCmd struct {
	Files []string `arg:"" optional:"" help:"Recipe JSON files to lint (default: testdata/recipe/*.json)."`
	JSON  bool     `help:"Emit the structured lint rows as JSON instead of a table."`
}

// lintRow is one recipe's lint result — the structured unit the table renders.
type lintRow struct {
	File     string   `json:"file"`
	Name     string   `json:"name"`
	Intent   string   `json:"intent"`
	Rung     string   `json:"rung"`
	Executor string   `json:"executor"`
	Verifier string   `json:"verifier"`
	DoF      int      `json:"degrees_of_freedom"`
	Valid    bool     `json:"valid"`
	Issues   []string `json:"issues,omitempty"`
}

func (c *RecipeCmd) Run() error {
	files := c.Files
	if len(files) == 0 {
		g, _ := filepath.Glob("testdata/recipe/*.json")
		files = g
	}
	if len(files) == 0 {
		return usageError{fmt.Errorf("no recipe files given and none under testdata/recipe/")}
	}

	var rows []lintRow
	for _, f := range files {
		r, err := recipe.Load(f)
		if err != nil {
			rows = append(rows, lintRow{File: f, Valid: false, Issues: []string{err.Error()}})
			continue
		}
		rows = append(rows, lintRow{
			File:     filepath.Base(f),
			Name:     r.Name,
			Intent:   r.Intent,
			Rung:     string(r.Rung),
			Executor: string(r.Executor),
			Verifier: r.Verifier,
			DoF:      r.DegreesOfFreedom(),
			Valid:    r.Valid(),
			Issues:   r.Validate(),
		})
	}

	if c.JSON {
		b, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Println(string(b))
		return failIfInvalid(rows)
	}

	// Table render, grouped by intent so a task authored at several rungs shows as
	// a ladder (highest model burden first).
	byIntent := map[string][]lintRow{}
	var intents []string
	for _, row := range rows {
		if _, ok := byIntent[row.Intent]; !ok {
			intents = append(intents, row.Intent)
		}
		byIntent[row.Intent] = append(byIntent[row.Intent], row)
	}
	sort.Strings(intents)

	rungBurden := map[string]int{"plan": 4, "recipe": 3, "pseudocode": 2, "code": 1, "memory": 0}
	fmt.Printf("crystal recipe: linted %d recipe(s) across %d intent(s)\n", len(rows), len(intents))
	fmt.Printf("(schema: docs/RECIPE_SCHEMA.md — rung=actuator expressivity, executor=model-burden floor, dof=narrow-waist width)\n\n")

	for _, intent := range intents {
		group := byIntent[intent]
		sort.SliceStable(group, func(i, j int) bool { return rungBurden[group[i].Rung] > rungBurden[group[j].Rung] })
		if intent != "" {
			fmt.Printf("intent: %s\n", intent)
		}
		fmt.Printf("  %-32s %-11s %-11s %-13s %4s  %s\n", "recipe", "rung", "executor", "verifier", "dof", "lint")
		for _, row := range group {
			status := "✓"
			if !row.Valid {
				status = "✗ " + strings.Join(row.Issues, "; ")
			}
			fmt.Printf("  %-32s %-11s %-11s %-13s %4d  %s\n",
				truncate(row.Name, 32), row.Rung, row.Executor, row.Verifier, row.DoF, status)
		}
		fmt.Println()
	}
	fmt.Printf("The same task descends the ladder: each rung down trades actuator expressivity for a\n")
	fmt.Printf("weaker executor — generalization shaping (Odendahl). The lint enforces the coupling:\n")
	fmt.Printf("a 'code' rung must run with no model and a deterministic verifier, or it is rejected.\n")
	return failIfInvalid(rows)
}

func failIfInvalid(rows []lintRow) error {
	bad := 0
	for _, r := range rows {
		if !r.Valid {
			bad++
		}
	}
	if bad > 0 {
		return fmt.Errorf("%d recipe(s) failed the schema lint", bad)
	}
	return nil
}
