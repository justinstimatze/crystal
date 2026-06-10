package cmd

import (
	"context"
	"fmt"

	"github.com/justinstimatze/crystal/internal/publicai"
)

// PublicAIProbeCmd makes one cheap chat completion against the Public AI Gateway
// to confirm the new open-model tier's plumbing works (key loads from .env,
// gateway reachable, OpenAI-compatible response decodes, disk cache writes).
// Defaults to apertus-8b — the cloud analog of the local qwen3:8b cheap tier.
type PublicAIProbeCmd struct {
	Model    string `help:"Gateway model to probe." default:"swiss-ai/apertus-8b-instruct"`
	CacheDir string `help:"Disk cache dir (shared with the other tiers)." default:".crystal-cache"`
}

func (c *PublicAIProbeCmd) Run() error {
	client, err := publicai.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	ctx := context.Background()
	if err := client.Reachable(ctx); err != nil {
		return fmt.Errorf("gateway check failed: %w", err)
	}
	r, err := client.Classify(ctx, c.Model,
		"You are a connectivity probe. Reply with exactly: OK", "Reply OK.", 16)
	if err != nil {
		return fmt.Errorf("live call failed: %w", err)
	}
	src := "gateway"
	if r.Cached {
		src = "disk cache (no spend)"
	}
	fmt.Printf("publicai-probe %s [%s] @ %s\n  reply: %q\n  tokens: in=%d out=%d  latency=%dms\n",
		c.Model, src, publicai.BaseURL(), r.Text, r.InputTokens, r.OutputTokens, r.LatencyMS)
	fmt.Println("  ✓ key loaded, gateway reached, OpenAI-compatible response decoded, cache wrote")
	return nil
}
