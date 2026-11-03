package publicai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

// ChatOpts are the sampling knobs Classify pins (temperature 0, no top_p) but a
// multi-role harness needs per role: team-solve's roles sample at 0.2–0.4, the
// way GVS5H's harness and memdoor's port of it do.
type ChatOpts struct {
	Temperature float64
	TopP        float64 // 0 = provider default
	MaxTokens   int64   // 0 = provider default
	// Sample distinguishes otherwise-identical calls so a cached reply is not
	// reused across independent passes of the same problem (pass 1, pass 2, …).
	Sample int
	// NoThink sends chat_template_kwargs.enable_thinking=false: a hybrid
	// reasoning model (Qwen3.x) answers directly, the setting memdoor's bench
	// ran in. omitempty keeps cache keys of earlier runs valid.
	NoThink bool `json:",omitempty"`
	// Effort is output_config.effort for Anthropic models (low..max); ignored
	// by OpenAI-compatible endpoints. omitempty keeps earlier cache keys.
	Effort string `json:",omitempty"`
}

// ChatResult is a Result plus why the reply ended. FinishReason "length" means
// the reply was cut at MaxTokens — the harness must know, since a cut-off
// program is not a program.
type ChatResult struct {
	Result
	FinishReason string `json:"finish_reason"`
	// FromReasoning is true when the reply had no content and Text is the
	// model's reasoning field instead (a reasoning model that spent the whole
	// budget thinking). memdoor's parity notes do the same.
	FromReasoning bool `json:"from_reasoning,omitempty"`
}

type chatRequestOpts struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	TopP        float64       `json:"top_p,omitempty"`
	MaxTokens   int64         `json:"max_tokens,omitempty"`
	Template    *struct {
		EnableThinking bool `json:"enable_thinking"`
	} `json:"chat_template_kwargs,omitempty"`
}

type chatResponseFull struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			Reasoning        string `json:"reasoning"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// chatTimeout is the per-call HTTP timeout for Chat: PUBLICAI_TIMEOUT_S, default
// 600s. Classify's 120s is right for one-word labels and wrong for an 8K-token
// program from a 70B model at 30–50 tokens/s.
func chatTimeout() time.Duration {
	if s := os.Getenv("PUBLICAI_TIMEOUT_S"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 600 * time.Second
}

// Chat runs one completion with explicit sampling options, disk-cached by a hash
// of everything that shapes the reply (model, messages, options, sample index).
func (c *Client) Chat(ctx context.Context, model, system, prompt string, o ChatOpts) (ChatResult, error) {
	kb, _ := json.Marshal(struct {
		M, S, P string
		O       ChatOpts
		U       string
	}{model, system, prompt, o, c.baseURL})
	sum := sha256.Sum256(kb)
	key := "chat-" + hex.EncodeToString(sum[:])
	if b, err := os.ReadFile(c.cachePath(key)); err == nil {
		var r ChatResult
		if json.Unmarshal(b, &r) == nil {
			r.Cached = true
			return r, nil
		}
	}
	var msgs []chatMessage
	if system != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: system})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: prompt})
	reqo := chatRequestOpts{Model: model, Messages: msgs, Temperature: o.Temperature, TopP: o.TopP, MaxTokens: o.MaxTokens}
	if o.NoThink {
		reqo.Template = &struct {
			EnableThinking bool `json:"enable_thinking"`
		}{false}
	}
	body, _ := json.Marshal(reqo)

	hc := &http.Client{Timeout: chatTimeout()}
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepCtx(ctx, backoff(attempt)); err != nil {
				return ChatResult{}, err
			}
		}
		r, retryable, err := c.chatOnce(ctx, hc, model, body)
		if err == nil {
			if b, err := json.MarshalIndent(r, "", "  "); err == nil {
				_ = os.WriteFile(c.cachePath(key), b, 0o644)
			}
			return r, nil
		}
		lastErr = err
		if !retryable {
			return ChatResult{}, err
		}
	}
	return ChatResult{}, fmt.Errorf("chat: gave up after %d retries: %w", c.maxRetries, lastErr)
}

func (c *Client) chatOnce(ctx context.Context, hc *http.Client, model string, body []byte) (ChatResult, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatResult{}, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", userAgent)
	if bill := os.Getenv("HF_BILL_TO"); bill != "" {
		req.Header.Set("X-HF-Bill-To", bill) // HF router: bill an org instead of the token's user
	}
	start := time.Now()
	resp, err := hc.Do(req)
	if err != nil {
		return ChatResult{}, true, fmt.Errorf("chat transport: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	lat := time.Since(start).Milliseconds()
	if resp.StatusCode >= 300 {
		retry := resp.StatusCode == 429 || resp.StatusCode >= 500 || isBudgetThrottle(resp.StatusCode, raw)
		return ChatResult{}, retry, fmt.Errorf("chat HTTP %d model=%s: %s", resp.StatusCode, model, snippet(raw))
	}
	var cr chatResponseFull
	if err := json.Unmarshal(raw, &cr); err != nil {
		return ChatResult{}, false, fmt.Errorf("chat decode: %w: %s", err, snippet(raw))
	}
	if cr.Error != nil {
		return ChatResult{}, false, fmt.Errorf("chat error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return ChatResult{}, true, fmt.Errorf("chat: no choices for model=%s", model)
	}
	ch := cr.Choices[0]
	r := ChatResult{
		Result:       Result{Text: ch.Message.Content, Model: model, InputTokens: cr.Usage.PromptTokens, OutputTokens: cr.Usage.CompletionTokens, LatencyMS: lat},
		FinishReason: ch.FinishReason,
	}
	if r.Text == "" {
		if rs := ch.Message.ReasoningContent + ch.Message.Reasoning; rs != "" {
			r.Text, r.FromReasoning = rs, true
		}
	}
	return r, false, nil
}

func (c *Client) cachePath(key string) string {
	return c.cacheDir + string(os.PathSeparator) + key + ".json"
}
