package flow

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func sample() Record {
	return Record{
		Run: "hook-loop", Oracle: "local", BigProvider: "publicai",
		Pair: "qwen3:8b + apertus-70b",
		Edges: []Edge{
			{Source: "stream", Target: "served-det", Value: 12, Kind: "shift-left"},
			{Source: "stream", Target: "deferred-model", Value: 8, Kind: "defer"},
			{Source: "reauthor", Target: "served-now", Value: 3, Kind: "shift-left"},
		},
	}
}

// TestWriteFileRoundTrips confirms the viz source serializes and parses back —
// the contract any renderer (lucida, the live dashboard) binds to.
func TestWriteFileRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "flow.json")
	if err := sample().WriteFile(path); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var got Record
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Edges) != 3 || got.Pair != "qwen3:8b + apertus-70b" {
		t.Errorf("round-trip lost data: %+v", got)
	}
}

// TestAppendHistoryAccretes pins the append-only time series the historical
// "shift-left over time" graphs read: two runs → two JSONL lines, with the
// summary counts derived from the edges.
func TestAppendHistoryAccretes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	if err := sample().AppendHistory(path, "2026-06-08T00:00:00Z"); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	if err := sample().AppendHistory(path, "2026-06-08T00:01:00Z"); err != nil {
		t.Fatalf("append 2: %v", err)
	}
	f, _ := os.Open(path)
	defer f.Close()
	var lines int
	var last HistoryEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines++
		if err := json.Unmarshal(sc.Bytes(), &last); err != nil {
			t.Fatalf("bad jsonl line: %v", err)
		}
	}
	if lines != 2 {
		t.Fatalf("want 2 history lines, got %d", lines)
	}
	if last.ServedDet != 12 || last.Deferred != 8 || last.ReservedDet != 3 {
		t.Errorf("summary counts wrong: %+v", last)
	}
}
