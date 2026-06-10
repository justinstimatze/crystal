// Package transcript walks Claude Code session transcripts
// (~/.claude/projects/*/*.jsonl) and reconstructs paired tool calls as
// record.Record values.
//
// It lifts hindcast's line/struct/scanner shape but its job is different:
// hindcast groups events into per-turn timing summaries; we pair each
// assistant tool_use to its toolUseResult by tool_use_id and type the
// result per tool. Subagent (isSidechain) events are skipped — Phase 1
// scores the main session only.
package transcript

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/justinstimatze/crystal/internal/record"
)

type rawEntry struct {
	Type        string          `json:"type"`
	IsSidechain bool            `json:"isSidechain"`
	SessionID   string          `json:"sessionId"`
	CWD         string          `json:"cwd"`
	GitBranch   string          `json:"gitBranch"`
	Timestamp   time.Time       `json:"timestamp"`
	Message     json.RawMessage `json:"message"`
	// toolUseResult is the rich, typed result attached to a tool-result
	// user entry. It is the source of truth for Output.
	ToolUseResult json.RawMessage `json:"toolUseResult"`
}

type rawMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type contentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ID        string          `json:"id,omitempty"`          // tool_use id
	ToolUseID string          `json:"tool_use_id,omitempty"` // tool_result link
	IsError   *bool           `json:"is_error,omitempty"`    // tool_result error flag
}

type pendingUse struct {
	tool    string
	args    map[string]any
	context string
}

// Walk parses transcript r and returns the reconstructed Records. dropped
// reports records lost to oversized lines or malformed JSON, so callers
// can fail loud instead of silently truncating.
func Walk(r io.Reader) (recs []record.Record, dropped int, err error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 32*1024*1024)

	pending := map[string]pendingUse{}
	var awaiting []int // indices into recs whose Followup is not yet filled
	var lastUserPrompt string

	fillFollowup := func(text string) {
		if text == "" || len(awaiting) == 0 {
			return
		}
		for _, i := range awaiting {
			recs[i].Followup = text
		}
		awaiting = awaiting[:0]
	}

	for sc.Scan() {
		var e rawEntry
		if jerr := json.Unmarshal(sc.Bytes(), &e); jerr != nil {
			dropped++
			continue
		}
		if e.IsSidechain {
			continue
		}
		var m rawMessage
		if len(e.Message) > 0 {
			_ = json.Unmarshal(e.Message, &m)
		}

		switch e.Type {
		case "user":
			// A fresh user prompt ends the assistant's turn: any results
			// still awaiting a followup get none.
			if isNewUserPrompt(m.Content) {
				awaiting = awaiting[:0]
				lastUserPrompt = extractUserString(m.Content)
				continue
			}
			// Otherwise this is a tool-result turn. Pair each tool_result
			// block to its pending tool_use; the entry-level
			// toolUseResult is the typed result.
			for _, b := range blocks(m.Content) {
				if b.Type != "tool_result" {
					continue
				}
				pu, ok := pending[b.ToolUseID]
				if !ok {
					continue
				}
				delete(pending, b.ToolUseID)
				rec := record.Record{
					SessionID: e.SessionID,
					Repo:      e.CWD,
					GitBranch: e.GitBranch,
					Timestamp: e.Timestamp,
					Context:   pu.context,
					Tool:      pu.tool,
					Args:      pu.args,
					Result:    typeOutput(pu.tool, e.ToolUseResult, b.IsError),
					ToolUseID: b.ToolUseID,
				}
				recs = append(recs, rec)
				awaiting = append(awaiting, len(recs)-1)
			}

		case "assistant":
			var text strings.Builder
			var uses []contentBlock
			for _, b := range blocks(m.Content) {
				switch b.Type {
				case "text":
					text.WriteString(b.Text)
				case "tool_use":
					uses = append(uses, b)
				}
			}
			// This assistant turn's text is the followup for any results
			// that preceded it.
			fillFollowup(strings.TrimSpace(text.String()))
			// ...and the context for the tool_use calls it makes now.
			ctx := strings.TrimSpace(text.String())
			if ctx == "" {
				ctx = lastUserPrompt
			}
			for _, u := range uses {
				if u.ID == "" {
					continue
				}
				pending[u.ID] = pendingUse{tool: u.Name, args: decodeArgs(u.Input), context: ctx}
			}
		}
	}
	if serr := sc.Err(); serr != nil {
		if errors.Is(serr, bufio.ErrTooLong) {
			// An oversized line: count it and keep going on a fresh
			// scanner is not possible mid-stream, so report loudly.
			return recs, dropped + 1, serr
		}
		return recs, dropped, serr
	}
	return recs, dropped, nil
}

// WalkFile is Walk over an opened path. A bufio.ErrTooLong is returned so
// the caller can log a per-file shortfall rather than silently truncate.
func WalkFile(path string) ([]record.Record, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	return Walk(f)
}

// typeOutput converts a raw toolUseResult into a typed Output. A scalar
// (bare-string) result is an error envelope, not a success value.
func typeOutput(tool string, raw json.RawMessage, blockIsError *bool) record.Output {
	out := record.Output{Raw: raw}
	if blockIsError != nil {
		out.IsError = *blockIsError
	}
	trimmed := strings.TrimSpace(string(raw))

	// Scalar / bare string → error envelope.
	if strings.HasPrefix(trimmed, `"`) {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			out.Scalar = s
			if blockIsError == nil && strings.HasPrefix(strings.TrimSpace(s), "Error") {
				out.IsError = true
			}
			return out
		}
	}

	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return out
	}
	switch tool {
	case "Bash":
		out.Stdout, _ = m["stdout"].(string)
		out.Stderr, _ = m["stderr"].(string)
		out.Interrupted, _ = m["interrupted"].(bool)
	case "Read":
		out.File = jsonString(m["file"])
	case "Edit":
		out.NewString, _ = m["newString"].(string)
		out.StructuredPatch = rawOf(m["structuredPatch"])
	case "Write":
		out.NewString, _ = m["content"].(string)
		out.StructuredPatch = rawOf(m["structuredPatch"])
	case "Grep":
		out.Filenames = stringSlice(m["filenames"])
		if n, ok := m["numFiles"].(float64); ok {
			out.NumFiles = int(n)
		}
		if out.File == "" {
			out.File = jsonString(m["content"])
		}
	}
	return out
}

func decodeArgs(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m
}

func jsonString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func rawOf(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// isNewUserPrompt reports whether content is a bare string (a fresh user
// prompt) vs. the array form used for tool_result injections.
func isNewUserPrompt(raw json.RawMessage) bool {
	return strings.HasPrefix(strings.TrimLeft(string(raw), " \t\r\n"), `"`)
}

func extractUserString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var b strings.Builder
	for _, blk := range blocks(raw) {
		if blk.Type == "text" {
			b.WriteString(blk.Text)
		}
	}
	return b.String()
}

func blocks(raw json.RawMessage) []contentBlock {
	var arr []contentBlock
	if json.Unmarshal(raw, &arr) == nil {
		return arr
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return []contentBlock{{Type: "text", Text: s}}
	}
	return nil
}
