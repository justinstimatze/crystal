package compare

import (
	"sort"
	"strconv"
	"strings"

	"github.com/justinstimatze/crystal/internal/record"
)

// Fingerprint returns a deterministic equivalence key for an Output that
// mirrors the tool's comparator: two Outputs with the same fingerprint
// would Compare as Match. It is used to measure within-group output
// variance (determinism) when sweeping signature granularities — reusing
// the eval's own notion of "same output" rather than inventing a new one.
func Fingerprint(tool string, o record.Output) string {
	if o.IsError {
		return "err:" + strings.TrimSpace(o.Scalar)
	}
	switch tool {
	case "Bash":
		s, _ := normalizeVolatile(strings.TrimRight(o.Stdout, " \t\r\n"))
		return "bash:" + boolStr(o.Interrupted) + ":" + s
	case "Read":
		return "read:" + o.File
	case "Edit", "Write":
		hs, _ := hunkSet(o.StructuredPatch)
		return tool + ":" + o.NewString + "|" + strings.Join(hs, ",")
	case "Grep":
		fs := append([]string(nil), o.Filenames...)
		sort.Strings(fs)
		return "grep:" + strconv.Itoa(o.NumFiles) + "|" + strings.Join(fs, ",") + "|" + o.File
	}
	return string(o.Raw)
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
