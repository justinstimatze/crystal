// Package flow emits a Sankey-shaped record of where a hybrid-loop run's requests
// actually went — served deterministically, deferred to the model, demoted on
// drift, labeled by the oracle, abstained, re-served after re-author. It is the
// data-driven viz source (option 2): real measured counts from a live run, not an
// illustration. The JSON shape is deliberately stable and watchable so lucida (or
// any Vega/D3 renderer) can bind to it without crystal knowing about the renderer.
package flow

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Node is one box in the flow — a menu cell or a request-fate, tagged with the
// placement REGIME it belongs to (for grouping/coloring in the viz).
type Node struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Regime string `json:"regime"` // owned-local | owned-house | private-cloud | vendor | fate
}

// Edge is a flow of `Value` requests from Source to Target. Kind colors the edge:
// shift-left (the gravitational pull), defer (escalation right), oracle (labeling),
// demote (drift bounce), resume (re-served after re-author).
type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Value  int    `json:"value"`
	Kind   string `json:"kind"`
}

// Record is one run's full flow. Counts are measured; the metadata names the
// composed loop (which menu cells the pair used).
type Record struct {
	Run         string `json:"run"`
	Oracle      string `json:"oracle"`
	BigProvider string `json:"big_provider"`
	Pair        string `json:"pair"`
	Note        string `json:"note"`
	Nodes       []Node `json:"nodes"`
	Edges       []Edge `json:"edges"`
}

// edgeValue returns the value of the first source→target edge, or 0.
func (r Record) edgeValue(source, target string) int {
	for _, e := range r.Edges {
		if e.Source == source && e.Target == target {
			return e.Value
		}
	}
	return 0
}

// HistoryEntry is one run's compact summary — the time series for the future
// "shift-left across the matrix over time" graphs. One JSON line is appended per
// run; the fields are the leftward-shift signal (how much of the work landed on
// the deterministic tier, this run).
type HistoryEntry struct {
	Timestamp   string `json:"ts"`
	Run         string `json:"run"`
	Oracle      string `json:"oracle"`
	BigProvider string `json:"big_provider"`
	Pair        string `json:"pair"`
	ServedDet   int    `json:"served_det"`   // requests served deterministically (0 model)
	Deferred    int    `json:"deferred"`     // requests escalated to the model
	ReservedDet int    `json:"reserved_det"` // once-drifting requests re-served after re-author
	Note        string `json:"note"`
}

// AppendHistory appends a one-line summary of this run to a JSONL history file
// (creating parent dirs). timestamp is passed in so the package stays free of
// wall-clock calls. Append-only: the file accretes the shift-left time series.
func (r Record) AppendHistory(path, timestamp string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	e := HistoryEntry{
		Timestamp:   timestamp,
		Run:         r.Run,
		Oracle:      r.Oracle,
		BigProvider: r.BigProvider,
		Pair:        r.Pair,
		ServedDet:   r.edgeValue("stream", "served-det"),
		Deferred:    r.edgeValue("stream", "deferred-model"),
		ReservedDet: r.edgeValue("reauthor", "served-now"),
		Note:        r.Note,
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

// WriteFile serializes the record to path (creating parent dirs). It is best-effort
// from the caller's view (a viz emit must never fail the run) — the error is
// returned so the caller can log it, not abort on it.
func (r Record) WriteFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
