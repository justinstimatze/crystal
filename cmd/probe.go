package cmd

import (
	"context"
	"fmt"

	"github.com/justinstimatze/crystal/internal/llm"
)

// ProbeCmd makes a single cheap connectivity call to confirm the live tier
// plumbing works (key loads from .env, SDK reaches the API, disk cache
// writes). Uses Haiku deliberately — this is a connectivity check and the
// cheapest tier, which is also the tier the project most cares about.
type ProbeCmd struct {
	Model    string `help:"Model to probe." default:"claude-haiku-4-5"`
	CacheDir string `help:"Disk cache dir." default:".crystal-cache"`
}

func (c *ProbeCmd) Run() error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	r, err := client.Complete(context.Background(), c.Model,
		"You are a connectivity probe. Reply with exactly: OK", "Reply OK.", 16)
	if err != nil {
		return fmt.Errorf("live call failed: %w", err)
	}
	src := "API"
	if r.Cached {
		src = "disk cache (no spend)"
	}
	fmt.Printf("probe %s [%s]\n  reply: %q\n  tokens: in=%d out=%d cacheRead=%d\n",
		c.Model, src, r.Text, r.InputTokens, r.OutputTokens, r.CacheRead)
	fmt.Println("  ✓ key loaded, SDK reached the API, cache wrote")
	return nil
}
