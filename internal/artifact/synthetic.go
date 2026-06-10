package artifact

import (
	"encoding/json"
	"strings"

	"github.com/justinstimatze/crystal/internal/record"
)

// Identity reproduces the historical Output verbatim. A correct eval must
// promote it — but this is a sanity precondition, not go/no-go evidence
// (it is tautological).
type Identity struct{}

func (Identity) Name() string { return "identity" }
func (Identity) Produce(in record.Record) (record.Output, error) {
	return cloneOutput(in.Result), nil
}

// mutator wraps a per-field corruption with a name and the tool it is
// designed for. It returns the (possibly) mutated output and whether it
// actually changed anything, so tests can assert sensitivity only over
// records the corruptor touched. A target of "" means it applies to any
// tool (the outcome-class corruptors operate on error envelopes).
type mutator struct {
	name   string
	target string
	fn     func(o *record.Output) bool
}

func (m mutator) Name() string   { return m.name }
func (m mutator) Target() string { return m.target }
func (m mutator) Produce(in record.Record) (record.Output, error) {
	o := cloneOutput(in.Result)
	m.fn(&o)
	return o, nil
}

// Mutated reports whether this artifact changes a given record (lets a
// test scope its sensitivity assertion to touched records).
func (m mutator) Mutated(in record.Record) bool {
	o := cloneOutput(in.Result)
	return m.fn(&o)
}

// Corruptors returns the deliberate, subtle, single-field corruptions used
// to prove sensitivity. Each is the smallest realistic regression for its
// tool. A correct eval must REJECT every one.
func Corruptors() []Mutator {
	return []Mutator{
		mutator{"bash-dropline", "Bash", func(o *record.Output) bool {
			if o.IsError || o.Stdout == "" {
				return false
			}
			lines := strings.Split(strings.TrimRight(o.Stdout, "\n"), "\n")
			if len(lines) < 2 {
				return false
			}
			o.Stdout = strings.Join(lines[:len(lines)-1], "\n")
			return true
		}},
		mutator{"bash-flipdigit", "Bash", func(o *record.Output) bool {
			if o.IsError {
				return false
			}
			s, ok := flipFirstDigit(o.Stdout)
			o.Stdout = s
			return ok
		}},
		mutator{"bash-flipexit", "Bash", func(o *record.Output) bool {
			if o.IsError || o.Stdout == "" {
				return false
			}
			o.Interrupted = !o.Interrupted
			return true
		}},
		mutator{"edit-drophunk", "Edit", func(o *record.Output) bool {
			arr := patchHunks(o.StructuredPatch)
			if len(arr) == 0 {
				return false
			}
			b, _ := json.Marshal(arr[:len(arr)-1])
			o.StructuredPatch = b
			return true
		}},
		mutator{"write-mutate", "Write", func(o *record.Output) bool {
			if o.IsError || len(o.NewString) < 2 {
				return false
			}
			o.NewString = o.NewString[:len(o.NewString)-1] // drop a char of written content
			return true
		}},
		mutator{"grep-dropfile", "Grep", func(o *record.Output) bool {
			if len(o.Filenames) == 0 {
				return false
			}
			o.Filenames = o.Filenames[:len(o.Filenames)-1]
			o.NumFiles = len(o.Filenames)
			return true
		}},
		mutator{"read-truncate", "Read", func(o *record.Output) bool {
			if len(o.File) < 2 {
				return false
			}
			o.File = o.File[:len(o.File)-1]
			return true
		}},
		mutator{"scalar-mutate", "", func(o *record.Output) bool {
			if !o.IsError || o.Scalar == "" {
				return false
			}
			o.Scalar = o.Scalar + "."
			return true
		}},
		mutator{"error-to-success", "", func(o *record.Output) bool {
			if !o.IsError {
				return false
			}
			// Flip a historical error into a success shape — the highest
			// value regression. The outcome-class gate must catch this.
			o.IsError = false
			o.Stdout = o.Scalar
			o.Scalar = ""
			return true
		}},
	}
}

// BenignVolatility re-stamps volatile spans (timestamps, tmp paths) and
// reorders Grep output without changing meaning. A correct eval must NOT
// reject it (specificity).
type BenignVolatility struct{}

func (BenignVolatility) Name() string { return "benign-volatility" }
func (BenignVolatility) Produce(in record.Record) (record.Output, error) {
	o := cloneOutput(in.Result)
	if o.Stdout != "" {
		o.Stdout = swapClock(o.Stdout)
	}
	if len(o.Filenames) > 1 {
		// reverse order — set comparison must ignore it
		for i, j := 0, len(o.Filenames)-1; i < j; i, j = i+1, j-1 {
			o.Filenames[i], o.Filenames[j] = o.Filenames[j], o.Filenames[i]
		}
	}
	return o, nil
}

// Mutator is the interface for corruptors (so tests can call Mutated and
// scope to the corruptor's target tool).
type Mutator interface {
	Artifact
	Mutated(in record.Record) bool
	Target() string // tool this corruptor is designed for ("" = any)
}

// ByName resolves a synthetic artifact by name (identity,
// benign-volatility, or any corruptor). Used by the `crystal eval` CLI.
func ByName(name string) (Artifact, bool) {
	switch name {
	case "identity":
		return Identity{}, true
	case "benign-volatility":
		return BenignVolatility{}, true
	}
	for _, c := range Corruptors() {
		if c.Name() == name {
			return c, true
		}
	}
	return nil, false
}

// Names lists every selectable synthetic artifact name.
func Names() []string {
	out := []string{"identity", "benign-volatility"}
	for _, c := range Corruptors() {
		out = append(out, c.Name())
	}
	return out
}

// --- helpers ---

func cloneOutput(o record.Output) record.Output {
	c := o
	c.Raw = append(json.RawMessage(nil), o.Raw...)
	c.StructuredPatch = append(json.RawMessage(nil), o.StructuredPatch...)
	c.Filenames = append([]string(nil), o.Filenames...)
	return c
}

func flipFirstDigit(s string) (string, bool) {
	b := []byte(s)
	for i, c := range b {
		if c >= '0' && c <= '9' {
			if c == '9' {
				b[i] = '0'
			} else {
				b[i] = c + 1
			}
			return string(b), true
		}
	}
	return s, false
}

func patchHunks(raw json.RawMessage) []json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) != nil {
		return nil
	}
	return arr
}

func swapClock(s string) string {
	// Replace a HH:MM:SS the cheap way: bump the first one found.
	for i := 0; i+8 <= len(s); i++ {
		if isClock(s[i : i+8]) {
			return s[:i] + "00:00:00" + s[i+8:]
		}
	}
	return s
}

func isClock(s string) bool {
	if len(s) != 8 || s[2] != ':' || s[5] != ':' {
		return false
	}
	for i, c := range s {
		if i == 2 || i == 5 {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
