// Package publicai is a thin, disk-cached client for the Public AI Gateway
// (https://api.publicai.co/v1) — an OpenAI-compatible endpoint serving open and
// sovereign models (Apertus, Olmo, SEA-LION, EuroLLM, …).
//
// It is a new rung on the crystal cost gradient:
//
//	frontier (Opus) → cloud-cheap CLOSED (Haiku) → cloud-cheap OPEN (this) → LOCAL (ollama) → deterministic
//
// Why it earns its place between Haiku and local ollama:
//   - Open/sovereign models at a fraction of Haiku's price (e.g. apertus-8b at
//     $0.10/$0.20 per 1M, Olmo-3.1-32B at $0.05/$0.20) — the same model FAMILY
//     as the local qwen tier but with no GPU to own and no VRAM-spill stall.
//   - A cloud-hosted ~32B that does NOT have the local 35B's spill latency (the
//     RTX 3080 spills ~70% to RAM and stalls past 120s). The two-model agreement
//     oracle can use a cloud 8B+32B pair with predictable latency.
//
// It mirrors internal/llm and internal/local cache discipline exactly: results
// are keyed by a content hash of (model, system, prompt, maxTokens) and persisted,
// so reruns are free and report the REAL first-measured latency. Because this is a
// PAID cloud tier, the Result also carries token accounting (cost discipline).
package publicai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Model IDs available on the gateway (from the published pricing list). The
// cheap-tier-relevant ones are named; the full catalog is larger.
const (
	// ModelApertus8B — Swiss AI's 8B instruct; the cloud analog of the local
	// qwen3:8b tier ($0.10 in / $0.20 out per 1M). Default cheap model.
	ModelApertus8B = "swiss-ai/apertus-8b-instruct"
	// ModelApertus70B — Swiss AI's 70B instruct ($0.82 / $2.92).
	ModelApertus70B = "swiss-ai/apertus-70b-instruct"
	// ModelOlmo32B — AllenAI's fully-open 32B instruct ($0.05 / $0.20) — a
	// cloud-hosted ~32B with no VRAM-spill latency, unlike the local 35B.
	ModelOlmo32B = "allenai/Olmo-3.1-32B-Instruct"
	// ModelEuroLLM22B — EuroLLM 22B instruct ($0.10 / $0.20).
	ModelEuroLLM22B = "utter-project/EuroLLM-22B-Instruct-2512"
)

// DefaultBaseURL is the OpenAI-compatible gateway base. Override with PUBLICAI_BASE_URL.
const DefaultBaseURL = "https://api.publicai.co/v1"

// userAgent identifies the client. The gateway documents User-Agent as a
// REQUIRED header (Go sends a default, but we set it explicitly to be safe).
const userAgent = "crystal/dev (+https://github.com/justinstimatze/crystal)"

// Result is one completion plus latency and token accounting.
type Result struct {
	Text         string `json:"text"`
	Model        string `json:"model"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	LatencyMS    int64  `json:"latency_ms"` // wall-clock of the live call; persisted so reruns report the real measured latency
	Cached       bool   `json:"-"`
}

// Client wraps the gateway's OpenAI-compatible API with a disk cache.
type Client struct {
	baseURL    string
	apiKey     string
	cacheDir   string
	http       *http.Client
	maxRetries int // generous retries for transient throttles (budget window, 429, 5xx)
}

// BaseURL returns PUBLICAI_BASE_URL or the gateway default.
func BaseURL() string {
	if u := os.Getenv("PUBLICAI_BASE_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return DefaultBaseURL
}

// New loads .env (best-effort) and constructs a client. Errors if no
// PUBLICAI_API_KEY is available after loading. The cacheDir is shared with the
// other tiers; keys are namespaced ("publicai-") so there is no collision.
func New(cacheDir string) (*Client, error) {
	_ = loadDotEnv(".env")
	key := os.Getenv("PUBLICAI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("PUBLICAI_API_KEY not set (put it in .env or the environment)")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	return &Client{
		baseURL:    BaseURL(),
		apiKey:     key,
		cacheDir:   cacheDir,
		http:       &http.Client{Timeout: 120 * time.Second},
		maxRetries: maxRetries(),
	}, nil
}

// maxRetries reads PUBLICAI_MAX_RETRIES (default 6 — with the exponential backoff
// that is willing to wait through a multi-minute budget window before giving up).
func maxRetries() int {
	if s := os.Getenv("PUBLICAI_MAX_RETRIES"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			return n
		}
	}
	return 6
}

// Reachable returns nil if the gateway answers the models endpoint (auth ok).
func (c *Client) Reachable(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("publicai unreachable at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("publicai 401 unauthorized: PUBLICAI_API_KEY rejected by %s", c.baseURL)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("publicai %s returned HTTP %d", c.baseURL, resp.StatusCode)
	}
	return nil
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int64         `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Classify runs one deterministic (temperature 0) chat completion, caching the
// result to disk by content hash. maxTokens caps the output; pass 0 to leave it
// to the gateway default.
func (c *Client) Classify(ctx context.Context, model, system, prompt string, maxTokens int64) (Result, error) {
	key := hashKey(model, system, prompt, maxTokens)
	if r, ok := c.readCache(key); ok {
		r.Cached = true
		return r, nil
	}
	var msgs []chatMessage
	if system != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: system})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: prompt})
	body, _ := json.Marshal(chatRequest{Model: model, Messages: msgs, Temperature: 0, MaxTokens: maxTokens})

	// Retry transient conditions with generous backoff. PublicAI's shared-Team
	// spend-budget throttle returns a 400 "Budget has been exceeded" that is
	// actually TRANSIENT (a windowed budget that refills — issue #46: "best effort
	// service … try again in a few minutes"), so we treat it like a 529/overload
	// rather than a hard error. Also retries 429 and 5xx. A genuine 400 (bad model),
	// 401 (auth), or empty-choices is NOT retried.
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepCtx(ctx, backoff(attempt)); err != nil {
				return Result{}, err // context cancelled while backing off
			}
		}
		r, retryable, err := c.attempt(ctx, key, model, body)
		if err == nil {
			return r, nil
		}
		lastErr = err
		if !retryable {
			return Result{}, err
		}
	}
	return Result{}, fmt.Errorf("publicai: gave up after %d retries: %w", c.maxRetries, lastErr)
}

// attempt makes one HTTP call. retryable is true for transient conditions
// (transport error, 429, 5xx, or the budget-exceeded throttle) so the caller backs
// off and tries again; false for terminal errors (bad request, auth, no choices).
func (c *Client) attempt(ctx context.Context, key, model string, body []byte) (r Result, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Result{}, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", userAgent)

	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, true, fmt.Errorf("publicai chat: %w", err) // transport error: retryable
	}
	defer resp.Body.Close()
	latency := time.Since(start).Milliseconds()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		retryable = resp.StatusCode == 429 || resp.StatusCode >= 500 || isBudgetThrottle(resp.StatusCode, raw)
		return Result{}, retryable, fmt.Errorf("publicai HTTP %d for model=%s: %s", resp.StatusCode, model, snippet(raw))
	}
	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return Result{}, false, fmt.Errorf("decoding publicai response (HTTP %d): %w: %s", resp.StatusCode, err, snippet(raw))
	}
	if cr.Error != nil {
		return Result{}, false, fmt.Errorf("publicai error (HTTP %d): %s", resp.StatusCode, cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return Result{}, false, fmt.Errorf("publicai returned no choices for model=%s", model)
	}
	r = Result{
		Text:         cr.Choices[0].Message.Content,
		Model:        model,
		InputTokens:  cr.Usage.PromptTokens,
		OutputTokens: cr.Usage.CompletionTokens,
		LatencyMS:    latency,
	}
	c.writeCache(key, r)
	return r, false, nil
}

// isBudgetThrottle reports whether a non-2xx response is PublicAI's transient
// shared-budget throttle (a 400 whose body names the budget) rather than a real
// bad request — so it is retried with backoff like an overload.
func isBudgetThrottle(status int, body []byte) bool {
	return status == http.StatusBadRequest && strings.Contains(string(body), "Budget has been exceeded")
}

// backoff is the generous wait before retry attempt n (1-based): exponential from
// a base with jitter, capped — willing to wait through a multi-minute budget window.
func backoff(attempt int) time.Duration {
	base := 3 * time.Second
	d := base << (attempt - 1) // 3s, 6s, 12s, 24s, 48s, …
	if d > 60*time.Second {
		d = 60 * time.Second
	}
	// +0–25% jitter so concurrent callers don't synchronize on the budget window.
	j := time.Duration(rand.Int63n(int64(d) / 4))
	return d + j
}

// sleepCtx sleeps for d unless ctx is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// snippet returns a one-line, length-capped view of a response body for errors.
func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	s = strings.Join(strings.Fields(s), " ") // collapse newlines/whitespace
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	if s == "" {
		return "(empty body)"
	}
	return s
}

func hashKey(model, system, prompt string, maxTokens int64) string {
	b, _ := json.Marshal(struct {
		M  string `json:"m"`
		S  string `json:"s"`
		P  string `json:"p"`
		MT int64  `json:"mt"`
	}{model, system, prompt, maxTokens})
	sum := sha256.Sum256(b)
	return "publicai-" + hex.EncodeToString(sum[:]) // namespaced so it never collides with other tiers' cache keys
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
// (without overwriting already-set vars). Tolerates `export ` prefixes, quotes,
// comments, and blank lines. Duplicated from internal/llm to keep the package
// self-contained (no import cycle / no cross-tier dependency).
func loadDotEnv(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
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
	return nil
}
