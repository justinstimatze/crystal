// Package llm is a thin, disk-cached wrapper over the official Anthropic Go
// SDK. It is the tier-invocation primitive the live crystal experiments
// need — calling Opus / Sonnet / Haiku and comparing their outputs through
// the eval gate.
//
// Cost discipline: every completion is cached to disk keyed by a content
// hash of (model, system, prompt, maxTokens). Re-runs hit the cache and
// cost nothing — only a genuinely new (model, prompt) pair spends tokens.
// This matters because live tier sweeps re-run the same prompts often.
package llm

import (
	"bufio"
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
)

// Model IDs (current, from the claude-api skill). Do not append date suffixes.
const (
	ModelOpus   = "claude-opus-4-8"
	ModelSonnet = "claude-sonnet-4-6"
	ModelHaiku  = "claude-haiku-4-5"
)

// Result is one completion plus token accounting.
type Result struct {
	Text         string `json:"text"`
	Model        string `json:"model"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CacheRead    int64  `json:"cache_read_tokens"`
	LatencyMS    int64  `json:"latency_ms"` // wall-clock of the live API call; persisted so reruns report the real measured latency
	Cached       bool   `json:"-"`          // true when served from local disk cache (no spend)
}

// Client wraps the SDK with a disk cache.
type Client struct {
	api      anthropic.Client
	cacheDir string
}

// New loads .env (if present) into the environment, then constructs a
// client. Errors if no ANTHROPIC_API_KEY is available after loading.
func New(cacheDir string) (*Client, error) {
	_ = loadDotEnv(".env")
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set (put it in .env or the environment)")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	return &Client{api: anthropic.NewClient(), cacheDir: cacheDir}, nil
}

// Complete runs one message request, caching the result to disk by content
// hash. A cache hit returns immediately with Cached=true and zero spend.
func (c *Client) Complete(ctx context.Context, model, system, prompt string, maxTokens int64) (Result, error) {
	return c.complete(ctx, model, system, prompt, maxTokens, false)
}

// Classify is Complete with thinking DISABLED. It exists because adaptive
// thinking tokens count against MaxTokens: a tiny one-word classification
// (e.g. "FAITHFUL"/"DRIFT") under a small MaxTokens can return EMPTY visible
// text when thinking eats the budget — silently defaulting every verdict to
// the same class. Disabling thinking guarantees the budget is spent on the
// answer. (This was the bug that invalidated the first ground-hop run.)
func (c *Client) Classify(ctx context.Context, model, system, prompt string, maxTokens int64) (Result, error) {
	return c.complete(ctx, model, system, prompt, maxTokens, true)
}

func (c *Client) complete(ctx context.Context, model, system, prompt string, maxTokens int64, noThink bool) (Result, error) {
	mode := ""
	if noThink {
		mode = "nothink"
	}
	key := hashKey(model, system, prompt, maxTokens, mode)
	if r, ok := c.readCache(key); ok {
		r.Cached = true
		return r, nil
	}
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(prompt))},
	}
	if noThink {
		d := anthropic.NewThinkingConfigDisabledParam()
		params.Thinking = anthropic.ThinkingConfigParamUnion{OfDisabled: &d}
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
	}
	start := time.Now()
	resp, err := c.api.Messages.New(ctx, params)
	if err != nil {
		return Result{}, err
	}
	latency := time.Since(start).Milliseconds()
	var sb strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			sb.WriteString(t.Text)
		}
	}
	r := Result{
		Text:         sb.String(),
		Model:        model,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
		CacheRead:    resp.Usage.CacheReadInputTokens,
		LatencyMS:    latency,
	}
	c.writeCache(key, r)
	return r, nil
}

// hashKey keys the disk cache. Mode is omitempty so existing thinking-on
// (mode="") cache entries keep their original keys; only thinking-disabled
// calls add the marker.
func hashKey(model, system, prompt string, maxTokens int64, mode string) string {
	b, _ := json.Marshal(struct {
		M    string `json:"m"`
		S    string `json:"s"`
		P    string `json:"p"`
		MT   int64  `json:"mt"`
		Mode string `json:"mode,omitempty"`
	}{model, system, prompt, maxTokens, mode})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (c *Client) readCache(key string) (Result, bool) {
	b, err := os.ReadFile(filepath.Join(c.cacheDir, key+".json"))
	if err != nil {
		return Result{}, false
	}
	var r Result
	if json.Unmarshal(b, &r) != nil {
		return Result{}, false
	}
	return r, true
}

func (c *Client) writeCache(key string, r Result) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(c.cacheDir, key+".json"), b, 0o644)
}

// loadDotEnv reads KEY=VALUE lines from path into the process environment
// (without overwriting already-set vars). Tolerates `export ` prefixes,
// quotes, comments, and blank lines.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		if k != "" && os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
	return sc.Err()
}
