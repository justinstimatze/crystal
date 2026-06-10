package eval

import (
	"path/filepath"
	"strings"

	"github.com/justinstimatze/crystal/internal/record"
)

// Signature maps a Record to a clustering key. GATE (Phase 3) groups the
// substrate by a Signature to find crystallization candidates. The right
// granularity is empirical: too strict (exact args) yields no frequency;
// too loose yields high within-group output variance. The measure sweep
// evaluates several so the choice is made from data.
type Signature func(record.Record) string

// NamedSignature pairs a granularity name with its function, ordered
// strict→loose.
type NamedSignature struct {
	Name string
	Fn   Signature
}

// Signatures is the granularity ladder swept by `crystal measure`.
var Signatures = []NamedSignature{
	{"exact", SignatureExact},
	{"normalized", SignatureNormalized},
	{"tool", SignatureTool},
}

// SignatureExact keys on the full args (the current promote-gate key).
func SignatureExact(r record.Record) string { return InputSignature(r) }

// SignatureTool keys on the tool name only (coarsest).
func SignatureTool(r record.Record) string { return r.Tool }

// SignatureNormalized keys on a per-tool template: the program + first
// argument token for Bash, the file extension for file tools. This is the
// "middle" granularity most likely to be both frequent and deterministic.
func SignatureNormalized(r record.Record) string {
	switch r.Tool {
	case "Bash":
		return "Bash:" + firstTokens(strArg(r, "command"), 2)
	case "Read", "Edit", "Write":
		return r.Tool + ":ext:" + ext(strArg(r, "file_path"))
	case "Grep":
		return "Grep:" + strArg(r, "output_mode")
	}
	return r.Tool
}

func strArg(r record.Record, key string) string {
	if r.Args == nil {
		return ""
	}
	s, _ := r.Args[key].(string)
	return s
}

// firstTokens returns the first n whitespace tokens of the first non-empty
// line of s (so multi-line scripts key on their first command).
func firstTokens(s string, n int) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > n {
			fields = fields[:n]
		}
		return strings.Join(fields, " ")
	}
	return ""
}

func ext(path string) string {
	e := filepath.Ext(path)
	if e == "" {
		return "(none)"
	}
	return e
}
