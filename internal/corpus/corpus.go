// Package corpus reads and writes the committed Record fixtures under
// testdata/corpus. Records are stored one JSON array per tool
// (testdata/corpus/<tool>.json) so cohorts load cleanly and diffs stay
// readable.
package corpus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/justinstimatze/crystal/internal/record"
)

// Save writes records grouped by tool into dir, one <tool>.json per tool.
func Save(dir string, recs []record.Record) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	byTool := map[string][]record.Record{}
	for _, r := range recs {
		byTool[r.Tool] = append(byTool[r.Tool], r)
	}
	for tool, rs := range byTool {
		b, err := json.MarshalIndent(rs, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, tool+".json"), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Load reads every <tool>.json under dir and returns the concatenated
// records in deterministic (tool-sorted) order.
func Load(dir string) ([]record.Record, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var out []record.Record
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return nil, err
		}
		var rs []record.Record
		if err := json.Unmarshal(b, &rs); err != nil {
			return nil, err
		}
		out = append(out, rs...)
	}
	return out, nil
}
