// Command crystal is the Hybrid Loop Crystallization Engine CLI. Phase 1
// ships the eval / demotion-gate harness: extract redacted Record fixtures
// from local Claude Code transcripts, then replay synthetic artifacts to
// prove the harness catches subtle regressions (sensitivity) without
// false alarms on benign volatility (specificity).
package main

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/justinstimatze/crystal/cmd"
)

func main() {
	var cli cmd.CLI
	ctx := kong.Parse(&cli,
		kong.Name("crystal"),
		kong.Description("Hybrid Loop Crystallization Engine — Phase 1 eval harness."),
	)
	err := ctx.Run()
	os.Exit(cmd.ExitCode(err))
}
