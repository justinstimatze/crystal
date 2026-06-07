// Package local is a thin, disk-cached client for a LOCAL model served by
// ollama (http://localhost:11434 by default). It is the sovereignty-end tier of
// the crystal cost gradient (frontier → cloud-cheap → LOCAL → deterministic):
// the cheap tier run on owned hardware, no cloud round-trip, no per-token spend.
//
// It mirrors internal/llm's cache discipline exactly — results are keyed by a
// content hash of (model, system, prompt, numPredict) and persisted, so reruns
// are free and report the REAL first-measured local latency (the number the A5
// probe exists to surface).
package local

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Result is one local completion plus its measured latency.
type Result struct {
	Text      string `json:"text"`
	Model     string `json:"model"`
	LatencyMS int64  `json:"latency_ms"` // wall-clock of the live local call; persisted so reruns report the real measured latency
	Cached    bool   `json:"-"`
}

// Client wraps the ollama HTTP API with a disk cache.
type Client struct {
	host     string
	cacheDir string
	http     *http.Client
}

// Host returns the ollama base URL (OLLAMA_HOST or the local default).
func Host() string {
	if h := os.Getenv("OLLAMA_HOST"); h != "" {
		if !strings.HasPrefix(h, "http") {
			return "http://" + h
		}
		return h
	}
	return "http://localhost:11434"
}

// New constructs a local client. The cacheDir is shared with the llm cache dir;
// keys are namespaced by model so there is no collision with cloud entries.
func New(cacheDir string) (*Client, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	// A generous timeout: CPU inference of a small model can take seconds.
	return &Client{host: Host(), cacheDir: cacheDir, http: &http.Client{Timeout: 120 * time.Second}}, nil
}

// Reachable returns nil if the ollama server answers, else a helpful error.
func (c *Client) Reachable(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.host+"/api/tags", nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ollama unreachable at %s: %w (is `ollama serve` running?)", c.host, err)
	}
	resp.Body.Close()
	return nil
}

type genRequest struct {
	Model   string         `json:"model"`
	System  string         `json:"system,omitempty"`
	Prompt  string         `json:"prompt"`
	Stream  bool           `json:"stream"`
	Think   bool           `json:"think"` // explicitly false: thinking-capable models (qwen3.x) otherwise
	Options map[string]any `json:"options,omitempty"` // spend the whole num_predict budget on hidden reasoning and return an empty response
}

type genResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

// Classify runs one /api/generate call (thinking-free, deterministic temp 0),
// caching the result to disk by content hash. numPredict caps the output tokens.
func (c *Client) Classify(ctx context.Context, model, system, prompt string, numPredict int) (Result, error) {
	key := hashKey(model, system, prompt, numPredict)
	if r, ok := c.readCache(key); ok {
		r.Cached = true
		return r, nil
	}
	body, _ := json.Marshal(genRequest{
		Model:  model,
		System: system,
		Prompt: prompt,
		Stream: false,
		Think:  false, // classification: no hidden reasoning — answer directly (else num_predict is eaten by thinking)
		Options: map[string]any{
			"num_predict": numPredict,
			"temperature": 0,
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("ollama generate: %w", err)
	}
	defer resp.Body.Close()
	latency := time.Since(start).Milliseconds()
	var gr genResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return Result{}, fmt.Errorf("decoding ollama response: %w", err)
	}
	if gr.Error != "" {
		return Result{}, fmt.Errorf("ollama error: %s", gr.Error)
	}
	r := Result{Text: gr.Response, Model: model, LatencyMS: latency}
	c.writeCache(key, r)
	return r, nil
}

func hashKey(model, system, prompt string, numPredict int) string {
	b, _ := json.Marshal(struct {
		M  string `json:"m"`
		S  string `json:"s"`
		P  string `json:"p"`
		NP int    `json:"np"`
	}{model, system, prompt, numPredict})
	sum := sha256.Sum256(b)
	return "local-" + hex.EncodeToString(sum[:]) // namespaced so it never collides with cloud cache keys
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
