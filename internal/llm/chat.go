package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/justinstimatze/crystal/internal/publicai"
)

// Chat satisfies teamsolve.Chatter on the Anthropic API, so a Claude model can
// sit in the same harness as the open models. Current Claude models reject
// sampling parameters and explicit thinking settings, so temperature/top_p
// are not sent and thinking runs adaptive; o.Effort sets output_config.effort.
// Streams, because a hard problem's reply can run tens of thousands of tokens.
// No refusal fallback: a fallback would swap the model under test, so a
// refusal is returned as an error and shows up as one in the results.
func (c *Client) Chat(ctx context.Context, model, system, prompt string, o publicai.ChatOpts) (publicai.ChatResult, error) {
	kb, _ := json.Marshal(struct {
		M, S, P, E string
		MT         int64
		N          int
	}{model, system, prompt, o.Effort, o.MaxTokens, o.Sample})
	sum := sha256.Sum256(kb)
	path := filepath.Join(c.cacheDir, "anthropic-chat-"+hex.EncodeToString(sum[:])+".json")
	if b, err := os.ReadFile(path); err == nil {
		var r publicai.ChatResult
		if json.Unmarshal(b, &r) == nil {
			r.Cached = true
			return r, nil
		}
	}
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: o.MaxTokens,
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(prompt))},
	}
	if o.Effort != "" {
		params.OutputConfig = anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffort(o.Effort)}
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
	}
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return publicai.ChatResult{}, ctx.Err()
			case <-time.After(time.Duration(10<<attempt) * time.Second):
			}
		}
		start := time.Now()
		stream := c.api.Messages.NewStreaming(ctx, params)
		msg := anthropic.Message{}
		for stream.Next() {
			if err := msg.Accumulate(stream.Current()); err != nil {
				lastErr = err
				break
			}
		}
		if err := stream.Err(); err != nil {
			lastErr = err
			continue // the SDK's typed errors include 429/5xx; retry a bounded number of times
		}
		if msg.StopReason == anthropic.StopReasonRefusal {
			return publicai.ChatResult{}, fmt.Errorf("refusal (no fallback, by design)")
		}
		var sb strings.Builder
		for _, block := range msg.Content {
			if t, ok := block.AsAny().(anthropic.TextBlock); ok {
				sb.WriteString(t.Text)
			}
		}
		r := publicai.ChatResult{
			Result: publicai.Result{Text: sb.String(), Model: model, InputTokens: msg.Usage.InputTokens,
				OutputTokens: msg.Usage.OutputTokens, LatencyMS: time.Since(start).Milliseconds()},
			FinishReason: string(msg.StopReason),
		}
		if msg.StopReason == anthropic.StopReasonMaxTokens {
			r.FinishReason = "length"
		}
		if b, err := json.MarshalIndent(r, "", "  "); err == nil {
			_ = os.WriteFile(path, b, 0o644)
		}
		return r, nil
	}
	return publicai.ChatResult{}, fmt.Errorf("anthropic chat: %w", lastErr)
}
