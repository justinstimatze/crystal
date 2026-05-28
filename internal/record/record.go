// Package record defines the substrate unit the eval harness replays
// against: one paired Claude Code tool call (the tool_use) joined to its
// result (the toolUseResult), plus enough surrounding context to cluster
// and verify it later.
//
// The same Record schema is emitted by the offline transcript walker
// (Phase 1) and, eventually, by the live PostToolUse hook (LENS, Phase 2),
// so replay over history and live capture are interchangeable.
package record

import "encoding/json"
import "time"

// Record is one tool_use ↔ toolUseResult pair, reconstructed from a
// transcript. Context/Followup are captured for later drift/clustering
// work but are NOT yet a verifiable promotion axis (see plan: tool_result
// fidelity only in Phase 1).
type Record struct {
	SessionID string         `json:"session_id"`
	Repo      string         `json:"repo"`       // from cwd
	GitBranch string         `json:"git_branch"` // from gitBranch
	Timestamp time.Time      `json:"timestamp"`
	Context   string         `json:"context"`  // preceding prompt/assistant text
	Tool      string         `json:"tool"`     // tool_use.name
	Args      map[string]any `json:"args"`     // tool_use.input
	Result    Output         `json:"result"`   // typed toolUseResult
	Followup  string         `json:"followup"` // next assistant text after the result
	ToolUseID string         `json:"tool_use_id"`
}

// Output is a typed view of a toolUseResult. Only the fields relevant to
// the producing tool are populated; Raw always holds the verbatim result
// as the source of truth.
//
// Outcome class: a scalar (bare-string) toolUseResult is an ERROR
// envelope, not an alternate success serialization. On the real corpus,
// scalar Bash/Edit/Write results are overwhelmingly "Error:"-prefixed,
// while object results carry the success fields. So IsError (scalar shape
// or an explicit error flag) is the first axis any comparator checks; we
// never copy a scalar error string into a success field like Stdout.
type Output struct {
	Raw     json.RawMessage `json:"raw,omitempty"`
	IsError bool            `json:"is_error"`
	Scalar  string          `json:"scalar,omitempty"` // error-envelope text

	// Bash
	Stdout      string `json:"stdout,omitempty"`
	Stderr      string `json:"stderr,omitempty"`
	Interrupted bool   `json:"interrupted,omitempty"`

	// Read / Edit
	File      string `json:"file,omitempty"`
	NewString string `json:"new_string,omitempty"`

	// Edit / Write
	StructuredPatch json.RawMessage `json:"structured_patch,omitempty"`

	// Grep
	Filenames []string `json:"filenames,omitempty"`
	NumFiles  int      `json:"num_files,omitempty"`
}

// TextFields returns pointers to every free-text field that may carry
// secrets/PII, so the redactor can scrub them in place. Raw and
// StructuredPatch are json.RawMessage and handled separately by the
// redactor (it scrubs their bytes).
func (o *Output) TextFields() []*string {
	return []*string{&o.Scalar, &o.Stdout, &o.Stderr, &o.File, &o.NewString}
}
