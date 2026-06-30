package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/justinstimatze/crystal/internal/defnverify"
	"github.com/justinstimatze/crystal/internal/llm"
	"github.com/justinstimatze/crystal/internal/sediment"
)

// AuthorFnCmd is the defn-backed Authorer: the first END-TO-END sediment of a
// real function (not a kong scaffold). It closes the loop on arbitrary Go logic:
//
//	SELECT — a definition that is test-COVERED (verifiable; else refuse) and
//	         low-blast-radius (safe to swap), via `defn impact`.
//	AUTHOR — Opus reimplements the function from its source alone (BLIND to the
//	         tests, so it cannot overfit the gate).
//	GATE   — splice the candidate in, run the covering tests SCOPED to the package
//	         (sediment.GateRewrite, which always restores the file). Pass → the
//	         replacement is behaviorally equivalent. Plus a NEGATIVE CONTROL: a
//	         compiling-but-wrong impl (identity / panic) MUST be rejected, proving
//	         the covering tests are load-bearing (the gate's g, made visible).
//	PROPOSE — write the passing candidate to a proposal dir; never auto-apply.
//
// Requires the repo indexed by defn (`defn ingest .`). The working tree is never
// left modified.
type AuthorFnCmd struct {
	Name        string `help:"The function to re-author and gate (must be a test-covered, plain function)." default:"Kebab"`
	Model       string `help:"Authoring model (the expensive tier)." default:"claude-opus-4-8"`
	CacheDir    string `help:"Disk cache dir for LLM calls." default:".crystal-cache"`
	ProposalDir string `help:"Where passing candidates are written for human review (never auto-applied)." default:".dogfood-scratch/proposals"`
	Verbose     bool   `help:"Print the authored candidate and the negative control source."`
}

func (c *AuthorFnCmd) Run() error {
	if err := defnverify.Available(); err != nil {
		return usageError{fmt.Errorf("defn not available: %w", err)}
	}
	imp, err := defnverify.ImpactOf(c.Name)
	if err != nil {
		return usageError{fmt.Errorf("defn impact %q: %w", c.Name, err)}
	}

	fmt.Printf("author-fn: end-to-end sediment of a real function via the defn-backed gate\n\n")

	// ---- SELECT ----
	fmt.Println("=== select (defn impact) ===")
	fmt.Printf("  %s — module %s (%s)\n", c.Name, imp.Module, imp.SourceFile)
	fmt.Printf("  Confidence (covering tests): %d   blast radius: %s (%d direct callers)\n", imp.Covered, imp.BlastRadius, imp.Callers)
	if imp.Covered == 0 {
		fmt.Printf("\nDecision: REFUSE — %s has no covering tests; UNVERIFIABLE. crystal does not crystallize what it cannot gate.\n", c.Name)
		return nil
	}
	info, err := sediment.Locate(imp.SourceFile, c.Name)
	if err != nil {
		return usageError{fmt.Errorf("locate %q: %w", c.Name, err)}
	}
	fmt.Printf("  located: %s (%d bytes of source)\n\n", info.File, len(info.DeclSrc))

	// ---- AUTHOR (blind to the tests) ----
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	fmt.Printf("=== author (%s reimplements it from source, BLIND to the tests) ===\n", c.Model)
	sys := "You reimplement a single Go function with IDENTICAL observable behavior. " +
		"Output ONLY the function declaration — no prose, no markdown fences, no comments. " +
		"Keep the EXACT signature and name. Use only standard-library packages already imported in the file. " +
		"Do not change behavior; this is a faithful re-expression, not an improvement."
	r, err := client.Complete(context.Background(), c.Model, sys, "Reimplement this function:\n\n"+info.DeclSrc, 2048)
	if err != nil {
		return usageError{fmt.Errorf("authoring: %w", err)}
	}
	candidate := stripFence(r.Text)
	if err := sediment.ValidateDecl(c.Name, candidate); err != nil {
		return usageError{fmt.Errorf("authored candidate invalid: %w", err)}
	}
	fmt.Printf("  authored a candidate (%d bytes)\n", len(candidate))
	if c.Verbose {
		fmt.Printf("  --- candidate ---\n%s\n", indentLines(candidate, "    "))
	}

	// ---- GATE (positive) ----
	pass, detail, err := sediment.GateRewrite(info, candidate, imp.Module, imp.Tests)
	if err != nil {
		return usageError{fmt.Errorf("gating candidate: %w", err)}
	}
	fmt.Printf("\n=== gate: covering tests (scoped to %s) ===\n", imp.Module)
	if pass {
		fmt.Printf("  candidate: PROMOTE — the %d covering test(s) pass; behaviorally equivalent.\n", imp.Covered)
	} else {
		fmt.Printf("  candidate: REJECT — covering tests failed:\n      %s\n", detail)
	}

	// ---- NEGATIVE CONTROL (the gate must catch a compiling-but-wrong impl) ----
	neg := info.LazyWrong()
	if c.Verbose {
		fmt.Printf("  --- negative control ---\n%s\n", indentLines(neg, "    "))
	}
	negPass, _, err := sediment.GateRewrite(info, neg, imp.Module, imp.Tests)
	if err != nil {
		return usageError{fmt.Errorf("gating negative control: %w", err)}
	}
	if negPass {
		fmt.Printf("  ⚠ negative control PASSED — the covering tests do NOT discriminate this function (g<1 here).\n")
		fmt.Printf("    The gate is not load-bearing for %s; a wrong replacement would sediment silently. Do not trust a PROMOTE.\n", c.Name)
	} else {
		fmt.Printf("  negative control (compiling-but-wrong): REJECTED ✓ — the covering tests ARE load-bearing here.\n")
	}

	// ---- PROPOSE ----
	if pass && !negPass {
		writeProposal(c.ProposalDir, c.Name, candidate)
		path := filepath.Join(c.ProposalDir, c.Name+".go.txt")
		fmt.Printf("\nDecision: PROMOTE — authored replacement verified equivalent AND the gate is load-bearing.\n")
		fmt.Printf("Proposal written (human reviews, NOT auto-applied): %s\n", path)
		fmt.Printf("\nThe loop closed end-to-end on a REAL function: Opus authored a replacement, its own tests\n")
		fmt.Printf("certified equivalence, and a wrong impl was rejected — the self-authoring gate on arbitrary\n")
		fmt.Printf("Go logic. The working tree was restored; nothing was applied. (Go-only; generalize later.)\n")
	} else {
		fmt.Printf("\nDecision: NO PROMOTE — %s.\n", noPromoteReason(pass, negPass))
	}
	return nil
}

func noPromoteReason(pass, negPass bool) string {
	switch {
	case !pass:
		return "the authored candidate failed the covering tests (not equivalent)"
	case negPass:
		return "the gate is not load-bearing here (negative control passed), so a PROMOTE can't be trusted"
	default:
		return "unknown"
	}
}
