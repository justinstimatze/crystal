package compare

import (
	"strings"

	"github.com/justinstimatze/crystal/internal/record"
)

type bashComparator struct{}

func (bashComparator) Tool() string { return "Bash" }

// Compare matches Bash on stdout (after volatility normalization and
// trailing-whitespace trim) and requires the interrupted flag to agree.
func (bashComparator) Compare(produced, historical record.Output) Verdict {
	if v, done := outcomeGate(produced, historical); done {
		return v
	}
	if produced.Interrupted != historical.Interrupted {
		return reject("interrupted flag differs")
	}
	p, _ := normalizeVolatile(strings.TrimRight(produced.Stdout, " \t\r\n"))
	h, _ := normalizeVolatile(strings.TrimRight(historical.Stdout, " \t\r\n"))
	if p != h {
		return reject("stdout differs")
	}
	return match()
}

func init() { register(bashComparator{}) }
