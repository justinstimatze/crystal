// Package cmd is the crystal CLI. Phase 1 exposes two subcommands:
// `extract` (build redacted Record fixtures from local transcripts) and
// `eval` (replay a synthetic artifact over a corpus and print the report).
package cmd

import (
	"errors"

	"github.com/alecthomas/kong"
)

// CLI is the root command struct.
type CLI struct {
	Extract ExtractCmd `cmd:"" help:"Walk Claude Code transcripts into a redacted, per-tool-balanced Record corpus."`
	Eval    EvalCmd    `cmd:"" help:"Replay a synthetic artifact over a corpus and print per-tool fidelity reports."`
	Measure MeasureCmd `cmd:"" help:"Sweep signature granularities over the full substrate to find crystallizable (frequent AND deterministic) patterns."`
}

// Exit codes: 0 ok, 2 input/usage error, 1 fatal.
const (
	ExitOK    = 0
	ExitInput = 2
	ExitFatal = 1
)

// usageError marks an error as a bad-input (exit 2) condition.
type usageError struct{ err error }

func (u usageError) Error() string { return u.err.Error() }

// ExitCode maps an error returned by a subcommand Run to a process code.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var ue usageError
	if errors.As(err, &ue) {
		return ExitInput
	}
	return ExitFatal
}

var _ = kong.Parse // ensure kong import is used by main
